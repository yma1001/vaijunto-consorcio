# Imagem de UMA empresa. O compose sobe tres containers iguais.
# Muda so o nome, a regiao, a porta e o arquivo.
# Nao instala banco. A coordenacao esta no nosso codigo.

FROM golang:1.22

WORKDIR /app
COPY . .
RUN go build -o /servidor ./cmd/servidor

CMD ["/servidor"]
