package usecases

import (
	"pbl-2-redes/internal/infrastructure/blockchain"
	"pbl-2-redes/internal/models"
)

func (u *UseCases) GetBlockchain() []blockchain.Block {
	return u.sync.GetLedger()
}

func (u *UseCases) AddNewBlock(newBlock blockchain.Block) error {
	err := u.sync.AddNewBlock(&newBlock)

	if err != nil {
		return err
	}

	// verifica se a transação deve ser guardada localmente
	for _, t := range newBlock.Transactions {
		switch t.Type {
		case models.PC:
			if u.UIDExists(t.Data[0]) {
				u.repos.Transactions.Enqueue(*t)
			}
		case models.TD:
			if u.UIDExists(t.Data[0]) || u.UIDExists(t.Data[1]) {
				u.repos.Transactions.Enqueue(*t)
			}
		case models.BR:
			if u.repos.Match.MatchExists(t.Data[0]) {
				u.repos.Transactions.Enqueue(*t)
			}
		}
	}

	return nil
}

func (u *UseCases) GetFirstTransaction() models.Transaction {
	return u.repos.Transactions.GetFirstTransaction()
}

func (u *UseCases) TransactionsLength() int {
	return u.repos.Transactions.Length()
}
