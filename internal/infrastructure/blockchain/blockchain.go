package blockchain

import (
	"encoding/json"
	"pbl-2-redes/internal/models"
)

type Blockchain struct {
	Ledger []*Block
	MPool  []models.Transaction
}

func New() *Blockchain {
	return &Blockchain{Ledger: []*Block{Genesis()}, MPool: []models.Transaction{}}
}

func (b *Blockchain) addTransaction(Type models.TransactionType, Data *json.RawMessage) {
	b.MPool = append(b.MPool, models.Transaction{Type: Type, Data: Data})
}

// Função que adiciona bloco na blockchain
func (b *Blockchain) AddBlock(transaction []models.Transaction) {
	prevBlock := b.Ledger[len(b.Ledger)-1]
	newBlock := NewBlock(prevBlock.PreviousHash, transaction)
	b.Ledger = append(b.Ledger, newBlock)
}

func Genesis() *Block {
	baseTransaction := models.Transaction{Type: models.NONE, Data: []byte{}}
	return NewBlock([]byte{}, []models.Transaction{baseTransaction})
}
