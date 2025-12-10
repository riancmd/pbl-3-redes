package main

import (
	"PlanoZ/internal/blockchain"
	"net/http"
	"time"

	"github.com/fatih/color"
	"github.com/gin-gonic/gin"
)

// retorna a chain inteira (ledger)
// usado pelo cliente pra ver a blockchain e novos nodes sincronizarem
func (s *Server) handleGetBlockchain(c *gin.Context) {
	s.Blockchain.MX.Lock()
	defer s.Blockchain.MX.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"height": s.Blockchain.Height,
		"ledger": s.Blockchain.Ledger,
	})
}

// retorna o que está pendente na mempool
func (s *Server) handleGetMempool(c *gin.Context) {
	s.Blockchain.MX.Lock()
	defer s.Blockchain.MX.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"count":   len(s.Blockchain.MPool),
		"mempool": s.Blockchain.MPool,
	})
}

// recebe um bloco minerado por outro server
// POST /blockchain/block
func (s *Server) handleReceiveBlock(c *gin.Context) {
	var block blockchain.Block
	if err := c.ShouldBindJSON(&block); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid block format"})
		return
	}

	color.Cyan("📦 [Blockchain] Recebido bloco de outro nó. Hash: %x...", block.Hash[:4])

	// canal pra pegar o resultado da validação
	resultChan := make(chan error, 1)

	// cria a task pro loop principal processar
	task := blockchain.BlockTask{
		Block: &block,
		OnFinish: func(err error) {
			resultChan <- err
		},
	}

	// manda para o canal de entrada (non blocking se tiver cheio)
	select {
	case s.Blockchain.IncomingBlocks <- task:
		// espera processar
		select {
		case err := <-resultChan:
			if err != nil {
				color.Red("❌ [Blockchain] Bloco rejeitado: %v", err)
				c.JSON(http.StatusNotAcceptable, gin.H{"error": err.Error()})
			} else {
				color.Green("✅ [Blockchain] Bloco aceito e adicionado!")
				c.JSON(http.StatusOK, gin.H{"message": "Block accepted"})
			}
		case <-time.After(5 * time.Second):
			color.Red("⚠️ [Blockchain] Timeout validando bloco externo")
			c.JSON(http.StatusRequestTimeout, gin.H{"error": "Validation timeout"})
		}
	default:
		color.Red("⚠️ [Blockchain] Fila de blocos cheia, descartando.")
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Block queue full"})
	}
}
