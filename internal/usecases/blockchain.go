package usecases

import "pbl-2-redes/internal/infrastructure/blockchain"

func (u *UseCases) AddNewBlock(newBlock blockchain.Block) error {
	err := u.sync.AddNewBlock(&newBlock)

	if err != nil {
		return err
	}

	return nil
}
