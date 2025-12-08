package usecases

import (
	"pbl-2-redes/internal/infrastructure/blockchain"
	"pbl-2-redes/internal/models"
)

func (u *UseCases) AddNewBlock(newBlock blockchain.Block) error {
	err := u.sync.AddNewBlock(&newBlock)

	if err != nil {
		return err
	}

	return nil
}

func (u *UseCases) GetFirstTransaction() models.Transaction {
	return u.repos.Transactions.GetFirstTransaction()
}

func (u *UseCases) TransactionsLength() int {
	return u.repos.Transactions.Length()
}
