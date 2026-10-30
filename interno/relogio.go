package interno

// Relogio de Lamport.
//
// No problema 1 desempatava quem chegasse primeiro, porque so tinha um
// processo e uma trava. Aqui os PCs do lab nao estao com o relogio igual,
// e a mensagem pode sair antes e chegar depois.
//
// Regras (Lamport):
//   - cada servidor tem um inteiro que so aumenta
//   - antes de eu mandar pedido, eu somo 1 (Tic)
//   - quando chega mensagem, fico com o maior entre o meu e o que veio, e somo 1
// O numero nao e hora. Menor numero tem preferencia na vaga.
// Empate: nome da empresa, depois o id do pedido.

type Relogio struct {
	Valor int
}

func (r *Relogio) Tic() int {
	// quem chama ja esta com a trava do estado. nao pus outra trava aqui
	// porque duas travas viram deadlock se a ordem inverter.
	r.Valor = r.Valor + 1
	return r.Valor
}

func (r *Relogio) Receber(chegou int) int {
	if chegou < 0 {
		return r.Valor
	}
	if chegou > r.Valor {
		r.Valor = chegou
	}
	r.Valor = r.Valor + 1
	return r.Valor
}
