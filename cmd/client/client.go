package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"pbl-2-redes/internal/models"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9" // Conexão Redis
)

var (
	enc *json.Encoder
	dec *json.Decoder

	// Variáveis de conexão ao Redis Cluster
	rdb *redis.ClusterClient
	ctx context.Context

	// Variáveis para conexão UDP
	udpPort  string
	pingChan chan bool //Canal para parar a goroutine de heartbeating

	//Canal no redis para requisições de batalha ou compra ao servidor logado
	serverChannel string

	// Dados do jogador
	uid          string
	username     string
	loggedIn     bool
	replyChannel string //Canal no Redis Cluster para o cliente receber respostas
	battleId     string // Id da batalha que entrou

	// dados do jogo
	inventory  []*models.Card
	invMu      sync.RWMutex
	hand       []*models.Card
	matchInfo  *models.Match
	inBattle   bool
	turnSignal chan struct{}

	// Novo mutex para dados da partida
	matchMu sync.RWMutex
)

const (
	//Tipos de requisições
	register string = "register"
	login    string = "login"
	buypack  string = "buyNewPack"
	battle   string = "battle"
	usecard  string = "useCard"
	giveup   string = "giveUp"
	ping     string = "ping"
	trade    string = "trade"

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
	error         string = "error"
	tradeEnqueued string = "tradeEnqueued"

	//Tipos de canais para dar Publish
	AuthResquestChannel string = "AuthResquestChannel"
	BuyResquestChannel  string = "BuyResquestChannel"
)

type CardType string

const (
	REM  CardType = "rem"
	NREM CardType = "nrem"
	Pill CardType = "pill"
)

type CardRarity string

const (
	Comum   CardRarity = "comum"
	Incomum CardRarity = "incomum"
	Rara    CardRarity = "rara"
)

type CardEffect string

const (
	AD   CardEffect = "adormecido"
	CONS CardEffect = "consciente"
	PAR  CardEffect = "paralisado"
	AS   CardEffect = "assustado"
	NEN  CardEffect = "nenhum"
)

type DreamState string

const (
	sleepy    DreamState = "adormecido"
	conscious DreamState = "consciente"
	paralyzed DreamState = "paralisado"
	scared    DreamState = "assustado"
)

func main() {
	//Endereços das instâncias dos redis
	clusterAddrs := []string{
		"redis-1:7000",
		"redis-2:7001",
		"redis-3:7002",
		"redis-4:7003",
		"redis-5:7004",
		"redis-6:7005",
	}

	ctx = context.Background()

	// Se conecta ao cluster
	rdb = redis.NewClusterClient(&redis.ClusterOptions{
		Addrs: clusterAddrs,
	})

	// Testa a conexão
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("❌ Falha ao conectar ao cluster Redis: %v", err)
	}

	fmt.Println("✅ Conectado ao Cluster Redis.")

	// Geração de Id único
	uid = uuid.New().String()
	replyChannel = fmt.Sprintf("ClientChannel:%s", uid) //Gera o nome do canal de respostas do cliente
	fmt.Printf("🆔 ID do Cliente: %s\n", uid)
	fmt.Printf("📬 Escutando Respostas em: %s\n", replyChannel)

	// Inicializa variáveis de estado
	turnSignal = make(chan struct{}, 1)
	matchInfo = &models.Match{
		Sanity:      make(map[string]int),
		DreamStates: make(map[string]models.DreamState),
	}

	// Goroutine para lidar com mensagens que chegam no canal pessoal do cliente
	go handleServerMessages()

	// Mostrar o menu do jogo
	showMenu()
}

func handleServerMessages() {
	//Criação do canal do REDIS
	pubsub := rdb.Subscribe(ctx, replyChannel)
	defer pubsub.Close()

	// Espera a confirmação da inscrição
	_, err := pubsub.Receive(ctx)
	if err != nil {
		log.Fatalf("Falha ao se inscrever no canal de resposta: %v", err)
	}

	//Canal da linguagem (diferente do canal em redis)
	ch := pubsub.Channel()

	for msg := range ch {
		//Respostas são recebidas em uma struct genérica que é decodificada para uma resposta específica
		var externalResponse models.ExternalResponse

		if err := json.Unmarshal([]byte(msg.Payload), &externalResponse); err != nil {
			fmt.Printf("❌ Erro ao decodificar mensagem do servidor: %v\n", err)
			continue
		}

		// Valida se a resposta é para este cliente
		if externalResponse.UserId != uid {
			fmt.Println("Recebida mensagem para outro UserId, ignorando.")
			continue
		}

		// Processa e Decodifica a resposta
		handleResponse(externalResponse)
	}
}

func showMenu() {
	reader := bufio.NewReader(os.Stdin)
	for {
		if inBattle {
			<-turnSignal
			handleBattleTurn()
			continue
		}

		clearScreen()
		fmt.Println("--- Menu ---")
		if !loggedIn {
			fmt.Println("1. Registrar")
			fmt.Println("2. Login")
		} else {
			fmt.Println("3. Comprar booster")
			fmt.Println("4. Ver inventário")
			fmt.Println("5. Batalhar")
			fmt.Println("6. Trocar")
			fmt.Println("7. Ping")
		}
		fmt.Println("8. Sair")
		fmt.Print("Escolha uma opção: ")

		input, _ := reader.ReadString('\n')
		choice := strings.TrimSpace(input)

		switch choice {
		case "1":
			if !loggedIn {
				handleRegister(reader)
			}
		case "2":
			if !loggedIn {
				handleLogin(reader)
			}
		case "3":
			if loggedIn {
				handleBuyPack()
			}
		case "4":
			if loggedIn {
				printInventory()
			}
		case "5":
			if loggedIn {
				handleEnqueue()
			}
		case "6":
			if loggedIn {
				handleTradeEnqueue()
			}
		case "7":
			if loggedIn {
				handlePing()
			}
		case "8":
			fmt.Println("💤 Bons sonhos...")
			stopPinger()
			return
		default:
			fmt.Println("Opção inválida.")
		}
	}
}

func handleResponse(extRes models.ExternalResponse) {
	clearScreen()
	switch extRes.Type { //Decodificar para tipo de resposta mais exata
	case registered:
		var authResp models.AuthResponse
		json.Unmarshal(extRes.Data, &authResp)

		if authResp.Status {
			loggedIn = true
			username = authResp.Username
			udpPort = authResp.UDPPort
			serverChannel = authResp.ServerChannel

			stopPinger()                  // Caso já exista algum pinger antigo (Deu login e saiu)
			pingChan = make(chan bool)    // Novo canal para controlar a parada do pinger heartbeating
			go heartBeatHandler(pingChan) // Inicia o HeartBeat

			fmt.Printf("✅ Bem vindo Jogador: %s\n", username)
			fmt.Printf("Você ganhou 4 boosters gratuitos! Eles já estão em seu inventário\n")
			fmt.Print("Você está conectado ao servidor de porta UDP %s", udpPort)
		} else {
			fmt.Printf("❌ Falha no registro: %s\n", authResp.Message)
		}

	case loggedin:
		var authResp models.AuthResponse
		json.Unmarshal(extRes.Data, &authResp)

		if authResp.Status {
			loggedIn = true
			udpPort = authResp.UDPPort
			serverChannel = authResp.ServerChannel

			stopPinger()                  // Caso já exista algum pinger antigo (Deu login e saiu)
			pingChan = make(chan bool)    // Novo canal para controlar a parada do pinger heartbeating
			go heartBeatHandler(pingChan) // Inicia o HeartBeat

			fmt.Printf("✅ Bem-vindo, %s!\n", username)
			fmt.Print("Você está conectado ao servidor de porta UDP %s", udpPort)

		} else {
			fmt.Printf("❌ Falha no login: %s\n", authResp.Message)
		}

	case packbought:
		var purchaseResp models.ClientPurchaseResponse
		json.Unmarshal(extRes.Data, &purchaseResp)

		if purchaseResp.Status {
			fmt.Println("🎁 Novo booster adquirido! Veja em seu inventário")
			invMu.Lock()

			for i := range purchaseResp.BoosterGenerated.Booster {
				c := purchaseResp.BoosterGenerated.Booster[i]
				inventory = append(inventory, &c)
			}
			invMu.Unlock()

		} else {
			fmt.Printf("❌ Erro ao comprar booster: %s\n", purchaseResp.Message)
		}

	case enqueued:
		var matchResp models.MatchResponse
		json.Unmarshal(extRes.Data, &matchResp)
		fmt.Printf("⏳ %s\n", matchResp.Message)

	case tradeEnqueued:
		var tradeResp models.TradeResponse
		json.Unmarshal(extRes.Data, &tradeResp)
		fmt.Printf("⏳ %s\n", tradeResp.Message)

	case gamestart:
		var payload models.PayLoad

		json.Unmarshal(extRes.Data, &payload)
		inBattle = true
		matchMu.Lock()
		hand = make([]*models.Card, len(payload.Hand))

		for i := range payload.Hand {
			hand[i] = &payload.Hand[i]
		}

		//Salvar id da batalha que entrou
		battleId = payload.BattleId

		matchInfo.P2 = payload.P2
		matchInfo.Sanity = payload.Sanity
		matchInfo.DreamStates = payload.DreamStates
		matchInfo.Turn = payload.Turn
		matchMu.Unlock()

		fmt.Printf("⚔️ Partida encontrada! Você está batalhando contra %s.\n", matchInfo.P2.Username)
		fmt.Println("Sanidade inicial:")
		fmt.Printf("Você: %d\n", matchInfo.Sanity[uid])
		fmt.Printf("Seu oponente: %d\n", matchInfo.P2.UID)
		if matchInfo.Turn == uid {
			turnSignal <- struct{}{}
		} else {
			fmt.Printf("⏳ Turno do seu oponente. Aguarde...\n")
		}

	case newturn:
		var payload models.PayLoad

		json.Unmarshal(extRes.Data, &payload)
		matchMu.Lock()
		matchInfo.Turn = payload.Turn
		matchMu.Unlock()

		if matchInfo.Turn == uid {
			fmt.Printf("\n--- Status do Jogo ---\n")
			fmt.Printf("Rodada: %d\n", matchInfo.CurrentRound)
			fmt.Printf("Sua Sanidade: %d (%s)\n", matchInfo.Sanity[uid], strings.Title(string(matchInfo.DreamStates[uid])))
			opponentUID := matchInfo.P2.UID
			fmt.Printf("Sanidade do Oponente: %d (%s)\n", matchInfo.Sanity[opponentUID], strings.Title(string(matchInfo.DreamStates[opponentUID])))
			fmt.Println("\n➡️ É o seu turno! Escolha uma carta para jogar (pelo número) ou digite `gv` para desistir.")
			select {
			case <-turnSignal:
			default:
			}
			turnSignal <- struct{}{}
		} else {
			fmt.Printf("\n--- Status do Jogo ---\n")
			fmt.Printf("Rodada: %d\n", matchInfo.CurrentRound)
			fmt.Printf("Sua Sanidade: %d (%s)\n", matchInfo.Sanity[uid], strings.Title(string(matchInfo.DreamStates[uid])))
			opponentUID := matchInfo.P2.UID
			fmt.Printf("Sanidade do Oponente: %d (%s)\n", matchInfo.Sanity[opponentUID], strings.Title(string(matchInfo.DreamStates[opponentUID])))
			fmt.Printf("\n⏳ Turno do seu oponente. Aguarde...\n")
		}

	case updateinfo:
		var payload models.PayLoad

		json.Unmarshal(extRes.Data, &payload)
		matchMu.Lock()
		matchInfo.Sanity = payload.Sanity
		matchInfo.DreamStates = payload.DreamStates
		matchInfo.CurrentRound = payload.Round
		matchMu.Unlock()

		fmt.Printf("\n--- Status do Jogo ---\n")
		fmt.Printf("Rodada: %d\n", matchInfo.CurrentRound)
		fmt.Printf("Sua Sanidade: %d (%s)\n", matchInfo.Sanity[uid], strings.Title(string(matchInfo.DreamStates[uid])))
		opponentUID := matchInfo.P2.UID
		fmt.Printf("Sanidade do Oponente: %d (%s)\n", matchInfo.Sanity[opponentUID], strings.Title(string(matchInfo.DreamStates[opponentUID])))

	case newvictory:
		inBattle = false
		fmt.Println("\n🎉 Vitória! Você venceu a partida!")

	case newloss:
		inBattle = false
		fmt.Println("\n💔 Derrota. Você perdeu a partida.")

	case newtie:
		inBattle = false
		fmt.Println("\n🤝 Empate! A partida terminou em um empate.")

	case error:
		var errResp models.ErrorResponse
		json.Unmarshal(extRes.Data, &errResp)
		fmt.Printf("❌ Erro do servidor (%s): %s\n", errResp.Type, errResp.Message)

	default:
		fmt.Printf("Recebida mensagem desconhecida do servidor: %s\n", extRes.Type)
	}
}

// As função abaixo para cada opção do menu atualizada para lógica de PUB/SUB
func handleRegister(reader *bufio.Reader) {
	fmt.Print("Digite seu nome de usuário: ")
	usernameInput, _ := reader.ReadString('\n')
	username = strings.TrimSpace(usernameInput)

	fmt.Print("Digite sua senha: ")
	password, _ := reader.ReadString('\n')
	password = strings.TrimSpace(password)

	req := models.AuthenticationRequest{
		Type:               register,
		UserId:             uid,
		ClientReplyChannel: replyChannel,
		Username:           username,
		Password:           password,
	}
	publishRequest(AuthResquestChannel, req)
}

func handleLogin(reader *bufio.Reader) {
	fmt.Print("Digite seu nome de usuário: ")
	usernameInput, _ := reader.ReadString('\n')
	username = strings.TrimSpace(usernameInput) // Armazena o username globalmente

	fmt.Print("Digite sua senha: ")
	password, _ := reader.ReadString('\n')
	password = strings.TrimSpace(password)

	req := models.AuthenticationRequest{
		Type:               login,
		UserId:             uid,
		ClientReplyChannel: replyChannel,
		Username:           username,
		Password:           password,
	}
	publishRequest(AuthResquestChannel, req)
}

func handleBuyPack() {
	req := models.PurchaseRequest{
		UserId:             uid,
		ClientReplyChannel: replyChannel,
	}
	publishRequest(BuyResquestChannel, req)
}

func handleEnqueue() {
	req := models.MatchRequest{
		UserId:             uid,
		ClientReplyChannel: replyChannel,
	}

	bytesReq, err := json.Marshal(req)
	if err != nil {
		log.Fatalf("Erro ao serializar requisição de batalha: %v", err)
	}

	extReq := models.ExternalRequest{
		Type:   battle,
		UserId: uid,
		Data:   json.RawMessage(bytesReq),
	}

	publishRequest(serverChannel, extReq)
}

// Função nova para troca
func handleTradeEnqueue() {
	if serverChannel == "" {
		fmt.Println("❌ Canal do servidor não definido. Tente logar novamente.")
		return
	}
	req := models.TradeRequest{ // <-- Usa TradeRequest
		UserId:             uid,
		ClientReplyChannel: replyChannel,
	}

	bytesReq, err := json.Marshal(req)
	if err != nil {
		fmt.Printf("❌ Erro ao serializar requisição de troca: %v\n", err)
		return
	}

	extReq := models.ExternalRequest{
		Type:   trade, // <-- Tipo 'trade'
		UserId: uid,
		Data:   json.RawMessage(bytesReq),
	}

	publishRequest(serverChannel, extReq) // <-- Envia para fila PESSOAL do servidor
}

func handleBattleTurn() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("\nSua mão atual:\n")
	printHand()
	fmt.Print("Sua jogada: ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "gv" {
		giveUp()
		return
	}

	matchMu.RLock()
	index, err := strconv.Atoi(input)
	if err != nil || index < 1 || index > len(hand) {
		matchMu.RUnlock()
		fmt.Println("❌ Entrada inválida. Por favor, jogue uma carta pelo seu número (ex: 1) ou digite `gv` para desistir.")
		// Envia um novo sinal para o canal para que o menu de batalha se repita
		select {
		case <-turnSignal:
		default:
		}
		turnSignal <- struct{}{}
		return
	}
	cardToPlay := hand[index-1]
	matchMu.RUnlock()

	useCard(cardToPlay)
}

func useCard(card *models.Card) {
	req := models.NewCardRequest{
		BattleId:           battleId,
		UserId:             uid,
		ClientReplyChannel: replyChannel,
		Card:               *card,
	}

	bytesReq, err := json.Marshal(req)
	if err != nil {
		log.Fatalf("Erro ao serializar requisição de uso de carta: %v", err)
	}

	extReq := models.ExternalRequest{
		Type:   usecard,
		UserId: uid,
		Data:   json.RawMessage(bytesReq),
	}

	publishRequest(serverChannel, extReq)

	matchMu.Lock()
	defer matchMu.Unlock()
	// remove a carta da mão localmente
	for i, c := range hand {
		if c.CID == card.CID {
			hand = append(hand[:i], hand[i+1:]...)
			break
		}
	}
}

func giveUp() {
	req := models.GameActionRequest{
		BattleId:           battleId,
		Type:               giveup,
		UserId:             uid,
		ClientReplyChannel: replyChannel,
	}

	bytesReq, err := json.Marshal(req)
	if err != nil {
		log.Fatalf("Erro ao serializar requisição de desistência: %v", err)
	}

	extReq := models.ExternalRequest{
		Type:   giveup,
		UserId: uid,
		Data:   json.RawMessage(bytesReq),
	}

	publishRequest(serverChannel, extReq)
}

func handlePing() {
	if udpPort == "" {
		fmt.Println("❌ Porta UDP do servidor não definida. Tente logar novamente.")
		return
	}

	serverAddr, err := net.ResolveUDPAddr("udp", udpPort)
	if err != nil {
		fmt.Printf("❌ erro ao resolver endereço: %v\n", err)
		return
	}

	connection, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		fmt.Printf("❌ erro ao conectar: %v\n", err)
		return
	}
	defer connection.Close()

	// timeout de 999 ms
	connection.SetReadDeadline(time.Now().Add(999 * time.Millisecond))

	start := time.Now()
	// Envia o UID como "ping"
	_, err = connection.Write([]byte(uid))
	if err != nil {
		fmt.Printf("❌ erro ao enviar ping: %v\n", err)
		return
	}

	buffer := make([]byte, 1024)
	n, _, err := connection.ReadFromUDP(buffer)
	if err != nil {
		fmt.Printf("⏰ timeout: %v\n", err) // Servidor demorou > 999ms
		return
	}

	if string(buffer[:n]) == "pong" {
		elapsed := time.Since(start).Milliseconds()
		fmt.Printf("🏓 latência: %d ms\n", elapsed)
	} else {
		fmt.Printf("❌ resposta inválida: %s\n", string(buffer[:n]))
	}
}

// Função auxiliar para fechar canal de ping
func stopPinger() {
	if pingChan != nil {
		// Fechar o canal envia um sinal para a goroutine parar
		close(pingChan)
		pingChan = nil
	}
}

// Função auxiliar para forçar a voltar ao login
func forceLogout() {
	if !loggedIn {
		return
	}

	loggedIn = false
	inBattle = false
	stopPinger() // Para a goroutine de ping

	clearScreen()
	fmt.Println("\n=============================================")
	fmt.Println("❌ Conexão com o servidor perdida (timeout UDP).")
	fmt.Println("Você foi desconectado. Por favor, faça login novamente.")
	fmt.Println("=============================================")
}

// Função que checa constantemente se o servidor está ativo (HeartBeating)
func heartBeatHandler(stopChan <-chan bool) {
	serverAddr, err := net.ResolveUDPAddr("udp", udpPort)
	if err != nil {
		fmt.Println("Endereço UDP inválido, parando heartbeat.")
		forceLogout()
		return
	}

	for {
		select {
		case <-stopChan:
			// O canal foi fechado (sinal para parar vindo do stopPinger)
			fmt.Println("Heartbeat parado.")
			return

		case <-time.After(5 * time.Second):
			// Espera 5 segundos antes de fazer o check
			conn, err := net.DialUDP("udp", nil, serverAddr)
			if err != nil {
				// Se nem consegue "discar", o servidor caiu feio
				forceLogout()
				return
			}

			// Timeout de 3 segundos para a RESPOSTA
			conn.SetReadDeadline(time.Now().Add(3 * time.Second))

			if _, err := conn.Write([]byte(uid)); err != nil {
				conn.Close()
				continue // Tenta de novo no próximo loop
			}

			buffer := make([]byte, 16)
			_, _, err = conn.ReadFromUDP(buffer)
			if err != nil {
				// --- DETECÇÃO DE FALHA ---
				// Servidor não respondeu em 3 segundos.
				conn.Close()
				forceLogout() // Desloga
				return        // Mata esta goroutine
			}

			// Se chegou aqui, está tudo OK.
			conn.Close()
		}
	}
}

// função que mostra inventário
func printInventory() {
	invMu.RLock()
	defer invMu.RUnlock()

	if len(inventory) == 0 {
		fmt.Println("inventário vazio.")
		time.Sleep(1 * time.Second)
		return
	}
	fmt.Println("\n📦 inventário:")
	for _, c := range inventory {
		fmt.Printf("%s) %s\n", c.CID, strings.Title(c.Name)) // Assumindo C.CID
		fmt.Printf(" Tipo: %s\n", strings.Title(string(c.CardType)))
		if c.Points == 0 {
			fmt.Printf(" Pontos: %d\n", c.Points)
		} else {
			if c.CardType == models.Pill {
				fmt.Printf(" Pontos: +%d\n", c.Points)
			} else {
				fmt.Printf(" Pontos: -%d\n", c.Points)
			}
		}
		fmt.Printf(" Raridade: %s\n", strings.Title(string(c.CardRarity)))
		fmt.Printf(" Efeito: %s\n", strings.Title(string(c.CardEffect)))
		fmt.Printf(" Descrição: %s\n", strings.Title(c.Desc))
		fmt.Println(strings.Repeat("-", 40))
	}

	fmt.Println("\nPressione Enter para continuar...")
	bufio.NewReader(os.Stdin).ReadBytes('\n')

}

func printHand() {
	matchMu.RLock()
	defer matchMu.RUnlock()

	if len(hand) == 0 {
		fmt.Println("Sua mão está vazia!")
		return
	}
	fmt.Println(strings.Repeat("=", 40))
	for i, c := range hand {
		fmt.Printf("%d) %s (Tipo: %s, Pontos: %d, Efeito: %s)\n", i+1, c.Name, c.CardType, c.Points, c.CardEffect)
	}
	fmt.Println(strings.Repeat("=", 40))
}

func clearScreen() {
	switch runtime.GOOS {
	case "linux", "darwin": // Unix-like systems
		cmd := exec.Command("clear")
		cmd.Stdout = os.Stdout
		cmd.Run()
	case "windows":
		cmd := exec.Command("cmd", "/c", "cls")
		cmd.Stdout = os.Stdout
		cmd.Run()
	default:
		fmt.Println(strings.Repeat("\n", 50)) // fallback
	}
}

// Função para colocar na fila do canal no redis uma requisição
func publishRequest(channel string, payload interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		fmt.Printf("❌ Erro ao codificar requisição: %v\n", err)
		return
	}

	if err := rdb.LPush(ctx, channel, data).Err(); err != nil {
		fmt.Printf("❌ Erro ao ENFILEIRAR  requisição: %v\n", err)
	}
}
