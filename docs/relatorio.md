# Relatorio — os 11 pontos do barema

O codigo que responde cada ponto esta citado aqui. A API e o diagrama ficam em `docs/api.md`.

## 1. Arquitetura

Cada empresa e um processo (`cmd/servidor`). Motorista e passageiro (`cmd/motorista`, `cmd/passageiro`) abrem HTTP so na empresa deles. Se o itinerario usa carona de outra, a empresa do passageiro vira cliente das outras, so naquela reserva. Nao tem servidor central nem coordenador fixo.

Cada processo guarda, na memoria e no JSON proprio (`interno/estado.go`):

- usuarios dela
- caronas dos motoristas dela (ela e dona de `vagas_livres`)
- reservas que ela coordenou
- seguros: vaga que alguem segurou ou confirmou nela, mesmo passageiro de fora
- pedidos: a fase da transacao que ela coordena

`PARES` e `URL_PROPRIA` nao vao pro arquivo. Sao o endereco do laboratorio.

No problema 1 havia um servidor e uma trava global. A reserva inteira cabia nessa trava. Aqui a trava de um processo nao enxerga o outro. Por isso a trava global saiu, e entrou segurar / confirmar / desfazer.

Regiao (reconcavo, sudoeste, norte-minas) e rotulo na tela. Nao filtra carona e nao e divisao por estado.

## 2. Comunicacao (API REST)

Especificacao completa, com exemplo de JSON e codigo HTTP, em `docs/api.md`.

Entre servidores: `GET /consorcio/saude`, `GET /consorcio/trechos`, `GET /consorcio/pedido`, `POST /consorcio/segurar`, `POST /consorcio/confirmar`, `POST /consorcio/desfazer` (e `/liberar`, a mesma funcao).

O cliente HTTP e `Pedir` em `interno/pedir.go`. O segundo retorno true significa que nao houve resposta (timeout, conexao recusada). Isso nao e 409. 409 e a outra empresa viva recusando.

Repetir a mesma requisicao:

- `segurar` de um id que ja esta segurado ou confirmado responde ok e nao marca de novo (`TratarSegurar`)
- `confirmar` de um id ja confirmado responde ok e nao baixa `vagas_livres` de novo (`TratarConfirmar`)
- `desfazer` de um id ja abortado responde ok e nao devolve a vaga outra vez (`TratarDesfazer`)

Sem resposta, o coordenador trata como falha daquela empresa e aborta o pedido (`AbortarPedido`).

## 3. Busca distribuida

Estrategia: consulta em tempo real. Nao tem catalogo replicado.

`BuscarItinerarios` (`interno/reserva.go`) pede `GET /consorcio/trechos?data=` para cada par, o dia inteiro, nao so origem e destino. Se filtrasse na pergunta, a empresa do meio (Feira -> Vitoria) responderia vazio e a baldeacao sumia. `MontarItinerarios` (`interno/busca.go`) junta os trechos. Limite de 3 trechos, no maximo 8 caminhos, mais barato primeiro. Busca nao segura vaga.

Empresa fora do ar: `Pedir` volta queda, o `for` da busca da `continue` e segue com quem respondeu. Salvador -> Cachoeira continua mesmo com a sudoeste morta. A viagem que precisava dela nao aparece.

## 4. Exclusao mutua

O assento mora na empresa dona do trecho. Dois passageiros, de servidores diferentes, mandam `POST /consorcio/segurar` pra ela.

Ela nao confirma na ordem em que o socket leu. Grava os dois como `esperando`, solta a trava, espera `JANELA_MS` e ordena (`decidirQuemFicaComAVaga`). So um vira `segurado`. O `confirmar` so baixa `vagas_livres` se o estado for `segurado`, e repetir nao baixa de novo. O teste `TestDisputaSimultanea` manda os dois ao mesmo tempo e exige exatamente um vencedor, com a vaga em 0, nao negativa.

A trava (`Estado.Trava`) so protege o mapa de um processo. Entre empresas, a exclusao e o seguro mais o confirmar idempotente.

## 5. Atomicidade distribuida

Quem decide e a empresa do passageiro, so naquela reserva (`Reservar`).

Protocolo, nesta ordem:

1. Grava fase `segurando`.
2. `POST /segurar` em ordem alfabetica (norteminas, sudoeste, vaijunto). Segurar nao baixa vaga.
3. Se uma recusa ou nao responde, `AbortarPedido` manda `desfazer` em quem ja segurou. O passageiro recebe erro. `vagas_livres` nao mudou.
4. Grava fase `confirmando` antes de mandar a rede.
5. `POST /confirmar`. Aí a dona baixa 1.
6. Se um confirmar falha, desfaz inclusive quem ja baixou.
7. Grava a reserva local como `confirmada`.

Diagrama de sequencia: `docs/api.md`, secao "Sequencia: reserva com tres empresas".

## 6. Precedencia

Criterio: menor relogio de Lamport. Empate: nome da empresa coordenadora, depois o id do pedido (`decidirQuemFicaComAVaga`). O relogio esta em `interno/relogio.go`. Nao usa o relogio de parede. `Tic` soma 1 antes de sair o pedido. `Receber` fica com o maior e soma 1. Os PCs podem estar com a hora diferente e o pacote pode atrasar: o carimbo vai dentro da mensagem.

Na demo os dois comecam em 0, os dois carimbam 1, "sudoeste" ganha de "vaijunto".

Limitacao, pra nao ser pego: a janela e curta (250 ms). Se o carimbo melhor chegar depois que o outro ja foi marcado `segurado`, fica com quem ja segurou. Depois de `confirmado`, relogio nao tira a vaga.

## 7. Deadlock distribuido

Dois passageiros podem pedir os mesmos trechos em ordens diferentes. Sem uma ordem combinada, um segura a vaijunto e espera a sudoeste, o outro faz o contrario, e os dois ficam presos.

`agruparPorEmpresa` ordena o nome (`sort.Strings`). Todo coordenador pede norteminas, depois sudoeste, depois vaijunto. O segundo encontra a vaga ocupada e desiste, em vez de segurar o proximo e esperar.

Consequencia no desempenho: os `segurar` vao um depois do outro. Uma reserva de tres empresas espera a soma das janelas mais a rede. A `Trava` local nao fica segurada durante o HTTP nem durante o `Sleep`.

## 8. Tolerancia a falhas

A falha e timeout em `Pedir` (`TIMEOUT_SEGUNDOS`, padrao 3). Nao tem detector central.

Antes da confirmacao: a busca nao grava nada. Se o cliente fecha, nao existe pedido.

Durante o `segurar`: quem ja segurou recebe `desfazer`. A vaga livre nao foi baixada. As empresas que nao participam dessa reserva seguem atendendo. O teste `TestEmpresaCai` mata a sudoeste e confere isso.

Durante o `confirmar`: a fase `confirmando` ja esta no disco. Se o coordenador morre ali, o item 9 retoma. Se um participante recusa, desfaz todos, inclusive quem ja baixou.

Depois de `confirmada`: o assento fica no JSON de cada dono. Cair uma empresa nao apaga o que as outras ja confirmaram. Quem nao depende dela continua buscando e reservando.

## 9. Recuperacao e persistencia

O que vai pro disco (`gravacao` em `estado.go`): usuarios, caronas, reservas, seguros, pedidos (com a fase), contador e relogio. Pares e trava nao.

Momento: `Salvar` e chamado antes de mandar `segurar`, de novo ao mudar pra `confirmando`, de novo ao confirmar ou abortar, e em cada mudanca de seguro. Escreve `arquivo.tmp`, da `Sync`, troca o nome. Queda no meio do write deixa o JSON anterior inteiro.

Ao voltar, `Rodar` carrega o arquivo, sobe a limpeza e chama `AoSubir` (`interno/recuperacao.go`):

- `segurando` ou `abortando`: aborta e desfaz
- `confirmando`: confirma de novo (repetir nao baixa duas vezes). Se um participante perdeu o seguro, desfaz todo mundo
- `abortada`: desfaz de novo
- `confirmada`: nao mexe

Seguro `segurado` ou `esperando` vencido: pergunta `GET /consorcio/pedido`. Se alguem ja confirmou, confirma tambem. Se ninguem responder, solta. Prefere perder a reserva a deixar assento preso.

## 10. Testes

Automatizado: `go test ./interno -count=1`

| teste | o que olha |
|---|---|
| `TestJSONRuim` | JSON quebrado volta 400 e nao mexe vaga |
| `TestBuscaEReserva` | itinerario das tres empresas, baixa 1 em cada, o segundo falha, Cachoeira continua 2 |
| `TestCancelarDevolve` | cancelar devolve uma vez so |
| `TestEmpresaCai` | injeta falha: derruba a sudoeste no meio |
| `TestSeguroExpira` | segurou e nao confirmou: a vaga volta |
| `TestDisputaSimultanea` | dois servidores ao mesmo tempo na ultima vaga: exatamente um ganha |

Carga: `go run ./cmd/simulador 12`, com os tres servidores ja nas portas 5001–5003. Varios passageiros buscam Salvador -> Montes Claros e reservam juntos. A carona de Montes Claros nasce com 1 vaga. O resultado esperado e 1 confirmacao. Mais de uma e falha de atomicidade.

Resultado do `go test ./interno -count=1` nesta maquina: passou (`ok vaijunto/interno`).

## 11. Emulacao

`emulacao/docker-compose.yml` sobe tres containers da mesma imagem (`emulacao/Dockerfile`). Volumes separados: `dados_vaijunto`, `dados_sudoeste`, `dados_norteminas`.

Na mesma maquina, eles se acham pelo nome do servico (`http://sudoeste:5002`). No laboratorio, um container por PC: troca `URL_PROPRIA` e `PARES` pelo IP da placa. `127.0.0.1` nao atravessa computador. `HOST=0.0.0.0` pra aceitar conexao de fora.

Pra provocar a falha na apresentacao: `docker stop sudoeste` (ou fechar o terminal dela, se estiver sem Docker). Mostrar que `GET` de Cachoeira na vaijunto ainda responde, e que reservar Salvador -> Montes Claros falha e nao deixa vaga presa. Religar: `docker start sudoeste`. Ela le `/data/estado.json` e `AoSubir` continua de onde a fase parou.
