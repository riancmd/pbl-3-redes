package usecases

import (
	"encoding/json"
	"errors"
	"log/slog"
	"pbl-2-redes/internal/models"
	"time"
)

// Retorna repositório contendo todas as partidas
// Serve para sincronizar as partidas
func (u *UseCases) GetAllMatches() []models.Match {
	matches := u.repos.Match.GetAll()
	return matches
}

// Adiciona uma partida à lista de partidas
func (u *UseCases) AddMatch(matchReq models.MatchInitialRequest) error {
	if !u.sync.IsLeader() {
		// Verifica se usuário já está em partida
		players := make([]string, 0)
		players = append(players, matchReq.P1, matchReq.P2)

		for _, player := range players {
			onMatch := u.repos.Match.UserOnMatch(player)
			if onMatch {
				slog.Error("user is already on a match")
				return errors.New("user already on a match")
			}
		}
		err := u.sync.MatchNew(matchReq)

		if err != nil {
			slog.Error("couldn't create match")
			return err
		}

		return nil
	}

	handP1, err := u.sync.GetHand(matchReq.P1)
	if err != nil {
		return err
	}
	handP2, err := u.sync.GetHand(matchReq.P1)
	if err != nil {
		return err
	}

	// estrutura request da partida
	mReq := models.Match{
		ID:      matchReq.ID,
		Server1: matchReq.Server1,
		Server2: matchReq.Server2,
		P1:      matchReq.P1,
		P2:      matchReq.P2,
		State:   models.Running,
		Turn:    matchReq.P1,

		Hand:             map[string][]*models.Card{},
		Sanity:           map[string]int{matchReq.P1: 40, matchReq.P2: 40},
		DreamStates:      map[string]models.DreamState{matchReq.P2: models.Sleepy, matchReq.P2: models.Sleepy},
		RoundsInState:    map[string]int{matchReq.P1: 0, matchReq.P2: 0},
		StateLockedUntil: map[string]int{matchReq.P1: 0, matchReq.P2: 0},
		CurrentRound:     1,
	}

	mReq.Hand[mReq.P1] = handP1
	mReq.Hand[mReq.P2] = handP2

	// descobrindo quem é

	u.matchesMU.Lock()

	u.repos.Match.Add(mReq)

	u.matchesMU.Unlock()

	return nil
}

// Finaliza partida
func (u *UseCases) EndMatch(ID string) error {
	// Verifica se partida realmente finalizou
	finished := u.repos.Match.MatchEnded(ID)

	if !finished {
		slog.Error("this battle hasn't finished yet", "battleID", ID)
		return errors.New("battle is still going")
	}

	u.sync.MatchEnd(ID)
	err := u.repos.Match.Remove(ID)

	if err != nil {
		return err
	}

	return nil
}

// Enviar msg
func (u *UseCases) SendMsg(msg models.BattleRequest) {
	select {
	case u.inbox <- msg:
		// entregou

	default:
		slog.Error("inbox cheio")
	}

}

// Dispatcher é a goroutine que ouve o canal principal e envia pras goroutines
func (u *UseCases) Dispatcher() {
	for msg := range u.inbox {
		u.inboxMU.Lock()

		targetInbox, found := u.inboxes[msg.BattleID]

		u.inboxMU.Unlock()

		if found {
			select {
			case targetInbox <- msg.MatchMsg:
				//entregou
			default:
				slog.Error("Inbox cheio")
			}
		}
	}

}

// Goroutine responsável por ouvir se existem batalhas
func (u *UseCases) CheckNewMatches() {
	allMatches := []models.Match{}

	// loop de verificação
	for {
		time.Sleep(50 * time.Millisecond)
		allMatches = u.repos.Match.GetAll()

		// se tiver mais de uma partida, passa pela lista
		if u.repos.Match.Length() >= 1 {
			// para cada partida, se for minha partida E não estiver gerenciada, gerencio
			for _, match := range allMatches {
				if match.Server1 != u.sync.GetServerID() && match.Server2 != u.sync.GetServerID() {
					continue
				}
				u.matchesMU.Lock()
				// se tiver sendo gerenciada já
				if u.managedMatches[match.ID] {
					u.matchesMU.Unlock()
					continue
				}
				slog.Error("new match found")
				u.managedMatches[match.ID] = true

				u.matchesMU.Unlock()

				go u.ManageMatch(match)
			}
		}
	}
}

// Gerencia a partida
func (u *UseCases) ManageMatch(match models.Match) {
	// crio o inbox
	matchInbox := make(chan models.MatchMsg, 16) // 16 é um bom buffer, como no código antigo
	u.inboxMU.Lock()
	u.inboxes[match.ID] = matchInbox
	u.inboxMU.Unlock()

	slog.Info("started a new match")

	// Limpeza ao final da partida
	defer func() {
		slog.Warn("Encerrando gerenciamento da partida", "matchID", match.ID)

		u.matchesMU.Lock()
		delete(u.managedMatches, match.ID) // u.managedMatches é o mapa em UseCases
		u.matchesMU.Unlock()

		// Limpa o inbox
		u.inboxMU.Unlock()
		delete(u.inboxes, match.ID)
		u.inboxMU.Unlock()
		close(matchInbox)
	}()

	// identificando o dono da partida
	// server1 é o "dono" da partida
	isPrimaryServer := match.Server1 == u.sync.GetServerID()
	if isPrimaryServer {
		slog.Info("this is the primary server", "matchID", match.ID)
		// (Opcional) Notifica os jogadores que a partida começou
		// u.NotifyGameStart(match) // -> Você precisará criar esta função
	} else {
		slog.Info("this is the secondary server", "matchID", match.ID)
	}

	var timeout <-chan time.Time

	for {
		// pego a partida como está no repositório
		currentMatch, err := u.repos.Match.GetMatch(match.ID)
		if err != nil {
			slog.Error("match not found", "matchID", match.ID, "err", err)
			return //partida foi removida ou deu erro
		}

		// Se a partida terminou, apenas encerra.
		if currentMatch.State == models.Finished {
			slog.Info("Partida já está finalizada, encerrando goroutine", "matchID", match.ID)
			return
		}

		// Configura o timeout
		// APENAS o servidor primário pode dar timeout.
		if isPrimaryServer {
			// Timeout 15 segundos
			timeout = time.After(15 * time.Second)
		} else {
			// Servidores secundários esperam para sempre (nil channel)
			timeout = nil
		}

		// CORE DA FUNÇÃO
		select {
		// lido com as mensagens de ação do jogador
		case msg, ok := <-matchInbox:
			if !ok {
				slog.Error("match channel was closed")
				return // O canal foi fechado (provavelmente pelo defer)
			}

			// Validação (do processTurn)
			// Verifica turno
			if msg.PlayerUID != currentMatch.Turn {
				continue
			}

			// Processa a Ação
			switch msg.Action {
			case "usecard":
				slog.Info("processing 'usecard'", "matchID", match.ID, "player", msg.PlayerUID)
				// HandleUseCard tem a lógica dos antigos:
				// 1. handleUseCard
				// 2. updateGameState
				// 3. checkGameEnd
				// 4. switchTurn
				// 5. Por fim, o novo u.sync.UpdateMatch(matchAtualizada)
				if err := u.HandleUseCard(currentMatch.ID, msg.Data); err != nil {
					slog.Error("error while processing 'usecard'", "err", err)
				}

			case "giveup":
				slog.Info("processing 'giveup'", "matchID", match.ID, "player", msg.PlayerUID)
				// HandleGiveUp tem a lógica de:
				// 1. handleGiveUp (do matchManager.go)
				// 2. u.sync.UpdateMatch(matchAtualizada)
				if err := u.HandleGiveUp(currentMatch.ID, msg.Data); err != nil {
					slog.Error("error while processing 'giveup'", "err", err)
				}
			}

		// se tiver timeout
		case <-timeout:
			// só o primeiro servidor que gerencia isso
			slog.Warn("timeout", "matchID", match.ID, "player", currentMatch.Turn)
			if err := u.HandleTimeout(currentMatch.ID); err != nil {
				slog.Error("Erro ao processar 'timeout'", "err", err)
			}
		}
		// O loop 'for' recomeça, busca o novo estado da partida no repositório,
		// e reinicia o timer para o (possivelmente) novo jogador.
	}
}

// HandleUsecard
func (u *UseCases) HandleUseCard(matchID string, data json.RawMessage) error {
	type cardReq struct {
		Card models.Card `json:"card"`
	}

	var req cardReq

	if err := json.Unmarshal(data, &req); err != nil {
		return err
	}

	return nil
	//if !u.
}

// HandleGiveUp
func (u *UseCases) HandleGiveUp(matchID string, data json.RawMessage) error {
	return nil
}

// HandleTimeOut
func (u *UseCases) HandleTimeout(matchID string) error {
	return nil
}

// Atualizar partida
func (u *UseCases) UpdateMatch(match models.Match) error {
	//u.sync.UpdateMatch(match)
	return nil
}
