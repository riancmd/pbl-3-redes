package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"pbl-2-redes/internal/usecases"
)

// Representa uma parte da camada de comunicação inbound, o servidor HTTP no cluster
// Possui como dependência o useCases
type Handlers struct {
	useCases *usecases.UseCases
}

func New(useCases *usecases.UseCases) *Handlers {
	return &Handlers{useCases: useCases}
}

// Função que inicia o servidor
// port: representa a porta onde o servidor irá ouvir
func (h Handlers) Listen(port int) error {
	// Possui o endpoint /internal/users
	h.registerUserEndpoints()
	// Possui o endpoint /internal/cards e internal/cards/purchase
	h.registerCardEndpoints()
	// Possui o endpoint /internal/battle_queue
	h.registerBattleQueueEndpoints()
	// Possui o endpoint /internal/trading_queue
	h.registerTradingQueueEndpoints()
	// Possui o endpoint /internal/matches
	h.registerMatchesEndpoints()
	// Possui o endpoint /internal/health
	h.registerHealthEndpoints()
	// Possui o endpoint /internal/blockchain/ledger
	h.registerLedgerEndpoints()
	// Possui o endpoint /internal/blockchain/mempool
	h.registerMempoolEndpoints()
	

	slog.Info("listening on", "port", port)

	return http.ListenAndServe(
		fmt.Sprintf(":%v", port),
		nil,
	)
}
