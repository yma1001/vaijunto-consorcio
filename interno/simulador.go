package interno

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// Simulador sem tela. Sobe os tres servidores antes e roda:
//   go run ./cmd/simulador 12
// A carona de Montes Claros nasce com 1 vaga. O normal e 1 sucesso.
// SIM_EMPRESAS troca o endereco. exemplo: vaijunto|http://192.168.0.10:5001,sudoeste|http://192.168.0.11:5002

// As tres portas locais, ou o que estiver em SIM_EMPRESAS.
func empresasDoSimulador() []Par {
	texto := os.Getenv("SIM_EMPRESAS")
	if texto == "" {
		return []Par{
			{Empresa: "vaijunto", URL: "http://127.0.0.1:5001"},
			{Empresa: "sudoeste", URL: "http://127.0.0.1:5002"},
			{Empresa: "norteminas", URL: "http://127.0.0.1:5003"},
		}
	}
	saida := []Par{}
	for _, parte := range strings.Split(texto, ",") {
		pedacos := strings.SplitN(parte, "|", 2)
		if len(pedacos) != 2 {
			continue
		}
		saida = append(saida, Par{Empresa: strings.TrimSpace(pedacos[0]), URL: strings.TrimSpace(pedacos[1])})
	}
	return saida
}

// Dispara varios passageiros ao mesmo tempo e conta quantos confirmaram.
func RodarSimulador() {
	quantidade := 8
	if len(os.Args) > 1 {
		fmt.Sscan(os.Args[1], &quantidade)
	}
	if quantidade < 1 {
		fmt.Println("uso: go run ./cmd/simulador 12")
		return
	}
	empresas := empresasDoSimulador()
	if len(empresas) == 0 {
		fmt.Println("SIM_EMPRESAS vazio")
		return
	}

	cadastro := 0
	busca := 0
	confirma := 0
	recusa := 0
	timeout := 0
	var trava sync.Mutex
	pronto := sync.WaitGroup{}
	fim := sync.WaitGroup{}
	comecar := make(chan struct{})
	pronto.Add(quantidade)
	fim.Add(quantidade)
	marca := fmt.Sprintf("%d", time.Now().UnixNano())
	inicio := time.Now()

	for i := 0; i < quantidade; i++ {
		go func(i int) {
			pronto.Done()
			<-comecar
			tipo := umPassageiro(i, marca, empresas, &trava)
			trava.Lock()
			if tipo == "confirma" {
				confirma = confirma + 1
				cadastro = cadastro + 1
				busca = busca + 1
			} else if tipo == "recusa" {
				recusa = recusa + 1
				cadastro = cadastro + 1
				busca = busca + 1
			} else if tipo == "timeout" {
				timeout = timeout + 1
			} else if tipo == "sem-busca" {
				cadastro = cadastro + 1
			}
			trava.Unlock()
			fim.Done()
		}(i)
	}
	pronto.Wait()
	close(comecar)
	fim.Wait()

	fmt.Println()
	fmt.Println("cadastro", cadastro, "busca", busca, "confirma", confirma, "recusa", recusa, "timeout", timeout)
	fmt.Println("durou", time.Since(inicio).Round(time.Millisecond))
	fmt.Println("se deu mais de uma confirma e a vaga de Montes Claros era 1, olha a atomicidade")
}

// Cadastra um usuario, busca Salvador ate Montes Claros e reserva o primeiro caminho.
func umPassageiro(indice int, marca string, empresas []Par, trava *sync.Mutex) string {
	par := empresas[indice%len(empresas)]
	usuario := fmt.Sprintf("simulado%s-%d", marca, indice)
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
		if erro {
			return "timeout"
		}
		return "sem-cadastro"
	}

	var busca Resposta
	_, erro = Pedir("GET", par.URL+"/buscar?origem=Salvador&destino=Montes%20Claros&data=2026-10-30", nil, 6*time.Second, &busca)
	if erro || !busca.OK || len(busca.Itinerarios) == 0 {
		log(usuario + " nao achou itinerario pela " + par.Empresa)
		if erro {
			return "timeout"
		}
		return "sem-busca"
	}

	var reserva Resposta
	_, erro = Pedir("POST", par.URL+"/reservar", map[string]any{
		"usuario":      usuario,
		"senha":        "123",
		"id_tentativa": usuario,
		"trechos":      busca.Itinerarios[0].Trechos,
	}, 15*time.Second, &reserva)
	if erro {
		log(usuario + " sem resposta na reserva")
		return "timeout"
	}
	if reserva.OK && reserva.Reserva != nil {
		log(usuario + " CONSEGUIU pela " + par.Empresa + " id " + reserva.Reserva.ID)
		return "confirma"
	}
	log(usuario + " nao reservou (" + reserva.Erro + ")")
	return "recusa"
}
