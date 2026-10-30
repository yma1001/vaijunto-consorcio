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
// Isso e diferente de um 409, que e a outra empresa viva dizendo que nao tem vaga.
func Pedir(metodo, url string, corpo any, timeout time.Duration, destino any) (int, bool) {
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
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, true
	}
	defer resp.Body.Close()
	if destino != nil {
		json.NewDecoder(resp.Body).Decode(destino)
	}
	return resp.StatusCode, false
}
