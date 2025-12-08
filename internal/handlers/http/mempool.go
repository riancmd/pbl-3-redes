package handlers

import (
	"encoding/json"
	"net/http"
	"pbl-2-redes/internal/models"
)

// Trabalha com os endpoints relacionadas ao mempool(array de transações pendentes)
func (h Handlers) registerMempoolEndpoints() {
	http.HandleFunc("POST /internal/blockchain/mempool", h.postTransation)	
}

// Insere uma transação na mempool do servidor nó
func (h Handlers) postTransation(w http.ResponseWriter, r *http.Request) {
	var tx models.Transaction
	
	err := json.NewDecoder(r.Body).Decode(&tx)
	if err != nil {
		http.Error(w, "Erro ao decodificar a transação", http.StatusBadRequest)
		return
	}

	err = h.useCases.repos.transactions.Enqueue(tx)
	if err != nil {
		http.Error(w, fmt.Sprintf("Erro ao enfileirar transação: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "Transação recebida e adicionada à mempool."})
}
