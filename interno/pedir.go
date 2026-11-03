package interno

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// Pedir fala HTTP com outra empresa ou com o servidor, a partir do cliente.
// O bool true no fim significa queda: timeout, conexao recusada, cabo.
// Processo morto e falha no meio da rede chegam iguais. Nao separo os dois.
// Isso e diferente de um 409, que e a outra empresa viva dizendo que nao tem vaga.
func Pedir(metodo, url string, corpo any, timeout time.Duration, destino any) (int, bool) {
	return pedir(metodo, url, "", "", corpo, timeout, destino)
}

// PedirComo e o Pedir com usuario e senha no cabecalho (HTTP Basic).
// A senha nao vai na URL, entao o log do servidor nao mostra ela.
func PedirComo(metodo, url, usuario, senha string, corpo any, timeout time.Duration, destino any) (int, bool) {
	return pedir(metodo, url, usuario, senha, corpo, timeout, destino)
}

// HTTP de verdade. Se o usuario vier preenchido, manda Basic.
// Bool true = queda, ou JSON que nao decodificou. Um 409 com JSON vale false.
func pedir(metodo, url, usuario, senha string, corpo any, timeout time.Duration, destino any) (int, bool) {
	var leitor io.Reader
	if corpo != nil {
		bruto, err := json.Marshal(corpo)
		if err != nil {
			return 0, true
		}
		leitor = bytes.NewReader(bruto)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, metodo, url, leitor)
	if err != nil {
		return 0, true
	}
	if corpo != nil {
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
	}
	if usuario != "" {
		req.SetBasicAuth(usuario, senha)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, true
	}
	defer resp.Body.Close()
	if destino != nil {
		if err := json.NewDecoder(resp.Body).Decode(destino); err != nil {
			return resp.StatusCode, true
		}
	}
	return resp.StatusCode, false
}
