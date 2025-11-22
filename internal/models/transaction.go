package models

type TransactionType string

const (
	PC   TransactionType = "Purchase"
	TD   TransactionType = "Trade"
	BR   TransactionType = "BattleResult"
	NONE TransactionType = ""
)

type Transaction struct {
	Type TransactionType
	Data []byte
}
