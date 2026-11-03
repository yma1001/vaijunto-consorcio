# VAIJUNTO - problema 2 (consorcio)

Versao em Go do consorcio. Cada empresa e um processo, com o proprio arquivo. O passageiro so fala com a empresa dele.

A pasta `cmd/` tem o servidor, o motorista e o passageiro. O codigo das empresas esta em `interno/`.

Salvador -> Feira de Santana (vaijunto)
Feira de Santana -> Vitoria da Conquista (sudoeste)
Vitoria da Conquista -> Montes Claros (norteminas)

A ultima carona nasce com 1 vaga, de proposito.

A tag `laboratorio` aponta para o commit que rodou no laboratorio em 03/11/2026 (`46608bf`). Se uma mudanca depois quebrar, `git checkout laboratorio` volta nesse codigo.

Em casa, Go 1.22 ou mais novo: https://go.dev/dl/

No laboratorio da UEFS o Go muda de maquina. Dentro da pasta, `go version` as vezes tenta baixar o Go 1.22 e responde `toolchain not available`. Nesse caso:

```bash
export GOTOOLCHAIN=local
go version
```

Se a versao local for 1.21, o `go.mod` pede 1.22 e o PC nao baixa. Troca so a linha da versao e segue:

```bash
sed -i 's/^go 1.22$/go 1.21/' go.mod
export GOTOOLCHAIN=local
```

Assim rodou no larsid06 (Go 1.21.6) e no ladica09, ladica11 e ladica12 (Go 1.25.1). Cada terminal novo precisa do `export GOTOOLCHAIN=local` de novo. O `sed` e uma vez por copia da pasta.

## Contas

Senha: senha123

- motorista1
- passageiro1
- passageiro2

Cada empresa tem a propria lista. O nome repetido e so pra demo. Criar `yago` na vaijunto grava so no arquivo dela. Na sudoeste e na norteminas esse login nao entra. A reserva tambem fica na empresa em que o passageiro entrou. A viagem e que pode usar carona das outras.

O valor `SEGREDO` e outro. Ele vai no cabecalho das rotas entre empresas (segurar, confirmar, desfazer). Nao e esta senha. Sem a variavel, o servidor para na hora. As tres maquinas usam o mesmo valor. Quem souber esse valor pode mandar o nome de outra empresa no usuario: o servidor compara o nome com a coordenadora gravada no pedido, e nao olha de qual maquina veio o pacote.

Data das caronas prontas: 30/10/2026

Regiao (`reconcavo`, `sudoeste`, `norte-minas`) e o rotulo da empresa na hora de mostrar o trecho. Nao existe tabela de cidade por regiao. A busca junta trecho pelo nome da cidade. O motorista publica as cidades que quiser.

## Um computador so

Tres terminais na mesma maquina. O endereco e `127.0.0.1`. Deixa cada servidor aberto. O quarto terminal e o cliente.

### Windows

Terminal 1

```powershell
$env:EMPRESA="vaijunto"
$env:REGIAO="reconcavo"
$env:PORTA="5001"
$env:URL_PROPRIA="http://127.0.0.1:5001"
$env:PARES="sudoeste|http://127.0.0.1:5002,norteminas|http://127.0.0.1:5003"
$env:ARQUIVO="dados/vaijunto.json"
$env:SEGREDO="segredo-lab"
go run ./cmd/servidor
```

Terminal 2

```powershell
$env:EMPRESA="sudoeste"
$env:REGIAO="sudoeste"
$env:PORTA="5002"
$env:URL_PROPRIA="http://127.0.0.1:5002"
$env:PARES="vaijunto|http://127.0.0.1:5001,norteminas|http://127.0.0.1:5003"
$env:ARQUIVO="dados/sudoeste.json"
$env:SEGREDO="segredo-lab"
go run ./cmd/servidor
```

Terminal 3

```powershell
$env:EMPRESA="norteminas"
$env:REGIAO="norte-minas"
$env:PORTA="5003"
$env:URL_PROPRIA="http://127.0.0.1:5003"
$env:PARES="vaijunto|http://127.0.0.1:5001,sudoeste|http://127.0.0.1:5002"
$env:ARQUIVO="dados/norteminas.json"
$env:SEGREDO="segredo-lab"
go run ./cmd/servidor
```

### Linux

Mesmos tres papeis. No Linux o `|` de `PARES` tem que ir entre aspas, senao o shell corta o comando. Se o `go version` da secao de cima pediu, repete `export GOTOOLCHAIN=local` em cada terminal.

Terminal 1, vaijunto:

```bash
export GOTOOLCHAIN=local
export EMPRESA=vaijunto
export REGIAO=reconcavo
export PORTA=5001
export URL_PROPRIA=http://127.0.0.1:5001
export PARES="sudoeste|http://127.0.0.1:5002,norteminas|http://127.0.0.1:5003"
export ARQUIVO=dados/vaijunto.json
export SEGREDO=segredo-lab
go run ./cmd/servidor
```

Terminal 2, sudoeste:

```bash
export GOTOOLCHAIN=local
export EMPRESA=sudoeste
export REGIAO=sudoeste
export PORTA=5002
export URL_PROPRIA=http://127.0.0.1:5002
export PARES="vaijunto|http://127.0.0.1:5001,norteminas|http://127.0.0.1:5003"
export ARQUIVO=dados/sudoeste.json
export SEGREDO=segredo-lab
go run ./cmd/servidor
```

Terminal 3, norteminas:

```bash
export GOTOOLCHAIN=local
export EMPRESA=norteminas
export REGIAO=norte-minas
export PORTA=5003
export URL_PROPRIA=http://127.0.0.1:5003
export PARES="vaijunto|http://127.0.0.1:5001,sudoeste|http://127.0.0.1:5002"
export ARQUIVO=dados/norteminas.json
export SEGREDO=segredo-lab
go run ./cmd/servidor
```

Quando os tres mostrarem `ouvindo`, outro terminal:

```bash
export GOTOOLCHAIN=local
go run ./cmd/passageiro
```

Na pergunta da URL, Enter aceita `http://127.0.0.1:5001`. Isso so vale neste modo, com as tres empresas no mesmo PC.

### Passos, um computador

1. Entrar: `1`, usuario `passageiro1`, senha `senha123`.
2. Buscar: `1`, origem `Salvador`, destino `Montes Claros`, data `30/10/2026`.
3. Tem que aparecer tres trechos: vaijunto, sudoeste e norteminas. A ultima nasce com 1 vaga.
4. Reservar: `2`, depois `1`. A resposta esperada e a reserva confirmada.
5. `3` lista as reservas desse passageiro nesta empresa.
6. Para ver a vaga acabar, sai com `0`, entra como `passageiro2` e busca de novo. Montes Claros fica com 0. A segunda reserva dessa opcao nao confirma.

Motorista, noutro terminal: `go run ./cmd/motorista`. Enter na URL, `1`, `motorista1`, `senha123`. `2` lista as caronas dele. `3` pede o id e mostra quem reservou.

## Tres computadores

Um processo por PC. Nao usa `127.0.0.1` entre maquinas: esse endereco e o proprio computador. O servidor ja escuta em `0.0.0.0`.

Em cada PC, na pasta do projeto:

```bash
hostname -I
```

Anota o endereco da rede do laboratorio. No ladica foi o `172.16.201.x`. Os `172.17`, `172.18`, `172.20` e parecidos sao ponte do Docker e nao alcancam o outro PC.

Exemplo que funcionou em 03/11/2026. Noutro laboratorio troca os tres IPs e mantem o resto.

| PC | IP | empresa | porta |
|---|---|---|---|
| ladica09 | 172.16.201.9 | vaijunto | 5001 |
| ladica11 | 172.16.201.11 | sudoeste | 5002 |
| ladica12 | 172.16.201.12 | norteminas | 5003 |

Se a pasta veio de um zip antigo e ja tem `dados/*.json`, apaga antes, senao a vaga de Montes Claros continua gasta:

```bash
rm -f dados/vaijunto.json dados/sudoeste.json dados/norteminas.json
```

ladica09, vaijunto:

```bash
cd ~/Downloads/vaijunto-consorcio-master
export GOTOOLCHAIN=local
export EMPRESA=vaijunto
export REGIAO=reconcavo
export PORTA=5001
export URL_PROPRIA=http://172.16.201.9:5001
export PARES="sudoeste|http://172.16.201.11:5002,norteminas|http://172.16.201.12:5003"
export ARQUIVO=dados/vaijunto.json
export SEGREDO=segredo-lab
go run ./cmd/servidor
```

ladica11, sudoeste:

```bash
cd ~/Downloads/vaijunto-consorcio-master
export GOTOOLCHAIN=local
export EMPRESA=sudoeste
export REGIAO=sudoeste
export PORTA=5002
export URL_PROPRIA=http://172.16.201.11:5002
export PARES="vaijunto|http://172.16.201.9:5001,norteminas|http://172.16.201.12:5003"
export ARQUIVO=dados/sudoeste.json
export SEGREDO=segredo-lab
go run ./cmd/servidor
```

ladica12, norteminas:

```bash
cd ~/Downloads/vaijunto-consorcio-master
export GOTOOLCHAIN=local
export EMPRESA=norteminas
export REGIAO=norte-minas
export PORTA=5003
export URL_PROPRIA=http://172.16.201.12:5003
export PARES="vaijunto|http://172.16.201.9:5001,sudoeste|http://172.16.201.11:5002"
export ARQUIVO=dados/norteminas.json
export SEGREDO=segredo-lab
go run ./cmd/servidor
```

O `SEGREDO` e o mesmo texto nos tres. A senha das contas continua `senha123`.

### Passos, tres computadores

O cliente abre noutro terminal, no mesmo PC do servidor. Nao digita no terminal que esta `ouvindo`.

Passageiro na vaijunto (ladica09):

```bash
cd ~/Downloads/vaijunto-consorcio-master
export GOTOOLCHAIN=local
go run ./cmd/passageiro
```

Na URL escreve `http://172.16.201.9:5001`. Enter sozinho manda para `127.0.0.1:5001` e falha nos outros PCs. Tem que aparecer `conectado em vaijunto`.

No ladica11 a URL do passageiro ou do motorista e `http://172.16.201.11:5002` (`conectado em sudoeste`). No ladica12 e `http://172.16.201.12:5003` (`conectado em norteminas`).

Viagem inteira, no passageiro da vaijunto: `1`, `passageiro1`, `senha123`, buscar `Salvador`, `Montes Claros`, `30/10/2026`, reservar `2` e `1`. Essa reserva aparece so no cliente da vaijunto. O `passageiro1` da sudoeste e outra conta, com o mesmo nome.

Disputa da ultima vaga, do jeito que fechou no laboratorio:

1. No motorista da norteminas, publica uma carona nova se a de Montes Claros ja estiver com 0. Menu `1`, cidades `Vitoria da Conquista, Montes Claros`, data `30/10/2026`, hora `16:00`, vagas `1`, preco `55`. O acento em Vitoria nao impede a busca: o nome e comparado sem acento.
2. Dois passageiros, um na vaijunto e outro na sudoeste, buscam `Vitoria da Conquista` -> `Montes Claros`, data `30/10/2026`. A opcao tem que mostrar a empresa `norteminas` e `vagas 1`.
3. Os dois reservam `2` e `1`, quase juntos. Um confirma. O outro recebe erro de vaga.
4. No motorista da norteminas, `3` e o id da carona. Aparece um passageiro, com a empresa de quem ganhou.

A carona pronta Feira de Santana -> Vitoria da Conquista tem 3 vagas e e da sudoeste. Ela nao serve para essa disputa.

## Teste automatico

```powershell
go test ./interno -count=1
```

No Linux, com o Go do laboratorio: `export GOTOOLCHAIN=local` antes. Tem que passar. Cobre reserva nas tres empresas, disputa da ultima vaga, cancelamento, queda de uma empresa e vaga que expira.

## Simulador (com os tres servidores ligados)

```powershell
go run ./cmd/simulador 12
```

Num quarto PC, o endereco troca:

```bash
export GOTOOLCHAIN=local
export SIM_EMPRESAS="vaijunto|http://172.16.201.9:5001,sudoeste|http://172.16.201.11:5002,norteminas|http://172.16.201.12:5003"
go run ./cmd/simulador 12
```

## Docker

```powershell
cd emulacao
docker compose up --build
```

Cada container tem o seu volume. Nao compartilham arquivo.

No laboratorio de 03/11/2026 nao usei Docker. Foi um processo por PC, com os IPs da tabela. `127.0.0.1` nao atravessa a rede.

## Na apresentacao

O texto dos 11 pontos do barema esta em `docs/relatorio.md`. A API e o diagrama de sequencia estao em `docs/api.md`. O artigo no formato da SBC ainda e um texto a parte.

Pra derrubar uma empresa no meio, na pasta `emulacao`: `docker compose stop sudoeste`. Sem Docker, fecha o terminal dela com Ctrl+C. A busca Salvador -> Cachoeira tem que continuar. A viagem ate Montes Claros tem que falhar sem baixar vaga, se a queda for antes da decisao gravada. Pra voltar: `docker compose start sudoeste`, ou sobe o processo de novo no mesmo arquivo. Essa queda no meio, com os tres PCs ligados, ainda nao foi o ensaio. O que ja rodou foi busca, reserva e a disputa da ultima vaga.

## Onde esta cada coisa

| arquivo | pra que |
|---|---|
| interno/estado.go | memoria, carona, arquivo JSON |
| interno/rotas.go | HTTP (por baixo e TCP) |
| interno/relogio.go | relogio de Lamport |
| interno/reserva.go | segurar, confirmar, desfazer |
| interno/busca.go | montar o itinerario |
| interno/recuperacao.go | queda e volta |
| interno/cli.go | tela do motorista e do passageiro |
| cmd/ | o main de cada programa, fino, igual o problema 1 |
| docs/api.md | a API REST e o diagrama da reserva |
| docs/relatorio.md | os 11 pontos do barema, com o lugar no codigo |

Preco em real inteiro. Senha em texto. E prototipo.
