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
	"sync"
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
		channel := make(chan int)
		client.Blockchain = &blockchain.Blockchain{Height: 0, Ledger: []*blockchain.Block{}, MPool: []models.Transaction{}, IncomingBlocks: make(chan blockchain.BlockTask), StateChan: &channel, MX: sync.Mutex{}}
		client.Nakamoto()
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
			c.Blockchain.Height = len(ledger)
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

// Opera a blockchain (go routine responsável)
func (c *Client) StartBlockchain() {
	go c.Blockchain.RunBlockchain()
}

func (c *Client) AddNewBlock(block *blockchain.Block) error {
	// recebe do usecases, joga no canal
	// e ai esse canal será verificado pela goroutine rodando NA BLOCKCHAIN

	// cria uma struct que tem o bloco e uma função
	// para executar logo depois de verificar se rolou ou não

	// cria um canal onde o RunBlockchain pode conversar
	resposta := make(chan error, 1)

	// cria a tarefa que vai ser enviada (bloco p verificacao)
	task := blockchain.BlockTask{
		Block: block,
		OnFinish: func(e error) {
			resposta <- e
		},
	}

	// envia
	c.Blockchain.IncomingBlocks <- task

	// espera a resposta
	select {
	case err := <-resposta:
		return err
	case <-time.After(5 * time.Second):
		return errors.New("timeout na validação")
	}
}

func (c *Client) AddNewTransaction(t models.Transaction) error {
	err := c.Blockchain.AddTransaction(t)

	if err != nil {
		return err
	}
	return nil
}

// Pega blockchain
func (c *Client) GetLedger() []blockchain.Block {
	var ledger []blockchain.Block

	for _, t := range c.Blockchain.Ledger {
		ledger = append(ledger, *(t))
	}
	return ledger
}
