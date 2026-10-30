package interno

import (
	"sort"
	"strconv"
	"strings"
)

// Busca o caminho. Nao reserva vaga.
// limite de 3 trechos cobre Salvador -> Feira -> Vitoria -> Montes Claros.
// Nao usei biblioteca de grafo. Com pouca carona, uma recursao da conta.

func NormalizarCidade(nome string) string {
	nome = strings.ToLower(strings.TrimSpace(nome))
	trocas := strings.NewReplacer(
		"á", "a", "à", "a", "ã", "a", "â", "a",
		"é", "e", "ê", "e",
		"í", "i",
		"ó", "o", "ô", "o", "õ", "o",
		"ú", "u",
		"ç", "c",
	)
	return trocas.Replace(nome)
}

func somarPreco(caminho []TrechoViagem) int {
	total := 0
	for _, t := range caminho {
		total = total + t.Preco
	}
	return total
}

func MontarItinerarios(trechos []TrechoViagem, origem, destino string, limite, maximo int) [][]TrechoViagem {
	origemN := NormalizarCidade(origem)
	destinoN := NormalizarCidade(destino)
	if origemN == "" || destinoN == "" || origemN == destinoN {
		return nil
	}

	uteis := make([]TrechoViagem, 0)
	for _, t := range trechos {
		if t.Vagas >= 1 {
			uteis = append(uteis, t)
		}
	}

	achados := make([][]TrechoViagem, 0)
	var andar func(cidade string, caminho []TrechoViagem, usados map[string]bool, cidades map[string]bool)
	andar = func(cidade string, caminho []TrechoViagem, usados map[string]bool, cidades map[string]bool) {
		if len(achados) >= maximo*4 {
			return
		}
		if NormalizarCidade(cidade) == destinoN && len(caminho) > 0 {
			copia := append([]TrechoViagem(nil), caminho...)
			achados = append(achados, copia)
			return
		}
		if len(caminho) >= limite {
			return
		}
		for _, t := range uteis {
			chave := t.Empresa + "|" + t.IDCarona + "|" + strconv.Itoa(t.Indice)
			if usados[chave] {
				// o mesmo trecho fisico duas vezes debitaria o assento duas vezes
				continue
			}
			if NormalizarCidade(t.Origem) != NormalizarCidade(cidade) {
				continue
			}
			destN := NormalizarCidade(t.Destino)
			if cidades[destN] {
				continue
			}
			usados[chave] = true
			cidades[destN] = true
			prox := append(append([]TrechoViagem{}, caminho...), t)
			andar(t.Destino, prox, usados, cidades)
			delete(usados, chave)
			delete(cidades, destN)
		}
	}

	andar(origem, nil, map[string]bool{}, map[string]bool{origemN: true})

	sort.Slice(achados, func(i, j int) bool {
		pi := somarPreco(achados[i])
		pj := somarPreco(achados[j])
		if pi != pj {
			return pi < pj
		}
		return len(achados[i]) < len(achados[j])
	})

	vistos := map[string]bool{}
	saida := make([][]TrechoViagem, 0)
	for _, caminho := range achados {
		chave := ""
		for _, t := range caminho {
			chave = chave + t.Empresa + "|" + t.IDCarona + "|" + strconv.Itoa(t.Indice) + ";"
		}
		if vistos[chave] {
			continue
		}
		vistos[chave] = true
		saida = append(saida, caminho)
		if len(saida) >= maximo {
			break
		}
	}
	return saida
}
