package models

import (
	"encoding/json"
	"time" 
)

// Resposta externa para o canal de resposta do cliente (dentro possui resposta especifica)
type ExternalResponse struct {
	Type   string          `json:"type"`
	UserId string          `json:"userId"`
	Data   json.RawMessage `json:"data"` //Struct especifica para ser decodificada
}

// Requisição externa para canal do servidor que lidará com requisições envolvendo pareamento (batalha ou troca)
type ExternalRequest struct {
	Type   string          `json:"type"`
	UserId string          `json:"userId"`
	Data   json.RawMessage `json:"data"` //Struct especifica para ser decodificada
}

// Requisição de login/cadastro
type AuthenticationRequest struct {
	Type               string `json:"type"` //login ou register
	UserId             string `json:"userId"`
	ClientReplyChannel string `json:"clientReplyChannel"` //Canal pessoal do cliente para PUBLISH
	Username           string `json:"username"`
	Password           string `json:"password"`
}

// Requisição de compra de pacote
type PurchaseRequest struct {
	UserId             string `json:"userId"`
	ClientReplyChannel string `json:"clientReplyChannel"` //Canal pessoal do cliente para PUBLISH
	PublicKey          []byte `json:"publicKey"`
	Signature          []byte `json:"signature"`
	Timestamp          int64  `json:"timestamp"`
}

// Requisição de entrar em batalha
type MatchRequest struct {
	UserId             string `json:"userId"`
	ClientReplyChannel string `json:"clientReplyChannel"` //Canal pessoal do cliente para PUBLISH
	PublicKey          []byte `json:"publicKey"`
	Signature          []byte `json:"signature"`
	Timestamp          int64  `json:"timestamp"`
}

// "Requisição" de envio de nova carta
type NewCardRequest struct {
	BattleId           string `json:"battleId"` //Id da batalha que solicitou a carta
	UserId             string `json:"userId"`
	ClientReplyChannel string `json:"clientReplyChannel"` //Canal pessoal do cliente para PUBLISH
	Card               Card   `json:"card"`
}

// Requisição de Troca de carta
type TradeRequest struct {
	UserId             string `json:"userId"`
	ClientReplyChannel string `json:"clientReplyChannel"` //Canal pessoal do cliente para PUBLISH
	PublicKey          []byte `json:"publicKey"`
	Signature          []byte `json:"signature"`
	Timestamp          int64  `json:"timestamp"`
}

// "Requisição" de envio de uma ação em uma batalha
type GameActionRequest struct {
	BattleId           string `json:"battleId"` //Id da batalha que recebe a ação
	Type               string `json:"type"`
	UserId             string `json:"userId"`
	ClientReplyChannel string `json:"clientReplyChannel"`
}

// Requisição de Visualização do Blockchain
type BlockchainViewRequest struct {
	UserId             string `json:"userId"`
	ClientReplyChannel string `json:"clientReplyChannel"`
}

// Resposta de Login/Cadastro
type AuthResponse struct {
	Status        bool   `json:"status"`
	Username      string `json:"username"`
	UDPPort       string `json:"udpPort"`
	ServerChannel string `json:"serverChannel"`
	Message       string `json:"message"`
}

// Resposta para batalhas (entrada ou não / solicitação de envio de nova carta)
type MatchResponse struct {
	Type    string `json:"type"`
	Status  bool   `json:"status"`
	Message string `json:"message"`
}

// Resposta de compra do cliente
type ClientPurchaseResponse struct {
	Status           bool   `json:"status"`
	Message          string `json:"message"`
	BoosterGenerated Booster `json:"boosterGenerated"`
}

// Resposta para trocas de cartas (pareamento ou não / solicitação de envio de nova carta)
type TradeResponse struct {
	Type    string `json:"type"`
	Status  bool   `json:"status"`
	Message string `json:"message"`
}

// Payload para ações em batalha
type PayLoad struct {
	P1 *User `json:"p1"`
	P2 *User `json:"p2"`

	Info        string           `json:"info"`
	Turn        string           `json:"turn"`
	Hand        []Card           `json:"hand"`
	Sanity      map[string]int   `json:"sanity"`
	DreamStates map[string]DreamState `json:"dreamStates"`
	Round       int              `json:"round"`
	BattleId    string           `json:"battleId"` //Id da batalha que entrou

}

// Resposta de erro
type ErrorResponse struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// Resposta de visualização da Blockchain
type BlockchainResponse struct {
	Blocks []Block `json:"blocks"`
}

// Estrutura do bloco
type Block struct {
	Timestamp    int64         `json:"timestamp"`    // contém o horário quando o bloco é criado
	Hash         string        `json:"hash"`         // contém o hash do bloco (tipo alterado de []byte para string)
	Transactions []Transaction `json:"transactions"` // contém a lista de transações
	PreviousHash string        `json:"previousHash"` // contém o hash do bloco anterior (tipo alterado de []byte para string)
	Target       int           `json:"target"`       // valor target do algoritmo PoW (campo adicionado)
	Nonce        int           `json:"nonce"`        // nonce usado para o hash (tipo alterado de int64 para int)
}

// Estrutura da transação
type Transaction struct {
	Type     TransactionType `json:"type"`     // (tipo alterado para usar TransactionType)
	Data     []string        `json:"data"`     // guarda informações essenciais da transação (tipo alterado de json.RawMessage para []string)
	UserData []string        `json:"userData"` // guarda informações para comparar assinatura (campo adicionado)

	// informações de assinatura, para garantir segurança da transação
	PublicKey []byte `json:"publicKey"` // public key do usuário que enviou primeiro
	Signature []byte `json:"signature"` // assinatura gerada pela chave privada do usuário
}

// Definições do jogo 
type User struct {
	UID string
	Username string
}

type Card struct {
	CID string
	Name string
	Points int
	CardType CardType
	CardRarity CardRarity
	CardEffect CardEffect
	Desc string
}

type Booster struct {
	Booster []Card
}

type Match struct {
	P1 *User
	P2 *User
	Sanity map[string]int
	DreamStates map[string]DreamState
	Turn string
	CurrentRound int
}

type CardType string
type CardRarity string
type CardEffect string
type DreamState string


type TransactionType string

const (
	PC   TransactionType = "Purchase"
	TD   TransactionType = "Trade"
	BR   TransactionType = "BattleResult"
)