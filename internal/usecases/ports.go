package usecases

import (
	"pbl-2-redes/internal/infrastructure/blockchain"
	"pbl-2-redes/internal/models"
)

// PORTS define interfaces para conexão entre usecases e o cluster
type ClusterSync interface {
	// Sincroniza o estoque de cartas
	SyncCards() ([]models.Booster, error)
	// Sincroniza o enqueue na fila de batalha
	BattleEnqueue(UID string) error
	// Sincroniza o dequeue na fila de batalha
	BattleDequeue() error
	// Sincroniza o enqueue na fila de troca
	TradingEnqueue(UID string) error
	// Sincroniza o dequeue na fila de troca
	TradingDequeue() error
	// Sincroniza nova batalha
	MatchNew(Match models.MatchInitialRequest) error
	// Sincroniza nova batalha
	MatchEnd(string) error
	// Sincroniza compra de carta
	BuyBooster(transaction models.Transaction) error
	// Sincroniza troca de carta
	TradeCard(UID, CID string, card models.Card) error
	// Sincroniza criação de usuários, para não permitir cópias
	UserNew(username string) error
	// Atualiza partida
	//UpdateMatch(match models.Match) error

	//..........
	// Pega a mão do usuário solicitando a um certo servidor
	GetHand(UID string) ([]*models.Card, error)
	// Verifica se é líder
	IsLeader() bool
	// Pego ID do server
	GetServerID() int
	// Encontra qual o servidor dono daquele usuário
	FindServer(UID string) int

	// BLOCKCHAIN
	// Adiciona na blockchain um incomingBlock
	AddNewBlock(incomingBlock *blockchain.Block) error
	// Pega o ledger
	GetLedger() []blockchain.Block
}
