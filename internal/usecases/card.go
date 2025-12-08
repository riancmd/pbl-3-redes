package usecases

import (
	"errors"
	"log/slog"
	"math/rand"
	"pbl-2-redes/internal/models"
	"strconv"
	"time"
)

// Retorna todo o vault de cartas
func (u *UseCases) GetAllCards() []models.Booster {
	cards := u.repos.Card.GetAll()
	return cards
}

// Adiciona novas cartas ao vault
func (u *UseCases) AddCards(newBooster models.Booster) error {
	if u.repos.Card == nil {
		return errors.New("vault doesn't exist")
	}

	u.repos.Card.Add(newBooster)

	return nil
}

// recebe as informações da transação, como a assinatura e publicKey
func (u *UseCases) BoosterRequest(txReq models.TransactionRequest) error {
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

// Pega um booster do vault e o retorna
func (u *UseCases) GetBooster(randomIndex int) (models.Booster, error) {
	return u.repos.Card.GetBooster(randomIndex)
}

// Remove um booster do vault
// Deve ser usado depois de comprar a carta, internamente
// Caso um servidor que não foi o que comprou a carta pra seu cliente receber uma notificação para remover certa carta,
// usa-se essa função
// Dessa forma, não é preciso sincronizar diretamente nela, pois ela só funciona caso tenha acontecido uma compra
func (u *UseCases) RemoveBooster(BID int) error {
	return u.repos.Card.Remove(BID)
}

// função que atualiza vault de cartas
// filename: indica onde está localizado o arquivo
// boosters_qt: indica a quantidade de boosters a serem criados
func (u *UseCases) AddCardsFromFile(filename string, boosters_qt int) error {
	// cria o glossário de cartas
	glossary, err := u.utils.CardDB.LoadCardsFromFile(filename)
	if err != nil {
		slog.Error("couldn't load cards from file")
		return err
	}

	// conta quantidade de cartas a partir do glossário
	// considerando as raridades
	cardCopies := u.utils.CardDB.CalculateCardCopies(glossary, boosters_qt)

	// chama funções para popular o vault a partir do glossário
	// primeiro, cria o pool de cartas
	cardPool := u.utils.CardDB.CreateCardPool(glossary, cardCopies)

	// depois, cria os boosters individualmente
	boosters, err := u.utils.CardDB.CreateBoosters(cardPool, boosters_qt)

	if err != nil {
		slog.Error("couldn't create boosters")
		return err
	}

	// se não houve nenhum erro, consegue adicionar os boosters ao repo
	for _, booster := range boosters {
		u.repos.Card.Add(booster)
	}

	return nil
}
