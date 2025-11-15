package bully

import (
	"errors"
	"log/slog"
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
