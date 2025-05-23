/*
O [arquivo MQTT] visa iniciar a comunicação via mqtt, conectando com o broker, criando canais
bem como suas funções auxiliares para enviar e receber mensagens
*/

package main

import (
	"fmt"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// COLOQUE AQ IP ONDE ESTA RODANDO O BROKER
var ipBroker string = "172.16.103.13"

var empresa Empresa
var mqttClient mqtt.Client

/*
Inicia codigo Mqtt para o servidor
*/
func startingMqtt(idCliente string) {
	empresa = getEmpresaPorId(idCliente)
	opts := mqtt.NewClientOptions().AddBroker("tcp://" + ipBroker + ":1883").SetClientID(idCliente)

	opts.OnConnect = func(c mqtt.Client) {
		fmt.Println("[MQTT] iniciado da Empresa " + idCliente + " e conectado ao broker")
		if token := c.Subscribe("mensagens/cliente", 0, messageHandler); token.Wait() && token.Error() != nil {
			fmt.Println("Erro ao assinar tópico:", token.Error())
		}
	}

	mqttClient = mqtt.NewClient(opts)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

}

// Publica mensagem MQTT
func publicaMensagem(client mqtt.Client, topico string, mensagem string) {
	token := client.Publish(topico, 0, false, mensagem)
	token.Wait()
	fmt.Printf("\nMensagem enviada para %s: %s\n", topico, mensagem)
}

// Obtém o cliente MQTT global
func getMQTTClient() mqtt.Client {
	return mqttClient
}

/*
Verifica se ponto pertence a empresa (este server)
*/
func pertenceAEstaEmpresa(ponto string) bool {
	for _, p := range empresa.Pontos {
		if p == ponto {
			return true
		}
	}
	return false
}

/*
Sempre que receber uma mensagem ele chama esse messageHandler que trata a mensagem
*/
func messageHandler(client mqtt.Client, msg mqtt.Message) {
	list := strings.Split(string(msg.Payload()), ",")
	fmt.Printf("(SERVIDOR RECEBEU UMA MENSAGEM DE CLIENTE): %s\n", msg.Payload())
	codigo := list[0]
	placaVeiculo := list[1]
	pontos := list[2:]

	switch codigo {
	case "1": // Reserva
		fmt.Printf("\nRecebida solicitação de reserva para o veículo: %s\n", placaVeiculo)
		if pertenceAEstaEmpresa(pontos[0]) {
			fmt.Printf("O ponto %s pertence a esta empresa. Processando reserva...\n", pontos[0])
			processaReservaMQTT(client, pontos, placaVeiculo)
		} else {
			fmt.Printf("Reserva aguardando confirmação via REST para a placa: %s\n", placaVeiculo)
		}
	case "3": // Cancelamento de reserva
		fmt.Printf("Recebida solicitação de cancelamento para a placa: %s\n", placaVeiculo)
		processaCancelamento(client, placaVeiculo)

	case "4": // Pré-reserva
		fmt.Printf("Recebida solicitação de pré-reserva para a placa: [%s], nos pontos: %v\n", placaVeiculo, pontos)
		if len(pontos) > 0 && pertenceAEstaEmpresa(pontos[0]) {
			processaPreReservaMQTT(client, pontos, placaVeiculo)
		} else if len(pontos) > 0 {
			coordenarPreReservaREST(placaVeiculo, pontos)
		}

	case "5": // Confirmar pré-reserva
		fmt.Printf("Recebida confirmação de pré-reserva para a placa: %s nos pontos: %v\n", placaVeiculo, pontos)
		if len(pontos) > 0 && pertenceAEstaEmpresa(pontos[0]) {
			confirmaPreReservaMQTT(client, pontos, placaVeiculo)
		} else if len(pontos) > 0 {
			sucesso := coordenarConfirmarPreReservaREST(placaVeiculo, pontos)
			if sucesso {
				publicaMensagem(client, "mensagens/cliente/"+placaVeiculo,
					"reserva_confirmada,Reserva confirmada com sucesso em todos os servidores")
				fmt.Printf("\nReserva CONFIRMADA para veículo %s em todos os pontos solicitados!\n", placaVeiculo)

			} else {
				publicaMensagem(client, "mensagens/cliente/"+placaVeiculo,
					"reserva_falhou,Não foi possível confirmar a reserva em todos os pontos")
				fmt.Printf("\nConfirmação de reserva FALHOU para veículo %s!\n", placaVeiculo)
			}
		}

	case "6": // Cancelar pré-reserva
		fmt.Printf("Recebida solicitação de cancelamento de pré-reserva para a placa: %s nos pontos: %v\n", placaVeiculo, pontos)
		if len(pontos) > 0 && pertenceAEstaEmpresa(pontos[0]) {
			cancelaPreReservaMQTT(client, pontos, placaVeiculo)
		} else if len(pontos) > 0 {
			coordenarCancelarPreReservaREST(placaVeiculo, pontos)
		}

	case "7":
		fmt.Printf("Recebida solicitação de liberação de pontos para a placa: %s nos pontos: %v\n", placaVeiculo, pontos)
		liberarPontosAposViagem(client, placaVeiculo, pontos)

	}
}

func processaReservaMQTT(client mqtt.Client, pontosParaReservar []string, placaVeiculo string) {
	var pontosReservadosTemp []string
	var indexesReservados []int

	pontosLocais := false
	falhaLocal := false

	for _, ponto := range pontosParaReservar {
		lock := pontoLocks[ponto]
		lock.Lock()

		for _, ponto := range pontosParaReservar {
			for _, pontoDaEmpresa := range empresa.Pontos {
				if ponto == pontoDaEmpresa {
					pontosStatus.RLock()
					estaConectado := pontosStatus.status[ponto]
					pontosStatus.RUnlock()

					if !estaConectado {
						fmt.Printf("(ERRO) Tentativa de reserva no ponto %s rejeitada: ponto desconectado.\n", ponto)
						publicaMensagem(client, "mensagens/cliente/"+placaVeiculo,
							fmt.Sprintf("ponto_desconectado,%s,Ponto %s está desconectado e não pode ser reservado", ponto, ponto))
						falhaLocal = true
						lock.Unlock()
						return
					}

					pontosLocais = true
					pontoRecarga, index := getPontoPorCidade(ponto)

					if pontoRecarga.Reservado == "" || pontoRecarga.Reservado == placaVeiculo {
						pontosReservadosTemp = append(pontosReservadosTemp, ponto)
						indexesReservados = append(indexesReservados, index)
					} else {
						fmt.Printf("[ERRO] O ponto %s da empresa %s já está reservado.\n",
							ponto, empresa.Nome)
						publicaMensagem(client, "mensagens/cliente/"+placaVeiculo,
							fmt.Sprintf("falha_reserva,%s,Ponto %s já reservado", ponto, ponto))
						falhaLocal = true
						lock.Unlock()
						return
					}
				}
			}
		}

		if falhaLocal {
			lock.Unlock()
			return
		}

		if pontosLocais {
			for i, ponto := range pontosReservadosTemp {
				index := indexesReservados[i]
				dadosRegiao.PontosDeRecarga[index].Reservado = placaVeiculo
				fmt.Printf("[INFO] Ponto %s da empresa %s reservado temporariamente para %s.\n",
					ponto, empresa.Nome, placaVeiculo)
			}

			sucesso := coordinatesReservations(placaVeiculo, pontosParaReservar)

			if sucesso {
				salvaDadosPontos()
				publicaMensagem(client, "mensagens/cliente/"+placaVeiculo,
					"reserva_confirmada,Todos os pontos foram reservados com sucesso")
				fmt.Printf("[SUCESSO] Reserva confirmada para o veículo %s em todos os pontos solicitados.\n", placaVeiculo)

			} else {
				// IMPLEMENTAÇÃO COMPLETA DO CASO DE FALHA (ATOMICIDADE)
				for i, ponto := range pontosReservadosTemp {
					index := indexesReservados[i]
					dadosRegiao.PontosDeRecarga[index].Reservado = ""
					fmt.Printf("[INFO] Reserva temporária cancelada no ponto %s.\n", ponto)
				}
				salvaDadosPontos()

				publicaMensagem(client, "mensagens/cliente/"+placaVeiculo,
					"reserva_falhou,Não foi possível reservar todos os pontos solicitados")
				fmt.Printf("[ERRO] Reserva falhou para o veículo %s.\n", placaVeiculo)
			}
		} else {
			sucesso := reservarPontosEmOutrosServidores(placaVeiculo, pontosParaReservar)

			if sucesso {
				publicaMensagem(client, "mensagens/cliente/"+placaVeiculo,
					"reserva_confirmada,Todos os pontos foram reservados com sucesso")
				fmt.Printf("[SUCESSO] Reserva confirmada para o veículo %s em servidores externos.\n", placaVeiculo)
			} else {
				publicaMensagem(client, "mensagens/cliente/"+placaVeiculo,
					"reserva_falhou,Não foi possível reservar todos os pontos solicitados")
				fmt.Printf("[ERRO] Reserva falhou para o veículo %s.\n", placaVeiculo)
			}
		}
		lock.Unlock()
	}
}

func processaCancelamento(client mqtt.Client, placaVeiculo string) {
	reservasMutex.Lock()
	defer reservasMutex.Unlock()

	cancelouAlgum := false

	// Verificar se existem reservas para essa placa
	if pontosMap, existe := reservas[placaVeiculo]; existe {
		for ponto := range pontosMap {
			lock := pontoLocks[ponto]
			lock.Lock()
			// Verificar se o ponto pertence a esta empresa
			pertenceEmpresa := false
			for _, pontoDaEmpresa := range empresa.Pontos {
				if ponto == pontoDaEmpresa {
					pertenceEmpresa = true
					break
				}
			}

			if pertenceEmpresa {
				// Cancelar a reserva local
				pontoObj, index := getPontoPorCidade(ponto)
				if pontoObj.Reservado == placaVeiculo {
					dadosRegiao.PontosDeRecarga[index].Reservado = ""
					delete(pontosMap, ponto)
					salvaDadosPontos()
					cancelouAlgum = true
					fmt.Printf("[INFO] Cancelamento de reserva para %s no ponto %s.\n", placaVeiculo, ponto)
				}
			}
			lock.Unlock()
		}

		if len(pontosMap) == 0 {
			delete(reservas, placaVeiculo)
		}
	}

	if cancelouAlgum {
		publicaMensagem(client, "mensagens/cliente/"+placaVeiculo, "cancelamento_confirmado")
	} else {
		publicaMensagem(client, "mensagens/cliente/"+placaVeiculo,
			"cancelamento_falhou,Nenhuma reserva encontrada neste servidor")
	}

}

// Processar solicitação de pré-reserva
func processaPreReservaMQTT(client mqtt.Client, pontosParaReservar []string, placaVeiculo string) {
	pontosLocais := false
	falhaLocal := false
	var pontosReservadosTemp []string
	var indexesReservados []int

	// First, gather all information about points
	for _, ponto := range pontosParaReservar {
		lock := pontoLocks[ponto]
		lock.Lock()

		// Move all the point processing logic here...
		for _, pontoDaEmpresa := range empresa.Pontos {
			if ponto == pontoDaEmpresa {
				// Point processing logic...
				pontosStatus.RLock()
				estaConectado := pontosStatus.status[ponto]
				pontosStatus.RUnlock()

				if !estaConectado {
					publicaMensagem(client, "mensagens/cliente/"+placaVeiculo,
						fmt.Sprintf("ponto_desconectado,%s,Ponto %s está desconectado", ponto, ponto))
					falhaLocal = true
					lock.Unlock() // Unlock before returning!
					return
				}

				pontosLocais = true
				pontoRecarga, index := getPontoPorCidade(ponto)

				// Verification logic...
				if pontoRecarga.Reservado == "" {
					pontosReservadosTemp = append(pontosReservadosTemp, ponto)
					indexesReservados = append(indexesReservados, index)
				} else if pontoRecarga.Reservado == "PRE_"+placaVeiculo || pontoRecarga.Reservado == placaVeiculo {
					pontosReservadosTemp = append(pontosReservadosTemp, ponto)
					indexesReservados = append(indexesReservados, index)
				} else {
					// Error handling...
					if strings.HasPrefix(pontoRecarga.Reservado, "PRE_") {
						outroVeiculo := pontoRecarga.Reservado[4:] // Remove "PRE_"
						publicaMensagem(client, "mensagens/cliente/"+placaVeiculo,
							fmt.Sprintf("falha_prereserva,%s,Ponto %s já está pré-reservado pelo veículo %s",
								ponto, ponto, outroVeiculo))
					} else {
						publicaMensagem(client, "mensagens/cliente/"+placaVeiculo,
							fmt.Sprintf("falha_prereserva,%s,Ponto %s já está reservado pelo veículo %s",
								ponto, ponto, pontoRecarga.Reservado))
					}
					falhaLocal = true
					lock.Unlock() // Unlock before returning!
					return
				}
			}
		}

		lock.Unlock() // Unlock at each iteration
	}

	// If there was a failure, don't proceed
	if falhaLocal {
		return
	}

	// For points belonging to this company, mark them as pre-reserved
	if pontosLocais {
		// Mark points as pre-reserved
		for i, ponto := range pontosReservadosTemp {
			lock := pontoLocks[ponto]
			lock.Lock()

			index := indexesReservados[i]
			dadosRegiao.PontosDeRecarga[index].Reservado = "PRE_" + placaVeiculo
			fmt.Printf("[INFO] Ponto %s da empresa %s pré-reservado para %s.\n",
				ponto, empresa.Nome, placaVeiculo)

			lock.Unlock()
		}

		// Save the pre-reservations
		salvaDadosPontos()

		// Notify client about successful pre-reservation
		publicaMensagem(client, "mensagens/cliente/"+placaVeiculo,
			"prereserva_confirmada,Os pontos foram pré-reservados com sucesso")

		// Set timeout for pre-reservation (15 minutes)
		liberarPreReservaTimeout(placaVeiculo, pontosReservadosTemp, 15*time.Minute)
	}
}

// Confirm pre-reservation, converting to full reservation
func confirmaPreReservaMQTT(client mqtt.Client, pontosParaReservar []string, placaVeiculo string) {
	pontosLocais := false
	sucesso := true

	for _, ponto := range pontosParaReservar {
		lock := pontoLocks[ponto]
		lock.Lock()

		for _, pontoDaEmpresa := range empresa.Pontos {
			if ponto == pontoDaEmpresa {
				pontosLocais = true
				pontoRecarga, index := getPontoPorCidade(ponto)

				// Verification logic for confirming pre-reservation
				if pontoRecarga.Reservado == "PRE_"+placaVeiculo {
					dadosRegiao.PontosDeRecarga[index].Reservado = placaVeiculo
					fmt.Printf("[SUCESSO] Ponto %s pré-reserva convertida para reserva completa para %s.\n",
						ponto, placaVeiculo)

				} else if pontoRecarga.Reservado == placaVeiculo {
					fmt.Printf("[INFO] Ponto %s já está reservado para %s.\n",
						ponto, placaVeiculo)
				} else {
					// Detailed error handling...
					if pontoRecarga.Reservado == "" {
						fmt.Printf("[ERRO] Ponto %s não está pré-reservado (está vazio).\n", ponto)
					} else if strings.HasPrefix(pontoRecarga.Reservado, "PRE_") {
						outroVeiculo := pontoRecarga.Reservado[4:]
						fmt.Printf("[ERRO] Ponto %s está pré-reservado para outro veículo: %s.\n",
							ponto, outroVeiculo)
					} else {
						fmt.Printf("[ERRO] Ponto %s está reservado para outro veículo: %s.\n",
							ponto, pontoRecarga.Reservado)
					}
					sucesso = false
				}
			}
		}

		lock.Unlock()
	}

	if pontosLocais {
		if sucesso {
			// Save the data
			salvaDadosPontos()

			// Register full reservation
			reservasMutex.Lock()
			if _, existe := reservas[placaVeiculo]; !existe {
				reservas[placaVeiculo] = make(map[string]string)
			}
			for _, ponto := range pontosParaReservar {
				reservas[placaVeiculo][ponto] = "confirmado"
			}
			reservasMutex.Unlock()

			publicaMensagem(client, "mensagens/cliente/"+placaVeiculo,
				"reserva_confirmada,Reserva confirmada com sucesso")

			// Set timeout for full reservation
			//liberarTimeout(placaVeiculo, pontosParaReservar, 3*time.Hour)
		} else {
			publicaMensagem(client, "mensagens/cliente/"+placaVeiculo,
				"reserva_falhou,Falha ao confirmar pré-reserva")
		}
	} else {
		publicaMensagem(client, "mensagens/cliente/"+placaVeiculo,
			"reserva_confirmada,Confirmação processada")
	}
}

// Cancel pre-reservation
func cancelaPreReservaMQTT(client mqtt.Client, pontosParaReservar []string, placaVeiculo string) {
	pontosLocais := false
	cancelouAlgum := false

	for _, ponto := range pontosParaReservar {
		lock := pontoLocks[ponto]
		lock.Lock()

		for _, pontoDaEmpresa := range empresa.Pontos {
			if ponto == pontoDaEmpresa {
				pontosLocais = true
				pontoRecarga, index := getPontoPorCidade(ponto)

				// Check if the point is pre-reserved for this vehicle
				if pontoRecarga.Reservado == "PRE_"+placaVeiculo {
					// Cancel pre-reservation
					dadosRegiao.PontosDeRecarga[index].Reservado = ""
					cancelouAlgum = true
					fmt.Printf("[INFO] Ponto %s pré-reserva cancelada para %s.\n", ponto, placaVeiculo)
				}
			}
		}

		lock.Unlock()
	}

	if pontosLocais && cancelouAlgum {
		salvaDadosPontos()
		publicaMensagem(client, "mensagens/cliente/"+placaVeiculo,
			"prereserva_cancelada,Pré-reserva cancelada com sucesso")
	} else if pontosLocais {
		publicaMensagem(client, "mensagens/cliente/"+placaVeiculo,
			"prereserva_cancelada,Nenhuma pré-reserva encontrada para cancelar")
	}
}

func liberarPreReservaTimeout(placa string, pontos []string, tempo time.Duration) {
	go func() {
		time.Sleep(tempo)
		fmt.Printf("Verificando timeout para pré-reservas do veículo %s...\n", placa)

		for _, ponto := range pontos {
			lock := pontoLocks[ponto]
			lock.Lock()

			pontoRecarga, index := getPontoPorCidade(ponto)
			if pontoRecarga.Reservado == "PRE_"+placa {
				dadosRegiao.PontosDeRecarga[index].Reservado = ""
				fmt.Printf("[INFO] Pré-reserva para %s no ponto %s expirou e foi liberada automaticamente.\n", placa, ponto)
			}

			lock.Unlock()
		}
		salvaDadosPontos()
	}()
}

// Libera os pontos de recarga após o término da viagem
func liberarPontosAposViagem(client mqtt.Client, placaVeiculo string, pontos []string) {
	reservasMutex.Lock()
	defer reservasMutex.Unlock()
	liberouAlgum := false

	for _, ponto := range pontos {
		lock := pontoLocks[ponto]
		lock.Lock()
		pontoObj, index := getPontoPorCidade(ponto)
		if pontoObj.Reservado == placaVeiculo {
			dadosRegiao.PontosDeRecarga[index].Reservado = ""
			liberouAlgum = true
			fmt.Printf("[INFO] Ponto %s liberado para a placa %s.\n", ponto, placaVeiculo)
		}
		lock.Unlock()
	}

	if liberouAlgum {
		salvaDadosPontos()
		delete(reservas, placaVeiculo)
		publicaMensagem(client, "mensagens/cliente/"+placaVeiculo, "pontos_liberados,Pontos liberados com sucesso")
	} else {
		publicaMensagem(client, "mensagens/cliente/"+placaVeiculo, "pontos_liberados,Nenhum ponto estava reservado para esta placa")
	}
}
