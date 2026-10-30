# Arquitetura (notas curtas)

O texto na ordem do barema esta em `docs/relatorio.md`. Aqui e o resumo.

## Problema 1 x problema 2

No problema 1 existia um servidor so. Motorista e passageiro abriam TCP nele.
Uma trava (mutex) em volta da memoria resolvia a reserva: ou baixava todos
os trechos, ou nao baixava nenhum, porque estava tudo no mesmo processo.

No problema 2 cada empresa tem o seu processo, a sua memoria e o seu arquivo.
A trava de um processo nao enxerga o outro. Por isso a trava global saiu.
No lugar entrou o protocolo segurar / confirmar / desfazer, com relogio de Lamport.

Nao tem servidor central, nao tem coordenador fixo, nao tem banco compartilhado.
Quem coordena uma reserva e a empresa do passageiro, so naquela reserva.
A proxima reserva pode ser coordenada por outra empresa, se o passageiro for de la.

## O que cada um guarda

- usuarios da propria empresa
- caronas publicadas pelos motoristas dela (ela e a dona das vagas)
- reservas que ela coordenou (pra o passageiro dela consultar)
- seguros: vaga que alguem (pode ser de fora) segurou ou confirmou aqui
- arquivo JSON so desta empresa

Regiao e so um campo (reconcavo, sudoeste, norte-minas) pra separar de quem
e o trecho na hora de mostrar o itinerario. Nao muda o protocolo. Todo mundo
fala com todo mundo do PARES. A regiao nao e um filtro que esconde carona.

## Concorrencia

Dentro de um processo: o net/http atende cada pedido numa goroutine, e tem
um Mutex no mapa. A trava nao fica presa durante a chamada de rede.
Se ficasse, um servidor esperando o outro, que espera o primeiro, travava tudo.

Entre processos: ordem alfabetica do nome da empresa na hora de segurar.
Todo coordenador pede norteminas, depois sudoeste, depois vaijunto.
Nao tem o ciclo "eu tenho A e quero B, voce tem B e quer A".

Ultima vaga: relogio de Lamport, nao o relogio de parede do Windows.

## Queda

Timeout na chamada HTTP. Empresa muda: a busca segue sem os trechos dela.
Reserva que precisava dela falha e desfaz o que ja tinha segurado.

Seguro tem prazo (EXPIRA_SEGUNDOS, padrao 20). Se ninguem confirmar, a
thread de limpeza solta. Antes de soltar, pergunta pros outros se alguem
ja confirmou. Se ja confirmou, confirma tambem. Se o coordenador morreu
no meio, o arquivo dele tem a fase (segurando, confirmando, confirmada,
abortada) e na volta ele continua de onde parou.

Gravacao do arquivo: escreve .tmp e troca o nome. Queda no meio do write
nao corrompe o JSON anterior.
