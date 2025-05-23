package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Empresa struct {
	Id     string
	Nome   string
	Pontos []string
}

type Ponto struct {
	ID        int     `json:"id"`
	Cidade    string  `json:"cidade"`
	Estado    string  `json:"estado"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Reservado string  `json:"reservado"`
}

type DadosRegiao struct {
	PontosDeRecarga     []Ponto  `json:"pontos_de_recarga"`
	RotaSalvadorSaoLuis []string `json:"rota_salvador_saoLuis"`
}

type DadosEmpresas struct {
	Empresas []Empresa `json:"empresas"`
}

var dadosEmpresas DadosEmpresas
var dadosRegiao DadosRegiao

/*
Função que Inicia o servidor
*/
func startingServer() {
	leArquivoJsonEmpresas()
	GetPontosDeRecargaJson()

	idEmpresa := os.Getenv("ID")
	porta := getPortaByID(idEmpresa)
	empresa = getEmpresaPorId(idEmpresa)

	doingInitializations(porta, idEmpresa)
	fmt.Printf("[SERVIDOR DA EMPRESA %s INICIADO]\n", empresa.Nome)
	select {}
}

/*
Pegando Porta pelo ID da Empresa
*/
func getPortaByID(id string) string {
	var porta string
	switch id {
	case "EMP1":
		porta = "8081"
	case "EMP2":
		porta = "8082"
	case "EMP3":
		porta = "8083"
	default:
		porta = "8080"
	}
	return porta
}

/*
Faz as inicializações necessárias
*/
func doingInitializations(porta string, idEmpresa string) {
	startingRest(porta)
	startMonitoringPoints()
	startingMqtt(idEmpresa)
}

/*
As funções abaixo tratam as alterações em arquivos e tb pegam dados em arquivos
*/
func leArquivoJsonEmpresas() {
	bytes, err := os.ReadFile("dadosEmpresas.json")
	if err != nil {
		fmt.Println("Erro ao abrir arquivo JSON:", err)
		return
	}

	err = json.Unmarshal(bytes, &dadosEmpresas)
	if err != nil {
		fmt.Println("Erro ao decodificar JSON:", err)
		return
	}
}

func OpenFile(arquivo string) (DadosRegiao, error) {
	file, erro := os.Open("/app/regiao.json")
	if erro != nil {
		return DadosRegiao{}, (fmt.Errorf("erro ao abrir: %v", erro))
	}
	defer file.Close()

	erro = json.NewDecoder(file).Decode(&dadosRegiao)
	if erro != nil {
		return DadosRegiao{}, (fmt.Errorf("erro ao ler: %v", erro))
	}
	return dadosRegiao, nil
}

func GetPontosDeRecargaJson() ([]Ponto, error) {
	dadosRegiao, erro := OpenFile("regiao.json")
	if erro != nil {
		return dadosRegiao.PontosDeRecarga, fmt.Errorf("erro ao carregar dados JSON: %v", erro)
	}
	return dadosRegiao.PontosDeRecarga, nil
}

// retorna a empresa com base no ID
func getEmpresaPorId(id string) Empresa {
	var empresa Empresa
	if len(dadosEmpresas.Empresas) > 0 {
		for _, emp := range dadosEmpresas.Empresas {
			if emp.Id == id {
				empresa = emp
			}
		}
	}
	return empresa
}

func getPontoPorCidade(cidade string) (Ponto, int) {
	var ponto Ponto
	var index int
	pontos := dadosRegiao.PontosDeRecarga
	if len(pontos) > 0 {
		for i, pont := range pontos {
			if pont.Cidade == cidade {
				ponto = pont
				index = i
			}
		}
	}
	return ponto, index
}

func salvaDadosPontos() {
	bytes, err := json.MarshalIndent(dadosRegiao, "", "  ")
	if err != nil {
		fmt.Println("Erro ao converter dados para JSON:", err)
		return
	}

	err = os.WriteFile("regiao.json", bytes, 0644)
	if err != nil {
		fmt.Println("Erro ao salvar no arquivo dadosPontos.json:", err)
		return
	}

	fmt.Println("\nDados salvos no arquivo Região!")
}
