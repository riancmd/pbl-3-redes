package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"math/big"
	"time"
)

const targetBits = 24

const (
	idle       = 0
	mining     = 1
	validating = 2
	cancel     = 3
)

type ProofOfWork struct {
	block  *Block
	target *big.Int
}

// gera novo algoritmo de PoW para blcoo especificado
func NewProofOfWork(b *Block) *ProofOfWork {
	target := big.NewInt(1)
	target.Lsh(target, uint(256-targetBits))

	pow := &ProofOfWork{b, target}

	return pow
}

// prepara os dados para o PoW
func (pow *ProofOfWork) prepareData(nonce int) []byte {
	// Serializa struct de transações
	data, err := json.Marshal(pow.block.Transactions)
	if err != nil {
		slog.Error(err.Error())
		return nil
	}

	data = bytes.Join(
		[][]byte{
			pow.block.PreviousHash,
			data,
			IntToHex(pow.block.Timestamp),
			IntToHex(int64(targetBits)),
			IntToHex(int64(nonce)),
		},
		[]byte{},
	)

	return data
}

// função core do PoW
func (pow *ProofOfWork) Run(sc *chan int) (n int, h []byte) {
	var hashInt big.Int
	var hash [32]byte
	nonce := 0

	fmt.Printf("Minerando novo bloco.\n")

	// coloca que o máximo que o nonce pode chegar é o valor máximo suportado de um inteiro de 64 bits
	for nonce < math.MaxInt64 {
		// verifica se precisa continuar minerando
		select {
		case msg := <-*sc:
			// se for para minerar
			if msg == mining {
				// prepara os dados
				data := pow.prepareData(nonce)
				// cria o hash
				hash = sha256.Sum256(data)
				//fmt.Printf("\r%x", hash)
				hashInt.SetBytes(hash[:])

				// faz a comparação
				if hashInt.Cmp(pow.target) == -1 {
					break
				} else {
					nonce++
				}
			}
			if msg == checkingNode {
				// resolver essa linha depois
				time.Sleep(1)
			}
			if msg == cancel {
				break
			}
		case <-time.After(1 * time.Second):
			continue
		}

	}

	for nonce < math.MaxInt64 {

	}
	fmt.Print("\n")
	return nonce, hash[:]
}

// converte para hexadecimal de inteiro
func IntToHex(data int64) []byte {
	buff := new(bytes.Buffer)

	err := binary.Write(buff, binary.BigEndian, data)
	if err != nil {
		slog.Error(err.Error())
		return []byte{}
	}

	return buff.Bytes()
}

// valida blocos recebidos
func (pow *ProofOfWork) Validate() bool {
	var hashInt big.Int

	data := pow.prepareData(pow.block.Nonce)
	hash := sha256.Sum256(data)
	hashInt.SetBytes(hash[:])

	isValid := hashInt.Cmp(pow.target) == -1

	return isValid
}
