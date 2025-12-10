package main

import (
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/fatih/color"
)

// fica pingando os outros pra ver quem ta vivo
func (s *Server) RunHealthChecks() {
	// espera estabilizar antes de sair atirando
	time.Sleep(5 * time.Second)

	ticker := time.NewTicker(HealthCheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		s.checkClusterHealth()
	}
}

func (s *Server) checkClusterHealth() {
	// lista temporaria pra saber quem respondeu agora
	liveNow := make(map[string]bool)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for id, host := range s.serverList {
		wg.Add(1)
		go func(sid, shost string) {
			defer wg.Done()
			// se respondeu, marca como vivo
			if s.checkServerHealth(shost) {
				mu.Lock()
				liveNow[sid] = true
				mu.Unlock()
			}
		}(id, host)
	}
	wg.Wait()

	// --- DIFERENCA DO ANTIGO ---
	// garante que EU to vivo pra mim mesmo, senao da ruim na votacao
	mu.Lock()
	liveNow[s.ID] = true
	mu.Unlock()
	// ---------------------------

	// atualiza mapa global
	s.muLiveServers.Lock()
	s.liveServers = liveNow
	s.muLiveServers.Unlock()

	// ve se o lider morreu
	s.muLeader.RLock()
	leader := s.currentLeader
	s.muLeader.RUnlock()

	// se o lider sumiu, faz eleicao nova
	if leader != "" && !liveNow[leader] {
		color.Red("🚨 [Cluster] Líder %s caiu! Iniciando nova eleição...", leader)
		// debug:
		//for _, peer := range []string{"server1", "server2", "server3"} {
		//s.liveServers[peer] = s.checkServerHealth(peer)
		//print("Verificando saúde do peer ", peer, "que está", s.liveServers[peer])
		//}
		s.electNewLeader(liveNow)
	} else if leader == "" {
		s.electNewLeader(liveNow)
	}
}

// manda um get /health
func (s *Server) checkServerHealth(host string) bool {
	// se for eu mesmo, retorna true logo
	if host == s.Host {
		return true
	}

	client := http.Client{Timeout: 1 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://%s/health", host))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// elege o lider novo baseado em quem ta vivo
func (s *Server) electNewLeader(liveNow map[string]bool) {
	color.Yellow("\n[Eleição] Iniciando processo de eleição...")

	// Se for a eleição inicial (liveNow == nil), faz health check completo
	if liveNow == nil {
		color.Yellow("[Eleição] Eleição inicial - verificando servidores...")
		liveNow = make(map[string]bool)

		var wg sync.WaitGroup
		var mu sync.Mutex

		for id, host := range s.serverList {
			wg.Add(1)
			go func(id, host string) {
				defer wg.Done()
				color.Cyan("[Eleição] Verificando servidor %s em %s", id, host)
				if s.checkServerHealth(host) {
					mu.Lock()
					liveNow[id] = true
					mu.Unlock()
					color.Green("[Eleição] ✓ %s está ONLINE", id)
				} else {
					color.Red("[Eleição] ✗ %s está OFFLINE", id)
				}
			}(id, host)
		}
		wg.Wait()

		// Atualiza o mapa global com a descoberta inicial
		s.muLiveServers.Lock()
		s.liveServers = liveNow
		s.muLiveServers.Unlock()
	}

	// Coleta os IDs dos servidores vivos
	liveIDs := []string{}
	for id, alive := range liveNow {
		if alive {
			liveIDs = append(liveIDs, id)
		}
	}

	color.Yellow("[Eleição] Servidores vivos detectados: %v", liveIDs)

	// Se a lista está vazia, adiciona a si mesmo
	if len(liveIDs) == 0 {
		liveNow[s.ID] = true
		liveIDs = append(liveIDs, s.ID)
		color.Yellow("[Eleição] FALLBACK: Nenhum servidor detectado, assumindo liderança solitária")
	}

	// Ordena alfabeticamente e escolhe o menor ID
	sort.Strings(liveIDs)
	newLeaderID := liveIDs[0]

	// Atualiza o líder atual
	s.muLeader.Lock()
	oldLeader := s.currentLeader
	s.currentLeader = newLeaderID
	s.muLeader.Unlock()

	// Atualiza liveServers com os servidores detectados
	s.muLiveServers.Lock()
	s.liveServers = liveNow
	s.muLiveServers.Unlock()

	// Anuncia mudança de liderança (se houver)
	if oldLeader != newLeaderID {
		color.Green("\n========================================")
		color.Green("👑 NOVO LÍDER ELEITO: %s", s.currentLeader)
		color.Green("   Candidatos vivos: %v", liveIDs)
		color.Green("========================================\n")

		// Se este servidor é o novo líder
		if newLeaderID == s.ID {
			color.Cyan("[Eleição] EU sou o novo líder!")
		}
	} else {
		color.Cyan("[Eleição] Líder mantido: %s", newLeaderID)
	}
}

func (s *Server) isLeader() bool {
	s.muLeader.RLock()
	defer s.muLeader.RUnlock()
	return s.currentLeader == s.ID
}
