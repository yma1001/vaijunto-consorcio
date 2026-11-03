# VAIJUNTO - problema 2 (consorcio)

Versao em Go do consorcio. Cada empresa e um processo, com o proprio arquivo. O passageiro so fala com a empresa dele.

A pasta `cmd/` tem o servidor, o motorista e o passageiro. O codigo das empresas esta em `interno/`.

Salvador -> Feira de Santana (vaijunto)
Feira de Santana -> Vitoria da Conquista (sudoeste)
Vitoria da Conquista -> Montes Claros (norteminas)

A ultima carona nasce com 1 vaga, de proposito.

Precisa de Go 1.22 ou mais novo. No lab da UEFS ele ja tem. Em casa: https://go.dev/dl/

## Contas

Senha: senha123

- motorista1
- passageiro1
- passageiro2

Cada empresa tem a propria lista. O nome repetido e so pra demo.

Data das caronas prontas: 30/10/2026

## Rodar no Windows (tres terminais)

Na pasta deste trabalho.

Terminal 1

```powershell
$env:EMPRESA="vaijunto"
$env:REGIAO="reconcavo"
$env:PORTA="5001"
$env:URL_PROPRIA="http://127.0.0.1:5001"
$env:PARES="sudoeste|http://127.0.0.1:5002,norteminas|http://127.0.0.1:5003"
$env:ARQUIVO="dados/vaijunto.json"
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
go run ./cmd/servidor
```

Passageiro:

```powershell
go run ./cmd/passageiro
```

Motorista:

```powershell
go run ./cmd/motorista
```

## Teste

```powershell
go test ./interno -count=1
```

Tem que passar. Cobre reserva nas tres empresas, disputa da ultima vaga, cancelamento, queda de uma empresa e vaga que expira.

## Simulador (com os tres servidores ligados)

```powershell
go run ./cmd/simulador 12
```

## Docker

```powershell
cd emulacao
docker compose up --build
```

Cada container tem o seu volume. Nao compartilham arquivo.

No laboratorio, em computadores diferentes: um processo (ou um container) por PC. Em cada um, `URL_PROPRIA` e o IP da propria placa, e `PARES` leva o IP dos outros dois. A porta publicada tem que ser a mesma que esta na URL. O volume (ou o `ARQUIVO`) fica no disco daquele PC. `127.0.0.1` nao atravessa a rede. O simulador, se rodar de um quarto PC, usa `SIM_EMPRESAS=vaijunto|http://IP1:5001,sudoeste|http://IP2:5002,norteminas|http://IP3:5003`.

Eu nao rodei essa cena em tres computadores. Os testes daqui sao na mesma maquina.

## Na apresentacao

O texto dos 11 pontos do barema esta em `docs/relatorio.md`. A API e o diagrama de sequencia estao em `docs/api.md`. O artigo no formato da SBC ainda e um texto a parte.

Pra derrubar uma empresa no meio, na pasta `emulacao`: `docker compose stop sudoeste`. Sem Docker, fecha o terminal dela. A busca Salvador -> Cachoeira tem que continuar. A viagem ate Montes Claros tem que falhar sem baixar vaga. Pra voltar: `docker compose start sudoeste`, ou sobe o processo de novo no mesmo arquivo.

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
