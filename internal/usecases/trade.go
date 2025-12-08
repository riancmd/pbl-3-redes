package usecases

import (
	"errors"
	"log/slog"
	"math/rand"
	"pbl-2-redes/internal/models"
	"strconv"
	"time"
)

// recebe as informações da transação, como a assinatura e publicKey
func (u *UseCases) TradeRequest(txReq models.TransactionRequest) error {
	// verifica se vault vazio
	empty := u.repos.Card.CardsEmpty()

	if empty {
		slog.Error("vault is empty")
		return errors.New("vault is empty")
	}

	// pega um indice aleatorio
	generator := rand.New(rand.NewSource(time.Now().UnixNano())) // gerador
	randomIndex := generator.Intn(u.repos.Card.Length())

	// cria a struct de transação
	transaction := models.Transaction{
		Type:      models.PC,
		Data:      []string{txReq.UserID, strconv.Itoa(randomIndex)},
		UserData:  []string{string(models.PC), txReq.UserID, txReq.Timestamp},
		PublicKey: txReq.PublicKey,
		Signature: txReq.Signature,
	}

	err := u.sync.BuyBooster(transaction)

	if err != nil {
		return err
	}

	return nil
}

// Troca carta de acordo com ID
func (u *UseCases) Trade(UID, CID string, card models.Card) error {
	err := u.sync.TradeCard(UID, CID, card)

	err = u.repos.User.SwitchCard(UID, CID, card)
	if err != nil {
		return err
	}

	return nil
}
