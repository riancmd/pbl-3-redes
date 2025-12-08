package main

import (
	"os"
	handlers "pbl-2-redes/internal/handlers/http"
	"pbl-2-redes/internal/infrastructure/cluster"
	"pbl-2-redes/internal/repositories"
	"pbl-2-redes/internal/usecases"
	"strconv"
)

// Ao iniciar o programa, utilizar a linha de comando "go run . PORT",
// onde PORT deve ser substituido pelo inteiro que representa a porta do server

func main() {
	// Configuração dos peers
	allPeerAddresses := []int{
		7700,
		7701,
		7702,
		7703,
		7704,
	}

	myPeers := []int{} // Mantém vazia a lista, pois ainda não não sabe quem são

	// Configuração da porta
	port, err := strconv.Atoi(os.Args[1])

	if err != nil {
		panic(err)
	}

	// Adiciona na lista de peers os que não são minha porta
	for _, address := range allPeerAddresses {
		if address != port {
			myPeers = append(myPeers, address)
		}
	}

	// Configurando a injeção de dependências
	repos := repositories.New() // Repositórios
	// Cria o Client no Cluster
	client := cluster.New(myPeers, port)
	useCases := usecases.New(repos, client) // UseCases

	// Handlers
	h := handlers.New(useCases)
	// adicionar o pubsub!!!!!!!!!!!!!!!!

	go h.Listen(port) // Roda na porta especificada

	// Se for o líder, vai criar o banco de cartas
	if client.IsLeader() {
		// Atualização do banco de dados
		err = useCases.AddCardsFromFile("../../internal/data/cardVault.json", 100000)
	}
	//go client.

	if err != nil {
		panic(err)
	}

}
