package blockchain

import "math/big"

type ProofOfWork struct {
	block  *Block
	target *big.Int
}
