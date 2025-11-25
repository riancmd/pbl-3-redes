package cluster

import (
	"bytes"
	"encoding/json"
	"errors"
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

			err = c.Blockchain.UpdateLedger(ledger)

			if err != nil {
				slog.Error(err.Error())
			}
		}
	}
}

// Sincroniza compra de carta
func (c *Client) BuyBooster(transaction models.Transaction) error {
	valid := blockchain.VerifySignature(transaction.PublicKey, transaction.Data, transaction.Signature)

	if !valid {
		return errors.New("invalid transaction")
	}

	// Encapsula o dado com JSON
	jsonData, err := json.Marshal(boosterID)

	if err != nil {
		return err
	}

	// AQUI, AO INVÉS DE FAZER O REQUEST
	// EU CHAMO UMA FUNÇÃO QUE IRÁ ADICIONAR A TRANSAÇÃO NA POOL DA BLOCKCHAIN
	// ACRESCENTADA A TRANSAÇÃO, PRECISO SÓ VERIFICAR SE AQUELA TRANSAÇÃO É OU NÃO VÁLIDA
	// UTILIZANDO O ID DO BOOSTER

	// Crio a request com HTTP
	req, err := http.NewRequest(
		http.MethodDelete,
		"http://localhost:"+strconv.Itoa(c.bullyElection.GetLeader())+"/internal/cards/{"+strconv.Itoa(boosterID)+"}",
		bytes.NewBuffer(jsonData))

	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	// Verifica a resposta
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusNotFound {
		return errors.New("booster doesn't exist")
	}

	return nil
}
