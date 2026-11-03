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

// O if das rotas. O log e o metodo e o caminho, sem a query, pra senha nao aparecer.
func (e *Estado) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("[%s] %s %s\n", e.Empresa, r.Method, r.URL.Path)
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
		usuario, senha, ok := r.BasicAuth()
		if !ok {
			responder(w, 401, Resposta{Erro: "usuario ou senha invalido"})
			return
		}
		resp := e.ListarMinhasCaronas(usuario, senha)
		responder(w, codigoHTTP(resp), resp)
		return
	}
	if r.Method == http.MethodGet && caminho == "/passageiros" {
		usuario, senha, ok := r.BasicAuth()
		if !ok {
			responder(w, 401, Resposta{Erro: "usuario ou senha invalido"})
			return
		}
		resp, lista := e.ListarPassageiros(usuario, senha, r.URL.Query().Get("id_carona"))
		if !resp.OK {
			responder(w, codigoHTTP(resp), resp)
			return
		}
		responder(w, 200, map[string]any{"ok": true, "trechos": lista})
		return
	}
	if r.Method == http.MethodGet && caminho == "/minhas-reservas" {
		usuario, senha, ok := r.BasicAuth()
		if !ok {
			responder(w, 401, Resposta{Erro: "usuario ou senha invalido"})
			return
		}
		resp := e.ListarMinhasReservas(usuario, senha)
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
			Senha   string   `json:"senha"`
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
		resp := e.PublicarCarona(corpo.Usuario, corpo.Senha, corpo.Cidades, corpo.Data, corpo.Hora, corpo.Vagas, corpo.Precos)
		responder(w, codigoHTTP(resp), resp)
		return
	}
	if r.Method == http.MethodPost && caminho == "/cancelar-carona" {
		var corpo struct {
			Usuario  string `json:"usuario"`
			Senha    string `json:"senha"`
			IDCarona string `json:"id_carona"`
		}
		if err := lerJSON(w, r, &corpo); err != nil {
			responder(w, 400, Resposta{Erro: "json invalido"})
			return
		}
		resp := e.CancelarCarona(corpo.Usuario, corpo.Senha, corpo.IDCarona)
		responder(w, codigoHTTP(resp), resp)
		return
	}
	if r.Method == http.MethodPost && caminho == "/reservar" {
		var corpo struct {
			Usuario     string         `json:"usuario"`
			Senha       string         `json:"senha"`
			IDTentativa string         `json:"id_tentativa"`
			Trechos     []TrechoViagem `json:"trechos"`
		}
		if err := lerJSON(w, r, &corpo); err != nil {
			responder(w, 400, Resposta{Erro: "json invalido"})
			return
		}
		resp := e.Reservar(corpo.Usuario, corpo.Senha, corpo.IDTentativa, corpo.Trechos)
		responder(w, codigoHTTP(resp), resp)
		return
	}
	if r.Method == http.MethodPost && caminho == "/cancelar-reserva" {
		var corpo struct {
			Usuario   string `json:"usuario"`
			Senha     string `json:"senha"`
			IDReserva string `json:"id_reserva"`
		}
		if err := lerJSON(w, r, &corpo); err != nil {
			responder(w, 400, Resposta{Erro: "json invalido"})
			return
		}
		resp := e.CancelarReserva(corpo.Usuario, corpo.Senha, corpo.IDReserva)
		responder(w, codigoHTTP(resp), resp)
		return
	}
	if r.Method == http.MethodPost && caminho == "/consorcio/segurar" {
		quem, cod := e.quemAutenticou(r)
		if cod != 200 {
			responder(w, cod, Resposta{Erro: "usuario ou senha invalido"})
			return
		}
		var corpo CorpoSegurar
		if err := lerJSON(w, r, &corpo); err != nil {
			responder(w, 400, Resposta{Erro: "json invalido"})
			return
		}
		if quem != corpo.EmpresaCoord {
			responder(w, 403, Resposta{Erro: "so o coordenador deste pedido mexe nele"})
			return
		}
		ok, motivo := e.TratarSegurar(corpo)
		if ok && e.CortarAckSegurar {
			hj, pode := w.(http.Hijacker)
			if pode {
				conn, _, err := hj.Hijack()
				if err == nil {
					conn.Close()
					return
				}
			}
		}
		if ok {
			responder(w, 200, Resposta{OK: true, Mensagem: motivo})
			return
		}
		responder(w, codigoHTTP(Resposta{Erro: motivo}), Resposta{Erro: motivo})
		return
	}
	if r.Method == http.MethodPost && caminho == "/consorcio/confirmar" {
		quem, cod := e.quemAutenticou(r)
		if cod != 200 {
			responder(w, cod, Resposta{Erro: "usuario ou senha invalido"})
			return
		}
		var corpo corpoID
		if err := lerJSON(w, r, &corpo); err != nil {
			responder(w, 400, Resposta{Erro: "json invalido"})
			return
		}
		if !e.podeMexer(quem, corpo.IDPedido) {
			responder(w, 403, Resposta{Erro: "so o coordenador deste pedido mexe nele"})
			return
		}
		ok, motivo := e.TratarConfirmar(corpo.IDPedido, quem, corpo.Relogio)
		if ok && e.CortarAckConfirmar {
			hj, pode := w.(http.Hijacker)
			if pode {
				conn, _, err := hj.Hijack()
				if err == nil {
					conn.Close()
					return
				}
			}
		}
		if ok {
			responder(w, 200, Resposta{OK: true, Mensagem: motivo})
			return
		}
		responder(w, codigoHTTP(Resposta{Erro: motivo}), Resposta{Erro: motivo})
		return
	}
	if r.Method == http.MethodPost && (caminho == "/consorcio/desfazer" || caminho == "/consorcio/liberar") {
		quem, cod := e.quemAutenticou(r)
		if cod != 200 {
			responder(w, cod, Resposta{Erro: "usuario ou senha invalido"})
			return
		}
		var corpo corpoID
		if err := lerJSON(w, r, &corpo); err != nil {
			responder(w, 400, Resposta{Erro: "json invalido"})
			return
		}
		if !e.podeMexer(quem, corpo.IDPedido) {
			responder(w, 403, Resposta{Erro: "so o coordenador deste pedido mexe nele"})
			return
		}
		ok, motivo := e.TratarDesfazer(corpo.IDPedido, quem)
		if ok {
			responder(w, 200, Resposta{OK: true, Mensagem: motivo})
			return
		}
		responder(w, codigoHTTP(Resposta{Erro: motivo}), Resposta{Erro: motivo})
		return
	}

	responder(w, 404, Resposta{Erro: "rota nao existe"})
}

// Le o Basic. O usuario tem que ser uma empresa conhecida e a senha tem que ser SEGREDO.
// Sem cabecalho, segredo vazio ou senha de conta: 401.
func (e *Estado) quemAutenticou(r *http.Request) (string, int) {
	user, senha, ok := r.BasicAuth()
	if !ok || e.Segredo == "" || senha != e.Segredo || e.urlDa(user) == "" {
		return "", 401
	}
	return user, 200
}

// Se o pedido ja tem coordenadora gravada, so ela mexe.
// Sem seguro ainda, um par autenticado pode registrar o aborto.
// Chamada interna de recuperacao nao passa por aqui.
func (e *Estado) podeMexer(quem, idPedido string) bool {
	e.Trava.Lock()
	defer e.Trava.Unlock()
	s := e.Seguros[idPedido]
	if s == nil || s.EmpresaCoord == "" {
		return true
	}
	return quem == s.EmpresaCoord
}

// Devolve ok, o nome da empresa, a regiao e o relogio.
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

// Limita o corpo em 1 MB e decodifica. Lixo depois do JSON e erro.
func lerJSON(w http.ResponseWriter, r *http.Request, destino any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	err := dec.Decode(destino)
	if err != nil {
		return err
	}
	if dec.More() {
		return fmt.Errorf("json com lixo no final")
	}
	return nil
}

// Manda JSON em UTF-8 com o status.
func responder(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// 200 se deu certo. 401 se o texto fala senha. 404 se nao achei.
// 500 se nao conseguiu gravar. 409 se fala vaga, segurar, confirmar ou expirou.
// O resto e 400. Se a resposta ja trouxe um codigo, esse ganha. O 202 entra por ai.
func codigoHTTP(resp Resposta) int {
	if resp.Codigo != 0 {
		return resp.Codigo
	}
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
	if strings.Contains(erro, "gravar") {
		return 500
	}
	if strings.Contains(erro, "vaga") || strings.Contains(erro, "segurar") || strings.Contains(erro, "confirmar") || strings.Contains(erro, "expirou") {
		return 409
	}
	return 400
}

// Cria o servidor HTTP. Timeout de cabecalho: 5 segundos.
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

// Comeca a atender numa goroutine. O teste usa isso.
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

// Le a variavel PARES no formato nome|url,nome|url.
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

// Devolve o padrao se a variavel estiver vazia.
func getenv(nome, padrao string) string {
	v := strings.TrimSpace(os.Getenv(nome))
	if v == "" {
		return padrao
	}
	return v
}

// Le o ambiente, exige SEGREDO, carrega o arquivo, sobe a limpeza e escuta a porta.
// SEGREDO nao e a senha das contas. Sem ela o processo para.
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
	segredo := strings.TrimSpace(os.Getenv("SEGREDO"))
	if segredo == "" {
		fmt.Println("falta a variavel SEGREDO. ela nao e a senha das contas")
		os.Exit(1)
	}
	e.Segredo = segredo
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
