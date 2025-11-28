package models

type TransactionType string

const (
	PC   TransactionType = "Purchase"
	TD   TransactionType = "Trade"
	BR   TransactionType = "BattleResult"
	NONE TransactionType = ""
)

// Molde de Data:
// * PC: [0] - UserID, [1] - BoosterID
// * TD: [0] - UserID1, [1] - UserID2, [2] - User1CardID, [3] - User2CardID
// * BR: [0] - BattleID, [1] - UserID1, [2] - UserID2, [3] - User1Result, [4] - User2Result

type TransactionRequest struct {
	Type      TransactionType
	UserID    string
	Timestamp string
	PublicKey []byte // public key do usuário que enviou primeiro
	Signature []byte // assinatura gerada pela chave privada do usuário
}

// Molde de UserData
// [0] - TransactionType, [1] - UID, [2] - Timestamp

type Transaction struct {
	Type     TransactionType
	Data     []string // guarda informações essenciais da transação
	UserData []string // guarda informações para comparar assinatura

	// informações de assinatura, para garantir segurança da transação
	PublicKey []byte // public key do usuário que enviou primeiro
	Signature []byte // assinatura gerada pela chave privada do usuário
}
