package blockchain

import (
	"pbl-2-redes/internal/models"
	"time"
)

type Block struct {
	Timestamp    int64                 // contém o horário quando o bloco é criado
	Hash         []byte                // contém o hash do bloco
	Transactions []*models.Transaction // contém as transações
	PreviousHash []byte                // contém o hash do bloco anterior
	Nonce        int                   // nonce usado para o hash
}

func NewBlock(prevHash []byte, t []*models.Transaction, StateChan *chan int) *Block {
	// Cria primeira versão do bloco, sem hash ainda
	block := &Block{Timestamp: time.Now().Unix(), Transactions: t, PreviousHash: prevHash}

	// Cria o PoW
	pow := NewProofOfWork(block)

	// Guarda o nonce que for encontrado e joga no bloco
	nonce, hash := pow.Run(StateChan)
	block.Hash = hash[:]
	block.Nonce = nonce

	return block
}
