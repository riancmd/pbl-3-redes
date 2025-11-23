package cluster

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"pbl-2-redes/internal/infrastructure/blockchain"
	"pbl-2-redes/internal/infrastructure/bully"
	"strconv"
	"time"
)

// Representa o Client (outbound) daquele servidor específico dentro do Cluster
type Client struct {
	peers         []int
	bullyElection *bully.BullyElection
	httpClient    *http.Client
	Blockchain    *blockchain.Blockchain
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

	// Cria uma blockchain
	if client.IsLeader() {
		client.Blockchain = blockchain.New()
	} else {
		client.GetLedger()
	}

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

// Verifica se a blockchain dos peers é maior que a sua
func (c *Client) CheckBlockchainHeight() {
	// dá um GET no endpoint de height de cada peer
	for _, peer := range c.peers {
		resp, err := c.httpClient.Get("http://localhost:" + strconv.Itoa(peer) + "/internal/blockchain/height") // Endereço temporário, resolver

		if err != nil {
			slog.Error(err.Error())
		}

		defer resp.Body.Close()

		var height int

		json.NewDecoder(resp.Body).Decode(&height)

		if height > c.Blockchain.Height {
			resp2, err := c.httpClient.Get("http://localhost:" + strconv.Itoa(peer) + "/internal/blockchain/ledger")
			if err != nil {
				slog.Error(err.Error())
			}

			defer resp2.Body.Close()

			var ledger []*blockchain.Block

			json.NewDecoder(resp2.Body).Decode(&ledger)

			c.Blockchain.UpdateLedger(ledger)
		}
	}
}
