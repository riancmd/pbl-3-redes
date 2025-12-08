package bully

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	inElection   int = 0
	postElection int = 1
	leaderless   int = 2
)

// Responsável pela lógica de eleição e sincronização entre servers
type BullyElection struct {
	serverID   int         // Por enquanto, é a porta do servidor
	leaderID   int         // ID do líder
	peers      []int       // IDs dos outros servidores
	peersState map[int]int // 1 se vivo, 0 se morto
	state      int         // pode estar em eleição ou pós-eleição
	mu         *sync.Mutex // mutex pra coisas importantes
}

func New(serverID int, peers []int) *BullyElection {
	// Seta todos os servidores como mortos
	peersSt := make(map[int]int)
	for _, peer := range peers {
		peersSt[peer] = 0
	}

	return &BullyElection{
		serverID:   serverID,
		leaderID:   0,
		peers:      peers,
		peersState: peersSt,
		state:      leaderless,
		mu:         &sync.Mutex{},
	}
}

func (b *BullyElection) GetServerID() int {
	return b.serverID
}

// Verifica se é líder
func (b *BullyElection) IsLeader() bool {
	return b.leaderID == b.serverID
}

// Retorna líder
func (b *BullyElection) GetLeader() int {
	return b.leaderID
}

// Modificar líder (função interna)
func (b *BullyElection) SetLeader(newLeader int) error {
	// Verifica se peer realmente existe
	for _, peerID := range b.peers {
		if newLeader == peerID {
			b.leaderID = newLeader
			return nil
		}
	}
	slog.Error("peer is offline")
	return errors.New("peer is offline")
}

// Modificar estado para pós eleição
func (b *BullyElection) endElection() {
	b.state = postElection
}

// Verifica os IDs
func (b *BullyElection) StartElection() {
	b.state = inElection
	var leaderID int
	leaderID = b.serverID
	for _, peerID := range b.peers {
		if leaderID < peerID {
			leaderID = peerID
		}
	}

	b.SetLeader(leaderID)
	b.endElection()
}

// Torna sem líder
func (b *BullyElection) SetLeaderless() {
	b.state = leaderless
}

// Envia get pro endpoint de health
func (b *BullyElection) IsAlive(port int, res map[int]int) {
	// Cria um mini cliente (no caso, de uso ligeiro) para mandar a request
	healthClient := &http.Client{
		Timeout: 500 * time.Millisecond,
	}
	resp, err := healthClient.Get("http://localhost:" + strconv.Itoa(port) + "/internal/health")

	if err != nil {
		slog.Error(err.Error())
	}

	defer resp.Body.Close()
	defer b.mu.Unlock()

	// Verifica os status code
	if resp.StatusCode == http.StatusOK {
		b.mu.Lock()
		res[port] = 1
	} else {
		b.mu.Lock()
		res[port] = 0
	}
}

// Verifica o estado dos outros servidores
func (b *BullyElection) HealthCheck() {
	// Cria um waitgroup de goroutines para guardar todas as healthchecks
	var wg sync.WaitGroup

	results := make(map[int]int)

	// passa por cada peer e lança um goroutine
	for _, peer := range b.peers {
		wg.Go(func() {
			b.IsAlive(peer, results)
		})
	}

	// espera todas completarem
	wg.Wait()

	b.peersState = results
}

// Função de heartbeat, para rodar a cada segundo
func (b *BullyElection) HeartBeat() {
	for {
		b.HealthCheck()

		// checa se líder ainda está vivo
		if b.peersState[b.GetLeader()] == 0 {
			b.SetLeaderless()
			b.StartElection()
		}
	}
}
