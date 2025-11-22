package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"log/slog"
	"pbl-2-redes/internal/models"
	"strconv"
	"time"
)

type Block struct {
	Timestamp    int64  // contém o horário quando o bloco é criado
	Hash         []byte // contém o hash do bloco
	Transactions []byte // contém as transações
	PreviousHash []byte // contém o hash do bloco anterior
	Target       int    // valor target do algoritmo PoW
	Nonce        int    // nonce usado para o hash
}

func NewBlock(p []byte, t []models.Transaction) *Block {
	// serializa struct de transações
	data, err := json.Marshal(t)
	if err != nil {
		slog.Error(err.Error())
		return nil
	}

	block := &Block{Timestamp: time.Now().Unix(), Transactions: data, PreviousHash: p, Target: 1, Nonce: 10000}
	block.SetHash()

	return block
}

/*
func ComputeHash(b *models.Block) {
	data := b.Data
	hash := sha256.Sum256([]byte(data))
	b.Hash = hex.EncodeToString(hash[:])
}*/

func (b *Block) SetHash() {
	timestamp := []byte(strconv.FormatInt(b.Timestamp, 10))
	headers := bytes.Join([][]byte{b.PreviousHash, b.Transactions, timestamp}, []byte{})
	hash := sha256.Sum256(headers)

	b.Hash = hash[:]
}
