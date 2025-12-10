# Estágio de Build
FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Compila o cliente
RUN go build -o client ./client

# Estágio Final
FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/client .

# O cliente roda interativamente, então o CMD é apenas o ponto de entrada
CMD ["./client"]