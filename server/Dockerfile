# Estágio de Build
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copia dependências
COPY go.mod go.sum ./
RUN go mod download

# Copia todo o código fonte (incluindo internal/, models/, etc)
COPY . .

# Compila o servidor
RUN go build -o server ./server

# Estágio Final (Runtime)
FROM alpine:latest

WORKDIR /root/

# Copia o binário
COPY --from=builder /app/server .

# Copia o banco de dados de cartas (DEVE estar na raiz do projeto local)
COPY cardVault.json .

# Expõe as portas (documentação apenas, o compose que define)
EXPOSE 9090
EXPOSE 8083/udp

CMD ["./server"]