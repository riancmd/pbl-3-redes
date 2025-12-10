package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"log/slog"
	"math"
	"math/big"
	"strconv"
)

// dificuldade do pow (quanto maior, mais foda de achar)
const targetBits = 20 // deixei baixo pra testar rapido, em prod sobe pra uns 24

// constantes pra controlar o estado da mineracao
const (
	Idle       = 0
	Mining     = 1
	Validating = 2
	Cancel     = 3
)

type ProofOfWork struct {
	block  *Block
	target *big.Int
}

// inicializa o pow pro bloco
func NewProofOfWork(b *Block) *ProofOfWork {
	target := big.NewInt(1)
	// shift left pra definir o alvo baseado na dificuldade
	target.Lsh(target, uint(256-targetBits))

	pow := &ProofOfWork{b, target}
	return pow
}

// converte int pra hex
func IntToHex(num int64) []byte {
	return []byte(strconv.FormatInt(num, 16))
}

// junta todos os dados do bloco num byte array pra calcular o hash
func (pow *ProofOfWork) prepareData(nonce int) []byte {
	// serializa as txs
	data, err := json.Marshal(pow.block.Transactions)
	if err != nil {
		slog.Error("PoW Marshal error", "error", err)
		return nil
	}

	// concatena: hash anterior + dados + timestamp + diff + nonce
	joined := bytes.Join(
		[][]byte{
			pow.block.PreviousHash,
			data,
			IntToHex(pow.block.Timestamp),
			IntToHex(int64(targetBits)),
			IntToHex(int64(nonce)),
		},
		[]byte{},
	)

	return joined
}

// loop principal da mineração
func (pow *ProofOfWork) Run(sc *chan int) (int, []byte) {
	var hashInt big.Int
	var hash [32]byte
	nonce := 0

	//fmt.Printf("Minerando bloco...\n")

	for nonce < math.MaxInt64 {
		// verifica se mandaram parar (tipo, se alguem minerou antes)
		select {
		case msg := <-*sc:
			if msg == Cancel {
				return nonce, []byte{}
			}
		default:
			// segue o baile minerando
			data := pow.prepareData(nonce)
			hash = sha256.Sum256(data)
			hashInt.SetBytes(hash[:])

			// se o hash for menor que o target, achamos!
			if hashInt.Cmp(pow.target) == -1 {
				//fmt.Printf("\rMinerado! Hash: %x\n", hash)
				return nonce, hash[:]
			} else {
				nonce++
			}
		}
	}
	return nonce, []byte{}
}

// valida se o pow ta certo (essa parte eh rapida)
func (pow *ProofOfWork) Validate() bool {
	var hashInt big.Int

	data := pow.prepareData(pow.block.Nonce)
	hash := sha256.Sum256(data)
	hashInt.SetBytes(hash[:])

	return hashInt.Cmp(pow.target) == -1
}

// so confere se o hash bate com os dados (integridade basica)
func (pow *ProofOfWork) ValidateHash(hashBytes []byte) bool {
	data := pow.prepareData(pow.block.Nonce)
	hash := sha256.Sum256(data)
	return bytes.Equal(hash[:], hashBytes)
}
