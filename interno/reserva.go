package interno

import (
	"fmt"
	"sort"
	"time"
)

// Reserva entre empresas.
//
// Fase 1, SEGURAR, na ordem alfabetica do nome da empresa.
// Se uma negar ou cair, desfaz o que as outras ja tinham segurado.
// Segurar NAO baixa vagas_livres. So marca.
//
// Fase 2, CONFIRMAR. So mando depois que todas seguraram.
// Ai o dono baixa a vaga. Se um confirmar falhar, desfaz inclusive
// quem ja tinha baixado.
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
		e.Salvar()
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
	e.Salvar()
	return true, "ok"
}

func (e *Estado) TratarConfirmar(idPedido string, relogio int, ignorarExpira bool) (bool, string) {
	if idPedido == "" {
		return false, "faltou id do pedido"
	}
	e.Trava.Lock()
	defer e.Trava.Unlock()
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
	if !ignorarExpira && s.Expira.Before(time.Now()) {
		return false, "expirou"
	}
	for _, t := range s.Trechos {
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
	e.Salvar()
	return true, "ok"
}

func (e *Estado) TratarDesfazer(idPedido string) (bool, string) {
	if idPedido == "" {
		return false, "faltou id do pedido"
	}
	e.Trava.Lock()
	defer e.Trava.Unlock()
	s := e.Seguros[idPedido]
	if s == nil || s.Estado == "abortado" {
		return true, "nada a desfazer"
	}
	if s.Estado == "confirmado" {
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
	s.Estado = "abortado"
	e.Relogio.Tic()
	e.Salvar()
	return true, "ok"
}

func (e *Estado) ConsultarPedido(idPedido string) string {
	e.Trava.Lock()
	defer e.Trava.Unlock()
	s := e.Seguros[idPedido]
	if s != nil {
		if s.Estado == "confirmado" {
			return "confirmado"
		}
		if s.Estado == "abortado" || s.Estado == "negado" {
			return "abortado"
		}
		return "segurado"
	}
	p := e.Pedidos[idPedido]
	if p == nil {
		return "desconhecido"
	}
	if p.Fase == "confirmada" {
		return "confirmado"
	}
	if p.Fase == "abortada" || p.Fase == "abortando" {
		return "abortado"
	}
	return "pendente"
}

func (e *Estado) segurarNa(empresa string, c CorpoSegurar) (bool, string) {
	if empresa == e.Empresa {
		return e.TratarSegurar(c)
	}
	url := e.urlDa(empresa)
	if url == "" {
		return false, "nao conheco a empresa " + empresa
	}
	var resp Resposta
	_, erroRede := Pedir("POST", url+"/consorcio/segurar", c, e.Timeout, &resp)
	if erroRede {
		return false, "sem resposta de " + empresa
	}
	if !resp.OK {
		if resp.Erro == "" {
			resp.Erro = "falhou em " + empresa
		}
		return false, resp.Erro
	}
	return true, "ok"
}

func (e *Estado) confirmarNa(empresa, idPedido string, relogio int) (bool, string) {
	if empresa == e.Empresa {
		return e.TratarConfirmar(idPedido, relogio, false)
	}
	url := e.urlDa(empresa)
	if url == "" {
		return false, "nao conheco a empresa " + empresa
	}
	var resp Resposta
	_, erroRede := Pedir("POST", url+"/consorcio/confirmar", corpoID{IDPedido: idPedido, Relogio: relogio}, e.Timeout, &resp)
	if erroRede {
		return false, "sem resposta de " + empresa
	}
	if !resp.OK {
		if resp.Erro == "" {
			resp.Erro = "falhou em " + empresa
		}
		return false, resp.Erro
	}
	return true, "ok"
}

func (e *Estado) desfazerNa(empresa, idPedido string) (bool, string) {
	if empresa == e.Empresa {
		return e.TratarDesfazer(idPedido)
	}
	url := e.urlDa(empresa)
	if url == "" {
		return false, "nao conheco a empresa " + empresa
	}
	var resp Resposta
	_, erroRede := Pedir("POST", url+"/consorcio/desfazer", corpoID{IDPedido: idPedido}, e.Timeout, &resp)
	if erroRede {
		return false, "sem resposta de " + empresa
	}
	if !resp.OK {
		if resp.Erro == "" {
			resp.Erro = "falhou em " + empresa
		}
		return false, resp.Erro
	}
	return true, "ok"
}

func (e *Estado) AbortarPedido(idPedido string) {
	e.Trava.Lock()
	pedido := e.Pedidos[idPedido]
	if pedido == nil {
		e.Trava.Unlock()
		return
	}
	participantes := append([]Par(nil), pedido.Participantes...)
	pedido.Fase = "abortando"
	e.Salvar()
	e.Trava.Unlock()

	for _, p := range participantes {
		e.desfazerNa(p.Empresa, idPedido)
	}

	e.Trava.Lock()
	pedido = e.Pedidos[idPedido]
	if pedido != nil {
		pedido.Fase = "abortada"
		e.Salvar()
	}
	e.Trava.Unlock()
}

func (e *Estado) Reservar(usuario string, trechos []TrechoViagem) Resposta {
	e.Trava.Lock()
	u, ok := e.Usuarios[usuario]
	if !ok || u.Papel != "passageiro" {
		e.Trava.Unlock()
		return Resposta{Erro: "usuario nao e passageiro desta empresa"}
	}
	erro := validarCadeia(trechos)
	if erro != "" {
		e.Trava.Unlock()
		return Resposta{Erro: erro}
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
	// um tic so. a viagem inteira tem o mesmo numero de precedencia.
	relogio := e.Relogio.Tic()
	idPedido := e.ProximoID("pedido")
	idReserva := e.ProximoID("reserva")
	e.Pedidos[idPedido] = &Pedido{
		IDPedido:      idPedido,
		IDReserva:     idReserva,
		Fase:          "segurando",
		Passageiro:    usuario,
		Relogio:       relogio,
		Trechos:       trechos,
		Participantes: participantes,
	}
	e.Salvar()
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
		ok, motivo := e.segurarNa(g.empresa, corpo)
		if !ok {
			fmt.Printf("[%s] nao segurou em %s: %s\n", e.Empresa, g.empresa, motivo)
			e.AbortarPedido(idPedido)
			return Resposta{Erro: "nao consegui segurar em " + g.empresa + ": " + motivo}
		}
	}

	// gravo confirmando ANTES de mandar o confirmar.
	// se eu morrer agora, na subida eu continuo, nao deixo metade.
	e.Trava.Lock()
	e.Pedidos[idPedido].Fase = "confirmando"
	e.Salvar()
	e.Trava.Unlock()

	for _, g := range grupos {
		ok, motivo := e.confirmarNa(g.empresa, idPedido, relogio)
		if !ok {
			fmt.Printf("[%s] nao confirmou em %s: %s\n", e.Empresa, g.empresa, motivo)
			e.AbortarPedido(idPedido)
			return Resposta{Erro: "falhou ao confirmar em " + g.empresa + ": " + motivo}
		}
	}

	e.Trava.Lock()
	preco := somarPreco(trechos)
	reserva := &Reserva{
		ID:         idReserva,
		IDPedido:   idPedido,
		Passageiro: usuario,
		Status:     "confirmada",
		PrecoTotal: preco,
		Trechos:    trechos,
	}
	e.Reservas[idReserva] = reserva
	e.Pedidos[idPedido].Fase = "confirmada"
	e.Relogio.Tic()
	e.Salvar()
	copia := copiarReserva(reserva)
	e.Trava.Unlock()
	fmt.Printf("[%s] reserva confirmada %s\n", e.Empresa, idReserva)
	return Resposta{OK: true, Reserva: &copia}
}

func (e *Estado) CancelarReserva(usuario, idReserva string) Resposta {
	e.Trava.Lock()
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
	pedido := e.Pedidos[r.IDPedido]
	if pedido == nil {
		e.Trava.Unlock()
		return Resposta{Erro: "nao achei o pedido dessa reserva"}
	}
	participantes := append([]Par(nil), pedido.Participantes...)
	idPedido := r.IDPedido
	e.Trava.Unlock()

	erros := ""
	for _, p := range participantes {
		ok, motivo := e.desfazerNa(p.Empresa, idPedido)
		if !ok {
			if erros != "" {
				erros = erros + ", "
			}
			erros = erros + p.Empresa + " (" + motivo + ")"
		}
	}
	if erros != "" {
		return Resposta{Erro: "nao consegui cancelar em: " + erros}
	}

	e.Trava.Lock()
	r = e.Reservas[idReserva]
	r.Status = "cancelada"
	if e.Pedidos[idPedido] != nil {
		e.Pedidos[idPedido].Fase = "abortada"
	}
	e.Salvar()
	copia := copiarReserva(r)
	e.Trava.Unlock()
	return Resposta{OK: true, Reserva: &copia}
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
			Trocas:     len(caminho) - 1,
			Trechos:    caminho,
		})
	}
	return Resposta{OK: true, Itinerarios: its}
}
