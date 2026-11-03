package interno

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

var entrada = bufio.NewReader(os.Stdin)

// Imprime o rotulo e le ate o enter.
func lerLinha(rotulo string) string {
	fmt.Print(rotulo)
	s, err := entrada.ReadString('\n')
	if err != nil && strings.TrimSpace(s) == "" {
		fmt.Println()
		os.Exit(0)
	}
	return strings.TrimSpace(s)
}

// Se vier dia/mes/ano, devolve aaaa-mm-dd. Sem barra, devolvo como veio.
func ArrumarData(texto string) string {
	texto = strings.TrimSpace(texto)
	if strings.Contains(texto, "/") {
		pedacos := strings.Split(texto, "/")
		if len(pedacos) != 3 {
			return texto
		}
		dia := pedacos[0]
		mes := pedacos[1]
		ano := pedacos[2]
		if len(dia) == 1 {
			dia = "0" + dia
		}
		if len(mes) == 1 {
			mes = "0" + mes
		}
		return ano + "-" + mes + "-" + dia
	}
	return texto
}

// Chama falarComo sem usuario. Queda imprime que nao falou com o servidor.
func falar(metodo, endereco string, corpo any, destino any) bool {
	return falarComo(metodo, endereco, "", "", corpo, destino)
}

// HTTP com 8 segundos. Se o usuario vier, manda Basic.
// 401 imprime usuario ou senha invalido.
func falarComo(metodo, endereco, usuario, senha string, corpo any, destino any) bool {
	var status int
	var erroRede bool
	if usuario == "" {
		status, erroRede = Pedir(metodo, endereco, corpo, 8*time.Second, destino)
	} else {
		status, erroRede = PedirComo(metodo, endereco, usuario, senha, corpo, 8*time.Second, destino)
	}
	if erroRede || destino == nil {
		fmt.Println("nao consegui falar com o servidor em " + endereco)
		return false
	}
	if status == 401 {
		fmt.Println("usuario ou senha invalido")
		return false
	}
	return true
}

// Monta a URL com a query escapada.
func urlComQuery(base, caminho string, params map[string]string) string {
	q := url.Values{}
	for k, v := range params {
		q.Set(k, v)
	}
	return strings.TrimRight(base, "/") + caminho + "?" + q.Encode()
}

// Pergunta o endereco. Enter vazio vira http://127.0.0.1:5001.
func pedirBase() string {
	base := lerLinha("url da empresa [http://127.0.0.1:5001]: ")
	if base == "" {
		base = "http://127.0.0.1:5001"
	}
	return strings.TrimRight(base, "/")
}

// Pede a saude e imprime empresa e regiao.
func mostrarSaude(base string) {
	var saude struct {
		OK      bool   `json:"ok"`
		Empresa string `json:"empresa"`
		Regiao  string `json:"regiao"`
	}
	if falar("GET", base+"/consorcio/saude", nil, &saude) && saude.OK {
		fmt.Println("conectado em", saude.Empresa, "regiao", saude.Regiao)
	}
}

// Menu de entrar ou cadastrar como motorista. Conta de passageiro eu recuso.
func RodarMotorista() {
	fmt.Println("cliente motorista do vaijunto (consorcio)")
	fmt.Println("fala SO com a sua empresa")
	base := pedirBase()
	mostrarSaude(base)

	for {
		fmt.Println()
		fmt.Println("1 entrar")
		fmt.Println("2 criar conta")
		fmt.Println("0 sair")
		op := lerLinha("> ")
		if op == "0" {
			return
		}
		if op != "1" && op != "2" {
			fmt.Println("opcao invalida")
			continue
		}
		usuario := lerLinha("usuario: ")
		senha := lerLinha("senha: ")
		if op == "2" {
			var resp Resposta
			if !falar("POST", base+"/cadastrar", map[string]string{
				"usuario": usuario, "senha": senha, "papel": "motorista",
			}, &resp) || !resp.OK {
				if resp.Erro != "" {
					fmt.Println("erro:", resp.Erro)
				}
				continue
			}
			fmt.Println("conta criada")
		}
		var resp Resposta
		if !falar("POST", base+"/entrar", map[string]string{"usuario": usuario, "senha": senha}, &resp) || !resp.OK {
			if resp.Erro != "" {
				fmt.Println("erro:", resp.Erro)
			}
			continue
		}
		if resp.Papel != "motorista" {
			fmt.Println("essa conta e de passageiro, abre o outro cliente")
			continue
		}
		menuMotorista(base, usuario, senha)
	}
}

// Publicar carona, listar as minhas, ver passageiros, ou sair.
func menuMotorista(base, usuario, senha string) {
	for {
		fmt.Println()
		fmt.Println("motorista", usuario)
		fmt.Println("1 publicar carona")
		fmt.Println("2 minhas caronas")
		fmt.Println("3 passageiros de uma carona")
		fmt.Println("4 cancelar carona")
		fmt.Println("0 sair")
		op := lerLinha("> ")
		if op == "1" {
			publicar(base, usuario, senha)
		} else if op == "2" {
			minhasCaronas(base, usuario, senha)
		} else if op == "3" {
			verPassageiros(base, usuario, senha)
		} else if op == "4" {
			cancelarCarona(base, usuario, senha)
		} else if op == "0" {
			return
		} else {
			fmt.Println("opcao invalida")
		}
	}
}

// Le cidades, data, hora, vagas e precos, e manda POST /caronas.
func publicar(base, usuario, senha string) {
	fmt.Println("cidades na ordem, separadas por virgula")
	fmt.Println("exemplo: Salvador, Feira de Santana")
	texto := lerLinha("cidades: ")
	cidades := make([]string, 0)
	for _, parte := range strings.Split(texto, ",") {
		parte = strings.TrimSpace(parte)
		if parte != "" {
			cidades = append(cidades, parte)
		}
	}
	if len(cidades) < 2 {
		fmt.Println("precisa de duas cidades no minimo")
		return
	}
	data := ArrumarData(lerLinha("data (dd/mm/aaaa): "))
	hora := lerLinha("hora (hh:mm): ")
	vagas, err := strconv.Atoi(lerLinha("vagas no carro: "))
	if err != nil {
		fmt.Println("vagas tem que ser numero")
		return
	}
	fmt.Println("um preco em reais por trecho. exemplo: 25, 40")
	textoPreco := lerLinha("precos: ")
	precos := make([]int, 0)
	for _, parte := range strings.Split(textoPreco, ",") {
		parte = strings.TrimSpace(parte)
		if parte == "" {
			continue
		}
		n, err := strconv.Atoi(parte)
		if err != nil {
			fmt.Println("preco invalido:", parte)
			return
		}
		precos = append(precos, n)
	}
	var resp Resposta
	if !falar("POST", base+"/caronas", map[string]any{
		"usuario": usuario,
		"senha":   senha,
		"cidades": cidades,
		"data":    data,
		"hora":    hora,
		"vagas":   vagas,
		"precos":  precos,
	}, &resp) || !resp.OK {
		if resp.Erro != "" {
			fmt.Println("erro:", resp.Erro)
		}
		return
	}
	if resp.Carona != nil {
		fmt.Println("publiquei", resp.Carona.ID)
	}
}

// Lista as caronas deste motorista. A senha vai no Basic, nao na URL.
func minhasCaronas(base, usuario, senha string) {
	var resp Resposta
	if !falarComo("GET", urlComQuery(base, "/minhas-caronas", map[string]string{"usuario": usuario}), usuario, senha, nil, &resp) || !resp.OK {
		if resp.Erro != "" {
			fmt.Println("erro:", resp.Erro)
		}
		return
	}
	if len(resp.Caronas) == 0 {
		fmt.Println("nenhuma carona")
		return
	}
	for _, c := range resp.Caronas {
		fmt.Println("---")
		fmt.Println(c.ID, c.Status, c.Data, c.Hora)
		fmt.Println(strings.Join(c.Cidades, " -> "), " vagas:", c.Vagas)
		for _, t := range c.Trechos {
			fmt.Println("   ", t.Origem, "->", t.Destino, "R$", t.Preco, "livres", t.VagasLivres)
		}
	}
}

// Pede o id da carona e mostra quem confirmou em cada trecho.
func verPassageiros(base, usuario, senha string) {
	id := lerLinha("id da carona: ")
	var resp struct {
		OK      bool             `json:"ok"`
		Erro    string           `json:"erro"`
		Trechos []trechoComGente `json:"trechos"`
	}
	if !falarComo("GET", urlComQuery(base, "/passageiros", map[string]string{
		"usuario": usuario, "id_carona": id,
	}), usuario, senha, nil, &resp) || !resp.OK {
		if resp.Erro != "" {
			fmt.Println("erro:", resp.Erro)
		}
		return
	}
	for _, t := range resp.Trechos {
		fmt.Println(t.Origem, "->", t.Destino, "livres", t.VagasLivres)
		if len(t.Passageiros) == 0 {
			fmt.Println("   ninguem")
		}
		for _, p := range t.Passageiros {
			fmt.Println("   ", p.Passageiro, "(empresa", p.Empresa+")")
		}
	}
}

// Igual ao motorista, com papel passageiro.
func RodarPassageiro() {
	fmt.Println("cliente passageiro do vaijunto (consorcio)")
	fmt.Println("fala SO com a sua empresa. a viagem pode usar carona das outras.")
	base := pedirBase()
	mostrarSaude(base)

	for {
		fmt.Println()
		fmt.Println("1 entrar")
		fmt.Println("2 criar conta")
		fmt.Println("0 sair")
		op := lerLinha("> ")
		if op == "0" {
			return
		}
		if op != "1" && op != "2" {
			fmt.Println("opcao invalida")
			continue
		}
		usuario := lerLinha("usuario: ")
		senha := lerLinha("senha: ")
		if op == "2" {
			var resp Resposta
			if !falar("POST", base+"/cadastrar", map[string]string{
				"usuario": usuario, "senha": senha, "papel": "passageiro",
			}, &resp) || !resp.OK {
				if resp.Erro != "" {
					fmt.Println("erro:", resp.Erro)
				}
				continue
			}
			fmt.Println("conta criada")
		}
		var resp Resposta
		if !falar("POST", base+"/entrar", map[string]string{"usuario": usuario, "senha": senha}, &resp) || !resp.OK {
			if resp.Erro != "" {
				fmt.Println("erro:", resp.Erro)
			}
			continue
		}
		if resp.Papel != "passageiro" {
			fmt.Println("essa conta e de motorista, abre o outro cliente")
			continue
		}
		menuPassageiro(base, usuario, senha)
	}
}

// Buscar, reservar uma opcao, ver reservas, cancelar, ou sair.
func menuPassageiro(base, usuario, senha string) {
	var ultima []Itinerario
	var chave string
	for {
		fmt.Println()
		fmt.Println("passageiro", usuario)
		fmt.Println("1 buscar")
		fmt.Println("2 reservar a opcao da ultima busca")
		fmt.Println("3 minhas reservas")
		fmt.Println("4 cancelar reserva")
		fmt.Println("0 sair")
		op := lerLinha("> ")
		if op == "1" {
			ultima = buscar(base)
		} else if op == "2" {
			reservarOpcao(base, usuario, senha, &chave, ultima)
		} else if op == "3" {
			minhasReservas(base, usuario, senha)
		} else if op == "4" {
			cancelar(base, usuario, senha)
		} else if op == "0" {
			return
		} else {
			fmt.Println("opcao invalida")
		}
	}
}

// Pede origem, destino e data. Devolve as opcoes, sem reservar.
func buscar(base string) []Itinerario {
	origem := lerLinha("origem: ")
	destino := lerLinha("destino: ")
	data := ArrumarData(lerLinha("data (dd/mm/aaaa): "))
	var resp Resposta
	if !falar("GET", urlComQuery(base, "/buscar", map[string]string{
		"origem": origem, "destino": destino, "data": data,
	}), nil, &resp) || !resp.OK {
		if resp.Erro != "" {
			fmt.Println("erro:", resp.Erro)
		}
		return nil
	}
	if len(resp.Itinerarios) == 0 {
		fmt.Println("nenhum itinerario")
		return nil
	}
	for i, it := range resp.Itinerarios {
		fmt.Println()
		fmt.Println("opcao", i+1, "- R$", it.PrecoTotal, "- trocas:", it.Trocas)
		for _, t := range it.Trechos {
			fmt.Println("   ", t.Origem, "->", t.Destino, "|", t.Empresa, "("+t.Regiao+")", "|", t.Data, t.Hora, "| R$", t.Preco, "| vagas", t.Vagas)
		}
	}
	return resp.Itinerarios
}

// Manda os trechos da opcao escolhida e guarda a chave da tentativa.
// Se voltar 202, a mesma chave pergunta de novo. Nao e recusa.
func reservarOpcao(base, usuario, senha string, chave *string, ultima []Itinerario) {
	if len(ultima) == 0 {
		fmt.Println("busca primeiro")
		return
	}
	n, err := strconv.Atoi(lerLinha("numero da opcao: "))
	if err != nil || n < 1 || n > len(ultima) {
		fmt.Println("nao tem essa opcao")
		return
	}
	// mando os trechos de volta. o servidor confere a vaga de novo.
	// se a tentativa anterior ainda nao fechou, repito a mesma chave.
	if *chave != "" {
		fmt.Println("tentativa pendente", *chave, ". consulto ela antes de uma compra nova")
	} else {
		*chave = fmt.Sprintf("t-%d", time.Now().UnixNano())
	}
	var resp Resposta
	status, erroRede := Pedir("POST", base+"/reservar", map[string]any{
		"usuario":      usuario,
		"senha":        senha,
		"id_tentativa": *chave,
		"trechos":      ultima[n-1].Trechos,
	}, 15*time.Second, &resp)
	if erroRede {
		fmt.Println("nao consegui falar com o servidor em " + base)
		return
	}
	if status == 202 {
		fmt.Println("pendente. id do pedido:", resp.Mensagem)
		fmt.Println("a proxima reserva repete esta chave")
		return
	}
	*chave = ""
	if !resp.OK {
		if resp.Erro != "" {
			fmt.Println("erro:", resp.Erro)
		}
		return
	}
	if resp.Reserva != nil {
		fmt.Println("reserva", resp.Reserva.ID, "confirmada. total R$", resp.Reserva.PrecoTotal)
	}
}

// Lista as reservas deste passageiro. A senha vai no Basic.
func minhasReservas(base, usuario, senha string) {
	var resp Resposta
	if !falarComo("GET", urlComQuery(base, "/minhas-reservas", map[string]string{"usuario": usuario}), usuario, senha, nil, &resp) || !resp.OK {
		if resp.Erro != "" {
			fmt.Println("erro:", resp.Erro)
		}
		return
	}
	if len(resp.Reservas) == 0 {
		fmt.Println("nenhuma reserva")
		return
	}
	for _, r := range resp.Reservas {
		fmt.Println("---")
		fmt.Println(r.ID, r.Status, "R$", r.PrecoTotal)
		for _, t := range r.Trechos {
			fmt.Println("   ", t.Origem, "->", t.Destino, "|", t.Empresa)
		}
	}
}

// Pede o id e manda POST /cancelar-reserva.
func cancelar(base, usuario, senha string) {
	id := lerLinha("id da reserva: ")
	var resp Resposta
	if !falar("POST", base+"/cancelar-reserva", map[string]string{
		"usuario": usuario, "senha": senha, "id_reserva": id,
	}, &resp) || !resp.OK {
		if resp.Erro != "" {
			fmt.Println("erro:", resp.Erro)
		}
		return
	}
	fmt.Println("cancelada")
}

// Pede o id e manda POST /cancelar-carona.
func cancelarCarona(base, usuario, senha string) {
	id := lerLinha("id da carona: ")
	var resp Resposta
	if !falar("POST", base+"/cancelar-carona", map[string]string{
		"usuario": usuario, "senha": senha, "id_carona": id,
	}, &resp) || !resp.OK {
		if resp.Erro != "" {
			fmt.Println("erro:", resp.Erro)
		}
		return
	}
	fmt.Println("carona cancelada")
}
