package main

import (
	"fmt"
	"sync"
	"testing"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// fakeClient implementa mqtt.Client com stubs para todos os métodos usados
type fakeClient struct{}

func (f *fakeClient) OptionsReader() mqtt.ClientOptionsReader {
	// Retorna um mock de ClientOptionsReader
	return mqtt.ClientOptionsReader{}
}
func (f *fakeClient) IsConnected() bool       { return true }
func (f *fakeClient) Connect() mqtt.Token     { return &mqtt.DummyToken{} }
func (f *fakeClient) Disconnect(quiesce uint) {}
func (f *fakeClient) Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
	return &mqtt.DummyToken{}
}
func (f *fakeClient) Subscribe(topic string, qos byte, callback mqtt.MessageHandler) mqtt.Token {
	return &mqtt.DummyToken{}
}
func (f *fakeClient) Unsubscribe(topics ...string) mqtt.Token { return &mqtt.DummyToken{} }
func (f *fakeClient) ResumePublish()                          {}
func (f *fakeClient) ResumeSubscriptions()                    {}
func (f *fakeClient) SubscribeMultiple(filters map[string]byte, callback mqtt.MessageHandler) mqtt.Token {
	return &mqtt.DummyToken{}
}
func (f *fakeClient) AddRoute(topic string, callback mqtt.MessageHandler) {}
func (f *fakeClient) IsConnectionOpen() bool {
	return true
}
func setup() {
	// Inicializar dadosRegiao com um ponto "PontoX" livre
	dadosRegiao = DadosRegiao{PontosDeRecarga: []Ponto{{ID: 1, Cidade: "PontoX", Reservado: ""}}}

	// Iniciar empresa com esse ponto
	empresa = Empresa{Id: "EMP_TEST", Nome: "Teste", Pontos: []string{"PontoX"}}

	// Criar o lock do ponto
	pontoLocks = map[string]*sync.Mutex{"PontoX": &sync.Mutex{}}

	// Marcar o ponto como conectado
	pontosStatus.Lock()
	pontosStatus.status = map[string]bool{"PontoX": true}
	pontosStatus.Unlock()
}

// TestProcessaReservaConcorrente testa se múltiplos veículos conseguem reservar
// ao mesmo ponto de recarga sem causar condições de corrida ou falhas.
func TestProcessaReservaConcorrente(t *testing.T) {
	setup()
	client := &fakeClient{}
	var wg sync.WaitGroup

	// 10 veículos tentam ao mesmo tempo
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			placa := fmt.Sprintf("VEIC%d", i)
			processaReservaMQTT(client, []string{"PontoX"}, placa)
		}(i)
	}
	wg.Wait()

	// Contar quem efetivamente reservou
	reservado := dadosRegiao.PontosDeRecarga[0].Reservado
	if reservado == "" {
		t.Fatal("nenhum veículo conseguiu reservar o ponto")
	}
	t.Logf("PontoX reservado por: %s", reservado)
}
