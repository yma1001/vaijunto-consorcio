# API REST do consorcio

Tem dois tipos de conversa.

1. O aplicativo (motorista ou passageiro) fala SO com a empresa dele.
2. As empresas falam entre si. O aplicativo nao chama essas rotas.

Tudo e JSON UTF-8. Se o JSON vier quebrado, a resposta e 400 e nada e gravado.
Codigo HTTP: 200 deu certo, 400 pedido torto, 401 senha errada, 404 nao achei, 409 sem vaga ou conflito.

Nao tem token. O cliente manda o nome do usuario. Prototipo, igual a senha em texto.

## Aplicativo -> a propria empresa

### POST /entrar
```json
{"usuario": "passageiro1", "senha": "senha123"}
```
```json
{"ok": true, "usuario": "passageiro1", "papel": "passageiro", "empresa": "vaijunto", "regiao": "reconcavo"}
```

### POST /cadastrar
```json
{"usuario": "ana", "senha": "123", "papel": "passageiro"}
```
papel: "motorista" ou "passageiro".

### POST /caronas
```json
{"usuario": "motorista1", "cidades": ["Salvador", "Feira de Santana"], "data": "2026-10-30", "hora": "08:00", "vagas": 3, "precos": [25]}
```
precos tem um numero por trecho (quantidade de cidades menos 1). Preco em reais, inteiro.

### POST /cancelar-carona
```json
{"usuario": "motorista1", "id_carona": "carona-vaijunto-1"}
```
Recusa se ja tem passageiro confirmado na carona.

### GET /minhas-caronas?usuario=motorista1

### GET /passageiros?usuario=motorista1&id_carona=carona-vaijunto-1
Devolve, por trecho, quem confirmou. O passageiro pode ser de outra empresa.

### GET /buscar?origem=Salvador&destino=Montes%20Claros&data=2026-10-30
Nao reserva. So lista no maximo 8 caminhos, mais barato primeiro, no maximo 3 trechos.
Cada trecho diz a empresa e a regiao.

### POST /reservar
```json
{"usuario": "passageiro1", "trechos": [ {"empresa": "vaijunto", "id_carona": "carona-vaijunto-1", "indice": 0, "origem": "Salvador", "destino": "Feira de Santana", "preco": 25} ]}
```
O normal e mandar os trechos que a busca devolveu. O servidor confere de novo se ainda tem vaga.

### GET /minhas-reservas?usuario=passageiro1

### POST /cancelar-reserva
```json
{"usuario": "passageiro1", "id_reserva": "reserva-vaijunto-4"}
```
Manda desfazer em todas as empresas do itinerario. Chamar de novo nao devolve a vaga outra vez.

## Empresa -> empresa

### GET /consorcio/saude
```json
{"ok": true, "empresa": "sudoeste", "regiao": "sudoeste", "relogio": 4}
```

### GET /consorcio/trechos?data=2026-10-30
Todos os trechos ativos daquele dia, desta empresa. Nao filtra origem e destino.
Se filtrasse, a baldeacao no meio do caminho sumia.

### GET /consorcio/pedido?id=pedido-vaijunto-3
```json
{"ok": true, "estado": "confirmado"}
```
estado: desconhecido, segurado, pendente, confirmado, abortado.
Serve pra uma empresa que ficou com vaga presa perguntar se o resto do grupo confirmou.

### POST /consorcio/segurar
Ainda nao baixa a vaga. So marca segurado, se ganhar na janela do relogio.
```json
{
  "id_pedido": "pedido-vaijunto-3",
  "relogio": 1,
  "empresa_coord": "vaijunto",
  "url_coord": "http://127.0.0.1:5001",
  "passageiro": "passageiro1",
  "participantes": [
    {"empresa": "norteminas", "url": "http://127.0.0.1:5003"},
    {"empresa": "sudoeste", "url": "http://127.0.0.1:5002"},
    {"empresa": "vaijunto", "url": "http://127.0.0.1:5001"}
  ],
  "trechos": [{"id_carona": "carona-norteminas-1", "indice": 0}]
}
```
200 se segurou. 409 se nao tem vaga.

### POST /consorcio/confirmar
```json
{"id_pedido": "pedido-vaijunto-3", "relogio": 1}
```
Agora baixa vagas_livres. Se esse id ja estava confirmado, responde 200 e nao baixa de novo.

### POST /consorcio/desfazer
```json
{"id_pedido": "pedido-vaijunto-3"}
```
Se so estava segurado, solta. Se ja tinha confirmado, devolve a vaga. A segunda chamada nao devolve de novo.

### POST /consorcio/liberar
A mesma coisa que desfazer. Deixei as duas rotas porque no texto a gente fala "liberar a trava" e "desfazer a confirmacao", mas no codigo e uma funcao so.

## Sequencia: reserva com tres empresas

O passageiro esta na vaijunto. O caminho passa pelas tres.
A ordem de PEDIR a vaga e alfabetica (norteminas, sudoeste, vaijunto), nao a ordem da estrada. Isso evita deadlock.

```
passageiro          vaijunto                 norteminas            sudoeste
    |                  |                         |                    |
    | POST /reservar   |                         |                    |
    |----------------->|                         |                    |
    |                  | grava fase=segurando    |                    |
    |                  | POST /segurar           |                    |
    |                  |------------------------>|                    |
    |                  |        200 segurado     |                    |
    |                  |<------------------------|                    |
    |                  | POST /segurar           |                    |
    |                  |--------------------------------------------->|
    |                  |        200 segurado     |                    |
    |                  |<---------------------------------------------|
    |                  | segurar local (ele mesmo)                    |
    |                  | grava fase=confirmando  |                    |
    |                  | POST /confirmar         |                    |
    |                  |------------------------>| baixa a vaga       |
    |                  | POST /confirmar         |                    |
    |                  |--------------------------------------------->|
    |                  | confirmar local         |                    |
    |                  | grava reserva           |                    |
    |    200 reserva   |                         |                    |
    |<-----------------|                         |                    |
```

Se o segurar da sudoeste der 409 ou timeout:

```
vaijunto grava abortando
vaijunto POST /desfazer na norteminas (solta o que segurou)
vaijunto POST /desfazer nela mesma
vaijunto grava abortada
passageiro recebe erro
nenhuma vaga_livres foi baixada
```

## Precedencia da ultima vaga

Os dois POST /segurar chegam no dono do trecho. Ele espera JANELA_MS
(padrao 250 ms) sem segurar a trava da memoria, junta quem chegou nessa
janela e ordena por (relogio, nome da empresa coordenadora, id do pedido).
O menor ganha. O outro recebe 409. O que perdeu manda desfazer o que
ja tinha segurado nas empresas anteriores da ordem alfabetica.
