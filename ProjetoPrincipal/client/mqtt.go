package main

import (
	"fmt"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var respostaRecebida bool
var operacaoSucesso bool

func verifyOption(op string, parts []string) {
	if op == "reserva_confirmada" {
		fmt.Println("\n✅ [Reserva Confirmada] Pontos reservados com sucesso.")
		respostaRecebida = true
		operacaoSucesso = true

	} else if op == "reserva_falhou" {
		fmt.Println("\n❌ [Falha] Não foi possível reservar os pontos.")
		respostaRecebida = true
		operacaoSucesso = false

	} else if op == "ponto_desconectado" {
		if len(parts) >= 2 {
			fmt.Printf("\n⚠️ [Ponto %s desconectado] Reserva não realizada.\n", parts[1])
		} else {
			fmt.Println("\n⚠️ [Ponto desconectado] Reserva não realizada.")
		}
		fmt.Println("🔁 Tente outra rota ou aguarde reconexão.")
		respostaRecebida = true
		operacaoSucesso = false

	} else if op == "falha_reserva" {
		if len(parts) >= 3 {
			fmt.Printf("\n❌ [Erro na reserva] %s\n", parts[2])
		} else {
			fmt.Println("\n❌ [Erro] Falha na reserva dos pontos.")
		}
		respostaRecebida = true
		operacaoSucesso = false

	} else if op == "cancelamento_confirmado" {
		fmt.Println("\n✅ [Cancelado] Reserva cancelada com sucesso.")
		respostaRecebida = true
		operacaoSucesso = true

	} else if op == "cancelamento_falhou" {
		if len(parts) >= 2 {
			fmt.Printf("\n❌ [Erro no cancelamento] %s\n", parts[1])
		} else {
			fmt.Println("\n❌ [Erro] Falha ao cancelar a reserva.")
		}
		respostaRecebida = true
		operacaoSucesso = false

	} else if op == "prereserva_confirmada" {
		fmt.Println("\n⏳ [Pré-reserva] Confirmada. Você tem 15 min.")
		respostaRecebida = true
		operacaoSucesso = true

	} else if op == "prereserva_cancelada" {
		fmt.Println("\n⚠️ [Pré-reserva] Cancelada.")
		respostaRecebida = true
		operacaoSucesso = true

	} else if op == "falha_prereserva" {
		if len(parts) >= 3 {
			fmt.Printf("\n❌ [Erro na pré-reserva] %s\n", parts[2])
		} else {
			fmt.Println("\n❌ [Erro] Falha na pré-reserva.")
		}
		respostaRecebida = true
		operacaoSucesso = false

	} else if op == "pontos_liberados" {
		if len(parts) >= 2 {
			fmt.Printf("\n🔓 [Liberado] %s\n", parts[1])
		} else {
			fmt.Println("\n🔓 [Liberado] Pontos liberados com sucesso.")
		}
		respostaRecebida = true
		operacaoSucesso = true
	}
}

func startingMqtt(mensagem string, idClient string) bool {
	//broker -> é o ip dessa maquina que está rodando codigo
	opts := mqtt.NewClientOptions().AddBroker("tcp://broker:1883").SetClientID(idClient)

	topicResponse := "mensagens/cliente/" + idClient
	opts.OnConnect = func(c mqtt.Client) {
		fmt.Printf("\nCliente %s conectado ao broker.\n", idClient)

		if token := c.Subscribe(topicResponse, 0, func(client mqtt.Client, msg mqtt.Message) {
			mensagemRecebida := string(msg.Payload())
			fmt.Printf("\n[Resposta]: %s\n", mensagemRecebida)
			parts := strings.Split(mensagemRecebida, ",")
			if len(parts) < 1 {
				return
			}

			verifyOption(parts[0], parts)
		}); token.Wait() && token.Error() != nil {
			fmt.Println("Erro ao assinar tópico:", token.Error())
		}
	}

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	time.Sleep(1 * time.Second)

	fmt.Printf("\nEnviando solicitação: %s\n", mensagem)
	token := client.Publish("mensagens/cliente", 0, false, mensagem)
	token.Wait()

	fmt.Println("Aguardando resposta do servidor...")
	timeout := time.After(10 * time.Second)
	ticker := time.Tick(500 * time.Millisecond)

	for !respostaRecebida {
		select {
		case <-timeout:
			fmt.Printf("\nTempo esgotado aguardando resposta do servidor.\n")
			return false
		case <-ticker:
			// Continua aguardando
		}
	}

	client.Disconnect(250)

	return operacaoSucesso
}
