// DisputaSimultanea: dois passageiros ao mesmo tempo na ultima vaga.
// EmpresaCai: uma empresa para no meio da reserva.
// A carga fica no cmd/simulador, com os tres servidores ja ligados.

package interno

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const segredoTeste = "segredo-lab"

// Se a condicao falhar, marca o teste. Nao para os outros checks.
func checar(t *testing.T, ok bool, msg string) {
	t.Helper()
	if ok {
		t.Log("ok -", msg)
		return
	}
	t.Error("FALHOU -", msg)
}

type trio struct {
	estados []*Estado
	srvs    []*http.Server
	lns     []net.Listener
	mortos  map[string]bool
}

// Sobe as tres empresas em portas aleatorias, com o mesmo SEGREDO de teste.
func subirTrio(t *testing.T, expira, janela, timeout time.Duration) *trio {
	t.Helper()
	pasta := t.TempDir()
	nomes := []struct {
		empresa string
		regiao  string
	}{
		{"vaijunto", "reconcavo"},
		{"sudoeste", "sudoeste"},
		{"norteminas", "norte-minas"},
	}
	tr := &trio{mortos: map[string]bool{}}
	for _, n := range nomes {
		e := NovoEstado(n.empresa, n.regiao, filepath.Join(pasta, n.empresa+".json"), "", nil, expira, janela, timeout)
		e.Segredo = segredoTeste
		ln, err := Abrir(e, "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		tr.estados = append(tr.estados, e)
		tr.lns = append(tr.lns, ln)
	}
	for _, e := range tr.estados {
		for _, outro := range tr.estados {
			if outro.Empresa == e.Empresa {
				continue
			}
			e.Pares = append(e.Pares, Par{Empresa: outro.Empresa, URL: outro.URLPropria})
		}
		if err := e.CarregarOuSemear(); err != nil {
			t.Fatal(err)
		}
		go LoopLimpeza(e)
		AoSubir(e)
	}
	for i, e := range tr.estados {
		tr.srvs = append(tr.srvs, Comecar(e, tr.lns[i]))
	}
	for _, e := range tr.estados {
		esperarSaude(t, e.URLPropria)
	}
	return tr
}

// Fica tentando GET /consorcio/saude ate responder ou estourar o prazo.
func esperarSaude(t *testing.T, base string) {
	t.Helper()
	limite := time.Now().Add(5 * time.Second)
	for time.Now().Before(limite) {
		var resp Resposta
		_, erro := Pedir("GET", base+"/consorcio/saude", nil, 500*time.Millisecond, &resp)
		if !erro && resp.OK {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("nao subiu " + base)
}

// Acha a URL pelo nome da empresa.
func (tr *trio) url(empresa string) string {
	for _, e := range tr.estados {
		if e.Empresa == empresa {
			return e.URLPropria
		}
	}
	return ""
}

// Para a limpeza e desliga quem ainda esta vivo.
func (tr *trio) derrubar() {
	for _, e := range tr.estados {
		e.Parar()
	}
	for i, srv := range tr.srvs {
		if tr.mortos[tr.estados[i].Empresa] {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		srv.Shutdown(ctx)
		cancel()
	}
}

// Desliga uma empresa e marca, pra o derrubar do fim nao desligar de novo.
func (tr *trio) derrubarEmpresa(empresa string) {
	for i, e := range tr.estados {
		if e.Empresa != empresa {
			continue
		}
		e.Parar()
		tr.mortos[empresa] = true
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		tr.srvs[i].Shutdown(ctx)
		cancel()
		return
	}
}

// Pede os trechos do dia e devolve a vaga visivel daquele par de cidades.
func vaga(t *testing.T, base, origem, destino string) int {
	t.Helper()
	var resp Resposta
	_, erro := Pedir("GET", base+"/consorcio/trechos?data=2026-10-30", nil, 3*time.Second, &resp)
	if erro || !resp.OK {
		t.Fatalf("sem trechos em %s", base)
	}
	for _, tr := range resp.Trechos {
		if NormalizarCidade(tr.Origem) == NormalizarCidade(origem) && NormalizarCidade(tr.Destino) == NormalizarCidade(destino) {
			return tr.Vagas
		}
	}
	t.Fatalf("nao achei trecho %s -> %s", origem, destino)
	return -1
}

// GET /buscar. O nome nao e buscar porque o cli.go ja tem essa funcao.
func buscarRota(base, origem, destino string) Resposta {
	var resp Resposta
	Pedir("GET", base+"/buscar?origem="+escapar(origem)+"&destino="+escapar(destino)+"&data=2026-10-30", nil, 5*time.Second, &resp)
	return resp
}

// POST /reservar com a senha da conta.
func reservar(base, usuario string, trechos []TrechoViagem) Resposta {
	var resp Resposta
	Pedir("POST", base+"/reservar", map[string]any{
		"usuario": usuario,
		"senha":   "senha123",
		"trechos": trechos,
	}, 8*time.Second, &resp)
	return resp
}

// Procura um itinerario de 3 trechos que use as tres empresas.
func acharCaminho(its []Itinerario) []TrechoViagem {
	for _, it := range its {
		if len(it.Trechos) != 3 {
			continue
		}
		tem := map[string]bool{}
		for _, t := range it.Trechos {
			tem[t.Empresa] = true
		}
		if tem["vaijunto"] && tem["sudoeste"] && tem["norteminas"] {
			return it.Trechos
		}
	}
	return nil
}

// JSON quebrado volta 400 e nao mexe na vaga.
func TestJSONRuim(t *testing.T) {
	tr := subirTrio(t, 20*time.Second, 40*time.Millisecond, 3*time.Second)
	defer tr.derrubar()
	antes := vaga(t, tr.url("vaijunto"), "Salvador", "Cachoeira")
	req, err := http.NewRequest("POST", tr.url("vaijunto")+"/reservar", strings.NewReader("{isso nao e json"))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	checar(t, resp.StatusCode == 400, "json quebrado volta 400")
	checar(t, vaga(t, tr.url("vaijunto"), "Salvador", "Cachoeira") == antes, "vaga continua igual")
}

// Reserva o caminho das tres, baixa 1 em cada, e a segunda pessoa falha.
func TestBuscaEReserva(t *testing.T) {
	tr := subirTrio(t, 20*time.Second, 40*time.Millisecond, 3*time.Second)
	defer tr.derrubar()
	antesFeira := vaga(t, tr.url("vaijunto"), "Salvador", "Feira de Santana")
	antesVitoria := vaga(t, tr.url("sudoeste"), "Feira de Santana", "Vitória da Conquista")
	antesMontes := vaga(t, tr.url("norteminas"), "Vitória da Conquista", "Montes Claros")
	checar(t, antesFeira == 3, "vaijunto comeca com 3 vagas")
	checar(t, antesMontes == 1, "norteminas comeca com 1 vaga")

	busca := buscarRota(tr.url("vaijunto"), "Salvador", "Montes Claros")
	checar(t, busca.OK, "busca respondeu")
	checar(t, len(busca.Itinerarios) >= 1, "achou pelo menos um itinerario")
	caminho := acharCaminho(busca.Itinerarios)
	checar(t, caminho != nil, "itinerario usa as tres empresas")
	if caminho == nil {
		return
	}

	resp := reservar(tr.url("vaijunto"), "passageiro1", caminho)
	checar(t, resp.OK, "primeira reserva confirmou")
	checar(t, vaga(t, tr.url("vaijunto"), "Salvador", "Feira de Santana") == antesFeira-1, "baixou 1 vaga na vaijunto")
	checar(t, vaga(t, tr.url("sudoeste"), "Feira de Santana", "Vitória da Conquista") == antesVitoria-1, "baixou 1 vaga na sudoeste")
	checar(t, vaga(t, tr.url("norteminas"), "Vitória da Conquista", "Montes Claros") == 0, "ultima vaga da norteminas foi embora")

	resp2 := reservar(tr.url("sudoeste"), "passageiro2", caminho)
	checar(t, !resp2.OK, "segunda pessoa nao leva a mesma vaga")
	checar(t, vaga(t, tr.url("norteminas"), "Vitória da Conquista", "Montes Claros") == 0, "vaga nao ficou negativa")
	checar(t, vaga(t, tr.url("vaijunto"), "Salvador", "Feira de Santana") == antesFeira-1, "a segunda tentativa nao baixou a vaijunto de novo")
	checar(t, vaga(t, tr.url("sudoeste"), "Feira de Santana", "Vitória da Conquista") == antesVitoria-1, "a segunda tentativa nao baixou a sudoeste de novo")
	checar(t, vaga(t, tr.url("vaijunto"), "Salvador", "Cachoeira") == 2, "carona que ninguem usou continua igual")
}

// Cancelar devolve a vaga uma vez. A segunda chamada nao devolve de novo.
func TestCancelarDevolve(t *testing.T) {
	tr := subirTrio(t, 20*time.Second, 40*time.Millisecond, 3*time.Second)
	defer tr.derrubar()
	busca := buscarRota(tr.url("vaijunto"), "Salvador", "Cachoeira")
	if len(busca.Itinerarios) == 0 {
		t.Fatal("sem itinerario local")
	}
	resp := reservar(tr.url("vaijunto"), "passageiro1", busca.Itinerarios[0].Trechos)
	checar(t, resp.OK, "reservou cachoeira")
	checar(t, vaga(t, tr.url("vaijunto"), "Salvador", "Cachoeira") == 1, "foi de 2 pra 1")
	var resp2 Resposta
	Pedir("POST", tr.url("vaijunto")+"/cancelar-reserva", map[string]string{
		"usuario": "passageiro1", "senha": "senha123", "id_reserva": resp.Reserva.ID,
	}, 5*time.Second, &resp2)
	checar(t, resp2.OK, "cancelou")
	checar(t, vaga(t, tr.url("vaijunto"), "Salvador", "Cachoeira") == 2, "vaga voltou")
	var resp3 Resposta
	Pedir("POST", tr.url("vaijunto")+"/cancelar-reserva", map[string]string{
		"usuario": "passageiro1", "senha": "senha123", "id_reserva": resp.Reserva.ID,
	}, 5*time.Second, &resp3)
	checar(t, resp3.OK, "cancelar de novo nao da erro")
	checar(t, vaga(t, tr.url("vaijunto"), "Salvador", "Cachoeira") == 2, "nao devolveu vaga duas vezes")
}

// Mata a sudoeste antes da reserva. A viagem falha e a vaga livre nao muda.
func TestEmpresaCai(t *testing.T) {
	tr := subirTrio(t, 20*time.Second, 40*time.Millisecond, time.Second)
	defer tr.derrubar()
	busca := buscarRota(tr.url("vaijunto"), "Salvador", "Montes Claros")
	caminho := acharCaminho(busca.Itinerarios)
	checar(t, caminho != nil, "tinha caminho antes da queda")
	if caminho == nil {
		return
	}
	tr.derrubarEmpresa("sudoeste")
	antes := vaga(t, tr.url("vaijunto"), "Salvador", "Feira de Santana")
	antesN := vaga(t, tr.url("norteminas"), "Vitória da Conquista", "Montes Claros")
	resp := reservar(tr.url("vaijunto"), "passageiro1", caminho)
	checar(t, !resp.OK, "reserva falhou porque a sudoeste nao responde")
	checar(t, vaga(t, tr.url("vaijunto"), "Salvador", "Feira de Santana") == antes, "vaijunto devolveu a vaga")
	checar(t, vaga(t, tr.url("norteminas"), "Vitória da Conquista", "Montes Claros") == antesN, "norteminas devolveu a vaga")
	local := buscarRota(tr.url("vaijunto"), "Salvador", "Cachoeira")
	checar(t, len(local.Itinerarios) >= 1, "busca local funciona com a outra empresa caída")
}

// Segurou e ninguem confirmou. Depois do prazo a vaga volta, porque ainda estava esperando.
func TestSeguroExpira(t *testing.T) {
	tr := subirTrio(t, 1200*time.Millisecond, 20*time.Millisecond, 3*time.Second)
	defer tr.derrubar()
	var lista Resposta
	_, erro := Pedir("GET", tr.url("norteminas")+"/consorcio/trechos?data=2026-10-30", nil, 3*time.Second, &lista)
	if erro || len(lista.Trechos) == 0 {
		t.Fatal("sem trecho da norteminas")
	}
	trecho := lista.Trechos[0]
	checar(t, trecho.Vagas == 1, "antes do seguro tem 1 vaga")
	corpo := CorpoSegurar{
		IDPedido:     "pedido-teste-expira",
		Relogio:      1,
		EmpresaCoord: "vaijunto",
		URLCoord:     tr.url("vaijunto"),
		Passageiro:   "passageiro1",
		Participantes: []Par{
			{Empresa: "vaijunto", URL: tr.url("vaijunto")},
			{Empresa: "norteminas", URL: tr.url("norteminas")},
		},
		Trechos: []RefTrecho{{IDCarona: trecho.IDCarona, Indice: trecho.Indice}},
	}
	var resp Resposta
	_, erro = PedirComo("POST", tr.url("norteminas")+"/consorcio/segurar", "vaijunto", segredoTeste, corpo, 4*time.Second, &resp)
	checar(t, !erro && resp.OK, "segurou")
	checar(t, vaga(t, tr.url("norteminas"), "Vitória da Conquista", "Montes Claros") == 0, "enquanto segurado a vaga some da busca")
	time.Sleep(2200 * time.Millisecond)
	checar(t, vaga(t, tr.url("norteminas"), "Vitória da Conquista", "Montes Claros") == 1, "depois do prazo a vaga voltou")
}

// Dois passageiros ao mesmo tempo na ultima vaga. Exatamente um ganha.
func TestDisputaSimultanea(t *testing.T) {
	tr := subirTrio(t, 20*time.Second, 80*time.Millisecond, 4*time.Second)
	defer tr.derrubar()
	busca := buscarRota(tr.url("vaijunto"), "Salvador", "Montes Claros")
	caminho := acharCaminho(busca.Itinerarios)
	if caminho == nil {
		t.Fatal("sem caminho")
	}
	type resultado struct {
		usuario string
		resp    Resposta
	}
	ch := make(chan resultado, 2)
	go func() {
		ch <- resultado{"passageiro1", reservar(tr.url("vaijunto"), "passageiro1", caminho)}
	}()
	go func() {
		ch <- resultado{"passageiro2", reservar(tr.url("sudoeste"), "passageiro2", caminho)}
	}()
	ganhos := 0
	for i := 0; i < 2; i++ {
		r := <-ch
		t.Log(r.usuario, "ok=", r.resp.OK, "erro=", r.resp.Erro)
		if r.resp.OK {
			ganhos = ganhos + 1
		}
	}
	checar(t, ganhos == 1, "exatamente um passageiro confirmou")
	checar(t, vaga(t, tr.url("norteminas"), "Vitória da Conquista", "Montes Claros") == 0, "sobrou zero, nao menos que zero")
	checar(t, vaga(t, tr.url("vaijunto"), "Salvador", "Feira de Santana") == 2, "so um assento saiu da vaijunto")
	checar(t, vaga(t, tr.url("sudoeste"), "Feira de Santana", "Vitória da Conquista") == 2, "so um assento saiu da sudoeste")
}

// Acha o Estado pelo nome.
func (tr *trio) estado(empresa string) *Estado {
	for _, e := range tr.estados {
		if e.Empresa == empresa {
			return e
		}
	}
	return nil
}

// Sobe a mesma empresa de novo, no mesmo arquivo e com o mesmo SEGREDO.
func (tr *trio) reiniciar(t *testing.T, empresa string) {
	t.Helper()
	for i, e := range tr.estados {
		if e.Empresa != empresa {
			continue
		}
		addr := tr.lns[i].Addr().String()
		arquivo := e.Arquivo
		regiao := e.Regiao
		url := e.URLPropria
		pares := append([]Par(nil), e.Pares...)
		expira, janela, timeout := e.Expira, e.Janela, e.Timeout
		e.Parar()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		tr.srvs[i].Shutdown(ctx)
		cancel()
		novo := NovoEstado(empresa, regiao, arquivo, url, pares, expira, janela, timeout)
		novo.Segredo = e.Segredo
		if err := novo.CarregarOuSemear(); err != nil {
			t.Fatal(err)
		}
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			t.Fatal(err)
		}
		go LoopLimpeza(novo)
		AoSubir(novo)
		tr.estados[i] = novo
		tr.lns[i] = ln
		tr.srvs[i] = Comecar(novo, ln)
		tr.mortos[empresa] = false
		esperarSaude(t, url)
		return
	}
	t.Fatal("nao achei " + empresa)
}

// Devolve a fase do pedido daquele passageiro.
func (tr *trio) fase(empresa, passageiro string) string {
	e := tr.estado(empresa)
	if e == nil {
		return ""
	}
	e.Trava.Lock()
	defer e.Trava.Unlock()
	for _, p := range e.Pedidos {
		if p.Passageiro == passageiro {
			return p.Fase
		}
	}
	return ""
}

// Le VagasLivres no JSON da memoria, sem descontar seguro.
func vagaBruta(tr *trio, empresa, origem, destino string) int {
	e := tr.estado(empresa)
	if e == nil {
		return -1
	}
	e.Trava.Lock()
	defer e.Trava.Unlock()
	for _, c := range e.Caronas {
		for _, t := range c.Trechos {
			if NormalizarCidade(t.Origem) == NormalizarCidade(origem) && NormalizarCidade(t.Destino) == NormalizarCidade(destino) {
				return t.VagasLivres
			}
		}
	}
	return -1
}

// A norteminas grava o confirmar e a resposta se perde. Ao voltar, a vaga baixa uma vez.
func TestAckPerdido(t *testing.T) {
	tr := subirTrio(t, 30*time.Second, 20*time.Millisecond, time.Second)
	defer tr.derrubar()
	busca := buscarRota(tr.url("vaijunto"), "Salvador", "Montes Claros")
	caminho := acharCaminho(busca.Itinerarios)
	if caminho == nil {
		t.Fatal("sem caminho")
	}
	norte := tr.estado("norteminas")
	norte.CortarAckConfirmar = true
	resp := reservar(tr.url("vaijunto"), "passageiro1", caminho)
	checar(t, !resp.OK, "o coordenador nao recebeu o ack")
	checar(t, tr.fase("vaijunto", "passageiro1") == "confirmando", "a vaijunto ficou confirmando, nao abortou")
	checar(t, vagaBruta(tr, "norteminas", "Vitória da Conquista", "Montes Claros") == 0, "a norteminas ja tinha baixado a vaga")

	tr.reiniciar(t, "norteminas")
	limite := time.Now().Add(8 * time.Second)
	for time.Now().Before(limite) {
		if tr.fase("vaijunto", "passageiro1") == "confirmada" {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	checar(t, tr.fase("vaijunto", "passageiro1") == "confirmada", "a repeticao fechou a reserva")
	checar(t, vagaBruta(tr, "norteminas", "Vitória da Conquista", "Montes Claros") == 0, "a vaga nao baixou duas vezes")
	checar(t, vagaBruta(tr, "vaijunto", "Salvador", "Feira de Santana") == 2, "vaijunto baixou uma")
	checar(t, vagaBruta(tr, "sudoeste", "Feira de Santana", "Vitória da Conquista") == 2, "sudoeste baixou uma")
}

// Trecho repetido nao reserva, sem senha nao reserva, e a mesma chave nao compra dois assentos.
func TestTrechoDuplicadoESenha(t *testing.T) {
	tr := subirTrio(t, 20*time.Second, 20*time.Millisecond, time.Second)
	defer tr.derrubar()
	var lista Resposta
	Pedir("GET", tr.url("norteminas")+"/consorcio/trechos?data=2026-10-30", nil, 3*time.Second, &lista)
	if len(lista.Trechos) == 0 {
		t.Fatal("sem trecho")
	}
	ruim := lista.Trechos[0]
	ruim.Preco = -7
	ruim.Origem = "X"
	ruim.Destino = "Y"
	copia := ruim
	copia.Origem = "Y"
	copia.Destino = "Z"
	var resp Resposta
	Pedir("POST", tr.url("vaijunto")+"/reservar", map[string]any{
		"usuario": "passageiro1",
		"senha":   "senha123",
		"trechos": []TrechoViagem{ruim, copia},
	}, 5*time.Second, &resp)
	checar(t, !resp.OK, "trecho repetido nao reserva")
	checar(t, vagaBruta(tr, "norteminas", "Vitória da Conquista", "Montes Claros") == 1, "vaga bruta continua 1")

	Pedir("POST", tr.url("vaijunto")+"/reservar", map[string]any{
		"usuario": "passageiro1",
		"trechos": []TrechoViagem{lista.Trechos[0]},
	}, 5*time.Second, &resp)
	checar(t, !resp.OK, "sem senha nao reserva")

	var locais Resposta
	Pedir("GET", tr.url("vaijunto")+"/consorcio/trechos?data=2026-10-30", nil, 3*time.Second, &locais)
	var cachoeira TrechoViagem
	achou := false
	for _, tch := range locais.Trechos {
		if NormalizarCidade(tch.Destino) == NormalizarCidade("Cachoeira") {
			cachoeira = tch
			achou = true
		}
	}
	checar(t, achou, "achei cachoeira")
	corpo := map[string]any{
		"usuario":      "passageiro1",
		"senha":        "senha123",
		"id_tentativa": "tenta-1",
		"trechos":      []TrechoViagem{cachoeira},
	}
	Pedir("POST", tr.url("vaijunto")+"/reservar", corpo, 5*time.Second, &resp)
	checar(t, resp.OK, "primeira tentativa reservou")
	var deNovo Resposta
	Pedir("POST", tr.url("vaijunto")+"/reservar", corpo, 5*time.Second, &deNovo)
	checar(t, deNovo.OK, "a mesma tentativa devolve a mesma reserva")
	checar(t, vagaBruta(tr, "vaijunto", "Salvador", "Cachoeira") == 1, "nao comprou o assento duas vezes")
}

// Conta reservas confirmadas daquele passageiro.
func reservasDo(tr *trio, empresa, passageiro string) int {
	e := tr.estado(empresa)
	e.Trava.Lock()
	defer e.Trava.Unlock()
	n := 0
	for _, r := range e.Reservas {
		if r.Passageiro == passageiro && r.Status == "confirmada" {
			n = n + 1
		}
	}
	return n
}

// Dez chamadas ao mesmo tempo com a mesma chave. Uma reserva so.
func TestMesmaTentativa(t *testing.T) {
	tr := subirTrio(t, 20*time.Second, 30*time.Millisecond, 3*time.Second)
	defer tr.derrubar()
	busca := buscarRota(tr.url("sudoeste"), "Feira de Santana", "Vitória da Conquista")
	if len(busca.Itinerarios) == 0 || len(busca.Itinerarios[0].Trechos) == 0 {
		t.Fatal("sem trecho da sudoeste")
	}
	corpo := map[string]any{
		"usuario":      "passageiro1",
		"senha":        "senha123",
		"id_tentativa": "compra-unica",
		"trechos":      busca.Itinerarios[0].Trechos,
	}
	ch := make(chan int, 10)
	for i := 0; i < 10; i++ {
		go func() {
			var resp Resposta
			status, _ := Pedir("POST", tr.url("vaijunto")+"/reservar", corpo, 12*time.Second, &resp)
			if resp.OK {
				ch <- 200
				return
			}
			ch <- status
		}()
	}
	ok := 0
	for i := 0; i < 10; i++ {
		if <-ch == 200 {
			ok = ok + 1
		}
	}
	checar(t, ok >= 1, "uma das chamadas fechou a compra")
	checar(t, reservasDo(tr, "vaijunto", "passageiro1") == 1, "a mesma chave gerou uma reserva so")
	checar(t, vagaBruta(tr, "sudoeste", "Feira de Santana", "Vitória da Conquista") == 2, "saiu um assento dos tres")
}

// A janela e maior que o prazo. Voto ja segurado nao vira aborto.
func TestPrazoNaoAbortaConfirmacao(t *testing.T) {
	tr := subirTrio(t, time.Second, 750*time.Millisecond, time.Second)
	defer tr.derrubar()
	busca := buscarRota(tr.url("vaijunto"), "Salvador", "Montes Claros")
	caminho := acharCaminho(busca.Itinerarios)
	if caminho == nil {
		t.Fatal("sem caminho")
	}
	var resp Resposta
	status, erro := Pedir("POST", tr.url("vaijunto")+"/reservar", map[string]any{
		"usuario": "passageiro1",
		"senha":   "senha123",
		"trechos": caminho,
	}, 20*time.Second, &resp)
	checar(t, !erro, "a reserva respondeu")
	checar(t, resp.OK || status == 202, "nao recusou por prazo")
	checar(t, tr.fase("vaijunto", "passageiro1") != "abortada", "a decisao nao virou aborto")
	limite := time.Now().Add(8 * time.Second)
	for time.Now().Before(limite) && tr.fase("vaijunto", "passageiro1") != "confirmada" {
		time.Sleep(200 * time.Millisecond)
	}
	checar(t, tr.fase("vaijunto", "passageiro1") == "confirmada", "a confirmacao fechou")
	checar(t, vagaBruta(tr, "norteminas", "Vitória da Conquista", "Montes Claros") == 0, "a ultima vaga baixou uma vez")
}

// Se o disco falha, a fase na memoria volta e o arquivo antigo fica.
func TestGravacaoFalha(t *testing.T) {
	tr := subirTrio(t, 20*time.Second, 20*time.Millisecond, time.Second)
	defer tr.derrubar()
	e := tr.estado("vaijunto")
	e.Trava.Lock()
	e.Pedidos["p1"] = &Pedido{IDPedido: "p1", Fase: "segurando", Passageiro: "passageiro1"}
	err := e.Salvar()
	e.Trava.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	original := e.Arquivo
	e.Arquivo = filepath.Join(original, "nao.json")
	e.Trava.Lock()
	err = e.gravarFase("p1", "confirmando")
	fase := e.Pedidos["p1"].Fase
	e.Trava.Unlock()
	checar(t, err != nil, "a escrita falhou")
	checar(t, fase == "segurando", "a memoria voltou para segurando")

	e.Arquivo = original
	outro := NovoEstado(e.Empresa, e.Regiao, original, e.URLPropria, e.Pares, e.Expira, e.Janela, e.Timeout)
	if err := outro.CarregarOuSemear(); err != nil {
		t.Fatal(err)
	}
	checar(t, outro.Pedidos["p1"] != nil && outro.Pedidos["p1"].Fase == "segurando", "o disco continua segurando")

	var lista Resposta
	Pedir("GET", tr.url("vaijunto")+"/consorcio/trechos?data=2026-10-30", nil, 3*time.Second, &lista)
	var cachoeira TrechoViagem
	for _, tch := range lista.Trechos {
		if NormalizarCidade(tch.Destino) == NormalizarCidade("Cachoeira") {
			cachoeira = tch
		}
	}
	corpo := CorpoSegurar{
		IDPedido:     "pedido-grava",
		Relogio:      1,
		EmpresaCoord: "vaijunto",
		Passageiro:   "passageiro1",
		Trechos:      []RefTrecho{{IDCarona: cachoeira.IDCarona, Indice: cachoeira.Indice}},
	}
	var segurou Resposta
	_, erroRede := PedirComo("POST", tr.url("vaijunto")+"/consorcio/segurar", "vaijunto", segredoTeste, corpo, 4*time.Second, &segurou)
	checar(t, !erroRede && segurou.OK, "segurou para testar o desfazer")
	e.Arquivo = filepath.Join(original, "nao.json")
	ok, _ := e.TratarDesfazer("pedido-grava", "vaijunto")
	checar(t, !ok, "desfazer nao mente sucesso se nao gravou")
	e.Trava.Lock()
	estado := ""
	if e.Seguros["pedido-grava"] != nil {
		estado = e.Seguros["pedido-grava"].Estado
	}
	e.Trava.Unlock()
	checar(t, estado == "segurado", "o seguro continua segurado na memoria")
}

// Voto prometido fica preso se o coordenador cai. Outra empresa ainda reserva a viagem dela.
func TestCoordenadorMudo(t *testing.T) {
	tr := subirTrio(t, 1500*time.Millisecond, 20*time.Millisecond, 400*time.Millisecond)
	defer tr.derrubar()
	var lista Resposta
	Pedir("GET", tr.url("norteminas")+"/consorcio/trechos?data=2026-10-30", nil, 3*time.Second, &lista)
	if len(lista.Trechos) == 0 {
		t.Fatal("sem trecho")
	}
	trecho := lista.Trechos[0]
	corpo := CorpoSegurar{
		IDPedido:     "pedido-mudo",
		Relogio:      1,
		EmpresaCoord: "vaijunto",
		Passageiro:   "passageiro1",
		Trechos:      []RefTrecho{{IDCarona: trecho.IDCarona, Indice: trecho.Indice}},
	}
	var segurou Resposta
	_, erro := PedirComo("POST", tr.url("norteminas")+"/consorcio/segurar", "vaijunto", segredoTeste, corpo, 4*time.Second, &segurou)
	checar(t, !erro && segurou.OK, "a norteminas segurou")
	tr.derrubarEmpresa("vaijunto")
	time.Sleep(2 * time.Second)
	checar(t, vaga(t, tr.url("norteminas"), "Vitória da Conquista", "Montes Claros") == 0, "com o coordenador mudo a vaga prometida fica")
	busca := buscarRota(tr.url("sudoeste"), "Feira de Santana", "Vitória da Conquista")
	if len(busca.Itinerarios) == 0 {
		t.Fatal("sem viagem da sudoeste")
	}
	resp := reservar(tr.url("sudoeste"), "passageiro2", busca.Itinerarios[0].Trechos)
	checar(t, resp.OK, "uma viagem que nao depende da vaijunto ainda fecha")
	checar(t, vagaBruta(tr, "sudoeste", "Feira de Santana", "Vitória da Conquista") == 2, "a sudoeste baixou a vaga dela")
}

// Falhou ao gravar confirmando. O pedido aborta quando o disco volta, sem reiniciar.
func TestDiscoVoltaSemReiniciar(t *testing.T) {
	tr := subirTrio(t, 20*time.Second, 20*time.Millisecond, time.Second)
	defer tr.derrubar()
	var lista Resposta
	Pedir("GET", tr.url("norteminas")+"/consorcio/trechos?data=2026-10-30", nil, 3*time.Second, &lista)
	if len(lista.Trechos) == 0 {
		t.Fatal("sem trecho")
	}
	tr.estado("vaijunto").FalharSeFase = "confirmando"
	var resp Resposta
	status, erro := Pedir("POST", tr.url("vaijunto")+"/reservar", map[string]any{
		"usuario":      "passageiro1",
		"senha":        "senha123",
		"id_tentativa": "disco-1",
		"trechos":      []TrechoViagem{lista.Trechos[0]},
	}, 8*time.Second, &resp)
	checar(t, !erro && !resp.OK && status != 202, "a gravacao da decisao falhou")
	limite := time.Now().Add(4 * time.Second)
	for time.Now().Before(limite) && tr.fase("vaijunto", "passageiro1") != "abortada" {
		time.Sleep(200 * time.Millisecond)
	}
	checar(t, tr.fase("vaijunto", "passageiro1") == "abortada", "o pedido abortou sem reiniciar")
	checar(t, vagaBruta(tr, "norteminas", "Vitória da Conquista", "Montes Claros") == 1, "a vaga voltou")
	var deNovo Resposta
	status, _ = Pedir("POST", tr.url("vaijunto")+"/reservar", map[string]any{
		"usuario":      "passageiro1",
		"senha":        "senha123",
		"id_tentativa": "disco-1",
		"trechos":      []TrechoViagem{lista.Trechos[0]},
	}, 5*time.Second, &deNovo)
	checar(t, !deNovo.OK && status != 202, "a mesma chave nao fica pendente")
	checar(t, vagaBruta(tr, "norteminas", "Vitória da Conquista", "Montes Claros") == 1, "nao vendeu de novo")
}

// A resposta do segurar se perde e a escrita de abortando falha. A vaga volta com os tres vivos.
func TestAbortoDiscoVolta(t *testing.T) {
	tr := subirTrio(t, 20*time.Second, 20*time.Millisecond, time.Second)
	defer tr.derrubar()
	var lista Resposta
	Pedir("GET", tr.url("norteminas")+"/consorcio/trechos?data=2026-10-30", nil, 3*time.Second, &lista)
	if len(lista.Trechos) == 0 {
		t.Fatal("sem trecho")
	}
	tr.estado("norteminas").CortarAckSegurar = true
	tr.estado("vaijunto").FalharSeFase = "abortando"
	var resp Resposta
	status, erro := Pedir("POST", tr.url("vaijunto")+"/reservar", map[string]any{
		"usuario":      "passageiro1",
		"senha":        "senha123",
		"id_tentativa": "aborto-1",
		"trechos":      []TrechoViagem{lista.Trechos[0]},
	}, 8*time.Second, &resp)
	checar(t, !erro && !resp.OK && status != 202, "a resposta do segurar se perdeu e o aborto nao gravou")
	limite := time.Now().Add(5 * time.Second)
	for time.Now().Before(limite) && tr.fase("vaijunto", "passageiro1") != "abortada" {
		time.Sleep(200 * time.Millisecond)
	}
	checar(t, tr.fase("vaijunto", "passageiro1") == "abortada", "o aborto entrou quando o disco voltou")
	checar(t, vagaBruta(tr, "norteminas", "Vitória da Conquista", "Montes Claros") == 1, "a vaga bruta voltou")
	checar(t, vaga(t, tr.url("norteminas"), "Vitória da Conquista", "Montes Claros") == 1, "a vaga voltou a ser oferecida")
	var deNovo Resposta
	status, _ = Pedir("POST", tr.url("vaijunto")+"/reservar", map[string]any{
		"usuario":      "passageiro1",
		"senha":        "senha123",
		"id_tentativa": "aborto-1",
		"trechos":      []TrechoViagem{lista.Trechos[0]},
	}, 5*time.Second, &deNovo)
	checar(t, !deNovo.OK && status != 202, "a mesma chave nao fica pendente")
	for _, emp := range []string{"vaijunto", "sudoeste", "norteminas"} {
		esperarSaude(t, tr.url(emp))
	}
}

// Sem credencial 401, senha da conta 401, outra empresa 403. A coordenadora repete o confirmar.
func TestDesfazerSoCoordenador(t *testing.T) {
	tr := subirTrio(t, 20*time.Second, 20*time.Millisecond, time.Second)
	defer tr.derrubar()
	var lista Resposta
	Pedir("GET", tr.url("norteminas")+"/consorcio/trechos?data=2026-10-30", nil, 3*time.Second, &lista)
	if len(lista.Trechos) == 0 {
		t.Fatal("sem trecho")
	}
	resp := reservar(tr.url("vaijunto"), "passageiro1", []TrechoViagem{lista.Trechos[0]})
	checar(t, resp.OK && resp.Reserva != nil, "reservou a ultima vaga")
	if resp.Reserva == nil {
		return
	}
	id := resp.Reserva.IDPedido
	corpo := map[string]string{"id_pedido": id, "empresa": "sudoeste"}
	var ruim Resposta
	status, _ := Pedir("POST", tr.url("norteminas")+"/consorcio/desfazer", corpo, 3*time.Second, &ruim)
	checar(t, status == 401, "sem credencial nao desfaz")
	status, _ = PedirComo("POST", tr.url("norteminas")+"/consorcio/desfazer", "vaijunto", "senha123", corpo, 3*time.Second, &ruim)
	checar(t, status == 401, "a senha da conta nao vale entre empresas")
	status, _ = PedirComo("POST", tr.url("norteminas")+"/consorcio/desfazer", "sudoeste", segredoTeste, corpo, 3*time.Second, &ruim)
	checar(t, status == 403, "outra empresa nao desfaz")
	checar(t, vagaBruta(tr, "norteminas", "Vitória da Conquista", "Montes Claros") == 0, "a vaga continua vendida")
	var ok Resposta
	status, _ = PedirComo("POST", tr.url("norteminas")+"/consorcio/confirmar", "vaijunto", segredoTeste, map[string]any{
		"id_pedido": id, "empresa": "vaijunto", "relogio": 1,
	}, 3*time.Second, &ok)
	checar(t, status == 200 && ok.OK, "a coordenadora repete o confirmar")
	checar(t, vagaBruta(tr, "norteminas", "Vitória da Conquista", "Montes Claros") == 0, "repetir nao devolve a vaga")
}

// Vazio se nao tem seguro esperando ou segurado, nem pedido no meio do protocolo.
// Reserva confirmada nao conta: a venda fechou.
func estadoSemPendencia(tr *trio, empresa string) string {
	e := tr.estado(empresa)
	if e == nil {
		return "sem estado"
	}
	e.Trava.Lock()
	defer e.Trava.Unlock()
	for _, s := range e.Seguros {
		if s.Estado == "segurado" || s.Estado == "esperando" {
			return "seguro " + s.Estado
		}
	}
	for _, p := range e.Pedidos {
		if p.Fase == "segurando" || p.Fase == "confirmando" || p.Fase == "abortando" || p.Fase == "cancelando" {
			return "pedido " + p.Fase
		}
	}
	return ""
}

// Verdadeiro se alguma empresa ja tem pedido ou seguro. O teste de carga espera isso pra derrubar.
func temAndamento(tr *trio) bool {
	for _, e := range tr.estados {
		e.Trava.Lock()
		n := len(e.Pedidos) + len(e.Seguros)
		e.Trava.Unlock()
		if n > 0 {
			return true
		}
	}
	return false
}

// Nomes dos passageiros com reserva confirmada naquele par de cidades.
func titularesDoTrecho(tr *trio, origem, destino string) []string {
	var nomes []string
	for _, e := range tr.estados {
		e.Trava.Lock()
		for _, r := range e.Reservas {
			if r.Status != "confirmada" {
				continue
			}
			for _, trecho := range r.Trechos {
				if NormalizarCidade(trecho.Origem) == NormalizarCidade(origem) && NormalizarCidade(trecho.Destino) == NormalizarCidade(destino) {
					nomes = append(nomes, r.Passageiro)
				}
			}
		}
		e.Trava.Unlock()
	}
	return nomes
}

// 18 passageiros. A sudoeste cai no meio e volta.
// No fim nao pode ter pendencia, e o saldo de cada trecho tem que bater com os titulares.
func TestCargaComQueda(t *testing.T) {
	tr := subirTrio(t, 20*time.Second, 40*time.Millisecond, time.Second)
	defer tr.derrubar()
	antesFeira := vagaBruta(tr, "vaijunto", "Salvador", "Feira de Santana")
	antesSul := vagaBruta(tr, "sudoeste", "Feira de Santana", "Vitória da Conquista")
	antesNorte := vagaBruta(tr, "norteminas", "Vitória da Conquista", "Montes Claros")
	empresas := []string{"vaijunto", "sudoeste", "norteminas"}
	for _, emp := range empresas {
		for i := 0; i < 6; i++ {
			var cad Resposta
			_, erro := Pedir("POST", tr.url(emp)+"/cadastrar", map[string]string{
				"usuario": fmt.Sprintf("carga%s%d", emp, i),
				"senha":   "123",
				"papel":   "passageiro",
			}, 3*time.Second, &cad)
			if erro || !cad.OK {
				t.Fatal("nao cadastrou", emp, i)
			}
		}
	}
	busca := buscarRota(tr.url("vaijunto"), "Salvador", "Montes Claros")
	caminho := acharCaminho(busca.Itinerarios)
	if caminho == nil {
		t.Fatal("sem caminho")
	}
	fim := make(chan struct{}, 18)
	for _, emp := range empresas {
		for i := 0; i < 6; i++ {
			go func(emp string, i int) {
				var resp Resposta
				Pedir("POST", tr.url(emp)+"/reservar", map[string]any{
					"usuario": fmt.Sprintf("carga%s%d", emp, i),
					"senha":   "123",
					"trechos": caminho,
				}, 12*time.Second, &resp)
				fim <- struct{}{}
			}(emp, i)
		}
	}
	limiteAnda := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(limiteAnda) && !temAndamento(tr) {
		time.Sleep(5 * time.Millisecond)
	}
	tr.derrubarEmpresa("sudoeste")
	for i := 0; i < 18; i++ {
		<-fim
	}
	tr.reiniciar(t, "sudoeste")
	limite := time.Now().Add(8 * time.Second)
	for time.Now().Before(limite) {
		limpo := true
		for _, emp := range empresas {
			if estadoSemPendencia(tr, emp) != "" {
				limpo = false
			}
		}
		if limpo {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	for _, emp := range empresas {
		checar(t, estadoSemPendencia(tr, emp) == "", emp+" ficou sem preparacao ou pedido pendente")
	}
	type trechoSaldo struct {
		empresa, origem, destino string
		antes                    int
	}
	saldos := []trechoSaldo{
		{"vaijunto", "Salvador", "Feira de Santana", antesFeira},
		{"sudoeste", "Feira de Santana", "Vitória da Conquista", antesSul},
		{"norteminas", "Vitória da Conquista", "Montes Claros", antesNorte},
	}
	for _, s := range saldos {
		agora := vagaBruta(tr, s.empresa, s.origem, s.destino)
		nomes := titularesDoTrecho(tr, s.origem, s.destino)
		checar(t, agora == s.antes-len(nomes), s.empresa+" saldo bate com os titulares")
		for _, nome := range nomes {
			checar(t, nome != "", s.empresa+" tem titular na vaga vendida")
		}
	}
}

// Le o estado do seguro no arquivo, nao na memoria.
func seguroNoDisco(t *testing.T, arquivo, id string) string {
	t.Helper()
	bruto, err := os.ReadFile(arquivo)
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Seguros map[string]struct {
			Estado string `json:"estado"`
		} `json:"seguros"`
	}
	if err := json.Unmarshal(bruto, &g); err != nil {
		t.Fatal(err)
	}
	if g.Seguros == nil {
		return ""
	}
	s, ok := g.Seguros[id]
	if !ok {
		return ""
	}
	return s.Estado
}

// O sim so vale se o segurado entrou no arquivo. Repetir o id com o disco em esperando nao confirma.
func TestVotoSemDisco(t *testing.T) {
	tr := subirTrio(t, 20*time.Second, 20*time.Millisecond, time.Second)
	defer tr.derrubar()
	var lista Resposta
	Pedir("GET", tr.url("norteminas")+"/consorcio/trechos?data=2026-10-30", nil, 3*time.Second, &lista)
	if len(lista.Trechos) == 0 {
		t.Fatal("sem trecho")
	}
	trecho := lista.Trechos[0]
	norte := tr.estado("norteminas")
	corpo := CorpoSegurar{
		IDPedido:     "pedido-voto-disco",
		Relogio:      1,
		EmpresaCoord: "vaijunto",
		Passageiro:   "passageiro1",
		Participantes: []Par{
			{Empresa: "vaijunto", URL: tr.url("vaijunto")},
			{Empresa: "norteminas", URL: tr.url("norteminas")},
		},
		Trechos: []RefTrecho{{IDCarona: trecho.IDCarona, Indice: trecho.Indice}},
	}
	norte.Trava.Lock()
	norte.Seguros["pedido-voto-outro"] = &Seguro{
		IDPedido:     "pedido-voto-outro",
		Relogio:      2,
		EmpresaCoord: "sudoeste",
		URLCoord:     tr.url("sudoeste"),
		Passageiro:   "passageiro2",
		Trechos:      []RefTrecho{{IDCarona: trecho.IDCarona, Indice: trecho.Indice}},
		Estado:       "esperando",
		Expira:       time.Now().Add(time.Minute),
	}
	if err := norte.Salvar(); err != nil {
		norte.Trava.Unlock()
		t.Fatal(err)
	}
	norte.FalharSeSeguro = "segurado"
	norte.Trava.Unlock()

	pedir := func() (int, Resposta) {
		t.Helper()
		var resp Resposta
		status, erro := PedirComo("POST", tr.url("norteminas")+"/consorcio/segurar", "vaijunto", segredoTeste, corpo, 4*time.Second, &resp)
		checar(t, !erro, "a norteminas respondeu")
		return status, resp
	}
	status, resp := pedir()
	checar(t, !resp.OK && status == 500, "a primeira preparacao falha com 500")
	checar(t, strings.Contains(resp.Erro, "gravar"), "o motivo e gravacao, nao falta de vaga")
	checar(t, seguroNoDisco(t, norte.Arquivo, "pedido-voto-disco") == "esperando", "o disco do pedido ficou esperando")
	checar(t, seguroNoDisco(t, norte.Arquivo, "pedido-voto-outro") == "esperando", "o outro da janela tambem voltou para esperando")

	norte.FalharSeSeguro = "segurado"
	status, resp = pedir()
	checar(t, !resp.OK && status == 500, "repetir o mesmo id nao responde sim")
	checar(t, resp.Mensagem != "ja estava comigo", "a repeticao nao diz que ja segurou")
	checar(t, seguroNoDisco(t, norte.Arquivo, "pedido-voto-disco") == "esperando", "o disco continua esperando")

	status, resp = pedir()
	checar(t, resp.OK && status == 200, "sem a falha de disco, o voto entra")
	checar(t, seguroNoDisco(t, norte.Arquivo, "pedido-voto-disco") == "segurado", "agora o arquivo diz segurado")
}

// Duas viagens usam os mesmos trechos em ordem inversa. Uma confirma, a outra nao, e as duas terminam.
func TestReservasCruzadas(t *testing.T) {
	tr := subirTrio(t, 20*time.Second, 30*time.Millisecond, 3*time.Second)
	defer tr.derrubar()
	publicar := func(emp string, cidades []string) {
		t.Helper()
		resp := tr.estado(emp).PublicarCarona("motorista1", "senha123", cidades, "2026-10-30", "18:00", 1, []int{10})
		if !resp.OK {
			t.Fatal(emp, resp.Erro)
		}
	}
	publicar("vaijunto", []string{"CidadeA", "CidadeB"})
	publicar("sudoeste", []string{"CidadeC", "CidadeD"})
	publicar("norteminas", []string{"CidadeB", "CidadeC"})
	publicar("norteminas", []string{"CidadeD", "CidadeA"})

	ida := buscarRota(tr.url("vaijunto"), "CidadeA", "CidadeD")
	volta := buscarRota(tr.url("sudoeste"), "CidadeC", "CidadeB")
	checar(t, len(ida.Itinerarios) >= 1, "achou o caminho de A ate D")
	checar(t, len(volta.Itinerarios) >= 1, "achou o caminho de C ate B")
	if len(ida.Itinerarios) == 0 || len(volta.Itinerarios) == 0 {
		return
	}

	ch := make(chan bool, 2)
	go func() {
		r := reservar(tr.url("vaijunto"), "passageiro1", ida.Itinerarios[0].Trechos)
		ch <- r.OK
	}()
	go func() {
		r := reservar(tr.url("sudoeste"), "passageiro2", volta.Itinerarios[0].Trechos)
		ch <- r.OK
	}()
	confirmou := 0
	recebidos := 0
	limite := time.After(15 * time.Second)
	for recebidos < 2 {
		select {
		case ok := <-ch:
			recebidos = recebidos + 1
			if ok {
				confirmou = confirmou + 1
			}
		case <-limite:
			t.Fatal("as duas reservas nao terminaram")
		}
	}
	checar(t, confirmou == 1, "uma reserva cruzada confirmou e a outra nao")
}
