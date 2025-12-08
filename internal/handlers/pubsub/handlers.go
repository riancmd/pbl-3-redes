package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"pbl-2-redes/internal/models"
	"pbl-2-redes/internal/usecases"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Constantes das filas globais de requisição
const (
	AuthRequestChannel string = "AuthRequestChannel"
	BuyRequestChannel  string = "BuyRequestChannel"
)

const (
	//Tipos de respostas
	registered    string = "registered"
	loggedin      string = "loggedIn"
	packbought    string = "packBought"
	enqueued      string = "enqueued"
	gamestart     string = "gameStart"
	cardused      string = "cardUsed"
	notify        string = "notify"
	updateinfo    string = "updateInfo"
	newturn       string = "newTurn"
	newloss       string = "newLoss"
	newvictory    string = "newVictory"
	newtie        string = "newTie"
	pong          string = "pong"
	errorResponse string = "error"
	tradeEnqueued string = "tradeEnqueued"
)

type PubSubHandlers struct {
	useCases    *usecases.UseCases
	rdb         *redis.ClusterClient
	ctx         context.Context
	serverQueue string
	udpPort     string
}

// New cria uma nova instância dos handlers
func New(useCases *usecases.UseCases, rdb *redis.ClusterClient, ctx context.Context, serverQueue string, udpPort string) *PubSubHandlers {
	return &PubSubHandlers{
		useCases:    useCases,
		rdb:         rdb,
		ctx:         ctx,
		serverQueue: serverQueue,
		udpPort:     udpPort,
	}
}

// Listen inicia todos os ouvintes de cada tipo de tópico do servidor em goroutines
func (h *PubSubHandlers) Listen() {
	// Inicia os ouvintes para as filas globais
	go h.SubscribeQueueAuth()
	go h.SubscribeQueueBuy()

	// Inicia o ouvinte para a fila pessoal deste servidor
	go h.SubscribeQueueServerPersonal()

	slog.Info("Servidor iniciado.",
		"auth_queue", AuthRequestChannel,
		"buy_queue", BuyRequestChannel,
		"personal_queue", h.serverQueue,
	)
}

// SubscribeQueueAuth ouve a fila de autenticação (login e cadastro)
func (h *PubSubHandlers) SubscribeQueueAuth() {
	slog.Info("Ouvinte de Auth iniciado. Aguardando trabalhos...")
	for {
		// Apenas UM servidor (que chamar primeiro) pegará o trabalho.
		result, err := h.rdb.BLPop(h.ctx, 0, AuthRequestChannel).Result()

		if err != nil {
			slog.Error("Erro no BLPop Auth", "error", err)
			time.Sleep(1 * time.Second)
			continue
		}

		if len(result) < 2 {
			continue // Resultado inesperado
		}
		payload := result[1] // Pega o json da requisição (em string)

		// Processa o trabalho em uma nova goroutine
		go h.processAuth(payload)
	}
}

// SubscribeQueueBuy ouve a fila de compra de booster
func (h *PubSubHandlers) SubscribeQueueBuy() {
	slog.Info("Ouvinte de Buy] iniciado. Aguardando trabalhos...")
	for {
		result, err := h.rdb.BLPop(h.ctx, 0, BuyRequestChannel).Result()

		if err != nil {
			slog.Error("Erro no BLPop Buy", "error", err)
			time.Sleep(1 * time.Second)
			continue
		}

		if len(result) < 2 {
			continue
		}
		payload := result[1] // Pega o json da requisição (em string)

		// Processa o trabalho em uma nova goroutine
		go h.processBuy(payload)
	}
}

// SubscribeQueueServerPersonal ouve a fila de Batalha/Troca (Pessoal do server)
func (h *PubSubHandlers) SubscribeQueueServerPersonal() {
	slog.Info("Ouvinte da Fila Pessoal  iniciado.", "Aguardando trabalhos em %s ...", h.serverQueue)

	for {
		result, err := h.rdb.BLPop(h.ctx, 0, h.serverQueue).Result()

		if err != nil {
			slog.Error("Erro no BLPop Pessoal %s: %v", h.serverQueue, err)
			time.Sleep(1 * time.Second)
			continue
		}
		if len(result) < 2 {
			continue
		}
		payload := result[1] // Pega o json da requisição (em string)

		// Processa o trabalho em uma nova goroutine
		go h.processServerPersonal(payload)
	}
}

// ProcessAuth processa qual tipo de requisição (login ou cadastro)
func (h *PubSubHandlers) processAuth(payload string) {
	var req models.AuthenticationRequest

	if err := json.Unmarshal([]byte(payload), &req); err != nil {
		slog.Error("Falha ao decodificar AuthenticationRequest [Auth]", "error", err)
		return
	}

	slog.Info("Processando trabalho. Tipo: %s. Id: %s", req.Type, req.UserId)

	var responseData models.AuthResponse
	var responseType string
	var processingError error

	switch req.Type {
	case "register":

		userRequest := models.CreateUserRequest{
			UID:      req.UserId,
			Username: req.Username,
			Password: req.Password,
		}

		//Usa função criada na camada de UseCases para processar a requisição dentro do cluster de servidores
		processingError = h.useCases.AddUser(userRequest)

		status := true
		message := "Registrado com sucesso"

		if processingError != nil {
			status = false
			message = "Não foi possível fazer cadastro"
		}

		responseData = models.AuthResponse{
			Status:        status,
			Username:      req.Username,
			UDPPort:       h.udpPort,     // Porta UDP do server
			ServerChannel: h.serverQueue, // Fila pessoal para simular conexão a server que tratou cliente
			Message:       message,
		}

		responseType = registered

	case "login":

		//Usa função criada na camada de UseCases para processar a requisição dentro do cluster de servidores
		status, err := h.useCases.Login(req.Username, req.Password)
		processingError = err

		message := "Sucesso no Login"
		if processingError != nil {
			message = "Não foi possível fazer login"
		}

		responseData = models.AuthResponse{
			Status:        status,
			Username:      req.Username,
			UDPPort:       h.udpPort,     // Porta UDP do server
			ServerChannel: h.serverQueue, // Fila pessoal para simular conexão a server que tratou cliente
			Message:       message,
		}

		responseType = loggedin

	default:
		processingError = fmt.Errorf("Tipo de requisição [Auth] desconhecido: %s", req.Type)
		responseType = errorResponse
	}

	if processingError != nil { // Erro
		slog.Error("Erro ao processar Auth]", "error", processingError)

		errResp := models.ErrorResponse{
			Type:    responseType,
			Message: processingError.Error(),
		}

		h.sendResponse(req.ClientReplyChannel, req.UserId, responseType, errResp)

	} else { //Sucesso
		h.sendResponse(req.ClientReplyChannel, req.UserId, responseType, responseData)
	}
}

// ProcessBuy decodifica e processa o trabalho de Compra
func (h *PubSubHandlers) processBuy(payload string) {
	var req models.PurchaseRequest
	if err := json.Unmarshal([]byte(payload), &req); err != nil {
		slog.Error("Falha ao decodificar PurchaseRequest [Buy]", "error", err, "payload", payload)
		return
	}

	slog.Info("Processando trabalho [Buy]", "user", req.UserId)

	var responseData models.ClientPurchaseResponse
	var responseType string = packbought

	//Usa função criada na camada de UseCases para processar a requisição dentro do cluster de servidores
	booster, processingError := h.useCases.GetBooster()

	// Envia a  resposta
	if processingError != nil { //Erro
		slog.Error("Erro ao processar job [Buy]", "error", processingError, "user", req.UserId)

		errResp := models.ErrorResponse{
			Type:    errorResponse,
			Message: processingError.Error(),
		}

		h.sendResponse(req.ClientReplyChannel, req.UserId, errorResponse, errResp)
	} else { //Sucesso
		responseData = models.ClientPurchaseResponse{
			Status:           true,
			Message:          "Compra realizada!",
			BoosterGenerated: booster,
		}

		h.sendResponse(req.ClientReplyChannel, req.UserId, responseType, responseData)
	}
}

// ProcessServerPersonal decodifica o envelope e externo
func (h *PubSubHandlers) processServerPersonal(payload string) {
	// Decodifica o ENVELOPE EXTERNO primeiro para pegar UserId e Tipo
	var envelope models.ExternalRequest

	if err := json.Unmarshal([]byte(payload), &envelope); err != nil {
		slog.Error("Falha ao decodificar ExternalRequest [Personal]", "error", err, "payload", payload)
		return
	}

	slog.Info("Processando trabalho [Personal]", "type", envelope.Type, "user", envelope.UserId)

	var replyChannel string      //Canal de reply do cliente extraido
	var responseData interface{} // Guarda a resposta de sucesso do usecase
	var responseType string
	var processingError error

	// Switch com base no tipo dentro do envelope
	switch envelope.Type {
	case "battle":
		var req models.MatchRequest

		if err := json.Unmarshal(envelope.Data, &req); err != nil {
			processingError = fmt.Errorf("falha ao decodificar MatchRequest: %w", err)

			var baseReq struct {
				ClientReplyChannel string `json:"clientReplyChannel"`
			}

			if json.Unmarshal(envelope.Data, &baseReq) == nil { // Tentar pegar apenas o canal de reply
				replyChannel = baseReq.ClientReplyChannel
			}
			break
		}

		//Usa função criada na camada de UseCases para processar a requisição dentro do cluster de servidores
		err := h.useCases.Battle_Enqueue(req.UserId)

		if err != nil {
			processingError = err
		} else {
			responseData = models.MatchResponse{
				Type:    enqueued,
				Status:  true,
				Message: "Entrou na fila de batalha",
			}

			responseType = enqueued
		}

	case "trade":
		var req models.TradeRequest

		if err := json.Unmarshal(envelope.Data, &req); err != nil {
			processingError = fmt.Errorf("falha ao decodificar TradeRequest: %w", err)

			var baseReq struct {
				ClientReplyChannel string `json:"clientReplyChannel"`
			}

			if json.Unmarshal(envelope.Data, &baseReq) == nil {
				replyChannel = baseReq.ClientReplyChannel
			}
			break
		}

		replyChannel = req.ClientReplyChannel

		//Usa função criada na camada de UseCases para processar a requisição dentro do cluster de servidores
		err := h.useCases.Trading_Enqueue(req.UserId)

		if err != nil {
			processingError = err
		} else {
			responseData = models.TradeResponse{
				Type:    tradeEnqueued,
				Status:  true,
				Message: "Entrou na fila de troca",
			}

			responseType = tradeEnqueued
		}

	case "useCard":

		var req models.NewCardRequest

		if err := json.Unmarshal(envelope.Data, &req); err != nil {
			processingError = fmt.Errorf("falha ao decodificar NewCardRequest: %w", err)

			var baseReq struct {
				ClientReplyChannel string `json:"clientReplyChannel"`
			}

			if json.Unmarshal(envelope.Data, &baseReq) == nil {
				replyChannel = baseReq.ClientReplyChannel
			}
			break
		}

		replyChannel = req.ClientReplyChannel

		if err != nil {
			processingError = err
		} else {
			responseData = models.MatchResponse{} //Mudar aqui
			responseType = cardused
		}

	case "giveUp":
		var req models.GameActionRequest

		if err := json.Unmarshal(envelope.Data, &req); err != nil {

			var baseReq struct {
				ClientReplyChannel string `json:"clientReplyChannel"`
			}

			if json.Unmarshal(envelope.Data, &baseReq) == nil {
				replyChannel = baseReq.ClientReplyChannel

			} else {
				var basePayload struct {
					ClientReplyChannel string `json:"clientReplyChannel"`
				}

				if json.Unmarshal([]byte(payload), &basePayload) == nil {
					replyChannel = basePayload.ClientReplyChannel
				}

			}
			processingError = fmt.Errorf("falha ao decodificar GameActionRequest (giveUp): %w", err)
			break
		}

		if req.ClientReplyChannel != "" {
			replyChannel = req.ClientReplyChannel
		}

		//Mudar aqui a lógica
		var err error
		if err != nil {
			processingError = err
		} else {
			responseData = models.MatchResponse{} //Mudar aqui
			responseType = cardused
		}

	default:
		processingError = fmt.Errorf("tipo de requisição [Personal] desconhecido: %s", envelope.Type)
		var baseReq struct {
			ClientReplyChannel string `json:"clientReplyChannel"`
		}
		if json.Unmarshal([]byte(payload), &baseReq) == nil {
			replyChannel = baseReq.ClientReplyChannel
		}
	}

	// Envia a resposta
	if processingError != nil {
		slog.Error("Erro ao processar job [Personal]", "type", envelope.Type, "error", processingError)
		if replyChannel != "" {

			errResp := models.ErrorResponse{
				Type:    errorResponse,
				Message: processingError.Error(),
			}

			h.sendResponse(replyChannel, envelope.UserId, errorResponse, errResp)
		} else {
			slog.Warn("Erro ao processar job [Personal], mas sem canal de resposta conhecido.", "type", envelope.Type, "userId", envelope.UserId)
		}

	} else if responseData != nil {
		h.sendResponse(replyChannel, envelope.UserId, responseType, responseData)
	}
}

// Função de envio de resposta para cliente
func (h *PubSubHandlers) sendResponse(replyChannel, userID, responseType string, data interface{}) {
	// Serializa a resposta específica (data) para json.RawData
	dataRaw, err := json.Marshal(data)
	if err != nil {
		slog.Error("Falha ao serializar dados da resposta", "type", responseType, "error", err)
		return
	}

	// Cria o envelope externo
	finalResp := models.ExternalResponse{
		Type:   responseType,
		UserId: userID,
		Data:   dataRaw,
	}

	// Serializa o envelope externo para ser enviado pelo canal de reply
	respPayload, err := json.Marshal(finalResp)
	if err != nil {
		slog.Error("Falha ao serializar envelope da resposta", "type", responseType, "error", err)
		return
	}

	// Publica no canal de reply do cliente
	if err := h.rdb.Publish(h.ctx, replyChannel, respPayload).Err(); err != nil {
		slog.Error("Falha ao publicar resposta para o cliente", "channel", replyChannel, "error", err)
	}
}

// Go routine que vai ficar verificando novas compras, trocas e resultados de batalha
func (h *PubSubHandlers) newTransactionLoop() {
	for {
		// fica num loop dando sleep
		time.Sleep(50 * time.Millisecond)
		// se tiver transação a ser realizada, realiza
		if h.useCases.TransactionsLength() >= 1 {
			// da pop na fila
			t := h.useCases.GetFirstTransaction()
			// olha o tipo de transação
			switch t.Type {
			// tipo: compra
			case models.PC:
				// pega id do booster
				id, er := strconv.Atoi(t.Data[1])
				if er != nil {
					slog.Error(er.Error())
				}
				// envia booster pro usuário
				booster, err := h.useCases.GetBooster(id)
				if err != nil {
					h.sendResponse("ClientChannel:"+t.Data[0], t.Data[0],
						"packBought",
						models.ClientPurchaseResponse{
							Status:           true,
							Message:          "Compar realizada!",
							BoosterGenerated: booster,
						})
				}
			// tipo: troca
			case models.TD:
				// pega id do usuário e verifica qual q ele tem que mandar (o que for do servidor)
				if h.useCases.UIDExists(t.Data[0]) {
					cartaRecebida, err := strconv.Atoi(t.Data[3])
					if err != nil {

					}
					cartaEnviada, err := strconv.Atoi(t.Data[2])
					if err != nil {

					}
				} else {
					cartaRecebida, err := strconv.Atoi(t.Data[2])
					if err != nil {

					}
					cartaEnviada, err := strconv.Atoi(t.Data[3])
					if err != nil {

					}
				}
			// tipo: resultado de batalha
			case models.BR:
				if h.useCases.UIDExists(t.Data[1]) {
					battleID, err := strconv.Atoi(t.Data[0])
					if err != nil {

					}
					result, err := t.Data[3]
					if err != nil {

					}
				} else {
					battleID, err := strconv.Atoi(t.Data[0])
					if err != nil {

					}
					result, err := t.Data[4]
					if err != nil {

					}
				}
			}
		}
	}
}
