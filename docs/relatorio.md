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

Sem resposta no `segurar`, o coordenador trata como falha daquela empresa e aborta o pedido (`AbortarPedido`). Sem resposta no `confirmar`, depois que a fase `confirmando` ja esta no disco, nao aborta: fica pendente e repete (`entregarConfirmacao`). O id da transacao e o `id_pedido`. Repetir o confirmar nao baixa a vaga outra vez.

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
4. Grava fase `confirmando` antes de mandar a rede. Se essa gravacao falhar, a fase volta para `segurando`.
5. `POST /confirmar`. Aí a dona baixa 1. O voto `segurado` nao expira nessa hora. Repetir nao baixa de novo.
6. Se a resposta do confirmar nao volta, ou se o estado que volta nao e o esperado, a fase continua `confirmando`. Nao viro isso em aborto. A limpeza manda o confirmar outra vez. O cliente recebe HTTP 202.
7. Grava a reserva local como `confirmada` so depois que todo mundo respondeu.

Diagrama de sequencia: `docs/api.md`, secao "Sequencia: reserva com tres empresas".

## 6. Precedencia

Criterio: menor relogio de Lamport. Empate: nome da empresa coordenadora, depois o id do pedido (`decidirQuemFicaComAVaga`). O relogio esta em `interno/relogio.go`. Nao usa o relogio de parede. `Tic` soma 1 antes de sair o pedido. `Receber` fica com o maior e soma 1. Os PCs podem estar com a hora diferente e o pacote pode atrasar: o carimbo vai dentro da mensagem.

Na demo os dois comecam em 0, os dois carimbam 1, "sudoeste" ganha de "vaijunto".

Limitacao, pra nao ser pego: a janela e curta (250 ms). Se o carimbo melhor chegar depois que o outro ja foi marcado `segurado`, fica com quem ja segurou. Depois de `confirmado`, relogio nao tira a vaga.

## 7. Deadlock distribuido

Dois passageiros podem pedir os mesmos trechos em ordens diferentes. Sem uma ordem combinada, um segura a vaijunto e espera a sudoeste, o outro faz o contrario, e os dois ficam presos.

`agruparPorEmpresa` ordena o nome (`sort.Strings`). Todo coordenador pede norteminas, depois sudoeste, depois vaijunto. O segundo encontra a vaga ocupada e desiste, em vez de segurar o proximo e esperar.

Consequencia: os `segurar` vao um depois do outro. Uma reserva de tres empresas espera a soma das janelas mais a rede. Quem perde a vaga aborta o que ja segurou. Ele para. Nao tenta de novo em loop, senao viraria livelock. A `Trava` local nao fica segurada durante o HTTP nem durante o `Sleep`.

## 8. Tolerancia a falhas

A falha e timeout em `Pedir` (`TIMEOUT_SEGUNDOS`, padrao 3). Nao tem detector central. Processo morto e cabo partido chegam do mesmo jeito. Nao da pra separar os dois olhando so o timeout.

Antes da confirmacao: a busca nao grava nada. Se o cliente fecha, nao existe pedido.

Durante o `segurar`: quem ja segurou recebe `desfazer`. A vaga livre nao foi baixada. As empresas que nao participam dessa reserva seguem atendendo. O teste `TestEmpresaCai` mata a sudoeste e confere isso.

Durante o `confirmar`: a fase `confirmando` ja esta no disco. Dali em diante eu so repito o confirmar. Nao aborto. Timeout e resposta estranha ficam pendentes (HTTP 202). Quem ja baixou a vaga responde de novo sem baixar outra.

Se o coordenador de um voto `segurado` ficar mudo, essa vaga continua ocupada. Uma viagem de outra empresa, que nao depende dele, segue normal. O teste `TestCoordenadorMudo` mostra isso.

Depois de `confirmada`: o assento fica no JSON de cada dono. Cair uma empresa nao apaga o que as outras ja confirmaram. Quem nao depende dela continua buscando e reservando.

## 9. Recuperacao e persistencia

O que vai pro disco (`gravacao` em `estado.go`): usuarios, caronas, reservas, seguros, pedidos (com a fase), contador e relogio. Pares e trava nao.

Momento: `Salvar` e chamado antes de mandar `segurar`, de novo ao mudar pra `confirmando`, de novo ao confirmar ou abortar, e em cada mudanca de seguro. Escreve `arquivo.tmp`, da `Sync`, troca o nome. Queda no meio do write deixa o JSON anterior inteiro.

Ao voltar, `Rodar` carrega o arquivo, sobe a limpeza e chama `AoSubir` (`interno/recuperacao.go`):

- `segurando`: aborta e desfaz
- `confirmando`: confirma de novo. Timeout deixa a fase quieta. Repetir nao baixa duas vezes
- `abortando` ou `cancelando`: repete ate todo mundo responder. So entao grava `abortada` ou `cancelada`
- `confirmada`: nao mexe

Se a escrita de `abortando` falha, a fase volta a `segurando` e o desfazer nao sai. O laco de limpeza tenta esse pedido de novo quando o disco aceita, sem reiniciar o processo.

Seguro `esperando` vencido: solta, porque a vaga ainda nao foi prometida. Seguro `segurado` vencido: pergunta `GET /consorcio/pedido` no coordenador. `confirmado` confirma. `abortado` ou `desconhecido` solta. Se o coordenador nao responder, espera. Nao solta no silencio.

## 10. Testes

Automatizado: `go test ./interno -count=1`

| teste | o que olha |
|---|---|
| `TestJSONRuim` | JSON quebrado volta 400 e nao mexe vaga |
| `TestBuscaEReserva` | itinerario das tres empresas, baixa 1 em cada, o segundo falha, Cachoeira continua 2 |
| `TestCancelarDevolve` | cancelar devolve uma vez so |
| `TestEmpresaCai` | derruba a sudoeste antes da reserva: a viagem falha e a vaga livre nao muda |
| `TestSeguroExpira` | segurou e o coordenador nao tem o pedido: depois do prazo a vaga volta |
| `TestDisputaSimultanea` | dois servidores ao mesmo tempo na ultima vaga: exatamente um ganha |
| `TestAckPerdido` | a norteminas grava o confirmar e a resposta se perde. A vaijunto fica `confirmando`. Ao religar a norteminas, a reserva fecha e a vaga baixa uma vez |
| `TestTrechoDuplicadoESenha` | trecho repetido nao reserva, sem senha nao reserva, o mesmo `id_tentativa` nao compra dois assentos |
| `TestMesmaTentativa` | dez chamadas ao mesmo tempo com a mesma chave: uma reserva |
| `TestPrazoNaoAbortaConfirmacao` | janela maior que o prazo: a confirmacao nao vira aborto |
| `TestGravacaoFalha` | se o disco falha, a fase na memoria volta e o arquivo antigo fica |
| `TestCoordenadorMudo` | voto prometido fica preso se o coordenador cai; outra empresa ainda reserva |
| `TestDiscoVoltaSemReiniciar` | falhou ao gravar a decisao: o pedido aborta quando o disco volta, sem reiniciar |
| `TestAbortoDiscoVolta` | resposta do segurar se perde e a escrita de abortando falha: a vaga volta com os tres processos vivos |
| `TestDesfazerSoCoordenador` | sem credencial 401, senha da conta 401, outra empresa 403, a coordenadora repete o confirmar |
| `TestCargaComQueda` | 18 passageiros, a sudoeste cai no meio e volta: sem pendencia, e o saldo de cada trecho bate com os titulares |

O que o checklist pede, e qual teste mostra:

- venda dupla do mesmo assento: `TestDisputaSimultanea` e `TestMesmaTentativa`. Tem que sobrar um so.
- reserva confirmada pela metade: `TestAckPerdido` e `TestCargaComQueda`. No fim nao fica fase `confirmando` sem reserva, nem vaga baixada sem titular.
- assento bloqueado depois da queda e da volta: `TestEmpresaCai`, `TestCargaComQueda`, `TestDiscoVoltaSemReiniciar` e `TestAbortoDiscoVolta`.
- desempenho: a suite inteira, nesta maquina, `go test ./interno -count=1` em 11,1s e `go test -race -count=1 ./...` em 13,3s. O simulador imprime a duracao da carga quando roda na hora.

Carga na apresentacao: `go run ./cmd/simulador 12`, com os tres servidores ja nas portas 5001–5003. O usuario muda a cada execucao. `SIM_EMPRESAS` troca o endereco, no formato `nome|url,nome|url`. A carona de Montes Claros nasce com 1 vaga. O resultado esperado e 1 confirmacao. O programa imprime `durou`.

Estes testes rodaram nesta maquina, com os tres processos em portas locais: `go build ./...`, `go vet ./...`, `go test ./interno -count=1` (ok, 11,1s) e `go test -race -count=1 ./...` (ok, 13,3s). Ainda falta repetir a queda no laboratorio, em tres computadores, e escrever o artigo no formato da SBC. Isso nao esta neste repositorio.

## 11. Emulacao

`emulacao/docker-compose.yml` sobe tres containers da mesma imagem (`emulacao/Dockerfile`). Volumes separados: `dados_vaijunto`, `dados_sudoeste`, `dados_norteminas`.

Na mesma maquina, eles se acham pelo nome do servico (`http://sudoeste:5002`). No laboratorio, um container por PC: troca `URL_PROPRIA` e `PARES` pelo IP da placa. `127.0.0.1` nao atravessa computador. `HOST=0.0.0.0` pra aceitar conexao de fora.

Pra provocar a falha na apresentacao, da pasta `emulacao`: `docker compose stop sudoeste` (ou fechar o terminal dela, se estiver sem Docker). Mostrar que a busca Salvador -> Cachoeira na vaijunto ainda responde, e que reservar Salvador -> Montes Claros falha sem baixar `vagas_livres`. Religar: `docker compose start sudoeste`. Ela le o volume dela e `AoSubir` continua da fase gravada. Os tres PCs do laboratorio eu ainda nao liguei juntos. O artigo da SBC fica fora deste codigo.
