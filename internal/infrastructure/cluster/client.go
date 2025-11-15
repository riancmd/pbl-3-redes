package cluster

import (
	"net/http"
	"pbl-2-redes/internal/infrastructure/bully"
	"time"
)

// Representa o Client (outbound) daquele servidor específico dentro do Cluster
type Client struct {
	peers         []int
	bullyElection *bully.BullyElection // erro no package por algum motivo [CONSERTAR]
	httpClient    *http.Client
}

// Cria um novo Client no Cluster
func New(allPeers []int, port int) *Client {
	// Guarda lista de peers no cluster
	var myPeers []int

	// Remove a porta da lista
	for _, address := range allPeers {
		if address != port {
			myPeers = append(myPeers, address)
		}
	}

	client := Client{
		peers:         myPeers,
		bullyElection: bully.New(port, myPeers),
		httpClient:    &http.Client{Timeout: 5 * time.Second},
	}

	// Faz eleição
	client.bullyElection.StartElection()

	return &client
}

// Verifica se é líder (uso externo)
func (c *Client) IsLeader() bool {
	return c.bullyElection.IsLeader()
}

// Pego meu ID
func (c *Client) GetServerID() int {
	return c.bullyElection.GetServerID()
}
