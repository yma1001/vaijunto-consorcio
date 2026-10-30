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
// fase segurando  -> ainda nao confirmei. na volta eu aborto.
// fase confirmando -> ja decidi confirmar (gravei antes de mandar).
//                    na volta eu confirmo de novo. repetir nao baixa vaga duas vezes.
//                    se alguem perdeu o seguro, ai eu desfaco todo mundo.
// fase confirmada -> nao mexo.
// fase abortada   -> mando desfazer de novo, porque o outro pode ter estado morto.
//
// Seguro vencido: pergunto pros participantes se alguem confirmou.
// se sim, confirmo tambem. se ninguem responder, solto a vaga.
// prefiro perder a reserva do que deixar assento preso pra sempre.

func AoSubir(e *Estado) {
	e.Trava.Lock()
	pedidos := make([]*Pedido, 0, len(e.Pedidos))
	for _, p := range e.Pedidos {
		pedidos = append(pedidos, p)
	}
	e.Trava.Unlock()

	for _, p := range pedidos {
		if p.Fase == "segurando" || p.Fase == "abortando" {
			fmt.Printf("[%s] retomei %s e vou abortar\n", e.Empresa, p.IDPedido)
			e.AbortarPedido(p.IDPedido)
		} else if p.Fase == "confirmando" {
			fmt.Printf("[%s] retomei %s e vou confirmar de novo\n", e.Empresa, p.IDPedido)
			retomarConfirmacao(e, p)
		} else if p.Fase == "abortada" {
			for _, parte := range p.Participantes {
				e.desfazerNa(parte.Empresa, p.IDPedido)
			}
		}
	}
	reconciliarSeguros(e)
}

func retomarConfirmacao(e *Estado, pedido *Pedido) {
	for _, parte := range pedido.Participantes {
		ok, motivo := e.confirmarNa(parte.Empresa, pedido.IDPedido, pedido.Relogio)
		if !ok {
			fmt.Printf("[%s] retomada falhou em %s: %s\n", e.Empresa, parte.Empresa, motivo)
			e.AbortarPedido(pedido.IDPedido)
			return
		}
	}
	e.Trava.Lock()
	defer e.Trava.Unlock()
	atual := e.Pedidos[pedido.IDPedido]
	if atual == nil || atual.Fase == "abortada" {
		return
	}
	if e.Reservas[atual.IDReserva] == nil {
		e.Reservas[atual.IDReserva] = &Reserva{
			ID:         atual.IDReserva,
			IDPedido:   atual.IDPedido,
			Passageiro: atual.Passageiro,
			Status:     "confirmada",
			PrecoTotal: somarPreco(atual.Trechos),
			Trechos:    atual.Trechos,
		}
	}
	atual.Fase = "confirmada"
	e.Salvar()
}

func reconciliarSeguros(e *Estado) {
	agora := time.Now()
	type vencido struct {
		id     string
		partes []Par
	}
	e.Trava.Lock()
	lista := make([]vencido, 0)
	for _, s := range e.Seguros {
		if (s.Estado == "segurado" || s.Estado == "esperando") && s.Expira.Before(agora) {
			lista = append(lista, vencido{
				id:     s.IDPedido,
				partes: append([]Par(nil), s.Participantes...),
			})
		}
	}
	e.Trava.Unlock()

	for _, item := range lista {
		alguemConfirmou := false
		for _, parte := range item.partes {
			if parte.Empresa == e.Empresa {
				continue
			}
			if parte.URL == "" {
				continue
			}
			var resp struct {
				OK     bool   `json:"ok"`
				Estado string `json:"estado"`
			}
			_, erroRede := Pedir("GET", parte.URL+"/consorcio/pedido?id="+escapar(item.id), nil, e.Timeout, &resp)
			if erroRede {
				continue
			}
			if resp.Estado == "confirmado" {
				alguemConfirmou = true
				break
			}
		}
		if alguemConfirmou {
			ok, _ := e.TratarConfirmar(item.id, 0, true)
			if !ok {
				e.TratarDesfazer(item.id)
			}
		} else {
			e.TratarDesfazer(item.id)
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
		}()
	}
}
