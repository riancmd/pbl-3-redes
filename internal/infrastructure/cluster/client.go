package cluster

import (
	"net/http"
	"pbl-2-redes/internal/infrastructure/bully"
	"pbl-2-redes/internal/models"
	"strconv"
	"sync"
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

// Verifica o estado dos outros servidores
func (c *Client) HealthCheck() map[int]int {
	// Cria um mini cliente (no caso, de uso ligeiro) para mandar a request
	healthClient := &http.Client{
		Timeout: 500 * time.Millisecond,
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	results := make(map[int]int)

	wg.Add(len(c.peers)) // Coloca no waitgroup a quantidade de peers

	for _, peer := range c.peers {

	}

	var addresses []string
	// Adiciona todos os endereços dos peers
	for _, peer := range c.peers {
		addresses := append(addresses, "http://localhost:"+strconv.Itoa(peer)+"/health")
	}

	// Define timeout e número de trablhadores
	timeoutMs := 500 // em ms
	numWorkers := 5

	jobs := make(chan string, len(addresses))
	results := make(chan models.CheckResult, len(addresses))

	// Começa a goroutine dos trabalhadores
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go worker(i, jobs, results, timeoutMs, &wg)
	}

}
