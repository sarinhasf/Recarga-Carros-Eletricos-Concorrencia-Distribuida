/*
O [VehicleClient] Contém tds funções a serem usadas pelo cliente
*/

package main

import (
	"bufio"
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

/*
Essas funções printam no terminal para o menu
*/
func listCapitaisNordeste() {
	fmt.Println("\nSelecione uma cidade...")
	fmt.Println("\nCIDADES DO LITORAL NORDESTE")
	fmt.Println("[1] Salvador    [2] Aracaju      [3] Maceio)")
	fmt.Println("[4] Recife     [5] Joao Pessoa  [6] Natal")
	fmt.Println("[7] Fortaleza  [8] Teresina      [9] Sao Luis")
}

func listMenu() {
	fmt.Println("\nOpening menu...")
	fmt.Println("\n[ MENU | Escolha uma Opção ]")
	fmt.Println("[1] Programar viagem")
	fmt.Println("[0] Sair do sistema")
}

func printTitle() {
	title := " RESERVA DE PONTOS DO LITORAL NORDESTE "
	borderChar := "*"
	width := len(title) + 6 // Espaço extra para borda e padding
	border := strings.Repeat(borderChar, width)
	fmt.Println(border)
	fmt.Printf("%s%s%s\n", borderChar+borderChar, title, borderChar+borderChar)
	fmt.Println(border)
}

/*
Essa função inicia o veiculo gerando seus dados automaticamente
*/
func startingVehicle() {
	printTitle()

	var veiculo Veiculo
	placa = setPlacaVeiculo()
	veiculo.Placa = placa
	setDadosVeiculo(&veiculo)

	erro := WriteFileVeiculos(veiculo)
	if erro != nil {
		fmt.Printf("Erro ao escrever no arquivo: %v\n", erro)
		return
	}

	defer RemoveVeiculoPorPlaca(veiculo.Placa)
	fmt.Printf("Veiculo com placa[%s] registrado com sucesso!\n", veiculo.Placa)

	canalSinal := make(chan os.Signal, 1)
	signal.Notify(canalSinal, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-canalSinal
		RemoveVeiculoPorPlaca(veiculo.Placa)
		os.Exit(0)
	}()

	leitor := bufio.NewReader(os.Stdin)
	online := true
	for online {
		listMenu()
		op, _ := leitor.ReadString('\n')
		op = strings.TrimSpace(op)

		switch op {
		case "1":
			ChooseRoute(&veiculo)
		default:
			fmt.Println("Saindo...")
			RemoveVeiculoPorPlaca(veiculo.Placa)
			online = false
		}
	}
}

// armazena a variavel da placa do veiculo
var placa string

/*
Funções para gerar a placa de forma automatica
*/
func setPlacaVeiculo() string {
	var placa []string
	placa = append(placa, gerarLetraAleatoria())
	placa = append(placa, gerarLetraAleatoria())
	placa = append(placa, gerarLetraAleatoria())
	placa = append(placa, strconv.Itoa(rand.Intn(10)))
	placa = append(placa, gerarLetraAleatoria())
	placa = append(placa, strconv.Itoa(rand.Intn(10)))
	placa = append(placa, strconv.Itoa(rand.Intn(10)))

	return strings.Join(placa, "")
}

func gerarLetraAleatoria() string {
	var letras []string
	letras = append(letras, "A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q", "R", "S", "T", "U", "V", "W", "X", "Y", "Z")
	indice := rand.Intn(26)
	return letras[indice]
}

func verificaPlaca(veiculosJson []Veiculo, placa string) bool {
	for _, veiculo := range veiculosJson {
		if strings.EqualFold(veiculo.Placa, placa) {
			return true
		}
	}
	return false
}

/*
Pega a distância total do trecho
*/
func decToRad(dec float64) float64 {
	rad := dec * (math.Pi / 180)
	return rad
}

func getDelta(x1 float64, x2 float64) float64 {
	return (x2 - x1)
}

func GetDistancia(latitude1, longitude1, latitude2, longitude2 float64) float64 {
	const raioTerra_km = 6371

	latitude1, latitude2 = decToRad(latitude1), decToRad(latitude2)
	longitude1, longitude2 = decToRad(longitude1), decToRad(longitude2)

	deltaLatitude := getDelta(latitude1, latitude2)
	deltaLongitude := getDelta(longitude1, longitude2)

	a := math.Pow(math.Sin(deltaLatitude/2), 2) + math.Cos(latitude1)*math.Cos(latitude2)*math.Pow(math.Sin(deltaLongitude/2), 2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	distancia := raioTerra_km * c
	return distancia
}

func setDadosVeiculo(veiculo *Veiculo) {
	level := float64(int(10 + rand.Float64()*(91)))
	autonomia := float64(int(500 + rand.Float64()*(201)))
	fmt.Printf("\nGerando dados automaticamente para o veículo...\n")
	fmt.Printf("Placa gerada: %s\n", veiculo.Placa)
	fmt.Printf("Bateria gerada: %.0f%%\n", level)
	fmt.Printf("Autonomia gerada: %.0fkm\n", autonomia)
	veiculo.NivelBateriaAtual = level
	veiculo.Autonomia = autonomia
}

func checkRefills(veiculo *Veiculo, cidadesViagem []string) []Ponto {
	var recargasNecessarias []Ponto
	var capacidadeKmComBateriaRestante, distanciaAoProxPonto float64
	pontosViagem := GetCidadesToPontos(cidadesViagem)
	max := len(pontosViagem)
	for i, ponto := range pontosViagem {
		pontoAtual := ponto
		if i < max-1 {
			proximoPonto := pontosViagem[i+1]
			distanciaAoProxPonto = GetDistancia(pontoAtual.Latitude, pontoAtual.Longitude, proximoPonto.Latitude, proximoPonto.Longitude)
			capacidadeKmComBateriaRestante = (veiculo.Autonomia * veiculo.NivelBateriaAtual) / 100

			if distanciaAoProxPonto > capacidadeKmComBateriaRestante {
				recargasNecessarias = append(recargasNecessarias, pontoAtual)
				veiculo.NivelBateriaAtual = 100
				capacidadeKmComBateriaRestante = (veiculo.Autonomia * veiculo.NivelBateriaAtual) / 100
			}
			percentualConsumido := (distanciaAoProxPonto / veiculo.Autonomia) * 100
			veiculo.NivelBateriaAtual -= percentualConsumido
			capacidadeKmComBateriaRestante = (veiculo.Autonomia * veiculo.NivelBateriaAtual) / 100
		}
	}

	return recargasNecessarias
}

func CancelarReserva() {
	leitor := bufio.NewReader(os.Stdin)

	fmt.Printf("\nInforme a sua placa para cancelar reservas existentes: ")
	placaInput, _ := leitor.ReadString('\n')
	placaInput = strings.TrimSpace(placaInput)

	if placaInput == "" {
		fmt.Println("Placa inválida. Operação cancelada.")
		return
	}

	fmt.Println("\nEnviando solicitação de cancelamento aos servidores...")

	msg := "3," + placaInput
	startingMqtt(msg, placaInput)

	fmt.Println("Solicitação de cancelamento enviada com sucesso!")

	fmt.Println("\nAguardando confirmação dos servidores...")
	go receberRespostasCancelamento(placaInput)

	time.Sleep(5 * time.Second)
}

func receberRespostasCancelamento(placaVeiculo string) {
	opts := mqtt.NewClientOptions().AddBroker("tcp://broker:1883")
	opts.SetClientID(placaVeiculo + "_cancel_listener")

	opts.OnConnect = func(c mqtt.Client) {
		fmt.Printf("\nAguardando confirmação de cancelamento...\n")

		if token := c.Subscribe("mensagens/cliente/"+placaVeiculo, 0, func(client mqtt.Client, msg mqtt.Message) {
			parts := strings.Split(string(msg.Payload()), ",")
			if len(parts) < 1 {
				return
			}

			switch parts[0] {
			case "cancelamento_confirmado":
				fmt.Printf("\nCancelamento realizado com sucesso!\n")
			case "cancelamento_falhou":
				fmt.Printf("\nFalha ao cancelar reservas: %s\n", parts[1])
			}
		}); token.Wait() && token.Error() != nil {
			fmt.Println("Erro ao assinar tópico:", token.Error())
		}
	}

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		fmt.Println("Erro ao conectar:", token.Error())
		return
	}

	// Aguardar respostas por um tempo
	time.Sleep(5 * time.Second)
	client.Disconnect(250)
}

func GetCidade(tipo string) string {
	leitor := bufio.NewReader(os.Stdin)
	on := true
	for on {
		listCapitaisNordeste()
		fmt.Printf("Selecione a cidade de %s: \n", tipo)
		opcao, _ := leitor.ReadString('\n')
		opcao = strings.TrimSpace(opcao)
		switch opcao {
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			return opcao
		case "0":
			on = false
			return "-2"
		default:
			fmt.Println("Opcao invalida. Tente novamente!")
			continue
		}
	}
	return "-1"
}

func GetDistanciaRota(origem, destino int) float64 {
	var pontosViagem []Ponto
	pontos, erro := GetPontosDeRecargaJson()
	if erro != nil {
		fmt.Printf("Erro ao carregar pontos: %v", erro)
		return -1
	}
	var latitudeOrigem, longitudeOrigem, latitudeDestino, longitudeDestino float64

	if origem <= destino {
		pontosViagem = pontos[origem : destino+1]
	} else {
		for i := origem; i >= destino; i-- {
			pontosViagem = append(pontosViagem, pontos[i])
		}
	}

	for i, ponto := range pontosViagem {
		max := len(pontosViagem) - 1
		if i == 0 {
			latitudeOrigem = ponto.Latitude
			longitudeOrigem = ponto.Longitude
		} else if i == max {
			latitudeDestino = ponto.Latitude
			longitudeDestino = ponto.Longitude
		}
	}
	distanciaTotal := GetDistancia(latitudeOrigem, longitudeOrigem, latitudeDestino, longitudeDestino)
	return distanciaTotal
}

func ChooseRoute(veiculo *Veiculo) {
	origem := GetCidade("Origem")
	if origem == "-2" {
		return
	}
	destino := GetCidade("Destino")
	rotaNordeste, erro := GetRotaSalvadorSaoLuis()
	if erro != nil {
		fmt.Printf("Erro ao carregar rota: %v", erro)
		return
	}

	rotaViagem, indexOrigem, indexDestino := GetTrechoRotaCompleta(origem, destino, rotaNordeste)
	distancia := GetDistanciaRota(indexOrigem, indexDestino)

	var pontos []string
	fmt.Printf("\nRota da viagem gerada com sucesso com seu origem e destino!\n")
	fmt.Printf("\n[Sua Rota]: \n")
	for i, cidade := range rotaViagem {
		max := len(rotaViagem)
		if i == 0 {
			fmt.Printf("Origem: %s -> ", cidade)
		} else if i == max-1 {
			fmt.Printf("%s: Destino Final.\n", cidade)
		} else {
			fmt.Print(cidade, " -> ")
			pontos = append(pontos, cidade)
		}
	}
	fmt.Printf("Distancia do trecho: [%.2fkm]\n\n", distancia)
	pontosNecessarios := checkRefills(veiculo, rotaViagem)

	if len(pontosNecessarios) == 0 {
		fmt.Printf("\nPara esse trajeto não é preciso reserva de pontos!\n")
		return
	}

	fmt.Printf("\nPontos necessários para recarregar: \n")
	for i, ponto := range pontosNecessarios {
		fmt.Printf("[%d°] Parada: Ponto %s\n", i+1, ponto.Cidade)
	}

	var listPontosFinal []string
	for _, ponto := range pontosNecessarios {
		listPontosFinal = append(listPontosFinal, ponto.Cidade)
	}

	pontosString := strings.Join(listPontosFinal, ",")

	fmt.Printf("\nRealizando pré-reserva dos pontos...\n")
	msg := "4," + placa + "," + pontosString
	preReservaSucesso := startingMqtt(msg, placa)

	if preReservaSucesso {
		leitor := bufio.NewReader(os.Stdin)
		fmt.Print("\nDeseja confirmar a reserva destes pontos? (S/N): ")
		opcao, _ := leitor.ReadString('\n')
		opcao = strings.TrimSpace(opcao)

		if strings.ToLower(opcao) == "s" || strings.ToLower(opcao) == "sim" {
			msg = "5," + placa + "," + pontosString
			fmt.Println("\nConfirmando reserva dos pontos de recarga...")
			startingMqtt(msg, placa)
			time.Sleep(15 * time.Second)
			liberarPontosMQTT(placa, listPontosFinal)
			fmt.Println("Os pontos reservados foram liberados (simulando que já passaram pelos pontos).")
		} else {
			msg = "6," + placa + "," + pontosString
			fmt.Println("\nCancelando pré-reserva dos pontos...")
			startingMqtt(msg, placa)
		}
	} else {
		fmt.Printf("\nPré-reserva falhou. Não é possível prosseguir com a reserva.\n")
		fmt.Printf("Tente novamente mais tarde ou escolha uma rota alternativa.\n")
	}
}

// Libera os pontos de recarga apos a viagem
func liberarPontosMQTT(placa string, pontos []string) {
	mensagem := "7," + placa + "," + strings.Join(pontos, ",")
	startingMqtt(mensagem, placa)
}
