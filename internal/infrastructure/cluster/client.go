package cluster

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"pbl-2-redes/internal/infrastructure/blockchain"
	"pbl-2-redes/internal/infrastructure/bully"
	"pbl-2-redes/internal/models"
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
func (c *Client) Nakamoto() {
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

			err = c.Blockchain.UpdateLedger(ledger)

			if err != nil {
				slog.Error(err.Error())
			}
		}
	}
}

// Sincroniza compra de carta
func (c *Client) BuyBooster(transaction models.Transaction) error {
	// Encapsula o dado com JSON
	jsonData, err := json.Marshal(transaction)

	if err != nil {
		return err
	}

	// AQUI, AO INVÉS DE FAZER O REQUEST
	// EU CHAMO UMA FUNÇÃO QUE IRÁ ADICIONAR A TRANSAÇÃO NA POOL DA BLOCKCHAIN
	// ACRESCENTADA A TRANSAÇÃO, PRECISO SÓ VERIFICAR SE AQUELA TRANSAÇÃO É OU NÃO VÁLIDA
	// UTILIZANDO O ID DO BOOSTER

	txErr := c.Blockchain.AddTransaction(transaction)

	if txErr != nil {
		return txErr
	}

	// passa por todos os peers enviando a nova transação
	for _, peer := range c.peers {
		req, err := http.NewRequest(
			http.MethodPost,
			"http://localhost:"+strconv.Itoa(peer)+"/internal/blockchain/mempool/",
			bytes.NewBuffer(jsonData))

		if err != nil {
			return err
		}

		resp, err := c.httpClient.Do(req)

		if err != nil {
			return err
		}

		defer resp.Body.Close()
	}

	return nil
}
