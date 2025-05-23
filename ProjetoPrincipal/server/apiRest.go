/*
A [API REST] dos Servidores é responsável por inicializar nossa api rest e criar suas rotas.
*/
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

var reservasMutex sync.Mutex

var servidores []string = []string{
	"http://172.16.103.13:8081",
	"http://172.16.103.13:8082",
	"http://172.16.103.13:8083",
}

// COLOQUE AQUI O IP DA MAQUINA Q ESTA RODANDO
var ipMaquina string = "172.16.103.13"

// IP UNIVERSAL
var ipUniversal string = "0.0.0.0:"

/*
Definindo estruturas que serão usadas
*/
type ReservationStruct struct {
	PlacaVeiculo string
	Pontos       []string
	EmpresaID    string
}

type ReservaResponse struct {
	Status    string
	Ponto     string
	Mensagem  string
	EmpresaID string
}

var reservas = make(map[string]map[string]string)

var pontosStatus = struct {
	sync.RWMutex
	status map[string]bool
}{status: make(map[string]bool)}

var pontoLocks = make(map[string]*sync.Mutex)

/*
Função que inicia a API REST pro servidor
*/
func startingRest(porta string) {
	getRoutes()
	fmt.Printf("[API REST] iniciada para porta %s\n", porta)

	endereco := ipUniversal + porta //com o ip universal ele escuta de tds portas q estão na rede
	go func() {
		if err := http.ListenAndServe(endereco, nil); err != nil {
			fmt.Printf("Erro ao iniciar servidor REST: %v\n", err)
		}
	}()
}

/*
Função responsável por criar as rotas
*/
func getRoutes() {
	http.HandleFunc("/api/reserva", handleReserva)
	http.HandleFunc("/api/status", handleStatus)
	http.HandleFunc("/api/cancelamento", handleCancelamento)
	http.HandleFunc("/api/admin/ponto/status", handlePontoStatus)
	http.HandleFunc("/api/confirmar-prereserva", handleConfirmarPreReserva)
}

/*
Funções usadas quando se tenta acessar uma rota:
*/
func handleReserva(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}
	var req ReservationStruct
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Erro ao decodificar JSON", http.StatusBadRequest)
		return
	}
	pontoLocalizado := false

	for _, pontoPedido := range req.Pontos {
		lock := pontoLocks[pontoPedido]
		lock.Lock()
		for _, pontoDaEmpresa := range empresa.Pontos {
			if pontoPedido == pontoDaEmpresa {
				pontoLocalizado = true
				// Verificar status de conexão
				pontosStatus.RLock()
				estaConectado := pontosStatus.status[pontoPedido]
				pontosStatus.RUnlock()

				if !estaConectado {
					// Ponto desconectado, não permitir reserva
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(ReservaResponse{
						Status:    "falha",
						Ponto:     pontoPedido,
						Mensagem:  fmt.Sprintf("Ponto %s está desconectado", pontoPedido),
						EmpresaID: empresa.Id,
					})
					fmt.Printf("(ATENÇÃO!) Tentativa de reserva no ponto %s rejeitada pois o ponto foi desconectado.\n", pontoPedido)
					return
				}

				// Ponto encontrado, verificar disponibilidade
				pontoRecarga, index := getPontoPorCidade(pontoPedido)
				reservasMutex.Lock()
				resposta := ReservaResponse{
					Ponto:     pontoPedido,
					EmpresaID: empresa.Id,
				}
				if pontoRecarga.Reservado == "" || pontoRecarga.Reservado == req.PlacaVeiculo {
					// O ponto está livre ou já reservado temporariamente pelo mesmo veículo
					dadosRegiao.PontosDeRecarga[index].Reservado = req.PlacaVeiculo
					salvaDadosPontos()

					// Registrar a reserva no mapa de controle
					if _, existe := reservas[req.PlacaVeiculo]; !existe {
						reservas[req.PlacaVeiculo] = make(map[string]string)
					}
					reservas[req.PlacaVeiculo][pontoPedido] = "confirmado"

					resposta.Status = "confirmado"
					resposta.Mensagem = fmt.Sprintf("Ponto %s reservado com sucesso", pontoPedido)
					fmt.Printf("(SUCESSO!) Ponto %s da empresa %s FOI reservado para %s.\n", pontoPedido, empresa.Nome, req.PlacaVeiculo)

				} else {
					resposta.Status = "falha"
					resposta.Mensagem = fmt.Sprintf("Ponto %s já está reservado", pontoPedido)
					fmt.Printf("(ERRO) O ponto %s da empresa %s já está reservado.\n", pontoPedido, empresa.Nome)
					reservasMutex.Unlock()
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(resposta)
					lock.Unlock()
					return
				}
				reservasMutex.Unlock()
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resposta)
			}
		}
		lock.Unlock()
	}

	if !pontoLocalizado {
		fmt.Printf("(ATENÇÃO!) Nenhum dos pontos solicitados pertence a este servidor. Nenhuma ação realizada para a placa %s.\n", req.PlacaVeiculo)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ReservaResponse{
			Status:    "ignorado",
			Mensagem:  "Nenhum dos pontos solicitados pertence a esta empresa. Nenhuma ação necessária.",
			EmpresaID: empresa.Id,
		})
	}
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":     "online",
		"empresa_id": empresa.Id,
	})
}

func handleCancelamento(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}
	var req ReservationStruct
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Erro ao decodificar JSON", http.StatusBadRequest)
		return
	}
	reservasMutex.Lock()
	defer reservasMutex.Unlock()
	resposta := ReservaResponse{
		EmpresaID: empresa.Id,
	}

	if pontosMap, existe := reservas[req.PlacaVeiculo]; existe {
		for _, pontoPedido := range req.Pontos {
			lock := pontoLocks[pontoPedido]
			lock.Lock()
			if _, reservado := pontosMap[pontoPedido]; reservado {
				// Cancelar a reserva
				pontoRecarga, index := getPontoPorCidade(pontoPedido)
				if pontoRecarga.Reservado == req.PlacaVeiculo {
					dadosRegiao.PontosDeRecarga[index].Reservado = ""
					salvaDadosPontos()
					delete(pontosMap, pontoPedido)
					fmt.Printf("(ATENÇÃO!) Reserva do ponto %s da empresa %s cancelada para %s.\n", pontoPedido, empresa.Nome, req.PlacaVeiculo)
					resposta.Status = "cancelado"
					resposta.Ponto = pontoPedido
					resposta.Mensagem = "Reserva cancelada com sucesso"
				}
			}
			lock.Unlock()
		}
	}
	if resposta.Status == "" {
		resposta.Status = "nao_encontrado"
		resposta.Mensagem = "Nenhuma reserva encontrada para cancelar"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resposta)
}

func fazerRequisicaoREST(metodo, url string, corpo interface{}, resposta interface{}) error {
	jsonCorpo, err := json.Marshal(corpo)
	if err != nil {
		return fmt.Errorf("erro ao codificar JSON: %v", err)
	}

	req, err := http.NewRequest(metodo, url, bytes.NewBuffer(jsonCorpo))
	if err != nil {
		return fmt.Errorf("erro ao criar requisição: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("erro ao fazer requisição: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status de resposta inválido: %d", resp.StatusCode)
	}

	if resposta != nil {
		if err := json.NewDecoder(resp.Body).Decode(resposta); err != nil {
			return fmt.Errorf("erro ao decodificar resposta: %v", err)
		}
	}

	return nil
}

/*
[RESPONSÁVEL PELA REQUISIÇÃO ATOMICA]
Coordena a reserva de pontos entre servidores, garantindo que todos sejam reservados ou nenhum.
*/
func coordinatesReservations(placaVeiculo string, pontos []string) bool {
	var pontosOutrosServidores []string
	for _, ponto := range pontos {
		pertenceEmpresaAtual := false
		for _, pontoDaEmpresa := range empresa.Pontos {
			if ponto == pontoDaEmpresa {
				pertenceEmpresaAtual = true
				break
			}
		}
		if !pertenceEmpresaAtual {
			pontosOutrosServidores = append(pontosOutrosServidores, ponto)
		}
	}

	if len(pontosOutrosServidores) == 0 {
		return true
	}

	meuIP := ipMaquina //Ip do servidor 1
	var porta string
	switch empresa.Id {
	case "EMP1":
		porta = "8081"
	case "EMP2":
		porta = "8082"
	case "EMP3":
		porta = "8083"
	default:
		porta = "8080"
	}
	meuEndereco := fmt.Sprintf("http://%s:%s", meuIP, porta)

	var outrosServidores []string
	for _, s := range servidores {
		if s != meuEndereco {
			outrosServidores = append(outrosServidores, s)
			fmt.Printf("Adicionando o servidor %s para coordenação...\n", s)
		}
	}

	req := ReservationStruct{
		PlacaVeiculo: placaVeiculo,
		Pontos:       pontosOutrosServidores,
		EmpresaID:    empresa.Id,
	}

	todasConfirmadas := true
	var respostasServidores []ReservaResponse

	//envia requisições para outros servidores para verificar se tds os pontos foram reservados ou não
	fmt.Printf("Tentando reservar pontos em %d outros servidores.\n", len(outrosServidores))
	for _, servidor := range outrosServidores {
		var resposta ReservaResponse
		url := servidor + "/api/reserva"
		fmt.Printf("[Enviando requisição para %s.\n", url)

		err := fazerRequisicaoREST("POST", url, req, &resposta)
		if err != nil {
			fmt.Printf("Falha ao comunicar com o servidor %s: %v.\n", servidor, err)
			todasConfirmadas = false
			break
		}

		fmt.Printf("Resposta do servidor %s: %s.\n", servidor, resposta.Status)
		if resposta.Status == "falha" {
			fmt.Printf("Reserva não realizada em %s: %s.\n", servidor, resposta.Mensagem)
			todasConfirmadas = false
			respostasServidores = append(respostasServidores, resposta)
			break
		} else if resposta.Status == "confirmado" {
			fmt.Printf("Reserva [confirmada] em %s para o ponto %s.\n", servidor, resposta.Ponto)
			respostasServidores = append(respostasServidores, resposta)
		} else if resposta.Status == "ignorado" {
			fmt.Printf("Servidor %s ignorou a solicitação: %s.\n", servidor, resposta.Mensagem)
		}
	}

	// Se alguma reserva falhou, cancela as confirmadas
	if !todasConfirmadas {
		fmt.Printf("Cancelando reservas já confirmadas devido a falha em algum servidor.\n")
		for _, resposta := range respostasServidores {
			if resposta.Status == "confirmado" {
				fmt.Printf("Cancelando reserva no servidor %s.\n", resposta.EmpresaID)
				cancelarReservaREST(resposta.EmpresaID, placaVeiculo, pontos)
			}
		}
	}

	return todasConfirmadas
}

func reservarPontosEmOutrosServidores(placaVeiculo string, pontos []string) bool {
	meuIP := ipMaquina //Ip do servidor 1
	var porta string
	switch empresa.Id {
	case "EMP1":
		porta = "8081"
	case "EMP2":
		porta = "8082"
	case "EMP3":
		porta = "8083"
	default:
		porta = "8080"
	}
	meuEndereco := fmt.Sprintf("http://%s:%s", meuIP, porta)

	// Prepara requisição
	req := ReservationStruct{
		PlacaVeiculo: placaVeiculo,
		Pontos:       pontos,
		EmpresaID:    empresa.Id,
	}

	// Envia requisições para todos os servidores
	sucessoEmAlgum := false

	for _, servidor := range servidores {
		if servidor == meuEndereco {
			continue // Pular o próprio servidor
		}

		var resposta ReservaResponse
		url := servidor + "/api/reserva"

		err := fazerRequisicaoREST("POST", url, req, &resposta)
		if err != nil {
			fmt.Printf("Erro ao comunicar com servidor %s: %v\n", servidor, err)
			continue
		}

		if resposta.Status == "confirmado" {
			sucessoEmAlgum = true
			fmt.Printf("(SUCESSO) Ponto %s reservado no servidor %s.\n", resposta.Ponto, resposta.EmpresaID)
		}
	}

	return sucessoEmAlgum
}

// Função para cancelar reserva em outro servidor
func cancelarReservaREST(empresaID string, placaVeiculo string, pontos []string) {
	meuIP := ipMaquina //Ip do servidor 1
	var porta string
	switch empresa.Id {
	case "EMP1":
		porta = "8081"
	case "EMP2":
		porta = "8082"
	case "EMP3":
		porta = "8083"
	default:
		porta = "8080"
	}
	url := fmt.Sprintf("http://%s:%s", meuIP, porta)

	req := ReservationStruct{
		PlacaVeiculo: placaVeiculo,
		Pontos:       pontos,
		EmpresaID:    empresa.Id,
	}

	var resposta ReservaResponse
	err := fazerRequisicaoREST("POST", url, req, &resposta)
	if err != nil {
		fmt.Printf("(ERRO) Falha ao cancelar reserva no servidor %s: %v.\n", empresaID, err)
	} else {
		fmt.Printf("(INFO) Reserva cancelada no servidor %s: %s.\n", empresaID, resposta.Status)
	}
}

/*
Fica sempre verificando o status do ponto para ver se ele se desconectou ou não
*/
func startMonitoringPoints() {
	for _, ponto := range dadosRegiao.PontosDeRecarga {
		pontoLocks[ponto.Cidade] = &sync.Mutex{}
	}

	pontosStatus.Lock()
	for _, ponto := range dadosRegiao.PontosDeRecarga {
		pertenceEmpresa := false
		for _, pontoDaEmpresa := range empresa.Pontos {
			if ponto.Cidade == pontoDaEmpresa {
				pertenceEmpresa = true
				break
			}
		}
		if pertenceEmpresa {
			pontosStatus.status[ponto.Cidade] = true
		} else {
			pontosStatus.status[ponto.Cidade] = false
		}
	}
	pontosStatus.Unlock()

	verificarStatusPontos()

	go func() {
		for {
			time.Sleep(30 * time.Second)
			verificarStatusPontos()
		}
	}()
}

func verificarStatusPontos() {
	for _, ponto := range empresa.Pontos {
		estaConectado := verificarPontoConectado(ponto)
		pontosStatus.Lock()
		statusAnterior := pontosStatus.status[ponto]
		pontosStatus.status[ponto] = estaConectado
		pontosStatus.Unlock()

		if statusAnterior != estaConectado {
			if estaConectado {
				fmt.Printf("(INFO) Ponto %s está conectado.\n", ponto)
			} else {
				fmt.Printf("(AVISO) Ponto %s está desconectado.\n", ponto)
				cancelarReservasPontosDesconectados(ponto)
			}
		}
	}
}

// Coordenar pré-reserva com outros servidores via REST
func coordenarPreReservaREST(placaVeiculo string, pontos []string) bool {
	// Filtrar pontos que não pertencem a esta empresa
	var pontosOutrosServidores []string
	for _, ponto := range pontos {
		pertenceEmpresaAtual := false
		for _, pontoDaEmpresa := range empresa.Pontos {
			if ponto == pontoDaEmpresa {
				pertenceEmpresaAtual = true
				break
			}
		}
		if !pertenceEmpresaAtual {
			pontosOutrosServidores = append(pontosOutrosServidores, ponto)
		}
	}

	// Se não há pontos para outros servidores, retorna sucesso
	if len(pontosOutrosServidores) == 0 {
		return true
	}

	meuIP := ipMaquina //Ip do servidor 1
	var porta string
	switch empresa.Id {
	case "EMP1":
		porta = "8081"
	case "EMP2":
		porta = "8082"
	case "EMP3":
		porta = "8083"
	default:
		porta = "8080"
	}
	meuEndereco := fmt.Sprintf("http://%s:%s", meuIP, porta)

	// Remove o próprio servidor da lista
	var outrosServidores []string
	for _, s := range servidores {
		// CORREÇÃO: Usar o nome do contêiner em vez de localhost
		if s != meuEndereco {
			outrosServidores = append(outrosServidores, s)
			fmt.Printf("Adicionando servidor %s para coordenação de pré-reserva.\n", s)
		}
	}

	// Prepara requisição para outros servidores
	req := ReservationStruct{
		PlacaVeiculo: "PRE_" + placaVeiculo, // Add prefix to indicar pre-reserva
		Pontos:       pontos,
		EmpresaID:    empresa.Id,
	}

	// Envia requisições para outros servidores
	todasConfirmadas := true
	var respostasServidores []ReservaResponse

	fmt.Printf("Tentando pré-reservar pontos em %d outros servidores.\n", len(outrosServidores))
	for _, servidor := range outrosServidores {
		var resposta ReservaResponse
		url := servidor + "/api/reserva"
		fmt.Printf("Enviando requisição para %s.\n", url)

		err := fazerRequisicaoREST("POST", url, req, &resposta)
		if err != nil {
			fmt.Printf("Falha ao comunicar com o servidor %s: %v.\n", servidor, err)
			todasConfirmadas = false
			break
		}

		fmt.Printf("Resposta do servidor %s: %s.\n", servidor, resposta.Status)
		if resposta.Status == "falha" {
			fmt.Printf("Pré-reserva não realizada em %s: %s.\n", servidor, resposta.Mensagem)
			todasConfirmadas = false
			respostasServidores = append(respostasServidores, resposta)
			break
		} else if resposta.Status == "confirmado" {
			fmt.Printf("(SUCESSO!) Pré-reserva confirmada em %s para o ponto %s.\n", servidor, resposta.Ponto)
			respostasServidores = append(respostasServidores, resposta)
		} else if resposta.Status == "ignorado" {
			fmt.Printf("(AVISO) Servidor %s ignorou a solicitação de pré-reserva: %s.\n", servidor, resposta.Mensagem)
			// Continua tentando com outros servidores
		}
	}

	// Se alguma pré-reserva falhou, cancela as confirmadas
	if !todasConfirmadas {
		fmt.Printf("(AVISO) Cancelando pré-reservas já confirmadas devido a falha.\n")
		for _, resposta := range respostasServidores {
			if resposta.Status == "confirmado" {
				fmt.Printf("Cancelando pré-reserva no servidor %s.\n", resposta.EmpresaID)
				cancelarReservaREST(resposta.EmpresaID, placaVeiculo, pontos)
			}
		}
	}

	return todasConfirmadas
}

// Coordenar confirmação de pré-reserva
func coordenarConfirmarPreReservaREST(placaVeiculo string, pontos []string) bool {
	// Filtrar pontos que não pertencem a esta empresa
	var pontosOutrosServidores []string
	for _, ponto := range pontos {
		pertenceEmpresaAtual := false
		for _, pontoDaEmpresa := range empresa.Pontos {
			if ponto == pontoDaEmpresa {
				pertenceEmpresaAtual = true
				break
			}
		}
		if !pertenceEmpresaAtual {
			pontosOutrosServidores = append(pontosOutrosServidores, ponto)
		}
	}

	// Se não há pontos para outros servidores, retorna sucesso
	if len(pontosOutrosServidores) == 0 {
		return true
	}

	meuIP := ipMaquina //Ip do servidor 1
	var porta string
	switch empresa.Id {
	case "EMP1":
		porta = "8081"
	case "EMP2":
		porta = "8082"
	case "EMP3":
		porta = "8083"
	default:
		porta = "8080"
	}
	meuEndereco := fmt.Sprintf("http://%s:%s", meuIP, porta)

	// Remove o próprio servidor da lista
	var outrosServidores []string
	for _, s := range servidores {
		// CORREÇÃO: Usar o nome do contêiner em vez de localhost
		if s != meuEndereco {
			outrosServidores = append(outrosServidores, s)
			fmt.Printf("Adicionando servidor %s para confirmação de pré-reserva.\n", s)
		}
	}

	// Prepara requisição para outros servidores com flag de confirmação
	req := ReservationStruct{
		PlacaVeiculo: "CONFIRM_" + placaVeiculo, // Adicionar prefixo para indicar confirmação
		Pontos:       pontosOutrosServidores,
		EmpresaID:    empresa.Id,
	}

	// Envia requisições para outros servidores
	todasConfirmadas := true
	var respostasFalhas []string

	fmt.Printf("Tentando confirmar pré-reservas em %d outros servidores.\n", len(outrosServidores))
	for _, servidor := range outrosServidores {
		var resposta ReservaResponse
		url := servidor + "/api/confirmar-prereserva" // Novo endpoint específico
		fmt.Printf("Enviando requisição de confirmação para %s.\n", url)

		err := fazerRequisicaoREST("POST", url, req, &resposta)
		if err != nil {
			fmt.Printf("Falha ao comunicar com o servidor %s: %v.\n", servidor, err)
			todasConfirmadas = false
			respostasFalhas = append(respostasFalhas, fmt.Sprintf("Erro de comunicação: %v", err))
			continue
		}

		if resposta.Status != "confirmado" {
			fmt.Printf("Falha na confirmação em %s: %s.\n", servidor, resposta.Mensagem)
			todasConfirmadas = false
			respostasFalhas = append(respostasFalhas, resposta.Mensagem)
		} else {
			fmt.Printf("Pré-reserva confirmada em %s para ponto %s.\n", servidor, resposta.Ponto)
		}
	}

	return todasConfirmadas
}

// Coordenar cancelamento de pré-reserva
func coordenarCancelarPreReservaREST(placaVeiculo string, pontos []string) bool {
	// Lógica para cancelar pré-reservas em todos os servidores
	req := ReservationStruct{
		PlacaVeiculo: placaVeiculo,
		Pontos:       pontos,
		EmpresaID:    empresa.Id,
	}

	// Enviar requisições para todos os servidores para cancelar a pré-reserva
	sucessoEmTodos := true

	meuIP := ipMaquina //Ip do servidor 1
	var porta string
	switch empresa.Id {
	case "EMP1":
		porta = "8081"
	case "EMP2":
		porta = "8082"
	case "EMP3":
		porta = "8083"
	default:
		porta = "8080"
	}

	meuEndereco := fmt.Sprintf("http://%s:%s", meuIP, porta)

	for _, servidor := range servidores {
		if servidor == meuEndereco {
			continue // Pular o próprio servidor
		}

		var resposta ReservaResponse
		url := servidor + "/api/cancelamento"

		err := fazerRequisicaoREST("POST", url, req, &resposta)
		if err != nil {
			fmt.Printf("Erro ao comunicar com servidor %s: %v\n", servidor, err)
			sucessoEmTodos = false
			continue
		}

		if resposta.Status != "cancelado" {
			fmt.Printf("[ERRO] Falha ao cancelar pré-reserva em %s: %s.\n", servidor, resposta.Mensagem)
			sucessoEmTodos = false
		}
	}

	return sucessoEmTodos
}

func verificarPontoConectado(ponto string) bool {
	pertenceEmpresa := false
	for _, pontoDaEmpresa := range empresa.Pontos {
		if ponto == pontoDaEmpresa {
			pertenceEmpresa = true
			break
		}
	}
	if pertenceEmpresa {
		return true
	}
	pontoObj, _ := getPontoPorCidade(ponto)
	return pontoObj.ID%2 == 0
}

func cancelarReservasPontosDesconectados(ponto string) {
	reservasMutex.Lock()
	defer reservasMutex.Unlock()

	for placa, pontosMap := range reservas {
		if _, reservado := pontosMap[ponto]; reservado {
			pontoObj, idx := getPontoPorCidade(ponto)
			if pontoObj.Reservado == placa {
				dadosRegiao.PontosDeRecarga[idx].Reservado = ""
				delete(pontosMap, ponto)
				salvaDadosPontos()
				fmt.Printf("(AVISO) Reserva para %s no ponto %s cancelada devido à desconexão.\n", placa, ponto)
				client := getMQTTClient()
				publicaMensagem(client, "mensagens/cliente/"+placa, fmt.Sprintf("ponto_desconectado,%s,Reserva cancelada devido a desconexão do ponto", ponto))
			}
		}
	}
}

func handlePontoStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Ponto  string `json:"ponto"`
		Status bool   `json:"status"` // true=conectado, false=desconectado
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Erro ao decodificar JSON", http.StatusBadRequest)
		return
	}

	// Atualizar status do ponto manualmente
	pontosStatus.Lock()
	pontosStatus.status[req.Ponto] = req.Status
	pontosStatus.Unlock()

	// Se desconectou, cancelar reservas existentes
	if !req.Status {
		cancelarReservasPontosDesconectados(req.Ponto)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ponto":      req.Ponto,
		"status":     req.Status,
		"empresa_id": empresa.Id,
	})

	fmt.Printf("Status do ponto %s alterado manualmente para: %v.\n", req.Ponto, req.Status)
}

// Handler para confirmação de pré-reservas
func handleConfirmarPreReserva(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	var req ReservationStruct
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Erro ao decodificar JSON", http.StatusBadRequest)
		return
	}

	// Remover prefixo CONFIRM_
	placaVeiculo := ""
	if strings.HasPrefix(req.PlacaVeiculo, "CONFIRM_") {
		placaVeiculo = req.PlacaVeiculo[8:] // Remove "CONFIRM_"
	} else {
		placaVeiculo = req.PlacaVeiculo
	}

	pontoLocalizado := false
	resposta := ReservaResponse{
		EmpresaID: empresa.Id,
	}

	for _, pontoPedido := range req.Pontos {
		lock := pontoLocks[pontoPedido]
		lock.Lock()
		for _, pontoDaEmpresa := range empresa.Pontos {
			if pontoPedido == pontoDaEmpresa {
				pontoLocalizado = true
				pontoRecarga, index := getPontoPorCidade(pontoPedido)
				if pontoRecarga.Reservado == "PRE_"+placaVeiculo || pontoRecarga.Reservado == placaVeiculo {
					dadosRegiao.PontosDeRecarga[index].Reservado = placaVeiculo
					salvaDadosPontos()
					resposta.Status = "confirmado"
					resposta.Ponto = pontoPedido
					resposta.Mensagem = fmt.Sprintf("Ponto %s reserva confirmada", pontoPedido)
					fmt.Printf("(SUCESSO!) Pré-reserva do ponto %s convertida para reserva completa para %s.\n", pontoPedido, placaVeiculo)
				} else {
					resposta.Status = "falha"
					resposta.Ponto = pontoPedido
					resposta.Mensagem = fmt.Sprintf("Ponto %s não estava pré-reservado para %s", pontoPedido, placaVeiculo)
					fmt.Printf("(ERRO) O ponto %s não estava pré-reservado para %s.\n", pontoPedido, placaVeiculo)
				}
			}
		}
		lock.Unlock()
	}
	if !pontoLocalizado {
		resposta.Status = "ignorado"
		resposta.Mensagem = "Nenhum dos pontos solicitados pertence a esta empresa"
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resposta)
}
