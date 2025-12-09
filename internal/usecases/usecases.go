package usecases

import (
	"pbl-2-redes/internal/models"
	"pbl-2-redes/internal/repositories"
	"pbl-2-redes/internal/utils"
	"sync"
)

type UseCases struct {
	repos     *repositories.Repositories
	utils     *utils.Utils
	sync      ClusterSync
	matchesMU sync.Mutex
	usersMU   sync.Mutex
	cardsMU   sync.Mutex
	inboxMU   sync.Mutex
	bqueue    sync.Mutex
	tqueue    sync.Mutex

	inbox chan models.BattleRequest // se eu criar esse inbox aqui, toda vez que o handler pubsub receber alguma requisição com o id de batalha, ele joga aqui
	// a goroutine de dispatcher fica ouvindo esse inbox e verifica se a coisa é da batalha de fulaninho
	// se a coisa for da batalha de fulaninho, dispatcher envia pro canal dele
	// lá na goroutine de batalha vai ter um case que verifica QUAL o tipo de requisição
	// de acordo com o tipo
	inboxes map[string]chan models.MatchMsg
	outboxes map[string]

	managedMatches map[string]bool
}

func New(repos *repositories.Repositories, csync ClusterSync) *UseCases {
	return &UseCases{
		repos:          repos,
		utils:          utils.New(),
		sync:           csync,
		matchesMU:      sync.Mutex{},
		usersMU:        sync.Mutex{},
		cardsMU:        sync.Mutex{},
		inboxMU:        sync.Mutex{},
		bqueue:         sync.Mutex{},
		tqueue:         sync.Mutex{},
		managedMatches: make(map[string]bool, 0),
	}
}
