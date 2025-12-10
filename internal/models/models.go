package models

// estruturas basicas do jogo, tipo carta e tanque

const (
	RarityCommon   = "comum"
	RarityUncommon = "incomum"
	RarityRare     = "rara"
)

type Tanque struct {
	ID        string `json:"id"`
	Modelo    string `json:"modelo"`
	Raridade  string `json:"raridade"`
	Vida      int    `json:"vida"`
	Ataque    int    `json:"ataque"`
	OwnerID   string `json:"owner_id"`
	Timestamp int64  `json:"timestamp"`
}

type CardData struct {
	Modelo   string `json:"modelo"`
	Raridade string `json:"raridade"`
	Vida     int    `json:"vida"`
	Ataque   int    `json:"ataque"`
}

type Booster struct {
	BID   int      `json:"bid"`
	Cards []Tanque `json:"cards"`
}

// estruturas pra controlar estado na memoria do servidor

// guarda onde o player ta conectado
type PlayerInfo struct {
	ServerID     string `json:"server_id"`
	ServerHost   string `json:"server_host"`
	ReplyChannel string `json:"reply_channel"`
}

// estado da partida
type Batalha struct {
	ID           string      `json:"id"`
	Jogador1     string      `json:"jogador1"`
	Jogador2     string      `json:"jogador2"`
	ServidorJ1   string      `json:"servidor_j1"`
	ServidorJ2   string      `json:"servidor_j2"`
	CanalJ1      chan Tanque `json:"-"`
	CanalJ2      chan Tanque `json:"-"`
	CanalEncerra chan bool   `json:"-"`
	Estado       string      `json:"estado"`
}

// estado da negociacao de troca
type Troca struct {
	ID         string           `json:"id"`
	Jogador1   string           `json:"jogador1"`
	Jogador2   string           `json:"jogador2"`
	ServidorJ1 string           `json:"servidor_j1"` // Onde J1 está
	ServidorJ2 string           `json:"servidor_j2"` // Onde J2 está
	CartaJ1    Tanque           `json:"carta_j1"`
	CartaJ2    Tanque           `json:"carta_j2"`
	CanalJ1    chan interface{} `json:"-"` // Canal genérico para troca
	CanalJ2    chan interface{} `json:"-"`
}

// o S2 usa isso pra saber onde ta rolando a batalha
type PeerBattleInfo struct {
	BattleID string
	HostAPI  string
	PlayerID string
}

// o S2 usa isso pra saber onde ta o host da troca
type PeerTradeInfo struct {
	HostAPI  string
	PlayerID string
}

// requests e responses da api e redis

type HealthCheckResponse struct {
	Status   string `json:"status"`
	ServerID string `json:"server_id"`
	IsLeader bool   `json:"is_leader"`
}

type LeaderConnectRequest struct {
	PlayerID     string `json:"player_id"`
	ServerID     string `json:"server_id"`
	ServerHost   string `json:"server_host"`
	ReplyChannel string `json:"reply_channel"`
}

// requests de batalha
type BattleInitiateRequest struct {
	IdBatalha      string `json:"id_batalha"`
	IdJogadorLocal string `json:"id_jogador_local"`
	IdOponente     string `json:"id_oponente"`
	HostServidor   string `json:"host_servidor"`
}

type RespostaInicioBatalha struct {
	Mensagem  string `json:"mensagem"`
	IdBatalha string `json:"id_batalha"`
}

type BattleRequestMoveRequest struct {
	IdBatalha string `json:"id_batalha"`
}

type BattleSubmitMoveRequest struct {
	IdBatalha string `json:"id_batalha"`
	Carta     Tanque `json:"carta"`
}

type BattleTurnResultRequest struct {
	IdBatalha string `json:"id_batalha"`
	Resultado string `json:"resultado"`
}

type BattleEndRequest struct {
	IdBatalha string `json:"id_batalha"`
	Resultado string `json:"resultado"`
}

// requests de troca
type TradeInitiateRequest struct {
	IdTroca        string `json:"id_troca"`
	IdJogadorLocal string `json:"id_jogador_local"`
	IdOponente     string `json:"id_oponente"`
	HostServidor   string `json:"host_servidor"`
}

type RespostaInicioTroca struct {
	Mensagem string `json:"mensagem"`
	IdTroca  string `json:"id_troca"`
}

type TradeRequestCardRequest struct {
	IdTroca string `json:"id_troca"`
}

type TradeSubmitCardRequest struct {
	IdTroca string `json:"id_troca"`
	Carta   Tanque `json:"carta"`
}

type TradeResultRequest struct {
	IdTroca       string `json:"id_troca"`
	CartaRecebida Tanque `json:"carta_recebida"`
}

// requests pro redis (coisa velha de compatibilidade)
type ReqConectar struct {
	PlayerID string `json:"player_id"`
}
type ReqComprarCarta struct {
	PlayerID string `json:"player_id"`
}

// parte da blockchain e transacoes

type TransactionType string

const (
	TxPurchase     TransactionType = "PC"
	TxTrade        TransactionType = "TD"
	TxBattleResult TransactionType = "BR"
)

type Transaction struct {
	ID        string          `json:"id"`
	Type      TransactionType `json:"type"`
	Timestamp int64           `json:"timestamp"`
	Data      []string        `json:"data"`
	UserData  []string        `json:"user_data"`
	PublicKey []byte          `json:"public_key"`
	Signature []byte          `json:"signature"`
}

type TransactionRequest struct {
	Type      TransactionType `json:"type"`
	UserID    string          `json:"user_id"`
	Timestamp int64           `json:"timestamp"`
	Payload   string          `json:"payload"`
	PublicKey []byte          `json:"public_key"`
	Signature []byte          `json:"signature"`
}

type PurchasePayload struct {
	Intent string `json:"intent"`
}

type TradePayload struct {
	TradeID    string `json:"trade_id"`
	UserTarget string `json:"user_target"`
	CardMy     string `json:"card_my"`
	CardTarget string `json:"card_target"`
}

type BattleResultPayload struct {
	BattleID string `json:"battle_id"`
	Winner   string `json:"winner"`
}

type AsyncResponse struct {
	Message string `json:"message"`
	TxID    string `json:"tx_id,omitempty"`
	Status  string `json:"status"`
}
