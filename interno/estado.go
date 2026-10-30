package interno

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Estado e a memoria de UMA empresa. Tres processos = tres Estados.
// Nao tem banco compartilhado. O arquivo JSON desta empresa nao e lido pelas outras.
//
// A trava so protege ESTE processo. No problema 1 uma trava global resolvia
// a reserva inteira. Aqui a reserva entre empresas esta em reserva.go.

type Usuario struct {
	Senha string `json:"senha"`
	Papel string `json:"papel"`
}

type TrechoCarona struct {
	Indice      int    `json:"indice"`
	Origem      string `json:"origem"`
	Destino     string `json:"destino"`
	Preco       int    `json:"preco"`
	VagasLivres int    `json:"vagas_livres"`
}

type Carona struct {
	ID        string         `json:"id"`
	Motorista string         `json:"motorista"`
	Cidades   []string       `json:"cidades"`
	Data      string         `json:"data"`
	Hora      string         `json:"hora"`
	Vagas     int            `json:"vagas"`
	Status    string         `json:"status"`
	Trechos   []TrechoCarona `json:"trechos"`
}

// TrechoViagem e o que a busca devolve e o que o passageiro manda de volta
// na reserva. Pode ser de outra empresa.
type TrechoViagem struct {
	Empresa   string `json:"empresa"`
	Regiao    string `json:"regiao,omitempty"`
	IDCarona  string `json:"id_carona"`
	Indice    int    `json:"indice"`
	Origem    string `json:"origem"`
	Destino   string `json:"destino"`
	Preco     int    `json:"preco"`
	Vagas     int    `json:"vagas"`
	Data      string `json:"data,omitempty"`
	Hora      string `json:"hora,omitempty"`
	Motorista string `json:"motorista,omitempty"`
}

type RefTrecho struct {
	IDCarona string `json:"id_carona"`
	Indice   int    `json:"indice"`
}

type Par struct {
	Empresa string `json:"empresa"`
	URL     string `json:"url"`
}

type Reserva struct {
	ID         string         `json:"id"`
	IDPedido   string         `json:"id_pedido"`
	Passageiro string         `json:"passageiro"`
	Status     string         `json:"status"`
	PrecoTotal int            `json:"preco_total"`
	Trechos    []TrechoViagem `json:"trechos"`
}

type Seguro struct {
	IDPedido      string      `json:"id_pedido"`
	Relogio       int         `json:"relogio"`
	EmpresaCoord  string      `json:"empresa_coord"`
	URLCoord      string      `json:"url_coord"`
	Passageiro    string      `json:"passageiro"`
	Participantes []Par       `json:"participantes"`
	Trechos       []RefTrecho `json:"trechos"`
	Estado        string      `json:"estado"`
	Expira        time.Time   `json:"expira"`
}

type Pedido struct {
	IDPedido      string         `json:"id_pedido"`
	IDReserva     string         `json:"id_reserva"`
	Fase          string         `json:"fase"`
	Passageiro    string         `json:"passageiro"`
	Relogio       int            `json:"relogio"`
	Trechos       []TrechoViagem `json:"trechos"`
	Participantes []Par          `json:"participantes"`
}

type Itinerario struct {
	PrecoTotal int            `json:"preco_total"`
	Trocas     int            `json:"trocas"`
	Trechos    []TrechoViagem `json:"trechos"`
}

type Resposta struct {
	OK          bool           `json:"ok"`
	Erro        string         `json:"erro,omitempty"`
	Mensagem    string         `json:"mensagem,omitempty"`
	Empresa     string         `json:"empresa,omitempty"`
	Regiao      string         `json:"regiao,omitempty"`
	Usuario     string         `json:"usuario,omitempty"`
	Papel       string         `json:"papel,omitempty"`
	Itinerarios []Itinerario   `json:"itinerarios,omitempty"`
	Reserva     *Reserva       `json:"reserva,omitempty"`
	Reservas    []Reserva      `json:"reservas,omitempty"`
	Caronas     []Carona       `json:"caronas,omitempty"`
	Carona      *Carona        `json:"carona,omitempty"`
	Trechos     []TrechoViagem `json:"trechos,omitempty"`
}

// o que vai pro disco. a trava e a lista de pares nao entram:
// par e configuracao (IP do lab), nao estado da carona.
type gravacao struct {
	Usuarios map[string]Usuario  `json:"usuarios"`
	Caronas  map[string]*Carona  `json:"caronas"`
	Reservas map[string]*Reserva `json:"reservas"`
	Seguros  map[string]*Seguro  `json:"seguros"`
	Pedidos  map[string]*Pedido  `json:"pedidos"`
	Contador int                 `json:"contador"`
	Relogio  int                 `json:"relogio"`
}

type Estado struct {
	Trava      sync.Mutex
	Empresa    string
	Regiao     string
	Arquivo    string
	URLPropria string
	Pares      []Par
	Expira     time.Duration
	Janela     time.Duration
	Timeout    time.Duration
	Relogio    Relogio
	Usuarios   map[string]Usuario
	Caronas    map[string]*Carona
	Reservas   map[string]*Reserva
	Seguros    map[string]*Seguro
	Pedidos    map[string]*Pedido
	Contador   int
	Ligado     bool
}

func NovoEstado(empresa, regiao, arquivo, urlPropria string, pares []Par, expira, janela, timeout time.Duration) *Estado {
	if pares == nil {
		pares = []Par{}
	}
	return &Estado{
		Empresa:    empresa,
		Regiao:     regiao,
		Arquivo:    arquivo,
		URLPropria: urlPropria,
		Pares:      pares,
		Expira:     expira,
		Janela:     janela,
		Timeout:    timeout,
		Usuarios:   map[string]Usuario{},
		Caronas:    map[string]*Carona{},
		Reservas:   map[string]*Reserva{},
		Seguros:    map[string]*Seguro{},
		Pedidos:    map[string]*Pedido{},
		Contador:   1,
		Ligado:     true,
	}
}

func (e *Estado) Parar() {
	e.Trava.Lock()
	e.Ligado = false
	e.Trava.Unlock()
}

func (e *Estado) ligado() bool {
	e.Trava.Lock()
	defer e.Trava.Unlock()
	return e.Ligado
}

// ProximoID assume a trava na mao.
func (e *Estado) ProximoID(prefixo string) string {
	n := e.Contador
	e.Contador = n + 1
	return fmt.Sprintf("%s-%s-%d", prefixo, e.Empresa, n)
}

// Salvar assume a trava na mao.
// Escreve .tmp e troca o nome. Se morrer no meio do write, o arquivo antigo fica.
func (e *Estado) Salvar() {
	g := gravacao{
		Usuarios: e.Usuarios,
		Caronas:  e.Caronas,
		Reservas: e.Reservas,
		Seguros:  e.Seguros,
		Pedidos:  e.Pedidos,
		Contador: e.Contador,
		Relogio:  e.Relogio.Valor,
	}
	pasta := filepath.Dir(e.Arquivo)
	if pasta != "" && pasta != "." {
		os.MkdirAll(pasta, 0755)
	}
	bruto, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		fmt.Println("["+e.Empresa+"] erro ao montar json:", err)
		return
	}
	tmp := e.Arquivo + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		fmt.Println("["+e.Empresa+"] erro ao criar arquivo:", err)
		return
	}
	_, err = f.Write(bruto)
	if err == nil {
		err = f.Sync()
	}
	f.Close()
	if err != nil {
		fmt.Println("["+e.Empresa+"] erro ao gravar:", err)
		return
	}
	err = os.Rename(tmp, e.Arquivo)
	if err != nil {
		// no Windows o rename as vezes nao cobre arquivo que ja existe
		os.Remove(e.Arquivo)
		err = os.Rename(tmp, e.Arquivo)
	}
	if err != nil {
		fmt.Println("["+e.Empresa+"] erro ao trocar arquivo:", err)
	}
}

func (e *Estado) CarregarOuSemear() error {
	_, err := os.Stat(e.Arquivo)
	if os.IsNotExist(err) {
		e.semear()
		e.Trava.Lock()
		e.Salvar()
		e.Trava.Unlock()
		fmt.Println("[" + e.Empresa + "] arquivo novo em " + e.Arquivo)
		return nil
	}
	if err != nil {
		return err
	}
	bruto, err := os.ReadFile(e.Arquivo)
	if err != nil {
		return err
	}
	var g gravacao
	err = json.Unmarshal(bruto, &g)
	if err != nil {
		return err
	}
	e.Trava.Lock()
	if g.Usuarios != nil {
		e.Usuarios = g.Usuarios
	}
	if g.Caronas != nil {
		e.Caronas = g.Caronas
	}
	if g.Reservas != nil {
		e.Reservas = g.Reservas
	}
	if g.Seguros != nil {
		e.Seguros = g.Seguros
	}
	if g.Pedidos != nil {
		e.Pedidos = g.Pedidos
	}
	if g.Contador > 0 {
		e.Contador = g.Contador
	}
	e.Relogio.Valor = g.Relogio
	e.Trava.Unlock()
	fmt.Println("[" + e.Empresa + "] carreguei " + e.Arquivo)
	return nil
}

func (e *Estado) semear() {
	// mesma conta nas tres empresas, mas sao listas separadas.
	// senha em texto. o problema 1 tambem era. nao e sistema de banco.
	e.Usuarios["motorista1"] = Usuario{Senha: "senha123", Papel: "motorista"}
	e.Usuarios["passageiro1"] = Usuario{Senha: "senha123", Papel: "passageiro"}
	e.Usuarios["passageiro2"] = Usuario{Senha: "senha123", Papel: "passageiro"}

	// fecha o exemplo do enunciado. a ultima tem 1 vaga de proposito.
	if e.Empresa == "vaijunto" {
		e.criarCarona("motorista1", []string{"Salvador", "Feira de Santana"}, "2026-10-30", "08:00", 3, []int{25})
		e.criarCarona("motorista1", []string{"Salvador", "Cachoeira"}, "2026-10-30", "09:00", 2, []int{15})
	} else if e.Empresa == "sudoeste" {
		e.criarCarona("motorista1", []string{"Feira de Santana", "Vitória da Conquista"}, "2026-10-30", "11:00", 3, []int{40})
	} else if e.Empresa == "norteminas" {
		e.criarCarona("motorista1", []string{"Vitória da Conquista", "Montes Claros"}, "2026-10-30", "15:00", 1, []int{55})
	}
}

func (e *Estado) criarCarona(motorista string, cidades []string, data, hora string, vagas int, precos []int) string {
	id := e.ProximoID("carona")
	trechos := make([]TrechoCarona, 0)
	for i := 0; i < len(cidades)-1; i++ {
		trechos = append(trechos, TrechoCarona{
			Indice:      i,
			Origem:      cidades[i],
			Destino:     cidades[i+1],
			Preco:       precos[i],
			VagasLivres: vagas,
		})
	}
	e.Caronas[id] = &Carona{
		ID:        id,
		Motorista: motorista,
		Cidades:   cidades,
		Data:      data,
		Hora:      hora,
		Vagas:     vagas,
		Status:    "ativa",
		Trechos:   trechos,
	}
	return id
}

func (e *Estado) Entrar(usuario, senha string) Resposta {
	e.Trava.Lock()
	defer e.Trava.Unlock()
	u, ok := e.Usuarios[usuario]
	if !ok || u.Senha != senha {
		return Resposta{Erro: "usuario ou senha invalido"}
	}
	return Resposta{OK: true, Usuario: usuario, Papel: u.Papel, Empresa: e.Empresa, Regiao: e.Regiao}
}

func (e *Estado) Cadastrar(usuario, senha, papel string) Resposta {
	if usuario == "" || senha == "" {
		return Resposta{Erro: "usuario e senha obrigatorios"}
	}
	if papel != "motorista" && papel != "passageiro" {
		return Resposta{Erro: "papel tem que ser motorista ou passageiro"}
	}
	e.Trava.Lock()
	defer e.Trava.Unlock()
	if _, ok := e.Usuarios[usuario]; ok {
		return Resposta{Erro: "usuario ja existe nesta empresa"}
	}
	e.Usuarios[usuario] = Usuario{Senha: senha, Papel: papel}
	e.Relogio.Tic()
	e.Salvar()
	return Resposta{OK: true, Usuario: usuario, Papel: papel}
}

func (e *Estado) PublicarCarona(usuario string, cidades []string, data, hora string, vagas int, precos []int) Resposta {
	if len(cidades) < 2 {
		return Resposta{Erro: "precisa de pelo menos duas cidades"}
	}
	if len(precos) != len(cidades)-1 {
		return Resposta{Erro: "um preco por trecho (quantidade de cidades menos 1)"}
	}
	if vagas < 1 {
		return Resposta{Erro: "vagas tem que ser pelo menos 1"}
	}
	e.Trava.Lock()
	defer e.Trava.Unlock()
	u, ok := e.Usuarios[usuario]
	if !ok || u.Papel != "motorista" {
		return Resposta{Erro: "so motorista publica carona"}
	}
	id := e.criarCarona(usuario, cidades, data, hora, vagas, precos)
	e.Relogio.Tic()
	e.Salvar()
	copia := copiarCarona(e.Caronas[id])
	return Resposta{OK: true, Carona: &copia}
}

func (e *Estado) CancelarCarona(usuario, idCarona string) Resposta {
	e.Trava.Lock()
	defer e.Trava.Unlock()
	c := e.Caronas[idCarona]
	if c == nil {
		return Resposta{Erro: "carona nao existe"}
	}
	if c.Motorista != usuario {
		return Resposta{Erro: "essa carona nao e sua"}
	}
	for _, s := range e.Seguros {
		if s.Estado != "confirmado" {
			continue
		}
		for _, t := range s.Trechos {
			if t.IDCarona == idCarona {
				return Resposta{Erro: "tem passageiro confirmado, nao cancelei"}
			}
		}
	}
	c.Status = "cancelada"
	e.Relogio.Tic()
	e.Salvar()
	return Resposta{OK: true}
}

func (e *Estado) ListarMinhasCaronas(usuario string) Resposta {
	e.Trava.Lock()
	defer e.Trava.Unlock()
	u, ok := e.Usuarios[usuario]
	if !ok || u.Papel != "motorista" {
		return Resposta{Erro: "so motorista lista as proprias caronas"}
	}
	lista := make([]Carona, 0)
	for _, c := range e.Caronas {
		if c.Motorista == usuario {
			lista = append(lista, copiarCarona(c))
		}
	}
	return Resposta{OK: true, Caronas: lista}
}

type gente struct {
	Passageiro string `json:"passageiro"`
	Empresa    string `json:"empresa"`
}

type trechoComGente struct {
	Indice      int     `json:"indice"`
	Origem      string  `json:"origem"`
	Destino     string  `json:"destino"`
	VagasLivres int     `json:"vagas_livres"`
	Passageiros []gente `json:"passageiros"`
}

func (e *Estado) ListarPassageiros(usuario, idCarona string) (Resposta, []trechoComGente) {
	e.Trava.Lock()
	defer e.Trava.Unlock()
	c := e.Caronas[idCarona]
	if c == nil {
		return Resposta{Erro: "carona nao existe"}, nil
	}
	if c.Motorista != usuario {
		return Resposta{Erro: "essa carona nao e sua"}, nil
	}
	saida := make([]trechoComGente, 0)
	for _, t := range c.Trechos {
		nomes := make([]gente, 0)
		for _, s := range e.Seguros {
			if s.Estado != "confirmado" {
				continue
			}
			for _, st := range s.Trechos {
				if st.IDCarona == idCarona && st.Indice == t.Indice {
					nomes = append(nomes, gente{Passageiro: s.Passageiro, Empresa: s.EmpresaCoord})
				}
			}
		}
		saida = append(saida, trechoComGente{
			Indice:      t.Indice,
			Origem:      t.Origem,
			Destino:     t.Destino,
			VagasLivres: t.VagasLivres,
			Passageiros: nomes,
		})
	}
	return Resposta{OK: true}, saida
}

func (e *Estado) ListarMinhasReservas(usuario string) Resposta {
	e.Trava.Lock()
	defer e.Trava.Unlock()
	u, ok := e.Usuarios[usuario]
	if !ok || u.Papel != "passageiro" {
		return Resposta{Erro: "so passageiro lista reserva"}
	}
	lista := make([]Reserva, 0)
	for _, r := range e.Reservas {
		if r.Passageiro == usuario {
			lista = append(lista, copiarReserva(r))
		}
	}
	return Resposta{OK: true, Reservas: lista}
}

// AcharTrecho devolve o trecho de verdade, dentro do map. trava na mao.
func (e *Estado) AcharTrecho(idCarona string, indice int) *TrechoCarona {
	c := e.Caronas[idCarona]
	if c == nil || c.Status != "ativa" {
		return nil
	}
	for i := range c.Trechos {
		if c.Trechos[i].Indice == indice {
			return &c.Trechos[i]
		}
	}
	return nil
}

// VagasOcupadas conta seguro ainda valendo. confirmado ja baixou vagas_livres,
// entao nao conta de novo. trava na mao.
func (e *Estado) VagasOcupadas(idCarona string, indice int) int {
	agora := time.Now()
	ocupadas := 0
	for _, s := range e.Seguros {
		if s.Estado != "segurado" {
			continue
		}
		if s.Expira.Before(agora) {
			continue
		}
		for _, t := range s.Trechos {
			if t.IDCarona == idCarona && t.Indice == indice {
				ocupadas = ocupadas + 1
			}
		}
	}
	return ocupadas
}

func (e *Estado) TrechosPublicos(data string) []TrechoViagem {
	e.Trava.Lock()
	defer e.Trava.Unlock()
	lista := make([]TrechoViagem, 0)
	for _, c := range e.Caronas {
		if c.Status != "ativa" {
			continue
		}
		if data != "" && c.Data != data {
			continue
		}
		for _, t := range c.Trechos {
			livres := t.VagasLivres - e.VagasOcupadas(c.ID, t.Indice)
			if livres < 0 {
				livres = 0
			}
			lista = append(lista, TrechoViagem{
				Empresa:   e.Empresa,
				Regiao:    e.Regiao,
				IDCarona:  c.ID,
				Indice:    t.Indice,
				Origem:    t.Origem,
				Destino:   t.Destino,
				Preco:     t.Preco,
				Vagas:     livres,
				Data:      c.Data,
				Hora:      c.Hora,
				Motorista: c.Motorista,
			})
		}
	}
	return lista
}

func (e *Estado) urlDa(nome string) string {
	if nome == e.Empresa {
		return e.URLPropria
	}
	for _, p := range e.Pares {
		if p.Empresa == nome {
			return p.URL
		}
	}
	return ""
}

func copiarCarona(c *Carona) Carona {
	copia := *c
	copia.Cidades = append([]string(nil), c.Cidades...)
	copia.Trechos = append([]TrechoCarona(nil), c.Trechos...)
	return copia
}

func copiarReserva(r *Reserva) Reserva {
	copia := *r
	copia.Trechos = append([]TrechoViagem(nil), r.Trechos...)
	return copia
}
