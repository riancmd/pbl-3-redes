package main

import (
	"PlanoZ/internal/blockchain"
	"PlanoZ/internal/models"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/fatih/color"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// handlers da api rest com gin

// handleHealthCheck: so pra saber se o servidor ta de pe
func (s *Server) handleHealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, models.HealthCheckResponse{
		Status:   "OK",
		ServerID: s.ID,
		IsLeader: s.isLeader(),
	})
}

// parte de sincronizacao do cluster

// avisa pro lider que um player novo conectou
func (s *Server) handleLeaderConnect(c *gin.Context) {
	if !s.isLeader() {
		c.JSON(http.StatusForbidden, gin.H{"error": "Eu não sou o líder"})
		return
	}

	var req models.LeaderConnectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Requisição inválida"})
		return
	}

	// atualiza a lista global de players
	s.muPlayers.Lock()
	oldInfo, exists := s.playerList[req.PlayerID]
	s.playerList[req.PlayerID] = models.PlayerInfo{
		ServerID:     req.ServerID,
		ServerHost:   req.ServerHost,
		ReplyChannel: req.ReplyChannel,
	}
	s.muPlayers.Unlock()

	// loga so se for novidade ou troca de server
	if !exists || oldInfo.ServerID != req.ServerID {
		color.Green("LÍDER: Player %s registrado no servidor %s", req.PlayerID, req.ServerID)
		// propaga pros outros servers ficarem sabendo
		go s.broadcastToServers("/players/update", s.playerList)
	}

	c.JSON(http.StatusOK, gin.H{"status": "registered"})
}

// recebe a lista atualizada do lider
func (s *Server) handlePlayerUpdate(c *gin.Context) {
	var newList map[string]models.PlayerInfo
	if err := c.ShouldBindJSON(&newList); err != nil {
		return
	}
	s.muPlayers.Lock()
	s.playerList = newList
	s.muPlayers.Unlock()
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

// recebe atualizacao de estoque (futuro)
func (s *Server) handleInventoryUpdate(c *gin.Context) {
	// implementacao futura pra consistencia eventual do estoque visual
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

// handlers de transacao (entrada da blockchain)

// compra de carta assinada e registrada na blockchain
func (s *Server) handleLeaderBuyCard(c *gin.Context) {
	// agora recebe um request assinado pelo cliente
	var req models.TransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Formato inválido"})
		return
	}

	// 1. confere se a assinatura bate
	if !blockchain.VerifyTransactionRequestSignature(req) {
		color.Red("COMPRA: Assinatura inválida do cliente %s", req.UserID)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Assinatura inválida"})
		return
	}

	// 2. ve se tem booster no estoque
	s.muTrades.Lock() // usando muTrades ou criar um muInventory especifico
	if len(s.Boosters) == 0 {
		s.muTrades.Unlock()
		c.JSON(http.StatusGone, gin.H{"error": "Estoque esgotado"})
		return
	}
	// pega o primeiro da fila (fifo)
	booster := s.Boosters[0]
	s.Boosters = s.Boosters[1:]
	s.muTrades.Unlock()

	// 3. prepara os dados da transacao
	// simplificacao: vamos registrar apenas a PRIMEIRA carta do booster no Data para o indice,
	// ou serializar as cartas no campo Data?
	// o modelo diz: [0]UserID, [1]CardID_Gerado, [2]CardModel
	// como o booster tem 3 cartas, vamos criar uma transacao que representa o PACOTE
	// ou criar 3 transacoes? pra simplificar e economizar blocos, vamos serializar o booster no Data[1]

	// adaptacao: guarda o json do booster direto no data pra facilitar pro cliente pegar depois
	boosterJson, _ := json.Marshal(booster)

	// monta a transacao pra blockchain
	tx := models.Transaction{
		ID:        uuid.New().String(),
		Type:      models.TxPurchase,
		Timestamp: time.Now().Unix(),
		Data: []string{
			req.UserID,          // quem comprou
			string(boosterJson), // o que comprou (json completo)
			"BOOSTER_PACK",      // metadado
		},
		UserData:  []string{req.Payload, fmt.Sprintf("%d", req.Timestamp), req.UserID, string(req.Type)}, // o que o user assinou
		PublicKey: req.PublicKey,
		Signature: req.Signature,
	}

	// 4. joga pra mempool
	if err := s.Blockchain.AddTransaction(tx); err != nil {
		color.Red("COMPRA: Erro ao adicionar na Mempool: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		// deveria devolver o booster pro estoque aqui se der erro, mas ignora por enquanto
		return
	}

	color.Green("COMPRA: Transação %s enviada para Mempool (User: %s)", tx.ID, req.UserID)

	// 5. avisa o cliente que ta processando
	c.JSON(http.StatusAccepted, models.AsyncResponse{
		Message: "Transação de compra enviada para processamento",
		TxID:    tx.ID,
		Status:  "processing",
	})
}

// registra o fim da batalha na chain
func (s *Server) handleRegisterBattle(c *gin.Context) {
	var req models.TransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Formato inválido"})
		return
	}

	// parse do payload especifico
	var payload models.BattleResultPayload
	if err := json.Unmarshal([]byte(req.Payload), &payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Payload malformado"})
		return
	}

	// 1. verifica assinatura
	if !blockchain.VerifyTransactionRequestSignature(req) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Assinatura inválida"})
		return
	}

	// 2. valida se a batalha rolou mesmo (confiando na assinatura por enquanto)
	// pra ser mais seguro o server deveria ter o registro, mas como limpamos da memoria no fim,
	// vamos confiar na criptografia do vencedor

	// monta transacao BR
	tx := models.Transaction{
		ID:        uuid.New().String(),
		Type:      models.TxBattleResult,
		Timestamp: time.Now().Unix(),
		Data: []string{
			payload.BattleID,
			req.UserID,     // quem mandou (vencedor ou perdedor)
			payload.Winner, // quem ganhou
			"BATTLE_END",
		},
		UserData:  []string{req.Payload, fmt.Sprintf("%d", req.Timestamp), req.UserID, string(req.Type)},
		PublicKey: req.PublicKey,
		Signature: req.Signature,
	}

	if err := s.Blockchain.AddTransaction(tx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, models.AsyncResponse{
		Message: "Resultado de batalha registrado",
		TxID:    tx.ID,
		Status:  "processing",
	})
}

// registra troca finalizada na chain
func (s *Server) handleRegisterTrade(c *gin.Context) {
	var req models.TransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Formato inválido"})
		return
	}

	// parse payload
	var payload models.TradePayload
	if err := json.Unmarshal([]byte(req.Payload), &payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Payload malformado"})
		return
	}

	// 1. verifica assinatura
	if !blockchain.VerifyTransactionRequestSignature(req) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Assinatura inválida"})
		return
	}

	// monta transacao TD
	tx := models.Transaction{
		ID:        uuid.New().String(),
		Type:      models.TxTrade,
		Timestamp: time.Now().Unix(),
		Data: []string{
			req.UserID,         // User 1
			payload.UserTarget, // User 2
			payload.CardMy,     // Card 1
			payload.CardTarget, // Card 2
		},
		UserData:  []string{req.Payload, fmt.Sprintf("%d", req.Timestamp), req.UserID, string(req.Type)},
		PublicKey: req.PublicKey,
		Signature: req.Signature,
	}

	if err := s.Blockchain.AddTransaction(tx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, models.AsyncResponse{
		Message: "Troca registrada",
		TxID:    tx.ID,
		Status:  "processing",
	})
}

// handlers de gameplay p2p (mantido do original)

// handleBattleInitiate: S1 (Host) -> S2 (Peer)
func (s *Server) handleBattleInitiate(c *gin.Context) {
	var req models.BattleInitiateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Bad request"})
		return
	}

	// registra quem eh o host e quem eh o peer
	s.muBatalhasPeer.Lock()
	s.batalhasPeer[req.IdBatalha] = models.PeerBattleInfo{
		HostAPI:  req.HostServidor,
		PlayerID: req.IdJogadorLocal, // o jogador conectado AQUI (J2)
	}
	s.muBatalhasPeer.Unlock()

	// avisa o player 2 via redis que vai comecar
	s.muPlayers.RLock()
	infoJ2, ok := s.playerList[req.IdJogadorLocal]
	s.muPlayers.RUnlock()

	if ok {
		resp := models.RespostaInicioBatalha{
			Mensagem:  req.IdOponente, // nome do oponente
			IdBatalha: req.IdBatalha,
		}
		s.sendToClient(infoJ2.ReplyChannel, "Inicio_Batalha", resp)
	}

	c.JSON(http.StatusOK, gin.H{"status": "ack"})
}

// handleBattleRequestMove: S1 -> S2 (pede jogada)
func (s *Server) handleBattleRequestMove(c *gin.Context) {
	var req models.BattleRequestMoveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return
	}

	//s.muTradesPeer.RLock() // ops, aqui seria muBatalhasPeer, mas o original usava logica similar
	//_, ok := s.batalhasPeer[req.IdBatalha]
	//s.muTradesPeer.RUnlock()

	// na arquitetura original mandava msg pro redis, vou simplificar mantendo a notificacao
	s.notificarClienteBatalha(req.IdBatalha, "Sua_Vez", "Escolha sua carta")

	c.JSON(http.StatusOK, gin.H{"status": "waiting_player"})
}

// handleBattleSubmitMove: S2 (Peer) -> S1 (Host) (recebe carta do J2)
func (s *Server) handleBattleSubmitMove(c *gin.Context) {
	var req models.BattleSubmitMoveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return
	}

	s.muBatalhas.Lock()
	batalha, ok := s.batalhas[req.IdBatalha]
	s.muBatalhas.Unlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Batalha não encontrada no Host"})
		return
	}

	// manda pro canal do player 2
	select {
	case batalha.CanalJ2 <- req.Carta:
		c.JSON(http.StatusOK, gin.H{"status": "received"})
	case <-time.After(2 * time.Second):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Timeout processing move"})
	}
}

// handleBattleTurnResult: S1 -> S2 (resultado do turno)
func (s *Server) handleBattleTurnResult(c *gin.Context) {
	var req models.BattleTurnResultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return
	}

	// repassa pro cliente J2
	s.notificarClienteBatalha(req.IdBatalha, "Resultado_Turno", req.Resultado)
	c.JSON(http.StatusOK, gin.H{"status": "ack"})
}

// handleBattleEnd: S1 -> S2 (fim de jogo)
func (s *Server) handleBattleEnd(c *gin.Context) {
	var req models.BattleEndRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return
	}

	// notifica J2
	s.notificarClienteBatalha(req.IdBatalha, "Fim_Batalha", req.Resultado)

	// limpa infos peer
	s.muBatalhasPeer.Lock()
	delete(s.batalhasPeer, req.IdBatalha)
	s.muBatalhasPeer.Unlock()

	c.JSON(http.StatusOK, gin.H{"status": "ended"})
}

// funcoes auxiliares pra nao repetir codigo
func (s *Server) notificarClienteBatalha(battleID string, tipo string, payload interface{}) {
	s.muBatalhasPeer.RLock()
	info, ok := s.batalhasPeer[battleID]
	s.muBatalhasPeer.RUnlock()

	if ok {
		s.muPlayers.RLock()
		playerInfo, pOk := s.playerList[info.PlayerID]
		s.muPlayers.RUnlock()
		if pOk {
			s.sendToClient(playerInfo.ReplyChannel, tipo, payload)
		}
	}
}

// --- handlers de troca (simplificados, mesma logica da batalha) ---

func (s *Server) handleTradeInitiate(c *gin.Context) {
	var req models.TradeInitiateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return
	}

	s.muTradesPeer.Lock()
	s.tradesPeer[req.IdTroca] = models.PeerTradeInfo{HostAPI: req.HostServidor, PlayerID: req.IdJogadorLocal}
	s.muTradesPeer.Unlock()

	s.notificarClienteTroca(req.IdTroca, "Inicio_Troca", models.RespostaInicioTroca{Mensagem: req.IdOponente, IdTroca: req.IdTroca})
	c.JSON(http.StatusOK, gin.H{"status": "ack"})
}

func (s *Server) handleTradeRequestCard(c *gin.Context) {
	var req models.TradeRequestCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return
	}
	s.notificarClienteTroca(req.IdTroca, "Sua_Vez_Troca", "Escolha carta para trocar")
	c.JSON(http.StatusOK, gin.H{"status": "ack"})
}

func (s *Server) handleTradeSubmitCard(c *gin.Context) {
	var req models.TradeSubmitCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return
	}

	s.muTrades.Lock()
	troca, ok := s.trades[req.IdTroca]
	s.muTrades.Unlock()

	if ok {
		select {
		case troca.CanalJ2 <- req.Carta:
			c.JSON(http.StatusOK, gin.H{"status": "received"})
		case <-time.After(2 * time.Second):
			c.JSON(http.StatusRequestTimeout, gin.H{"error": "timeout"})
		}
	} else {
		c.JSON(http.StatusNotFound, gin.H{"error": "trade not found"})
	}
}

func (s *Server) handleTradeResult(c *gin.Context) {
	var req models.TradeResultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return
	}

	s.notificarClienteTroca(req.IdTroca, "Resultado_Troca", req.CartaRecebida)

	s.muTradesPeer.Lock()
	delete(s.tradesPeer, req.IdTroca)
	s.muTradesPeer.Unlock()

	c.JSON(http.StatusOK, gin.H{"status": "ack"})
}

func (s *Server) notificarClienteTroca(tradeID, tipo string, payload interface{}) {
	s.muTradesPeer.RLock()
	info, ok := s.tradesPeer[tradeID]
	s.muTradesPeer.RUnlock()
	if ok {
		s.muPlayers.RLock()
		pInfo, pOk := s.playerList[info.PlayerID]
		s.muPlayers.RUnlock()
		if pOk {
			s.sendToClient(pInfo.ReplyChannel, tipo, payload)
		}
	}
}
