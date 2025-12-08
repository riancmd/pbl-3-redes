package handlers

import (
	"encoding/json"
	"net/http"
	"pbl-2-redes/internal/models"
)

// Trabalha com os endpoints relacionadas ao ledger (livro)
func (h Handlers) registerLedgerEndpoints() {
	http.HandleFunc("GET /internal/blockchain/ledger", h.getLedger)	
}

// Retorna todo o legder
func (h Handlers) getLedger(w http.ResponseWriter, r *http.Request) {
	ledger := h.useCases.GetBlockchain()
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ledger)
}

