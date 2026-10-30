package interno

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// Simulador sem tela. Sobe os tres servidores antes e roda:
//   go run ./cmd/simulador 12
// A carona de Montes Claros nasce com 1 vaga. O normal e 1 sucesso.

var empresasSimulador = []Par{
	{Empresa: "vaijunto", URL: "http://127.0.0.1:5001"},
	{Empresa: "sudoeste", URL: "http://127.0.0.1:5002"},
	{Empresa: "norteminas", URL: "http://127.0.0.1:5003"},
}

func RodarSimulador() {
	quantidade := 8
	if len(os.Args) > 1 {
		fmt.Sscan(os.Args[1], &quantidade)
	}
	if quantidade < 1 {
		fmt.Println("uso: go run ./cmd/simulador 12")
		return
	}

	resultados := make([]bool, quantidade)
	var trava sync.Mutex
	pronto := sync.WaitGroup{}
	fim := sync.WaitGroup{}
	comecar := make(chan struct{})
	pronto.Add(quantidade)
	fim.Add(quantidade)

	for i := 0; i < quantidade; i++ {
		go func(i int) {
			pronto.Done()
			<-comecar
			resultados[i] = umPassageiro(i, &trava)
			fim.Done()
		}(i)
	}
	pronto.Wait()
	close(comecar)
	fim.Wait()

	ok := 0
	for _, r := range resultados {
		if r {
			ok = ok + 1
		}
	}
	fmt.Println()
	fmt.Println("fim. reservas confirmadas:", ok, "de", quantidade)
	fmt.Println("se deu mais de uma e a vaga de Montes Claros era 1, olha a atomicidade")
}

func umPassageiro(indice int, trava *sync.Mutex) bool {
	par := empresasSimulador[indice%len(empresasSimulador)]
	usuario := fmt.Sprintf("simulado%d", indice)
	log := func(texto string) {
		trava.Lock()
		fmt.Println(texto)
		trava.Unlock()
	}

	var cad Resposta
	_, erro := Pedir("POST", par.URL+"/cadastrar", map[string]string{
		"usuario": usuario, "senha": "123", "papel": "passageiro",
	}, 5*time.Second, &cad)
	if erro || !cad.OK {
		log(usuario + " nao cadastrou em " + par.Empresa)
		return false
	}

	var busca Resposta
	_, erro = Pedir("GET", par.URL+"/buscar?origem=Salvador&destino=Montes%20Claros&data=2026-10-30", nil, 6*time.Second, &busca)
	if erro || !busca.OK || len(busca.Itinerarios) == 0 {
		log(usuario + " nao achou itinerario pela " + par.Empresa)
		return false
	}

	var reserva Resposta
	_, erro = Pedir("POST", par.URL+"/reservar", map[string]any{
		"usuario": usuario,
		"trechos": busca.Itinerarios[0].Trechos,
	}, 10*time.Second, &reserva)
	if erro {
		log(usuario + " sem resposta na reserva")
		return false
	}
	if reserva.OK && reserva.Reserva != nil {
		log(usuario + " CONSEGUIU pela " + par.Empresa + " id " + reserva.Reserva.ID)
		return true
	}
	log(usuario + " nao reservou (" + reserva.Erro + ")")
	return false
}
