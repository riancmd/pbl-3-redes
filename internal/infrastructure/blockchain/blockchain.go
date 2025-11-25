package blockchain

import (
	"errors"
	"log/slog"
	"pbl-2-redes/internal/models"
)

type Blockchain struct {
	Height int
	Ledger []*Block
	MPool  []models.Transaction
}

func New() *Blockchain {
	return &Blockchain{Height: 1, Ledger: []*Block{Genesis()}, MPool: []models.Transaction{}}
}

func (b *Blockchain) addTransaction(transaction models.Transaction) bool {
	if !verifySignature(transaction.PublicKey, transaction.Data, transaction.Signature) {
		slog.Error("Erro: Assinatura inválida.")
		return false
	}
	b.MPool = append(b.MPool, transaction)
	return true
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

// Função que atualiza ledger a depender do resultado de comparação de height no cluster
func (b *Blockchain) UpdateLedger(l []*Block) error {
	if len(b.Ledger) < len(l) {
		b.Ledger = l
		return nil
	}
	return errors.New("Ledger já atualizado.")
}
