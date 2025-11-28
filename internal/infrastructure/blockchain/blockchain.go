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

func (b *Blockchain) AddTransaction(transaction models.Transaction) error {
	if !VerifySignature(transaction.PublicKey, transaction.UserData, transaction.Signature) {
		slog.Error("invalid signature")
		return errors.New("invalid signature")
	}

	if !b.AntiReplay(transaction.PublicKey, transaction.Signature) {
		slog.Error("duplicated signature")
		return errors.New("duplicated signature")
	}

	err := b.GreedyCheck(transaction)

	if err != nil {
		slog.Error(err.Error())
		return err
	}

	b.MPool = append(b.MPool, transaction)
	return nil
}

// Função que adiciona bloco na blockchain
func (b *Blockchain) AddBlock(transactions []*models.Transaction) {
	prevBlock := b.Ledger[len(b.Ledger)-1]
	newBlock := NewBlock(prevBlock.PreviousHash, transactions)
	b.Ledger = append(b.Ledger, newBlock)
}

func Genesis() *Block {
	baseTransaction := models.Transaction{Type: models.NONE, Data: []string{}}
	return NewBlock([]byte{}, []*models.Transaction{&baseTransaction})
}

// Função que atualiza ledger a depender do resultado de comparação de height no cluster
func (b *Blockchain) UpdateLedger(l []*Block) error {
	if len(b.Ledger) < len(l) {
		b.Ledger = l
		return nil
	}
	return errors.New("Ledger já atualizado.")
}

// Função que verifica ilegalidade de transação (uso de recurso já utilizado / transação duplicada sobre mesmo recurso)
func (b *Blockchain) GreedyCheck(t models.Transaction) error {
	// verifica situações de ilegalidade no ledger
	for _, block := range b.Ledger {
		for _, transaction := range block.Transactions {
			// na compra, se booster já foi comprado, desiste
			if t.Type == models.PC && t.Data[1] == transaction.Data[1] {
				return errors.New("illegal action, booster already purchased")
			}
			// na troca, se usuário 1 já trocou aquela carta, não troca
			if t.Type == models.TD && t.Data[0] == transaction.Data[0] && t.Data[1] == transaction.Data[1] {
				return errors.New("illegal action, card already traded by user")
			}
			// na troca, se usuário 2 já trocou aquela carta, não troca
			if t.Type == models.TD && t.Data[1] == transaction.Data[1] && t.Data[3] == transaction.Data[3] {
				return errors.New("illegal action, card already traded by user")
			}
			// na troca, se batalha já tiver registrada, desiste
			if t.Type == models.BR && t.Data[0] == transaction.Data[0] {
				return errors.New("illegal action, battle already exists")
			}
		}
	}

	// verifica situações de ilegalidade na mempool
	for _, transaction := range b.MPool {
		// na compra, se booster já tem intenção de compra
		if t.Type == models.PC && t.Data[1] == transaction.Data[1] {
			return errors.New("illegal action, booster cannot be purchased")
		}
		// na troca, se usuário 1 já está tentando trocar aquela carta, não troca
		if t.Type == models.TD && t.Data[0] == transaction.Data[0] && t.Data[1] == transaction.Data[1] {
			return errors.New("illegal action, card already traded by user")
		}
		// na troca, se usuário 2 já está tentando trocar aquela carta, não troca
		if t.Type == models.TD && t.Data[1] == transaction.Data[1] && t.Data[3] == transaction.Data[3] {
			return errors.New("illegal action, card already traded by user")
		}
		// na troca, se batalha já tiver registrada, desiste
		if t.Type == models.BR && t.Data[0] == transaction.Data[0] {
			return errors.New("illegal action, battle already exists")
		}
	}

	return nil
}

// Verifica se precisa minerar novo bloco
func (b *Blockchain) Mine() {
	// verifica se tem x quantidade de transações ou se passou timeout
	// IMPORTANTE: essa lógica tem que ser feita junto a logica de verificar a chegada de novos blocos
	for {

	}
}
