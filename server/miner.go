package main

import (
	"time"

	"github.com/fatih/color"
)

// loop eterno tentando achar bloco, rodando como goroutine na main
func (s *Server) RunMiner() {
	color.Cyan("⛏️  [Miner] Iniciando minerador...")

	time.Sleep(5 * time.Second)

	for {
		// 1. verifica mempool
		s.Blockchain.MX.Lock()
		mempoolSize := len(s.Blockchain.MPool)
		s.Blockchain.MX.Unlock()

		if mempoolSize == 0 {
			// nenhuma transação pendente
			time.Sleep(2 * time.Second)
			continue
		}

		color.Yellow("⛏️  [Miner] Minerando bloco com %d transações...", mempoolSize)

		// 2. tenta resolver o desafio 
		newBlock, err := s.Blockchain.MineBlock()

		if err != nil {
			if err.Error() == "mining cancelled" {
				color.Yellow("⚠️  [Miner] Mineração cancelada (outro nó minerou antes)")
			} else {
				color.Red("❌ [Miner] Erro ao minerar: %v", err)
			}
			time.Sleep(1 * time.Second)
			continue
		}

		// 3. mineração bem sucedida
		s.Blockchain.AddBlock(newBlock)
		color.Green("✅ [Miner] Bloco #%d minerado com sucesso!", s.Blockchain.Height-1)

		// 4. broadcast do bloco minerado
		go s.broadcastBlock(newBlock)

		// 5. espera antes de poder minerar de novo
		time.Sleep(500 * time.Millisecond)
	}
}

// manda o bloco minerado pros outros servers
func (s *Server) broadcastBlock(block interface{}) {
	s.muLiveServers.RLock()
	targets := make([]string, 0)
	for id, alive := range s.liveServers {
		if alive && id != s.ID {
			if host, ok := s.serverList[id]; ok {
				targets = append(targets, host)
			}
		}
	}
	s.muLiveServers.RUnlock()

	for _, host := range targets {
		go func(h string) {
			err := s.sendToHost(h, "/blockchain/block", block)
			if err != nil {
				color.Red("❌ [Miner] Falha ao enviar bloco para %s: %v", h, err)
			} else {
				color.Green("📤 [Miner] Bloco enviado para %s", h)
			}
		}(host)
	}
}
