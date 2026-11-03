package interno

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	IDTentativa   string         `json:"id_tentativa,omitempty"`
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
	Codigo      int            `json:"-"`
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
	Trava              sync.Mutex
	Empresa            string
	Regiao             string
	Arquivo            string
	URLPropria         string
	Pares              []Par
	Expira             time.Duration
	Janela             time.Duration
	Timeout            time.Duration
	Relogio            Relogio
	Usuarios           map[string]Usuario
	Caronas            map[string]*Carona
	Reservas           map[string]*Reserva
	Seguros            map[string]*Seguro
	Pedidos            map[string]*Pedido
	Contador           int
	Ligado             bool
	CortarAckConfirmar bool            // o teste fecha a conexao depois de gravar o confirmar
	CortarAckSegurar   bool            // o teste fecha a conexao depois de gravar o segurar
	FalharSeFase       string          // a proxima gravacao desta fase falha uma vez. nao vai pro arquivo
	FalharSeSeguro     string          // a proxima gravacao com seguro neste estado falha uma vez. nao vai pro arquivo
	Segredo            string          // senha entre empresas. nao e a senha das contas. nao vai pro arquivo
	Abandonados        map[string]bool // ids cuja decisao nao entrou no disco. nao vai pro arquivo
}

// Monta a memoria vazia de uma empresa. Contador comeca em 1.
// Segredo fica vazio. Quem sobe o servidor preenche com SEGREDO.
func NovoEstado(empresa, regiao, arquivo, urlPropria string, pares []Par, expira, janela, timeout time.Duration) *Estado {
	if pares == nil {
		pares = []Par{}
	}
	return &Estado{
		Empresa:     empresa,
		Regiao:      regiao,
		Arquivo:     arquivo,
		URLPropria:  urlPropria,
		Pares:       pares,
		Expira:      expira,
		Janela:      janela,
		Timeout:     timeout,
		Usuarios:    map[string]Usuario{},
		Caronas:     map[string]*Carona{},
		Reservas:    map[string]*Reserva{},
		Seguros:     map[string]*Seguro{},
		Pedidos:     map[string]*Pedido{},
		Contador:    1,
		Ligado:      true,
		Abandonados: map[string]bool{},
	}
}

// Marca o processo pra parar. A limpeza olha isso e sai do laco.
func (e *Estado) Parar() {
	e.Trava.Lock()
	e.Ligado = false
	e.Trava.Unlock()
}

// Le a marca Ligado com a trava.
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
// Escreve .tmp, da Sync e troca o nome. Se morrer no meio, o arquivo antigo fica.
// Se a troca falhar, o .bak guarda o ultimo arquivo bom.
// Se FalharSeFase estiver preenchido e algum pedido ja esta nessa fase, esta gravacao falha uma vez.
// FalharSeSeguro faz o mesmo quando algum seguro ja esta naquele estado.
func (e *Estado) Salvar() error {
	if e.FalharSeFase != "" {
		for _, p := range e.Pedidos {
			if p.Fase == e.FalharSeFase {
				e.FalharSeFase = ""
				return fmt.Errorf("nao consegui gravar")
			}
		}
	}
	if e.FalharSeSeguro != "" {
		for _, s := range e.Seguros {
			if s.Estado == e.FalharSeSeguro {
				e.FalharSeSeguro = ""
				return fmt.Errorf("nao consegui gravar")
			}
		}
	}
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
		return err
	}
	tmp := e.Arquivo + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		fmt.Println("["+e.Empresa+"] erro ao criar arquivo:", err)
		return err
	}
	_, err = f.Write(bruto)
	if err == nil {
		err = f.Sync()
	}
	f.Close()
	if err != nil {
		fmt.Println("["+e.Empresa+"] erro ao gravar:", err)
		os.Remove(tmp)
		return err
	}
	err = os.Rename(tmp, e.Arquivo)
	if err != nil {
		bak := e.Arquivo + ".bak"
		if ren := os.Rename(e.Arquivo, bak); ren == nil {
			err = os.Rename(tmp, e.Arquivo)
			if err != nil {
				os.Rename(bak, e.Arquivo)
			} else {
				os.Remove(bak)
			}
		}
	}
	if err != nil {
		fmt.Println("["+e.Empresa+"] erro ao trocar arquivo:", err)
		return err
	}
	return nil
}

// Se o arquivo nao existe, semeia as caronas e grava.
// Se existe, le o JSON. Mapa nil no arquivo nao apaga o mapa vazio.
func (e *Estado) CarregarOuSemear() error {
	_, err := os.Stat(e.Arquivo)
	if os.IsNotExist(err) {
		e.semear()
		e.Trava.Lock()
		err = e.Salvar()
		e.Trava.Unlock()
		if err != nil {
			return err
		}
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

// Cria as tres contas da demo e as caronas desta empresa.
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

// Gera o id, um trecho por par de cidades, status ativa, e guarda no mapa.
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

// Confere usuario e senha. Nao grava nada.
func (e *Estado) Entrar(usuario, senha string) Resposta {
	e.Trava.Lock()
	defer e.Trava.Unlock()
	u, ok := e.Usuarios[usuario]
	if !ok || u.Senha != senha {
		return Resposta{Erro: "usuario ou senha invalido"}
	}
	return Resposta{OK: true, Usuario: usuario, Papel: u.Papel, Empresa: e.Empresa, Regiao: e.Regiao}
}

// Recusa nome vazio, papel errado e nome repetido. Grava, da Tic e salva.
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
	if err := e.Salvar(); err != nil {
		delete(e.Usuarios, usuario)
		return Resposta{Erro: "nao consegui gravar"}
	}
	return Resposta{OK: true, Usuario: usuario, Papel: papel}
}

// So motorista. Exige duas cidades, um preco por trecho e pelo menos uma vaga.
func (e *Estado) PublicarCarona(usuario, senha string, cidades []string, data, hora string, vagas int, precos []int) Resposta {
	if len(cidades) < 2 {
		return Resposta{Erro: "precisa de pelo menos duas cidades"}
	}
	for _, cidade := range cidades {
		if strings.TrimSpace(cidade) == "" {
			return Resposta{Erro: "cidade vazia"}
		}
	}
	if len(precos) != len(cidades)-1 {
		return Resposta{Erro: "um preco por trecho (quantidade de cidades menos 1)"}
	}
	for _, preco := range precos {
		if preco < 0 {
			return Resposta{Erro: "preco nao pode ser negativo"}
		}
	}
	if vagas < 1 {
		return Resposta{Erro: "vagas tem que ser pelo menos 1"}
	}
	if _, err := time.Parse("2006-01-02", data); err != nil {
		return Resposta{Erro: "data tem que ser aaaa-mm-dd"}
	}
	if _, err := time.Parse("15:04", hora); err != nil {
		return Resposta{Erro: "hora tem que ser hh:mm"}
	}
	e.Trava.Lock()
	defer e.Trava.Unlock()
	u, ok := e.Usuarios[usuario]
	if !ok || u.Senha != senha || u.Papel != "motorista" {
		return Resposta{Erro: "usuario ou senha invalido"}
	}
	id := e.criarCarona(usuario, cidades, data, hora, vagas, precos)
	e.Relogio.Tic()
	if err := e.Salvar(); err != nil {
		delete(e.Caronas, id)
		return Resposta{Erro: "nao consegui gravar"}
	}
	copia := copiarCarona(e.Caronas[id])
	return Resposta{OK: true, Carona: &copia}
}

// So o dono. Se ja tem passageiro confirmado nessa carona, recusa.
// Ja cancelada responde ok e nao mexe no arquivo.
// Se o disco falhar, o status volta ao que era. Nao desfaz reserva nas outras empresas.
func (e *Estado) CancelarCarona(usuario, senha, idCarona string) Resposta {
	e.Trava.Lock()
	defer e.Trava.Unlock()
	u, tem := e.Usuarios[usuario]
	if !tem || u.Senha != senha {
		return Resposta{Erro: "usuario ou senha invalido"}
	}
	c := e.Caronas[idCarona]
	if c == nil {
		return Resposta{Erro: "carona nao existe"}
	}
	if c.Motorista != usuario {
		return Resposta{Erro: "essa carona nao e sua"}
	}
	if c.Status == "cancelada" {
		copia := copiarCarona(c)
		return Resposta{OK: true, Carona: &copia}
	}
	for _, s := range e.Seguros {
		if s.Estado != "confirmado" && s.Estado != "segurado" {
			continue
		}
		for _, t := range s.Trechos {
			if t.IDCarona == idCarona {
				return Resposta{Erro: "tem passageiro nesta carona, nao cancelei"}
			}
		}
	}
	antes := c.Status
	c.Status = "cancelada"
	e.Relogio.Tic()
	if err := e.Salvar(); err != nil {
		c.Status = antes
		return Resposta{Erro: "nao consegui gravar"}
	}
	copia := copiarCarona(c)
	return Resposta{OK: true, Carona: &copia}
}

// So o motorista. Devolve copia das caronas dele.
func (e *Estado) ListarMinhasCaronas(usuario, senha string) Resposta {
	e.Trava.Lock()
	defer e.Trava.Unlock()
	u, ok := e.Usuarios[usuario]
	if !ok || u.Senha != senha || u.Papel != "motorista" {
		return Resposta{Erro: "usuario ou senha invalido"}
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

// So o dono da carona. Junta, por trecho, quem esta confirmado.
// O passageiro pode ser de outra empresa.
func (e *Estado) ListarPassageiros(usuario, senha, idCarona string) (Resposta, []trechoComGente) {
	e.Trava.Lock()
	defer e.Trava.Unlock()
	u, ok := e.Usuarios[usuario]
	if !ok || u.Senha != senha {
		return Resposta{Erro: "usuario ou senha invalido"}, nil
	}
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

// So o passageiro. Devolve copia das reservas dele.
func (e *Estado) ListarMinhasReservas(usuario, senha string) Resposta {
	e.Trava.Lock()
	defer e.Trava.Unlock()
	u, ok := e.Usuarios[usuario]
	if !ok || u.Senha != senha || u.Papel != "passageiro" {
		return Resposta{Erro: "usuario ou senha invalido"}
	}
	lista := make([]Reserva, 0)
	for _, r := range e.Reservas {
		if r.Passageiro != usuario {
			continue
		}
		copia := copiarReserva(r)
		pedido := e.Pedidos[r.IDPedido]
		if pedido != nil && pedido.Fase == "cancelando" {
			copia.Status = "cancelando"
		}
		lista = append(lista, copia)
	}
	return Resposta{OK: true, Reservas: lista}
}

// AcharTrecho devolve o trecho de verdade, dentro do map. trava na mao.
// Devolve o trecho de verdade se a carona existe e esta ativa.
// Quem chama ja tem a trava.
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

// VagasOcupadas conta seguro prometido (segurado). confirmado ja baixou vagas_livres.
// Promessa vencida continua contando ate a limpeza saber a decisao. trava na mao.
// Conta seguro segurado que ainda nao venceu, naquele trecho.
// Confirmado nao entra: a vaga ja saiu de VagasLivres.
func (e *Estado) VagasOcupadas(idCarona string, indice int) int {
	ocupadas := 0
	for _, s := range e.Seguros {
		if s.Estado != "segurado" {
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

// Lista os trechos ativos do dia.
// A vaga mostrada e a livre menos a ocupada. Se der negativo, mostra 0.
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

// Se o nome sou eu, devolve a URL propria. Senao procura nos pares.
// Vazio quer dizer que nao conheco essa empresa.
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

// Copia a carona. A resposta HTTP nao devolve o ponteiro interno.
func copiarCarona(c *Carona) Carona {
	copia := *c
	copia.Cidades = append([]string(nil), c.Cidades...)
	copia.Trechos = append([]TrechoCarona(nil), c.Trechos...)
	return copia
}

// Copia a reserva, pelo mesmo motivo da carona.
func copiarReserva(r *Reserva) Reserva {
	copia := *r
	copia.Trechos = append([]TrechoViagem(nil), r.Trechos...)
	return copia
}
