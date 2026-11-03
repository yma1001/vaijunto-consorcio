package interno

import (
	"fmt"
	"sort"
	"strconv"
	"time"
)

// Reserva entre empresas.
//
// Fase 1, SEGURAR, na ordem alfabetica do nome da empresa.
// Se uma negar ou cair, desfaz o que as outras ja tinham segurado.
// Segurar NAO baixa vagas_livres. So marca.
//
// Fase 2, CONFIRMAR. So mando depois que todas seguraram.
// Gravo "confirmando" antes de mandar. Se a resposta se perder,
// repito o confirmar. Quem ja baixou a vaga responde de novo sem baixar outra.
// Timeout nao e conflito: fico em "confirmando" e tento de novo.
// Depois que gravei "confirmando", nao aborto mais. So repito o confirmar.
// Voto ja segurado nao olha o prazo. O prazo so vale enquanto esta esperando.
// Se eu nao conseguir gravar a decisao, marco o pedido e aborto quando o disco voltar.
// Nao mando desfazer antes de gravar esse aborto.
//
// A ordem alfabetica evita deadlock: todo mundo pede norteminas,
// depois sudoeste, depois vaijunto. Ninguem segura A esperando B
// enquanto o outro segura B esperando A.
//
// Ultima vaga: o dono espera a janela (sem a trava na mao, senao o
// segundo pedido nem entra) e ordena por (relogio, empresa, id).
// Menor ganha. Nao e quem o socket leu primeiro.

type CorpoSegurar struct {
	IDPedido      string      `json:"id_pedido"`
	Relogio       int         `json:"relogio"`
	EmpresaCoord  string      `json:"empresa_coord"`
	URLCoord      string      `json:"url_coord"`
	Passageiro    string      `json:"passageiro"`
	Participantes []Par       `json:"participantes"`
	Trechos       []RefTrecho `json:"trechos"`
}

type corpoID struct {
	IDPedido string `json:"id_pedido"`
	Relogio  int    `json:"relogio,omitempty"`
	Empresa  string `json:"empresa"`
}

type grupo struct {
	empresa string
	trechos []TrechoViagem
}

func validarCadeia(trechos []TrechoViagem) string {
	if len(trechos) == 0 {
		return "itinerario vazio"
	}
	for i, t := range trechos {
		if t.Empresa == "" || t.IDCarona == "" || t.Origem == "" || t.Destino == "" {
			return "trecho incompleto"
		}
		if i > 0 {
			antes := trechos[i-1]
			if NormalizarCidade(antes.Destino) != NormalizarCidade(t.Origem) {
				return "os trechos nao conectam (" + antes.Destino + " -> " + t.Origem + ")"
			}
			if trechos[0].Data != "" && t.Data != trechos[0].Data {
				return "os trechos nao sao do mesmo dia"
			}
		}
	}
	return ""
}

func agruparPorEmpresa(trechos []TrechoViagem) []grupo {
	porNome := map[string][]TrechoViagem{}
	for _, t := range trechos {
		porNome[t.Empresa] = append(porNome[t.Empresa], t)
	}
	nomes := make([]string, 0, len(porNome))
	for nome := range porNome {
		nomes = append(nomes, nome)
	}
	sort.Strings(nomes)
	saida := make([]grupo, 0, len(nomes))
	for _, nome := range nomes {
		saida = append(saida, grupo{empresa: nome, trechos: porNome[nome]})
	}
	return saida
}

func (e *Estado) decidirQuemFicaComAVaga() {
	esperando := make([]*Seguro, 0)
	for _, s := range e.Seguros {
		if s.Estado == "esperando" {
			esperando = append(esperando, s)
		}
	}
	if len(esperando) == 0 {
		return
	}
	sort.Slice(esperando, func(i, j int) bool {
		a := esperando[i]
		b := esperando[j]
		if a.Relogio != b.Relogio {
			return a.Relogio < b.Relogio
		}
		if a.EmpresaCoord != b.EmpresaCoord {
			return a.EmpresaCoord < b.EmpresaCoord
		}
		return a.IDPedido < b.IDPedido
	})

	// marco o vencedor como segurado na hora. o proximo da fila ja ve essa vaga ocupada.
	// se eu somasse num contador E tambem marcasse, contaria duas vezes.
	for _, s := range esperando {
		cabe := true
		for _, t := range s.Trechos {
			trecho := e.AcharTrecho(t.IDCarona, t.Indice)
			if trecho == nil {
				cabe = false
				break
			}
			ja := e.VagasOcupadas(t.IDCarona, t.Indice)
			if trecho.VagasLivres-ja < 1 {
				cabe = false
				break
			}
		}
		if cabe {
			s.Estado = "segurado"
			s.Expira = time.Now().Add(e.Expira)
		} else {
			s.Estado = "negado"
		}
	}
}

func (e *Estado) TratarSegurar(c CorpoSegurar) (bool, string) {
	if c.IDPedido == "" {
		return false, "faltou id do pedido"
	}
	if len(c.Trechos) == 0 {
		return false, "faltou trecho"
	}

	e.Trava.Lock()
	if c.Relogio > 0 {
		e.Relogio.Receber(c.Relogio)
	}
	ja := e.Seguros[c.IDPedido]
	if ja != nil && (ja.Estado == "segurado" || ja.Estado == "confirmado") {
		e.Trava.Unlock()
		return true, "ja estava comigo"
	}
	if ja != nil && ja.Estado == "negado" {
		e.Trava.Unlock()
		return false, "sem vaga"
	}
	if ja != nil && ja.Estado == "abortado" {
		e.Trava.Unlock()
		return false, "pedido ja abortado"
	}
	urlCoord := e.urlDa(c.EmpresaCoord)
	if c.EmpresaCoord == "" || urlCoord == "" {
		e.Trava.Unlock()
		return false, "nao conheco o coordenador"
	}
	c.URLCoord = urlCoord
	vistos := map[string]bool{}
	for _, t := range c.Trechos {
		chave := t.IDCarona + "|" + strconv.Itoa(t.Indice)
		if vistos[chave] {
			e.Trava.Unlock()
			return false, "trecho repetido"
		}
		vistos[chave] = true
	}
	for _, t := range c.Trechos {
		if e.AcharTrecho(t.IDCarona, t.Indice) == nil {
			e.Trava.Unlock()
			return false, "trecho nao existe aqui"
		}
	}
	if ja == nil {
		e.Seguros[c.IDPedido] = &Seguro{
			IDPedido:      c.IDPedido,
			Relogio:       c.Relogio,
			EmpresaCoord:  c.EmpresaCoord,
			URLCoord:      c.URLCoord,
			Passageiro:    c.Passageiro,
			Participantes: c.Participantes,
			Trechos:       c.Trechos,
			Estado:        "esperando",
			Expira:        time.Now().Add(e.Expira),
		}
		if err := e.Salvar(); err != nil {
			delete(e.Seguros, c.IDPedido)
			e.Trava.Unlock()
			return false, "nao consegui gravar"
		}
	}
	e.Trava.Unlock()

	// soltei a trava antes de dormir. se dormisse com ela, o outro pedido
	// ficaria parado la fora e a janela decidiria sozinha.
	time.Sleep(e.Janela)

	e.Trava.Lock()
	defer e.Trava.Unlock()
	e.decidirQuemFicaComAVaga()
	seguro := e.Seguros[c.IDPedido]
	if seguro == nil || seguro.Estado != "segurado" {
		if seguro != nil && seguro.Estado == "negado" {
			delete(e.Seguros, c.IDPedido)
			e.Salvar()
		}
		return false, "sem vaga"
	}
	seguro.Expira = time.Now().Add(e.Expira)
	if err := e.Salvar(); err != nil {
		return false, "nao consegui gravar"
	}
	return true, "ok"
}

func (e *Estado) TratarConfirmar(idPedido, empresa string, relogio int) (bool, string) {
	if idPedido == "" {
		return false, "faltou id do pedido"
	}
	e.Trava.Lock()
	defer e.Trava.Unlock()
	if e.urlDa(empresa) == "" {
		return false, "nao conheco a empresa " + empresa
	}
	if relogio > 0 {
		e.Relogio.Receber(relogio)
	}
	s := e.Seguros[idPedido]
	if s == nil {
		return false, "pedido desconhecido"
	}
	if s.Estado == "confirmado" {
		// repetiu. nao baixo a vaga de novo.
		return true, "ja confirmado"
	}
	if s.Estado != "segurado" {
		return false, "nao estava segurado"
	}
	vistos := map[string]bool{}
	for _, t := range s.Trechos {
		chave := t.IDCarona + "|" + strconv.Itoa(t.Indice)
		if vistos[chave] {
			return false, "trecho repetido"
		}
		vistos[chave] = true
		trecho := e.AcharTrecho(t.IDCarona, t.Indice)
		if trecho == nil || trecho.VagasLivres < 1 {
			return false, "sem vaga na confirmacao"
		}
	}
	for _, t := range s.Trechos {
		trecho := e.AcharTrecho(t.IDCarona, t.Indice)
		trecho.VagasLivres = trecho.VagasLivres - 1
	}
	s.Estado = "confirmado"
	s.Expira = time.Time{}
	e.Relogio.Tic()
	if err := e.Salvar(); err != nil {
		for _, t := range s.Trechos {
			trecho := e.AcharTrecho(t.IDCarona, t.Indice)
			if trecho != nil {
				trecho.VagasLivres = trecho.VagasLivres + 1
			}
		}
		s.Estado = "segurado"
		return false, "nao consegui gravar"
	}
	return true, "ok"
}

func (e *Estado) TratarDesfazer(idPedido, empresa string) (bool, string) {
	if idPedido == "" {
		return false, "faltou id do pedido"
	}
	e.Trava.Lock()
	defer e.Trava.Unlock()
	if e.urlDa(empresa) == "" {
		return false, "nao conheco a empresa " + empresa
	}
	s := e.Seguros[idPedido]
	if s != nil && s.Estado == "abortado" {
		return true, "nada a desfazer"
	}
	if s == nil {
		e.Seguros[idPedido] = &Seguro{IDPedido: idPedido, Estado: "abortado"}
		if err := e.Salvar(); err != nil {
			delete(e.Seguros, idPedido)
			return false, "nao consegui gravar"
		}
		return true, "nada a desfazer"
	}
	eraConfirmado := s.Estado == "confirmado"
	if eraConfirmado {
		for _, t := range s.Trechos {
			trecho := e.AcharTrecho(t.IDCarona, t.Indice)
			if trecho == nil {
				continue
			}
			trecho.VagasLivres = trecho.VagasLivres + 1
			carona := e.Caronas[t.IDCarona]
			if carona != nil && trecho.VagasLivres > carona.Vagas {
				trecho.VagasLivres = carona.Vagas
			}
		}
	}
	antes := s.Estado
	s.Estado = "abortado"
	e.Relogio.Tic()
	if err := e.Salvar(); err != nil {
		if eraConfirmado {
			for _, t := range s.Trechos {
				trecho := e.AcharTrecho(t.IDCarona, t.Indice)
				if trecho != nil && trecho.VagasLivres > 0 {
					trecho.VagasLivres = trecho.VagasLivres - 1
				}
			}
		}
		s.Estado = antes
		return false, "nao consegui gravar"
	}
	return true, "ok"
}

func (e *Estado) ConsultarPedido(idPedido string) string {
	e.Trava.Lock()
	defer e.Trava.Unlock()
	p := e.Pedidos[idPedido]
	if p != nil {
		if p.Fase == "confirmada" || p.Fase == "confirmando" {
			return "confirmado"
		}
		if p.Fase == "abortada" || p.Fase == "abortando" || p.Fase == "cancelada" || p.Fase == "cancelando" {
			return "abortado"
		}
		return "pendente"
	}
	s := e.Seguros[idPedido]
	if s == nil {
		return "desconhecido"
	}
	if s.Estado == "confirmado" {
		return "confirmado"
	}
	if s.Estado == "abortado" || s.Estado == "negado" {
		return "abortado"
	}
	return "segurado"
}

func (e *Estado) segurarNa(empresa string, c CorpoSegurar) (bool, bool, string) {
	if empresa == e.Empresa {
		ok, motivo := e.TratarSegurar(c)
		return ok, false, motivo
	}
	url := e.urlDa(empresa)
	if url == "" {
		return false, false, "nao conheco a empresa " + empresa
	}
	var resp Resposta
	_, erroRede := PedirComo("POST", url+"/consorcio/segurar", e.Empresa, e.Segredo, c, e.Timeout, &resp)
	if erroRede {
		return false, true, "sem resposta de " + empresa
	}
	if !resp.OK {
		if resp.Erro == "" {
			resp.Erro = "falhou em " + empresa
		}
		return false, false, resp.Erro
	}
	return true, false, "ok"
}

func (e *Estado) confirmarNa(empresa, idPedido string, relogio int) (bool, bool, string) {
	if empresa == e.Empresa {
		ok, motivo := e.TratarConfirmar(idPedido, e.Empresa, relogio)
		return ok, false, motivo
	}
	url := e.urlDa(empresa)
	if url == "" {
		return false, false, "nao conheco a empresa " + empresa
	}
	var resp Resposta
	_, erroRede := PedirComo("POST", url+"/consorcio/confirmar", e.Empresa, e.Segredo, corpoID{IDPedido: idPedido, Relogio: relogio, Empresa: e.Empresa}, e.Timeout, &resp)
	if erroRede {
		return false, true, "sem resposta de " + empresa
	}
	if !resp.OK {
		if resp.Erro == "" {
			resp.Erro = "falhou em " + empresa
		}
		return false, false, resp.Erro
	}
	return true, false, "ok"
}

func (e *Estado) desfazerNa(empresa, idPedido string) (bool, bool, string) {
	if empresa == e.Empresa {
		ok, motivo := e.TratarDesfazer(idPedido, e.Empresa)
		return ok, false, motivo
	}
	url := e.urlDa(empresa)
	if url == "" {
		return false, false, "nao conheco a empresa " + empresa
	}
	var resp Resposta
	_, erroRede := PedirComo("POST", url+"/consorcio/desfazer", e.Empresa, e.Segredo, corpoID{IDPedido: idPedido, Empresa: e.Empresa}, e.Timeout, &resp)
	if erroRede {
		return false, true, "sem resposta de " + empresa
	}
	if !resp.OK {
		if resp.Erro == "" {
			resp.Erro = "falhou em " + empresa
		}
		return false, false, resp.Erro
	}
	return true, false, "ok"
}

func (e *Estado) gravarFase(id, nova string) error {
	pedido := e.Pedidos[id]
	if pedido == nil {
		return fmt.Errorf("pedido sumiu")
	}
	antes := pedido.Fase
	pedido.Fase = nova
	if err := e.Salvar(); err != nil {
		pedido.Fase = antes
		return err
	}
	return nil
}

func (e *Estado) AbortarPedido(idPedido string) {
	e.Trava.Lock()
	pedido := e.Pedidos[idPedido]
	if pedido == nil || pedido.Fase == "abortada" || pedido.Fase == "confirmada" || pedido.Fase == "confirmando" || pedido.Fase == "cancelada" {
		e.Trava.Unlock()
		return
	}
	participantes := append([]Par(nil), pedido.Participantes...)
	err := e.gravarFase(idPedido, "abortando")
	e.Trava.Unlock()
	if err != nil {
		return
	}

	for _, p := range participantes {
		ok, rede, motivo := e.desfazerNa(p.Empresa, idPedido)
		if !ok || rede {
			fmt.Printf("[%s] desfazer de %s ainda pendente: %s\n", e.Empresa, p.Empresa, motivo)
			return
		}
	}

	e.Trava.Lock()
	err = e.gravarFase(idPedido, "abortada")
	e.Trava.Unlock()
	if err != nil {
		fmt.Printf("[%s] nao gravei abortada de %s\n", e.Empresa, idPedido)
	}
}

func (e *Estado) entregarConfirmacao(idPedido string) (bool, string) {
	e.Trava.Lock()
	pedido := e.Pedidos[idPedido]
	if pedido == nil {
		e.Trava.Unlock()
		return false, "pedido sumiu"
	}
	if pedido.Fase == "confirmada" {
		e.Trava.Unlock()
		return true, "ok"
	}
	if pedido.Fase != "confirmando" {
		e.Trava.Unlock()
		return false, "nao estou confirmando"
	}
	partes := append([]Par(nil), pedido.Participantes...)
	relogio := pedido.Relogio
	e.Trava.Unlock()

	for _, p := range partes {
		ok, rede, motivo := e.confirmarNa(p.Empresa, idPedido, relogio)
		if rede {
			return false, "sem resposta de " + p.Empresa
		}
		if !ok {
			return false, motivo
		}
	}

	e.Trava.Lock()
	pedido = e.Pedidos[idPedido]
	if pedido == nil || pedido.Fase != "confirmando" {
		e.Trava.Unlock()
		return false, "pedido mudou"
	}
	criou := false
	if e.Reservas[pedido.IDReserva] == nil {
		criou = true
		e.Reservas[pedido.IDReserva] = &Reserva{
			ID:         pedido.IDReserva,
			IDPedido:   pedido.IDPedido,
			Passageiro: pedido.Passageiro,
			Status:     "confirmada",
			PrecoTotal: somarPreco(pedido.Trechos),
			Trechos:    pedido.Trechos,
		}
	}
	pedido.Fase = "confirmada"
	err := e.Salvar()
	if err != nil {
		pedido.Fase = "confirmando"
		if criou {
			delete(e.Reservas, pedido.IDReserva)
		}
		e.Trava.Unlock()
		return false, "nao consegui gravar"
	}
	e.Trava.Unlock()
	return true, "ok"
}

func trechoRepetido(trechos []TrechoViagem) string {
	visto := map[string]bool{}
	for _, t := range trechos {
		chave := t.Empresa + "|" + t.IDCarona + "|" + strconv.Itoa(t.Indice)
		if visto[chave] {
			return "trecho repetido na reserva"
		}
		visto[chave] = true
	}
	return ""
}

func (e *Estado) trechoOficial(t TrechoViagem) (TrechoViagem, string) {
	if t.Empresa == e.Empresa {
		e.Trava.Lock()
		tr := e.AcharTrecho(t.IDCarona, t.Indice)
		if tr == nil {
			e.Trava.Unlock()
			return t, "trecho nao existe"
		}
		c := e.Caronas[t.IDCarona]
		t.Origem = tr.Origem
		t.Destino = tr.Destino
		t.Preco = tr.Preco
		t.Vagas = tr.VagasLivres
		if c != nil {
			t.Data = c.Data
			t.Hora = c.Hora
		}
		e.Trava.Unlock()
		if t.Preco < 0 || t.Origem == "" || t.Destino == "" {
			return t, "trecho invalido"
		}
		return t, ""
	}
	url := e.urlDa(t.Empresa)
	if url == "" {
		return t, "nao conheco a empresa " + t.Empresa
	}
	var resp Resposta
	_, rede := Pedir("GET", url+"/consorcio/trechos?data="+escapar(t.Data), nil, e.Timeout, &resp)
	if rede || !resp.OK {
		return t, "sem dados oficiais de " + t.Empresa
	}
	for _, pub := range resp.Trechos {
		if pub.IDCarona == t.IDCarona && pub.Indice == t.Indice {
			pub.Empresa = t.Empresa
			if pub.Preco < 0 || pub.Origem == "" || pub.Destino == "" {
				return t, "trecho invalido"
			}
			return pub, ""
		}
	}
	return t, "trecho nao existe em " + t.Empresa
}

func (e *Estado) responderTentativa(fase, idPedido, idReserva string) Resposta {
	if fase == "confirmando" {
		ok, motivo := e.entregarConfirmacao(idPedido)
		if !ok {
			return Resposta{Codigo: 202, Mensagem: idPedido, Erro: "reserva ainda em andamento: " + motivo}
		}
		fase = "confirmada"
	}
	if fase == "confirmada" {
		e.Trava.Lock()
		r := e.Reservas[idReserva]
		if r == nil {
			e.Trava.Unlock()
			return Resposta{Erro: "nao achei a reserva"}
		}
		copia := copiarReserva(r)
		e.Trava.Unlock()
		return Resposta{OK: true, Reserva: &copia}
	}
	if fase == "abortada" || fase == "cancelada" {
		return Resposta{Erro: "essa tentativa ja foi recusada"}
	}
	return Resposta{Codigo: 202, Mensagem: idPedido, Erro: "reserva ainda em andamento"}
}

func (e *Estado) acharTentativa(usuario, idTentativa string) (string, string, string, bool) {
	if idTentativa == "" {
		return "", "", "", false
	}
	for _, p := range e.Pedidos {
		if p.Passageiro == usuario && p.IDTentativa == idTentativa {
			return p.Fase, p.IDPedido, p.IDReserva, true
		}
	}
	return "", "", "", false
}

func (e *Estado) Reservar(usuario, senha, idTentativa string, trechos []TrechoViagem) Resposta {
	e.Trava.Lock()
	u, ok := e.Usuarios[usuario]
	if !ok || u.Papel != "passageiro" || u.Senha != senha {
		e.Trava.Unlock()
		return Resposta{Erro: "usuario ou senha invalido"}
	}
	if idTentativa != "" {
		if fase, idPedido, idReserva, achou := e.acharTentativa(usuario, idTentativa); achou {
			e.Trava.Unlock()
			return e.responderTentativa(fase, idPedido, idReserva)
		}
	}
	e.Trava.Unlock()

	if msg := trechoRepetido(trechos); msg != "" {
		return Resposta{Erro: msg}
	}
	oficiais := make([]TrechoViagem, 0, len(trechos))
	for _, t := range trechos {
		oficial, msg := e.trechoOficial(t)
		if msg != "" {
			return Resposta{Erro: msg}
		}
		oficiais = append(oficiais, oficial)
	}
	trechos = oficiais
	if msg := validarCadeia(trechos); msg != "" {
		return Resposta{Erro: msg}
	}
	for _, t := range trechos {
		if t.Preco < 0 {
			return Resposta{Erro: "preco nao pode ser negativo"}
		}
	}

	e.Trava.Lock()
	if fase, idPedidoExistente, idReservaExistente, achou := e.acharTentativa(usuario, idTentativa); achou {
		e.Trava.Unlock()
		return e.responderTentativa(fase, idPedidoExistente, idReservaExistente)
	}
	grupos := agruparPorEmpresa(trechos)
	participantes := make([]Par, 0, len(grupos))
	for _, g := range grupos {
		url := e.urlDa(g.empresa)
		if url == "" {
			e.Trava.Unlock()
			return Resposta{Erro: "nao conheco a empresa " + g.empresa}
		}
		participantes = append(participantes, Par{Empresa: g.empresa, URL: url})
	}
	relogio := e.Relogio.Tic()
	idPedido := e.ProximoID("pedido")
	idReserva := e.ProximoID("reserva")
	e.Pedidos[idPedido] = &Pedido{
		IDPedido:      idPedido,
		IDReserva:     idReserva,
		Fase:          "segurando",
		Passageiro:    usuario,
		Relogio:       relogio,
		IDTentativa:   idTentativa,
		Trechos:       trechos,
		Participantes: participantes,
	}
	if err := e.Salvar(); err != nil {
		delete(e.Pedidos, idPedido)
		e.Trava.Unlock()
		return Resposta{Erro: "nao consegui gravar"}
	}
	e.Trava.Unlock()

	fmt.Printf("[%s] segurando %s relogio=%d\n", e.Empresa, idPedido, relogio)

	for _, g := range grupos {
		refs := make([]RefTrecho, 0, len(g.trechos))
		for _, t := range g.trechos {
			refs = append(refs, RefTrecho{IDCarona: t.IDCarona, Indice: t.Indice})
		}
		corpo := CorpoSegurar{
			IDPedido:      idPedido,
			Relogio:       relogio,
			EmpresaCoord:  e.Empresa,
			URLCoord:      e.URLPropria,
			Passageiro:    usuario,
			Participantes: participantes,
			Trechos:       refs,
		}
		segurou, _, motivo := e.segurarNa(g.empresa, corpo)
		if !segurou {
			fmt.Printf("[%s] nao segurou em %s: %s\n", e.Empresa, g.empresa, motivo)
			e.AbortarPedido(idPedido)
			return Resposta{Erro: "nao consegui segurar em " + g.empresa + ": " + motivo}
		}
	}

	e.Trava.Lock()
	if e.Pedidos[idPedido] == nil {
		e.Trava.Unlock()
		return Resposta{Erro: "pedido sumiu"}
	}
	err := e.gravarFase(idPedido, "confirmando")
	if err != nil {
		// a preparacao ja terminou, mas a decisao nao entrou no disco.
		// marco so este pedido. o laco aborta quando a escrita voltar.
		// um segurando que ainda esta na rede nao entra neste mapa.
		if e.Abandonados == nil {
			e.Abandonados = map[string]bool{}
		}
		e.Abandonados[idPedido] = true
		e.Trava.Unlock()
		e.AbortarPedido(idPedido)
		return Resposta{Erro: "nao consegui gravar a confirmacao"}
	}
	e.Trava.Unlock()

	okConf, motivo := e.entregarConfirmacao(idPedido)
	if !okConf {
		fmt.Printf("[%s] confirmar pendente: %s\n", e.Empresa, motivo)
		return Resposta{Codigo: 202, Mensagem: idPedido, Erro: "confirmacao pendente. manda de novo com o mesmo id"}
	}

	e.Trava.Lock()
	reserva := e.Reservas[idReserva]
	var copia Reserva
	if reserva != nil {
		copia = copiarReserva(reserva)
	}
	e.Trava.Unlock()
	fmt.Printf("[%s] reserva confirmada %s\n", e.Empresa, idReserva)
	return Resposta{OK: true, Reserva: &copia}
}

func (e *Estado) tentarCancelar(idPedido, idReserva string) (bool, string) {
	e.Trava.Lock()
	pedido := e.Pedidos[idPedido]
	if pedido == nil {
		e.Trava.Unlock()
		return false, "nao achei o pedido dessa reserva"
	}
	if pedido.Fase == "cancelada" {
		e.Trava.Unlock()
		return true, "ok"
	}
	participantes := append([]Par(nil), pedido.Participantes...)
	err := e.gravarFase(idPedido, "cancelando")
	e.Trava.Unlock()
	if err != nil {
		return false, "nao consegui gravar"
	}

	for _, p := range participantes {
		ok, rede, motivo := e.desfazerNa(p.Empresa, idPedido)
		if !ok || rede {
			return false, p.Empresa + " (" + motivo + ")"
		}
	}

	e.Trava.Lock()
	r := e.Reservas[idReserva]
	pedido = e.Pedidos[idPedido]
	statusAntes := ""
	if r != nil {
		statusAntes = r.Status
		r.Status = "cancelada"
	}
	faseAntes := ""
	if pedido != nil {
		faseAntes = pedido.Fase
		pedido.Fase = "cancelada"
	}
	err = e.Salvar()
	if err != nil {
		if r != nil {
			r.Status = statusAntes
		}
		if pedido != nil {
			pedido.Fase = faseAntes
		}
		e.Trava.Unlock()
		return false, "nao consegui gravar"
	}
	e.Trava.Unlock()
	return true, "ok"
}

func (e *Estado) CancelarReserva(usuario, senha, idReserva string) Resposta {
	e.Trava.Lock()
	u, ok := e.Usuarios[usuario]
	if !ok || u.Senha != senha {
		e.Trava.Unlock()
		return Resposta{Erro: "usuario ou senha invalido"}
	}
	r := e.Reservas[idReserva]
	if r == nil {
		e.Trava.Unlock()
		return Resposta{Erro: "reserva nao existe"}
	}
	if r.Passageiro != usuario {
		e.Trava.Unlock()
		return Resposta{Erro: "essa reserva nao e sua"}
	}
	if r.Status == "cancelada" {
		copia := copiarReserva(r)
		e.Trava.Unlock()
		return Resposta{OK: true, Reserva: &copia}
	}
	idPedido := r.IDPedido
	e.Trava.Unlock()

	conseguiu, motivo := e.tentarCancelar(idPedido, idReserva)
	if !conseguiu {
		return Resposta{Erro: "nao consegui cancelar em: " + motivo}
	}
	e.Trava.Lock()
	r = e.Reservas[idReserva]
	copia := copiarReserva(r)
	e.Trava.Unlock()
	return Resposta{OK: true, Reserva: &copia}
}

func contarTrocas(caminho []TrechoViagem) int {
	if len(caminho) == 0 {
		return 0
	}
	n := 0
	anterior := caminho[0].Empresa + "|" + caminho[0].IDCarona
	for _, t := range caminho[1:] {
		chave := t.Empresa + "|" + t.IDCarona
		if chave != anterior {
			n = n + 1
		}
		anterior = chave
	}
	return n
}

func (e *Estado) BuscarItinerarios(origem, destino, data string) Resposta {
	// peco TODOS os trechos do dia. se eu pedisse so origem e destino,
	// a empresa do meio (que nao faz a viagem inteira) responderia vazio.
	todos := e.TrechosPublicos(data)
	for _, par := range e.Pares {
		var resp Resposta
		_, erroRede := Pedir("GET", par.URL+"/consorcio/trechos?data="+escapar(data), nil, e.Timeout, &resp)
		if erroRede || !resp.OK {
			fmt.Printf("[%s] sem trechos de %s\n", e.Empresa, par.Empresa)
			continue
		}
		todos = append(todos, resp.Trechos...)
	}
	caminhos := MontarItinerarios(todos, origem, destino, 3, 8)
	its := make([]Itinerario, 0, len(caminhos))
	for _, caminho := range caminhos {
		its = append(its, Itinerario{
			PrecoTotal: somarPreco(caminho),
			Trocas:     contarTrocas(caminho),
			Trechos:    caminho,
		})
	}
	return Resposta{OK: true, Itinerarios: its}
}
