package models

type Block struct {
	Timestamp    int64         // contém o horário quando o bloco é criado
	Hash         string        // contém o hash do bloco
	Transactions []Transaction // contém a lista de transações
	PreviousHash string        // contém o hash do bloco anterior
	Target       int           // valor target do algoritmo PoW
	Nonce        int           // nonce usado para o hash
}
