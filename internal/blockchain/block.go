package blockchain

import (
	"PlanoZ/internal/models"
	"encoding/hex"
	"time"
)

type Block struct {
	Timestamp    int64                 `json:"timestamp"`
	Hash         []byte                `json:"hash"`
	Transactions []*models.Transaction `json:"transactions"`
	PreviousHash []byte                `json:"previous_hash"`
	Nonce        int                   `json:"nonce"`
}

// estrutura pra passar o bloco no canal entre rotinas
type BlockTask struct {
	Block    *Block
	OnFinish func(error)
}

// cria um bloco novo e ja dispara o pow pra tentar minerar
func NewBlock(prevHash []byte, t []*models.Transaction, StateChan *chan int) *Block {
	// monta o esqueleto do bloco
	block := &Block{
		Timestamp:    time.Now().Unix(),
		Transactions: t,
		PreviousHash: prevHash,
	}

	// prepara o pow
	pow := NewProofOfWork(block)

	// roda a mineração (fica travado aqui ate achar ou cancelarem pelo canal)
	nonce, hash := pow.Run(StateChan)

	// se o hash vier vazio é porque cancelaram (ex, outro node achou antes)
	block.Hash = hash
	block.Nonce = nonce

	return block
}

// gera o genesis, o primeiro bloco da corrente
func Genesis() *Block {
	// transacao vazia so pra constar
	genesisTx := &models.Transaction{
		ID:        "GENESIS",
		Type:      "GENESIS",
		Timestamp: time.Now().Unix(),
		Data:      []string{"Genesis Block"},
	}

	return &Block{
		Timestamp:    time.Now().Unix(),
		Hash:         []byte("GENESIS_HASH"),
		Transactions: []*models.Transaction{genesisTx},
		PreviousHash: []byte{},
		Nonce:        0,
	}
}

// helpers pra converter hash pra string
func (b *Block) HashHex() string {
	return hex.EncodeToString(b.Hash)
}

func (b *Block) PrevHashHex() string {
	return hex.EncodeToString(b.PreviousHash)
}