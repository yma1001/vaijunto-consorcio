package interno

import (
	"fmt"
	"net/url"
	"time"
)

func escapar(s string) string {
	return url.QueryEscape(s)
}

// Queda e volta.
//
// fase segurando   -> ainda nao confirmei. na volta eu aborto.
// fase confirmando -> a decisao ja esta no arquivo. repito o confirmar.
//                    timeout nao vira aborto. quem ja baixou a vaga responde de novo.
// fase abortando   -> repito o desfazer ate todo mundo responder.
// fase cancelando  -> o passageiro pediu pra sair. so marco cancelada quando todos respondem.
// fase confirmada  -> nao mexo.
//
// Seguro vencido e ainda so "esperando": solto, porque eu nao prometi a vaga.
// Seguro "segurado": pergunto ao coordenador. confirmado eu confirmo.
// abortado ou desconhecido eu solto. se ele nao responder, espero.

func AoSubir(e *Estado) {
	e.Trava.Lock()
	var segurando []string
	for _, p := range e.Pedidos {
		if p.Fase == "segurando" {
			segurando = append(segurando, p.IDPedido)
		}
	}
	e.Trava.Unlock()
	for _, id := range segurando {
		fmt.Printf("[%s] retomei %s e vou abortar\n", e.Empresa, id)
		e.AbortarPedido(id)
	}
	retomarPendentes(e)
	reconciliarSeguros(e)
}

func retomarPendentes(e *Estado) {
	e.Trava.Lock()
	var confirmar []string
	var abortar []string
	type canc struct {
		pedido  string
		reserva string
	}
	var cancelar []canc
	for _, p := range e.Pedidos {
		if p.Fase == "confirmando" {
			confirmar = append(confirmar, p.IDPedido)
		} else if p.Fase == "abortando" {
			abortar = append(abortar, p.IDPedido)
		} else if p.Fase == "cancelando" {
			cancelar = append(cancelar, canc{pedido: p.IDPedido, reserva: p.IDReserva})
		}
	}
	e.Trava.Unlock()
	for _, id := range confirmar {
		fmt.Printf("[%s] retomei %s e vou confirmar de novo\n", e.Empresa, id)
		e.entregarConfirmacao(id)
	}
	for _, id := range abortar {
		e.AbortarPedido(id)
	}
	for _, c := range cancelar {
		e.tentarCancelar(c.pedido, c.reserva)
	}
}

func reconciliarSeguros(e *Estado) {
	agora := time.Now()
	type item struct {
		id   string
		fase string
		url  string
	}
	e.Trava.Lock()
	lista := make([]item, 0)
	for _, s := range e.Seguros {
		if s.Estado != "esperando" && s.Estado != "segurado" {
			continue
		}
		if !s.Expira.Before(agora) {
			continue
		}
		lista = append(lista, item{id: s.IDPedido, fase: s.Estado, url: s.URLCoord})
	}
	e.Trava.Unlock()

	for _, it := range lista {
		if it.fase == "esperando" {
			e.TratarDesfazer(it.id)
			continue
		}
		estado := ""
		if it.url == "" || it.url == e.URLPropria {
			estado = e.ConsultarPedido(it.id)
		} else {
			var resp struct {
				OK     bool   `json:"ok"`
				Estado string `json:"estado"`
			}
			_, erroRede := Pedir("GET", it.url+"/consorcio/pedido?id="+escapar(it.id), nil, e.Timeout, &resp)
			if erroRede {
				continue
			}
			estado = resp.Estado
		}
		if estado == "confirmado" {
			e.TratarConfirmar(it.id, 0, true)
			continue
		}
		if estado == "abortado" || estado == "desconhecido" {
			e.TratarDesfazer(it.id)
		}
	}
}

func LoopLimpeza(e *Estado) {
	for e.ligado() {
		time.Sleep(400 * time.Millisecond)
		if !e.ligado() {
			break
		}
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					fmt.Println("["+e.Empresa+"] limpeza deu erro:", rec)
				}
			}()
			reconciliarSeguros(e)
			retomarPendentes(e)
		}()
	}
}
