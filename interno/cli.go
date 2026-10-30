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

func lerLinha(rotulo string) string {
	fmt.Print(rotulo)
	s, _ := entrada.ReadString('\n')
	return strings.TrimSpace(s)
}

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

func falar(metodo, endereco string, corpo any, destino any) bool {
	_, erroRede := Pedir(metodo, endereco, corpo, 8*time.Second, destino)
	if erroRede || destino == nil {
		fmt.Println("nao consegui falar com o servidor em " + endereco)
		return false
	}
	return true
}

func urlComQuery(base, caminho string, params map[string]string) string {
	q := url.Values{}
	for k, v := range params {
		q.Set(k, v)
	}
	return strings.TrimRight(base, "/") + caminho + "?" + q.Encode()
}

func pedirBase() string {
	base := lerLinha("url da empresa [http://127.0.0.1:5001]: ")
	if base == "" {
		base = "http://127.0.0.1:5001"
	}
	return strings.TrimRight(base, "/")
}

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
		menuMotorista(base, usuario)
	}
}

func menuMotorista(base, usuario string) {
	for {
		fmt.Println()
		fmt.Println("motorista", usuario)
		fmt.Println("1 publicar carona")
		fmt.Println("2 minhas caronas")
		fmt.Println("3 passageiros de uma carona")
		fmt.Println("0 sair")
		op := lerLinha("> ")
		if op == "1" {
			publicar(base, usuario)
		} else if op == "2" {
			minhasCaronas(base, usuario)
		} else if op == "3" {
			verPassageiros(base, usuario)
		} else if op == "0" {
			return
		} else {
			fmt.Println("opcao invalida")
		}
	}
}

func publicar(base, usuario string) {
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

func minhasCaronas(base, usuario string) {
	var resp Resposta
	if !falar("GET", urlComQuery(base, "/minhas-caronas", map[string]string{"usuario": usuario}), nil, &resp) || !resp.OK {
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

func verPassageiros(base, usuario string) {
	id := lerLinha("id da carona: ")
	var resp struct {
		OK      bool             `json:"ok"`
		Erro    string           `json:"erro"`
		Trechos []trechoComGente `json:"trechos"`
	}
	if !falar("GET", urlComQuery(base, "/passageiros", map[string]string{
		"usuario": usuario, "id_carona": id,
	}), nil, &resp) || !resp.OK {
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
		menuPassageiro(base, usuario)
	}
}

func menuPassageiro(base, usuario string) {
	var ultima []Itinerario
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
			reservarOpcao(base, usuario, ultima)
		} else if op == "3" {
			minhasReservas(base, usuario)
		} else if op == "4" {
			cancelar(base, usuario)
		} else if op == "0" {
			return
		} else {
			fmt.Println("opcao invalida")
		}
	}
}

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

func reservarOpcao(base, usuario string, ultima []Itinerario) {
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
	// a busca pode ter ficado velha se outra pessoa reservou no meio.
	var resp Resposta
	if !falar("POST", base+"/reservar", map[string]any{
		"usuario": usuario,
		"trechos": ultima[n-1].Trechos,
	}, &resp) || !resp.OK {
		if resp.Erro != "" {
			fmt.Println("erro:", resp.Erro)
		}
		return
	}
	if resp.Reserva != nil {
		fmt.Println("reserva", resp.Reserva.ID, "confirmada. total R$", resp.Reserva.PrecoTotal)
	}
}

func minhasReservas(base, usuario string) {
	var resp Resposta
	if !falar("GET", urlComQuery(base, "/minhas-reservas", map[string]string{"usuario": usuario}), nil, &resp) || !resp.OK {
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

func cancelar(base, usuario string) {
	id := lerLinha("id da reserva: ")
	var resp Resposta
	if !falar("POST", base+"/cancelar-reserva", map[string]string{
		"usuario": usuario, "id_reserva": id,
	}, &resp) || !resp.OK {
		if resp.Erro != "" {
			fmt.Println("erro:", resp.Erro)
		}
		return
	}
	fmt.Println("cancelada")
}
