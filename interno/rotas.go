package interno

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// HTTP em cima de TCP. O enunciado pediu REST entre as empresas.
// A coordenacao (relogio, segurar, desfazer) esta neste codigo.

func (e *Estado) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("[%s] %s %s\n", e.Empresa, r.Method, r.URL.RequestURI())
	caminho := r.URL.Path

	if r.Method == http.MethodGet && (caminho == "/" || caminho == "/consorcio/saude") {
		e.rotaSaude(w)
		return
	}
	if r.Method == http.MethodGet && caminho == "/consorcio/trechos" {
		data := r.URL.Query().Get("data")
		responder(w, 200, Resposta{OK: true, Trechos: e.TrechosPublicos(data)})
		return
	}
	if r.Method == http.MethodGet && caminho == "/consorcio/pedido" {
		id := r.URL.Query().Get("id")
		responder(w, 200, map[string]any{"ok": true, "estado": e.ConsultarPedido(id)})
		return
	}
	if r.Method == http.MethodGet && caminho == "/buscar" {
		origem := r.URL.Query().Get("origem")
		destino := r.URL.Query().Get("destino")
		data := r.URL.Query().Get("data")
		if origem == "" || destino == "" || data == "" {
			responder(w, 400, Resposta{Erro: "manda origem, destino e data"})
			return
		}
		resp := e.BuscarItinerarios(origem, destino, data)
		responder(w, 200, resp)
		return
	}
	if r.Method == http.MethodGet && caminho == "/minhas-caronas" {
		resp := e.ListarMinhasCaronas(r.URL.Query().Get("usuario"))
		responder(w, codigoHTTP(resp), resp)
		return
	}
	if r.Method == http.MethodGet && caminho == "/passageiros" {
		resp, lista := e.ListarPassageiros(r.URL.Query().Get("usuario"), r.URL.Query().Get("id_carona"))
		if !resp.OK {
			responder(w, codigoHTTP(resp), resp)
			return
		}
		responder(w, 200, map[string]any{"ok": true, "trechos": lista})
		return
	}
	if r.Method == http.MethodGet && caminho == "/minhas-reservas" {
		resp := e.ListarMinhasReservas(r.URL.Query().Get("usuario"))
		responder(w, codigoHTTP(resp), resp)
		return
	}

	if r.Method == http.MethodPost && caminho == "/entrar" {
		var corpo struct {
			Usuario string `json:"usuario"`
			Senha   string `json:"senha"`
		}
		if err := lerJSON(w, r, &corpo); err != nil {
			responder(w, 400, Resposta{Erro: "json invalido"})
			return
		}
		resp := e.Entrar(corpo.Usuario, corpo.Senha)
		responder(w, codigoHTTP(resp), resp)
		return
	}
	if r.Method == http.MethodPost && caminho == "/cadastrar" {
		var corpo struct {
			Usuario string `json:"usuario"`
			Senha   string `json:"senha"`
			Papel   string `json:"papel"`
		}
		if err := lerJSON(w, r, &corpo); err != nil {
			responder(w, 400, Resposta{Erro: "json invalido"})
			return
		}
		resp := e.Cadastrar(corpo.Usuario, corpo.Senha, corpo.Papel)
		responder(w, codigoHTTP(resp), resp)
		return
	}
	if r.Method == http.MethodPost && caminho == "/caronas" {
		var corpo struct {
			Usuario string   `json:"usuario"`
			Cidades []string `json:"cidades"`
			Data    string   `json:"data"`
			Hora    string   `json:"hora"`
			Vagas   int      `json:"vagas"`
			Precos  []int    `json:"precos"`
		}
		if err := lerJSON(w, r, &corpo); err != nil {
			responder(w, 400, Resposta{Erro: "json invalido"})
			return
		}
		resp := e.PublicarCarona(corpo.Usuario, corpo.Cidades, corpo.Data, corpo.Hora, corpo.Vagas, corpo.Precos)
		responder(w, codigoHTTP(resp), resp)
		return
	}
	if r.Method == http.MethodPost && caminho == "/cancelar-carona" {
		var corpo struct {
			Usuario  string `json:"usuario"`
			IDCarona string `json:"id_carona"`
		}
		if err := lerJSON(w, r, &corpo); err != nil {
			responder(w, 400, Resposta{Erro: "json invalido"})
			return
		}
		resp := e.CancelarCarona(corpo.Usuario, corpo.IDCarona)
		responder(w, codigoHTTP(resp), resp)
		return
	}
	if r.Method == http.MethodPost && caminho == "/reservar" {
		var corpo struct {
			Usuario string         `json:"usuario"`
			Trechos []TrechoViagem `json:"trechos"`
		}
		if err := lerJSON(w, r, &corpo); err != nil {
			responder(w, 400, Resposta{Erro: "json invalido"})
			return
		}
		resp := e.Reservar(corpo.Usuario, corpo.Trechos)
		responder(w, codigoHTTP(resp), resp)
		return
	}
	if r.Method == http.MethodPost && caminho == "/cancelar-reserva" {
		var corpo struct {
			Usuario   string `json:"usuario"`
			IDReserva string `json:"id_reserva"`
		}
		if err := lerJSON(w, r, &corpo); err != nil {
			responder(w, 400, Resposta{Erro: "json invalido"})
			return
		}
		resp := e.CancelarReserva(corpo.Usuario, corpo.IDReserva)
		responder(w, codigoHTTP(resp), resp)
		return
	}
	if r.Method == http.MethodPost && caminho == "/consorcio/segurar" {
		var corpo CorpoSegurar
		if err := lerJSON(w, r, &corpo); err != nil {
			responder(w, 400, Resposta{Erro: "json invalido"})
			return
		}
		ok, motivo := e.TratarSegurar(corpo)
		if ok {
			responder(w, 200, Resposta{OK: true, Mensagem: motivo})
			return
		}
		responder(w, 409, Resposta{Erro: motivo})
		return
	}
	if r.Method == http.MethodPost && caminho == "/consorcio/confirmar" {
		var corpo corpoID
		if err := lerJSON(w, r, &corpo); err != nil {
			responder(w, 400, Resposta{Erro: "json invalido"})
			return
		}
		ok, motivo := e.TratarConfirmar(corpo.IDPedido, corpo.Relogio, false)
		if ok {
			responder(w, 200, Resposta{OK: true, Mensagem: motivo})
			return
		}
		responder(w, 409, Resposta{Erro: motivo})
		return
	}
	if r.Method == http.MethodPost && (caminho == "/consorcio/desfazer" || caminho == "/consorcio/liberar") {
		var corpo corpoID
		if err := lerJSON(w, r, &corpo); err != nil {
			responder(w, 400, Resposta{Erro: "json invalido"})
			return
		}
		ok, motivo := e.TratarDesfazer(corpo.IDPedido)
		if ok {
			responder(w, 200, Resposta{OK: true, Mensagem: motivo})
			return
		}
		responder(w, 400, Resposta{Erro: motivo})
		return
	}

	responder(w, 404, Resposta{Erro: "rota nao existe"})
}

func (e *Estado) rotaSaude(w http.ResponseWriter) {
	e.Trava.Lock()
	valor := e.Relogio.Valor
	e.Trava.Unlock()
	responder(w, 200, map[string]any{
		"ok":      true,
		"empresa": e.Empresa,
		"regiao":  e.Regiao,
		"relogio": valor,
	})
}

func lerJSON(w http.ResponseWriter, r *http.Request, destino any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	return dec.Decode(destino)
}

func responder(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func codigoHTTP(resp Resposta) int {
	if resp.OK {
		return 200
	}
	erro := strings.ToLower(resp.Erro)
	if strings.Contains(erro, "senha") {
		return 401
	}
	if strings.Contains(erro, "nao existe") || strings.Contains(erro, "nao achei") {
		return 404
	}
	if strings.Contains(erro, "vaga") || strings.Contains(erro, "segurar") || strings.Contains(erro, "confirmar") || strings.Contains(erro, "expirou") {
		return 409
	}
	return 400
}

func novoServidor(e *Estado) *http.Server {
	return &http.Server{
		Handler:           e,
		ReadHeaderTimeout: 5 * time.Second,
	}
}

// Abrir so reserva a porta. Ainda nao atende.
// O teste precisa da URL antes de ligar os pares e o arquivo.
func Abrir(e *Estado, addr string) (net.Listener, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	if e.URLPropria == "" {
		e.URLPropria = "http://" + ln.Addr().String()
	}
	return ln, nil
}

func Comecar(e *Estado, ln net.Listener) *http.Server {
	srv := novoServidor(e)
	go func() {
		err := srv.Serve(ln)
		if err != nil && err != http.ErrServerClosed {
			fmt.Println("["+e.Empresa+"]", err)
		}
	}()
	return srv
}

func lerPares(texto string) ([]Par, error) {
	pares := []Par{}
	texto = strings.TrimSpace(texto)
	if texto == "" {
		return pares, nil
	}
	for _, parte := range strings.Split(texto, ",") {
		parte = strings.TrimSpace(parte)
		if parte == "" {
			continue
		}
		nome, url, ok := strings.Cut(parte, "|")
		if !ok {
			return nil, fmt.Errorf("PARES tem que ser nome|url,nome|url. recebi: %s", parte)
		}
		pares = append(pares, Par{
			Empresa: strings.TrimSpace(nome),
			URL:     strings.TrimRight(strings.TrimSpace(url), "/"),
		})
	}
	return pares, nil
}

func getenv(nome, padrao string) string {
	v := strings.TrimSpace(os.Getenv(nome))
	if v == "" {
		return padrao
	}
	return v
}

func Rodar() {
	empresa := getenv("EMPRESA", "vaijunto")
	regiao := getenv("REGIAO", "reconcavo")
	porta := getenv("PORTA", "5001")
	host := getenv("HOST", "0.0.0.0")
	urlPropria := strings.TrimRight(getenv("URL_PROPRIA", "http://127.0.0.1:"+porta), "/")
	arquivo := getenv("ARQUIVO", "dados/"+empresa+".json")
	expiraSeg, _ := strconv.Atoi(getenv("EXPIRA_SEGUNDOS", "20"))
	janelaMs, _ := strconv.Atoi(getenv("JANELA_MS", "250"))
	timeoutSeg, _ := strconv.Atoi(getenv("TIMEOUT_SEGUNDOS", "3"))
	if expiraSeg < 1 {
		expiraSeg = 20
	}
	if janelaMs < 0 {
		janelaMs = 250
	}
	if timeoutSeg < 1 {
		timeoutSeg = 3
	}
	pares, err := lerPares(os.Getenv("PARES"))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	e := NovoEstado(
		empresa, regiao, arquivo, urlPropria, pares,
		time.Duration(expiraSeg)*time.Second,
		time.Duration(janelaMs)*time.Millisecond,
		time.Duration(timeoutSeg)*time.Second,
	)
	err = e.CarregarOuSemear()
	if err != nil {
		fmt.Println("nao consegui abrir o arquivo:", err)
		os.Exit(1)
	}
	go LoopLimpeza(e)
	AoSubir(e)

	fmt.Printf("[%s] ouvindo em %s:%s\n", empresa, host, porta)
	fmt.Printf("[%s] regiao %s arquivo %s\n", empresa, regiao, arquivo)
	fmt.Printf("[%s] pares: %v\n", empresa, pares)
	ln, err := Abrir(e, host+":"+porta)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	srv := novoServidor(e)
	err = srv.Serve(ln)
	if err != nil && err != http.ErrServerClosed {
		fmt.Println(err)
		os.Exit(1)
	}
}
