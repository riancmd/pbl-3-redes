package cardDB

import (
	"PlanoZ/internal/models"
	"encoding/json"
	"errors"
	"math/rand"
	"os"
	"time"

	"github.com/google/uuid"
)

const CARDS_PER_BOOSTER = 3

type CardDB struct {
	// cachezinho pra guardar o que leu do json
	definitions map[string]models.CardData
}

func New() *CardDB {
	return &CardDB{
		definitions: make(map[string]models.CardData),
	}
}

// le o json com os status base dos tanques
func (cd *CardDB) InitializeCardsFromJSON(filename string) (map[string]models.CardData, error) {
	file, err := os.ReadFile(filename)
	if err != nil {
		return nil, errors.New("reading file error: " + err.Error())
	}

	// wrapper so pra bater com o formato do json {"cards": { "key": { ... } }}
	type JsonWrapper struct {
		Cards map[string]models.CardData `json:"cards"`
	}

	var wrapper JsonWrapper
	err = json.Unmarshal(file, &wrapper)
	if err != nil {
		return nil, errors.New("unmarshal error: " + err.Error())
	}

	cd.definitions = wrapper.Cards
	return wrapper.Cards, nil
}

// calcula quantas copias de cada carta vai ter no total, baseado na raridade (50/40/10)
func (cd *CardDB) CalculateCardCopies(glossary map[string]models.CardData, totalBoosters int) map[string]int {
	totalCards := totalBoosters * CARDS_PER_BOOSTER
	copies := make(map[string]int)

	// separa os ids por raridade
	var commons, uncommons, rares []string
	for id, card := range glossary {
		switch card.Raridade {
		case models.RarityCommon:
			commons = append(commons, id)
		case models.RarityUncommon:
			uncommons = append(uncommons, id)
		case models.RarityRare:
			rares = append(rares, id)
		default:
			commons = append(commons, id)
		}
	}

	// contas de padaria pra saber os totais
	countCommon := int(float64(totalCards) * 0.50)
	countUncommon := int(float64(totalCards) * 0.40)
	countRare := int(float64(totalCards) * 0.10)

	// distribui igual entre os modelos que tem
	distribute := func(ids []string, total int) {
		if len(ids) > 0 {
			perCard := total / len(ids)
			if perCard < 1 {
				perCard = 1
			} // garante pelo menos uma carta
			for _, id := range ids {
				copies[id] = perCard
			}
		}
	}

	distribute(commons, countCommon)
	distribute(uncommons, countUncommon)
	distribute(rares, countRare)

	return copies
}

// cria as instancias reais dos tanques, com uuids unicos pra blockchain
func (cd *CardDB) CreateCardPool(glossary map[string]models.CardData, copies map[string]int) []models.Tanque {
	var pool []models.Tanque

	for cid, quantity := range copies {
		data, exists := glossary[cid]
		if !exists {
			continue
		}

		for i := 0; i < quantity; i++ {
			instance := models.Tanque{
				ID:        uuid.New().String(), // gera id unico, importante pro ledger
				Modelo:    data.Modelo,
				Raridade:  data.Raridade,
				Vida:      data.Vida,
				Ataque:    data.Ataque,
				Timestamp: time.Now().Unix(),
				OwnerID:   "", // vai ser preenchido na hora da compra
			}
			pool = append(pool, instance)
		}
	}
	return pool
}

// embaralha tudo e monta os pacotinhos
func (cd *CardDB) CreateBoosters(cardPool []models.Tanque) []models.Booster {
	// usa rand local pra nao dar ruim com concorrencia
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// mistura
	r.Shuffle(len(cardPool), func(i, j int) {
		cardPool[i], cardPool[j] = cardPool[j], cardPool[i]
	})

	var boosters []models.Booster
	numBoosters := len(cardPool) / CARDS_PER_BOOSTER

	for i := 0; i < numBoosters; i++ {
		start := i * CARDS_PER_BOOSTER
		end := start + CARDS_PER_BOOSTER

		// copia pra um slice novo pra evitar referencia compartilhada
		pack := make([]models.Tanque, CARDS_PER_BOOSTER)
		copy(pack, cardPool[start:end])

		b := models.Booster{
			BID:   i + 1,
			Cards: pack,
		}
		boosters = append(boosters, b)
	}

	return boosters
}
