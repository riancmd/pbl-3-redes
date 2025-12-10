package blockchain

import (
	"PlanoZ/internal/models"
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"sync"
)

type Blockchain struct {
	Height         int
	Ledger         []*Block
	MPool          []models.Transaction
	IncomingBlocks chan BlockTask // canal pra receber blocos da rede
	StateChan      *chan int      // controle da mineracao
	MX             sync.Mutex     // mutex pra proteger a mempool
}

// inicializa a blockchain
func New() *Blockchain {
	channel := make(chan int)
	// comeca com o genesis
	return &Blockchain{
		Height:         1,
		Ledger:         []*Block{Genesis()},
		MPool:          []models.Transaction{},
		IncomingBlocks: make(chan BlockTask, 10),
		StateChan:      &channel,
		MX:             sync.Mutex{},
	}
}

// valida a tx e joga na mempool se tiver tudo ok
func (b *Blockchain) AddTransaction(tx models.Transaction) error {
	b.MX.Lock()
	defer b.MX.Unlock()

	// 1. verifica se a assinatura eh valida
	if !VerifySignature(tx.PublicKey, tx.UserData, tx.Signature) {
		slog.Error("Blockchain: Assinatura inválida", "txID", tx.ID)
		return errors.New("invalid signature")
	}

	// 2. anti-replay (vê se o cara nao ta mandando de novo a mesma coisa)
	if !b.AntiReplay(tx.ID) {
		slog.Error("Blockchain: Transação duplicada (Replay Attack)", "txID", tx.ID)
		return errors.New("duplicated transaction")
	}

	// 3. validacao de campos obrigatorios
	if err := b.ValidateFormat(tx); err != nil {
		slog.Error("Blockchain: Formato inválido", "error", err)
		return err
	}

	// adiciona na fila
	b.MPool = append(b.MPool, tx)
	// fmt.Printf("Transação adicionada à Mempool: %s (%s)\n", tx.ID, tx.Type)
	return nil
}

// pega txs da mempool e tenta fechar um bloco
func (b *Blockchain) MineBlock() (*Block, error) {
	b.MX.Lock()
	// pega o que tem pendente (limitei a 50 pra nao ficar gigante)
	count := len(b.MPool)
	if count == 0 {
		b.MX.Unlock()
		return nil, errors.New("no transactions to mine")
	}
	if count > 50 {
		count = 50
	}

	txsToMine := make([]*models.Transaction, count)
	for i := 0; i < count; i++ {
		// cuidado com ponteiro em loop, mas aqui ta acessando por indice entao de boa
		tx := b.MPool[i]
		txsToMine[i] = &tx
	}

	prevHash := b.Ledger[len(b.Ledger)-1].Hash
	b.MX.Unlock() // libera o lock pq o pow demora

	// comeca a mineracao
	// passamos o StateChan pra poder cancelar se chegar bloco de outro no
	newBlock := NewBlock(prevHash, txsToMine, b.StateChan)

	// se o hash for vazio, eh sinal que cancelaram
	if len(newBlock.Hash) == 0 {
		return nil, errors.New("mining cancelled")
	}

	return newBlock, nil
}

// bota o bloco validado no ledger e limpa a mempool
func (b *Blockchain) AddBlock(block *Block) {
	b.MX.Lock()
	defer b.MX.Unlock()

	// adiciona no final da cadeia
	b.Ledger = append(b.Ledger, block)
	b.Height++

	// remove da mempool as txs que entraram nesse bloco
	// cria mapa pra busca rapida
	minedIDs := make(map[string]bool)
	for _, tx := range block.Transactions {
		minedIDs[tx.ID] = true
	}

	// recria a mpool so com o que sobrou
	newPool := []models.Transaction{}
	for _, tx := range b.MPool {
		if !minedIDs[tx.ID] {
			newPool = append(newPool, tx)
		}
	}
	b.MPool = newPool

	fmt.Printf("⛓️  Bloco #%d adicionado! Hash: %x | Txs: %d\n", b.Height, block.Hash[:4], len(block.Transactions))
}

// verifica se o bloco que chegou de outro no eh valido
func (b *Blockchain) CheckNewBlock(block *Block) error {
	// 1. verifica se encaixa no hash anterior
	// aqui que entra o consenso de nakamoto sobre a cadeia mais longa (height)
	lastBlock := b.Ledger[len(b.Ledger)-1]
	if !bytes.Equal(block.PreviousHash, lastBlock.Hash) {
		return fmt.Errorf("invalid previous hash. Got %x, expected %x", block.PreviousHash, lastBlock.Hash)
	}

	// 2. verifica o pow
	pow := NewProofOfWork(block)
	if !pow.Validate() {
		return errors.New("invalid proof of work")
	}

	// 3. verifica assinaturas das transacoes
	for _, tx := range block.Transactions {
		// ignora genesis
		if tx.Type == "GENESIS" {
			continue
		}

		if !VerifySignature(tx.PublicKey, tx.UserData, tx.Signature) {
			return fmt.Errorf("block contains invalid transaction signature: %s", tx.ID)
		}
	}

	return nil
}

// confere se a tx ja existe na mpool ou no ledger
func (b *Blockchain) AntiReplay(txID string) bool {
	// olha na mempool
	for _, t := range b.MPool {
		if t.ID == txID {
			return false
		}
	}

	// olha no ledger (ta linear simples, ideal era ter um indice no banco)
	for _, block := range b.Ledger {
		for _, t := range block.Transactions {
			if t.ID == txID {
				return false
			}
		}
	}
	return true
}

// checa se os dados batem com o tipo de transacao
func (b *Blockchain) ValidateFormat(tx models.Transaction) error {
	var requiredLen int
	switch tx.Type {
	case models.TxPurchase: // [0]UserID, [1]CardID, [2]CardModel
		requiredLen = 3
	case models.TxTrade: // [0]U1, [1]U2, [2]C1, [3]C2
		requiredLen = 4
	case models.TxBattleResult: // [0]BattleID, [1]U1, [2]U2, [3]Winner
		requiredLen = 4
	default:
		return errors.New("unknown transaction type")
	}

	if len(tx.Data) < requiredLen {
		return fmt.Errorf("invalid data length for type %s. Expected %d, got %d", tx.Type, requiredLen, len(tx.Data))
	}
	return nil
}

// loop principal pra monitorar canais (chamado pelo server)
func (b *Blockchain) RunBlockchainLoop() {
	for {
		select {
		case task := <-b.IncomingBlocks:
			// chegou bloco novo da rede
			*b.StateChan <- 3 // manda sinal de CANCEL pra quem tiver minerando agora

			err := b.CheckNewBlock(task.Block)
			if err != nil {
				slog.Error("Bloco rejeitado", "erro", err)
				if task.OnFinish != nil {
					task.OnFinish(err)
				}
			} else {
				b.AddBlock(task.Block)
				if task.OnFinish != nil {
					task.OnFinish(nil)
				}
			}
		}
	}
}
