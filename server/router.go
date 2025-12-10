package main

import (
	"github.com/gin-gonic/gin"
)

// configura o router do gin e as rotas
func (s *Server) setupRouter() *gin.Engine {
	// gin.SetMode(gin.ReleaseMode) 
	r := gin.Default()

	// rota de heartbeating e eleição
	r.GET("/health", s.handleHealthCheck)

	// --- rotas da blockchain ---
	blockchainGroup := r.Group("/blockchain")
	{
		// visualizacao
		blockchainGroup.GET("/", s.handleGetBlockchain)     // ver o ledger todo
		blockchainGroup.GET("/mempool", s.handleGetMempool) // ver transações pendentes

		// p2p da blockchain
		blockchainGroup.POST("/block", s.handleReceiveBlock) // recebe bloco de outro server
	}

	// --- rotas de sync (lider x seguidores) ---

	// player management
	playerGroup := r.Group("/players")
	{
		// seguidor avisa lider sobre novos jogadores
		playerGroup.POST("/connect", s.handleLeaderConnect)

		// lider manda a lista atualizada para os servers seguidores
		playerGroup.POST("/update", s.handlePlayerUpdate)
	}

	// cartas e compras
	cardGroup := r.Group("/cards")
	{
		// seguidor pede para lider processar compra (sincronização de dados do sistema distribuído)
		cardGroup.POST("/buy", s.handleLeaderBuyCard)
	}

	// inventário
	inventoryGroup := r.Group("/inventory")
	{
		// líder avisa mudança de estoque
		inventoryGroup.POST("/update", s.handleInventoryUpdate)
	}

	// chamadas de fim de jogo/troca pra registrar na chain
	r.POST("/battle/register", s.handleRegisterBattle)
	r.POST("/trade/register", s.handleRegisterTrade)

	// --- rotas de gameplay p2p (tempo real) ---

	// rotas para simplificar batalhas 
	battleGroup := r.Group("/battle")
	{
		battleGroup.POST("/initiate", s.handleBattleInitiate)
		battleGroup.POST("/request_move", s.handleBattleRequestMove)
		battleGroup.POST("/turn_result", s.handleBattleTurnResult)
		battleGroup.POST("/end", s.handleBattleEnd)
		battleGroup.POST("/submit_move", s.handleBattleSubmitMove)
	}

	// rotas de troca
	tradeGroup := r.Group("/trade")
	{
		tradeGroup.POST("/initiate", s.handleTradeInitiate)
		tradeGroup.POST("/request_card", s.handleTradeRequestCard)
		tradeGroup.POST("/result", s.handleTradeResult)
		tradeGroup.POST("/submit_card", s.handleTradeSubmitCard)
	}

	return r
}
