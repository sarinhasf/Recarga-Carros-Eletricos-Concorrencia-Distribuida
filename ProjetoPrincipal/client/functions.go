/*
O [Functions] serve para trazer funções auxiliares para cliente (veiculo)
*/

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

/*
Definindo estruturas a serem usadas
*/
type DadosRegiao struct {
	PontosDeRecarga     []Ponto  
	RotaSalvadorSaoLuis []string 
}

type Recarga struct {
	Data    string  
	PontoID int     
	Valor   float64 
}

type Ponto struct {
	ID        int     
	Cidade    string  
	Estado    string  
	Latitude  float64 
	Longitude float64 
	Reservado string  
}

type Veiculo struct {
	Placa             string    
	Autonomia         float64   
	NivelBateriaAtual float64   
	Recargas          []Recarga 
}

type DadosVeiculos struct {
	Veiculos []Veiculo 
}

/*
O GetPontoById serve para localizar um ponto de recarga específico com base no seu ID.
Retorna o ponto encontrado e um código de erro (0 = sucesso, 1 = erro ao abrir arquivo, 2 = ponto não encontrado).
*/
func GetPontoById(id int) (Ponto, int) {
	dadosRegiao, erro := OpenFileRegiao()
	if erro != nil {
		return Ponto{}, 1 //Erro ao carregar dados JSON
	}

	for _, ponto := range dadosRegiao.PontosDeRecarga {
		if ponto.ID == id {
			return ponto, 0
		}
	}
	return Ponto{}, 2 //Erro ao localizar ponto
}

/*
O GetTotalPontosJson serve para contar quantos pontos de recarga existem no sistema.
Retorna a quantidade de pontos ou -1 em caso de erro.
*/
func GetTotalPontosJson() int {
	pontos, erro := GetPontosDeRecargaJson()
	if erro != nil {
		return -1
	}
	return len(pontos)
}

/*
O GetVeiculoPlaca serve para buscar um veículo específico com base na sua placa.
Retorna o veículo e um código de erro (0 = sucesso, 1 = erro ao carregar, 2 = não encontrado).
*/
func GetVeiculoPlaca(placa string) (Veiculo, int) {
	dadosVeiculos, erro := OpenFileVeiculos()
	if erro != nil {
		return Veiculo{}, 1 
	}
	for _, veiculo := range dadosVeiculos.Veiculos {
		if veiculo.Placa == placa {
			return veiculo, 0
		}
	}
	return Veiculo{}, 2 
}

/*
O GetCidadesToPontos serve para obter todos os pontos de recarga que pertencem às cidades passadas como parâmetro.
Retorna uma lista de pontos de recarga encontrados.
*/
func GetCidadesToPontos(cidades []string) []Ponto {
	var pontos []Ponto
	pontosJson, erro := GetPontosDeRecargaJson()
	if erro != nil {
		return []Ponto{}
	}
	for _, c := range cidades {
		for _, ponto := range pontosJson {
			if strings.EqualFold(c, ponto.Cidade) {
				pontos = append(pontos, ponto)
			}
		}
	}
	return pontos
}

/*
O GetTrechoRotaCompleta serve para obter um trecho de uma rota com base em índices de origem e destino fornecidos como string.
Retorna a sub-rota correspondente, o índice de origem e o índice de destino.
*/
func GetTrechoRotaCompleta(origem string, destino string, rotaCompleta []string) ([]string, int, int) {
	var trechoViagem []string

	indexOrigem, err1 := strconv.Atoi(origem)
	indexDestino, err2 := strconv.Atoi(destino)

	if err1 != nil || err2 != nil || 1 > indexOrigem || 9 < indexOrigem || 1 > indexDestino || 9 < indexDestino {
		return []string{}, -1, -1
	}

	if indexOrigem-1 <= indexDestino-1 {
		trechoViagem = rotaCompleta[indexOrigem-1 : indexDestino]
	} else {
		for i := indexOrigem - 1; i >= indexDestino-1; i-- {
			trechoViagem = append(trechoViagem, rotaCompleta[i])
		}
	}
	return trechoViagem, indexOrigem - 1, indexDestino - 1
}

/*
O OpenFileVeiculos serve para abrir e decodificar o arquivo JSON que contém os dados dos veículos.
Retorna os dados dos veículos e um possível erro.
*/
func OpenFileVeiculos() (DadosVeiculos, error) {
	file, erro := os.Open("/app/dadosVeiculos.json")
	if erro != nil {
		return DadosVeiculos{}, (fmt.Errorf("Erro ao abrir: %v", erro))
	}
	defer file.Close()

	var dadosVeiculos DadosVeiculos
	erro = json.NewDecoder(file).Decode(&dadosVeiculos)
	if erro != nil {
		return DadosVeiculos{}, (fmt.Errorf("Erro ao ler: %v", erro))
	}
	return dadosVeiculos, nil
}

/*
O WriteFileVeiculos serve para adicionar um novo veículo ao arquivo de dados dos veículos.
Retorna um erro caso a operação falhe.
*/
func WriteFileVeiculos(veiculo Veiculo) error {
	dadosVeiculos, erro := OpenFileVeiculos()
	if erro != nil {
		fmt.Printf("Erro ao abrir arquivo: %v\n", erro)
		return erro
	}
	dadosVeiculos.Veiculos = append(dadosVeiculos.Veiculos, veiculo)

	file, erro := os.OpenFile("/app/dadosVeiculos.json", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if erro != nil {
		fmt.Printf("Erro ao criar arquivo: %v\n", erro)
		return erro
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	erro = encoder.Encode(dadosVeiculos)
	return erro
}

/*
O RemoveVeiculoPorPlaca serve para remover um veículo do arquivo com base em sua placa.
Retorna um erro caso a operação falhe.
*/
func RemoveVeiculoPorPlaca(placa string) error {
	dadosVeiculos, erro := OpenFileVeiculos()
	if erro != nil {
		return fmt.Errorf("Erro ao abrir arquivo: %v", erro)
	}

	var listaAtualizada []Veiculo
	for _, v := range dadosVeiculos.Veiculos {
		if strings.ToUpper(v.Placa) != strings.ToUpper(placa) {
			listaAtualizada = append(listaAtualizada, v)
		}
	}
	dadosVeiculos.Veiculos = listaAtualizada

	file, erro := os.OpenFile("/app/dadosVeiculos.json", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if erro != nil {
		return fmt.Errorf("Erro ao salvar arquivo: %v", erro)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(dadosVeiculos)
}

/*
O OpenFileRegiao serve para abrir e decodificar o arquivo JSON que contém os dados da região.
Retorna os dados da região e um possível erro.
*/
func OpenFileRegiao() (DadosRegiao, error) {
	file, erro := os.Open("/app/regiao.json")
	if erro != nil {
		return DadosRegiao{}, (fmt.Errorf("Erro ao abrir: %v", erro))
	}
	defer file.Close()

	var dadosRegiao DadosRegiao
	erro = json.NewDecoder(file).Decode(&dadosRegiao)
	if erro != nil {
		return DadosRegiao{}, (fmt.Errorf("Erro ao ler: %v", erro))
	}
	return dadosRegiao, nil
}

/*
O GetRotaSalvadorSaoLuis serve para retornar a rota completa entre Salvador e São Luís a partir dos dados da região.
Retorna a lista de cidades da rota ou um erro.
*/
func GetRotaSalvadorSaoLuis() ([]string, error) {
	dadosRegiao, erro := OpenFileRegiao()
	if erro != nil {
		return dadosRegiao.RotaSalvadorSaoLuis, fmt.Errorf("Erro ao carregar dados JSON: %v", erro)
	}
	return dadosRegiao.RotaSalvadorSaoLuis, nil
}

/*
O GetPontosDeRecargaJson serve para retornar todos os pontos de recarga disponíveis a partir dos dados da região.
Retorna uma lista de pontos ou um erro.
*/
func GetPontosDeRecargaJson() ([]Ponto, error) {
	dadosRegiao, erro := OpenFileRegiao()
	if erro != nil {
		return dadosRegiao.PontosDeRecarga, fmt.Errorf("Erro ao carregar dados JSON: %v", erro)
	}
	return dadosRegiao.PontosDeRecarga, nil
}

/*
O GetVeiculosAtivosJson serve para retornar a lista de veículos atualmente cadastrados no sistema.
Retorna a lista de veículos ou um erro.
*/
func GetVeiculosAtivosJson() ([]Veiculo, error) {
	DadosVeiculos, erro := OpenFileVeiculos()
	if erro != nil {
		return DadosVeiculos.Veiculos, fmt.Errorf("Erro ao carregar dados JSON: %v", erro)
	}
	return DadosVeiculos.Veiculos, nil
}
