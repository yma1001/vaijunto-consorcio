// DisputaSimultanea: dois passageiros ao mesmo tempo na ultima vaga.
// EmpresaCai: uma empresa para no meio da reserva.
// A carga fica no cmd/simulador, com os tres servidores ja ligados.

package interno

import (
	"context"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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

func (tr *trio) url(empresa string) string {
	for _, e := range tr.estados {
		if e.Empresa == empresa {
			return e.URLPropria
		}
	}
	return ""
}

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

func buscarRota(base, origem, destino string) Resposta {
	var resp Resposta
	Pedir("GET", base+"/buscar?origem="+escapar(origem)+"&destino="+escapar(destino)+"&data=2026-10-30", nil, 5*time.Second, &resp)
	return resp
}

func reservar(base, usuario string, trechos []TrechoViagem) Resposta {
	var resp Resposta
	Pedir("POST", base+"/reservar", map[string]any{
		"usuario": usuario,
		"trechos": trechos,
	}, 8*time.Second, &resp)
	return resp
}

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
		"usuario": "passageiro1", "id_reserva": resp.Reserva.ID,
	}, 5*time.Second, &resp2)
	checar(t, resp2.OK, "cancelou")
	checar(t, vaga(t, tr.url("vaijunto"), "Salvador", "Cachoeira") == 2, "vaga voltou")
	var resp3 Resposta
	Pedir("POST", tr.url("vaijunto")+"/cancelar-reserva", map[string]string{
		"usuario": "passageiro1", "id_reserva": resp.Reserva.ID,
	}, 5*time.Second, &resp3)
	checar(t, resp3.OK, "cancelar de novo nao da erro")
	checar(t, vaga(t, tr.url("vaijunto"), "Salvador", "Cachoeira") == 2, "nao devolveu vaga duas vezes")
}

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
	_, erro = Pedir("POST", tr.url("norteminas")+"/consorcio/segurar", corpo, 4*time.Second, &resp)
	checar(t, !erro && resp.OK, "segurou")
	checar(t, vaga(t, tr.url("norteminas"), "Vitória da Conquista", "Montes Claros") == 0, "enquanto segurado a vaga some da busca")
	time.Sleep(2200 * time.Millisecond)
	checar(t, vaga(t, tr.url("norteminas"), "Vitória da Conquista", "Montes Claros") == 1, "depois do prazo a vaga voltou")
}

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
