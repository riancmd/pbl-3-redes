package main

import (
	"PlanoZ/internal/blockchain"
	"PlanoZ/internal/models"
	"encoding/json"
	"time"

	"github.com/fatih/color"
	"github.com/gin-gonic/gin"
)

// fica ouvindo a blockchain pra processar as transacoes novas
// tem que rodar como goroutine no main
func (s *Server) RunBlockListener() {
	color.Cyan("🎧 [Listener] Iniciando monitoramento da Blockchain...")

	// comeca do 1, pulando o genesis
	lastProcessedHeight := 0

	for {
		s.Blockchain.MX.Lock()
		currentHeight := len(s.Blockchain.Ledger)
		s.Blockchain.MX.Unlock()

		if currentHeight > lastProcessedHeight {
			// processa tudo que chegou de novo
			for i := lastProcessedHeight; i < currentHeight; i++ {
				s.Blockchain.MX.Lock()
				block := s.Blockchain.Ledger[i]
				s.Blockchain.MX.Unlock()

				s.processBlock(block)
			}
			lastProcessedHeight = currentHeight
		}

		// espera para não ser muito tudo rápido
		time.Sleep(1 * time.Second)
	}
}

// varre as transações do bloco
func (s *Server) processBlock(block *blockchain.Block) {
	// color.Blue("⚙️ [Listener] Processando Bloco #%d com %d transações", block.Nonce, len(block.Transactions))

	for _, tx := range block.Transactions {
		if tx.Type == "GENESIS" {
			continue
		}

		// roda em goroutine pra nao travar o loop
		go s.processTransaction(tx)
	}
}

// aplica as mudancas de estado e avisa os players
func (s *Server) processTransaction(tx *models.Transaction) {
	switch tx.Type {
	case models.TxPurchase:
		s.processPurchase(tx)
	case models.TxTrade:
		s.processTrade(tx)
	case models.TxBattleResult:
		s.processBattleResult(tx)
	}
}

// processPurchase: [0]UserID, [1]BoosterJSON, [2]Meta
func (s *Server) processPurchase(tx *models.Transaction) {
	if len(tx.Data) < 2 {
		return
	}

	userID := tx.Data[0]
	boosterJson := tx.Data[1]

	// vê se o usuário está nesse server pra notificar
	s.muPlayers.RLock()
	info, isLocal := s.playerList[userID]
	s.muPlayers.RUnlock()

	if isLocal {
		// parse do json pra mandar com formato correto
		var booster models.Booster
		json.Unmarshal([]byte(boosterJson), &booster)

		// avisa no redis
		s.sendToClient(info.ReplyChannel, "Compra_Sucesso", gin.H{
			"mensagem": "Sua compra foi confirmada na Blockchain!",
			"booster":  booster,
			"tx_id":    tx.ID,
		})
		color.Green("💰 [Listener] Compra confirmada para %s (Tx: %s)", userID, tx.ID)
	}
}

// processTrade: [0]User1, [1]User2, [2]Card1, [3]Card2
func (s *Server) processTrade(tx *models.Transaction) {
	if len(tx.Data) < 4 {
		return
	}

	u1 := tx.Data[0]
	u2 := tx.Data[1]
	// c1 := tx.Data[2]
	// c2 := tx.Data[3]

	// helper pra notificar
	notify := func(uid, msg string) {
		s.muPlayers.RLock()
		info, ok := s.playerList[uid]
		s.muPlayers.RUnlock()
		if ok {
			s.sendToClient(info.ReplyChannel, "Troca_Confirmada", gin.H{
				"mensagem": msg,
				"tx_id":    tx.ID,
			})
		}
	}

	notify(u1, "Troca realizada com sucesso na Blockchain!")
	notify(u2, "Troca realizada com sucesso na Blockchain!")

	color.Green("🤝 [Listener] Troca confirmada entre %s e %s", u1, u2)
}

// processBattleResult: [0]BattleID, [1]ReporterID, [2]WinnerID, [3]Meta
func (s *Server) processBattleResult(tx *models.Transaction) {
	if len(tx.Data) < 3 {
		return
	}

	winnerID := tx.Data[2]

	// avisa só quem ganhou 
	s.muPlayers.RLock()
	info, ok := s.playerList[winnerID]
	s.muPlayers.RUnlock()

	if ok {
		s.sendToClient(info.ReplyChannel, "Rank_Update", gin.H{
			"mensagem": "Vitória registrada na Blockchain!",
			"tx_id":    tx.ID,
		})
	}
	color.Yellow("🏆 [Listener] Vitória registrada para %s", winnerID)
}
