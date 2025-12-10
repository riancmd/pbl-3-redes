package main

import (
	"PlanoZ/internal/models"
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/fatih/color"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// estados possiveis do cliente
const (
	EstadoLivre = iota
	EstadoPareado
	EstadoEsperandoResposta
	EstadoBatalhando
	EstadoTrocando
	EstadoReconectando
)

// variaveis globais pra controlar o jogo
var (
	idPessoal           string
	idParceiro          string
	idBatalha           string
	idTroca             string
	minhasCartas        []models.Tanque
	indiceCartaOfertada int
	estadoAtual         int

	// parte de infraestrutura e rede
	serverAPI          string
	serverUDP          string
	serverID           string // id do server onde to conectado
	redisClient        *redis.ClusterClient
	canalRedisResposta string // meu canal exclusivo no redis
	httpClient         *http.Client
	ctx                = context.Background()

	// variaváveis para Ping
	latenciaMedia     int64 // em ms
	cancelarHeartbeat atomic.Bool

	// criptografia (assinatura)
	chavePrivada      *ecdsa.PrivateKey
	chavePublicaBytes []byte
)

func main() {
	// 1. config inicial basica
	lerVariaveisAmbiente() // pega o SERVER_API das variaveis ou pede pro usuario
	gerarChaves()          // gera as chaves da carteira

	idPessoal = uuid.New().String()
	canalRedisResposta = "player_reply_" + idPessoal
	httpClient = &http.Client{Timeout: 5 * time.Second}

	color.Cyan("==============================================")
	color.Cyan("      PLANO Z - CLIENTE (BLOCKCHAIN WALLET)   ")
	color.Cyan("==============================================")
	color.White("ID Jogador: %s", idPessoal)
	color.White("Chave Pública (Hash): %x...", chavePublicaBytes[:10])
	fmt.Println()

	// 2. conecta no cluster redis
	conectarRedis()

	// 3. faz login no server 
	if !registrarNoServidor() {
		color.Red("Falha fatal ao registrar no servidor. Encerrando.")
		return
	}

	// 4. sobe os listeners em background
	go listenRedis()
	go monitorarLatencia()

	// 5. loop principal do menu
	reader := bufio.NewReader(os.Stdin)
	for {
		exibirMenu()
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		processarComando(input, reader)
	}
}

// função de gerar as chaves para criptografia
func gerarChaves() {
	var err error
	chavePrivada, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic("Erro ao gerar chaves criptográficas: " + err.Error())
	}

	// serializa a chave publica pra bytes pra poder mandar na rede
	chavePublicaBytes = elliptic.Marshal(elliptic.P256(), chavePrivada.PublicKey.X, chavePrivada.PublicKey.Y)
}

// função de criar assinatura digital pra request
func assinarRequest(req *models.TransactionRequest) error {
	// dados que vao ser assinados: payload + timestamp + user + tipo
	// a ordem TEM que ser a mesma que o server usa pra verificar
	dataToSign := []string{
		req.Payload,
		fmt.Sprintf("%d", req.Timestamp), // transformar em timestamp em string
		req.UserID,
		string(req.Type),
	}

	jsonData, _ := json.Marshal(dataToSign)
	hash := sha256.Sum256(jsonData)

	r, s, err := ecdsa.Sign(rand.Reader, chavePrivada, hash[:])
	if err != nil {
		return err
	}

	// junta R e S
	req.Signature = append(r.Bytes(), s.Bytes()...)
	req.PublicKey = chavePublicaBytes
	return nil
}

// função principal de para iniciar o menu do cliente
func exibirMenu() {
	color.Yellow("\n--- MENU ---")
	switch estadoAtual {
	case EstadoLivre:
		fmt.Println("1. Abrir Booster (Comprar Carta - Blockchain)")
		fmt.Println("2. Ver Minhas Cartas")
		fmt.Println("3. Parear com Jogador")
		fmt.Println("4. Trocar de Servidor (Re-login)")
		fmt.Println("5. Sair")
		color.Blue("6. Ver Blockchain (Ledger)")
	case EstadoPareado:
		fmt.Println("1. Iniciar Batalha")
		fmt.Println("2. Iniciar Troca")
		fmt.Println("3. Desparear")
	case EstadoEsperandoResposta:
		color.Magenta("Aguardando confirmação da Blockchain ou do Oponente...")
	case EstadoBatalhando:
		color.Red("Batalha em andamento! Aguarde seu turno...")
	case EstadoTrocando:
		color.Green("Negociação em andamento...")
	}
	fmt.Print("Escolha: ")
}

func processarComando(input string, reader *bufio.Reader) {
	if estadoAtual == EstadoLivre {
		switch input {
		case "1":
			comprarBoosterSigned()
		case "2":
			verCartas()
		case "3":
			fmt.Print("Digite o ID do oponente: ")
			opID, _ := reader.ReadString('\n')
			parear(strings.TrimSpace(opID))
		case "4":
			registrarNoServidor()
		case "5":
			os.Exit(0)
		case "6":
			verBlockchain()
		case "7":
			verMempool()
		default:
			fmt.Println("Opção inválida")
		}
	} else if estadoAtual == EstadoPareado {
		switch input {
		case "1":
			solicitarBatalha()
		case "2":
			solicitarTroca()
		case "3":
			desparear()
		default:
			fmt.Println("Opção inválida")
		}
	}
}


// função para comprar booster atualizada para lógica de blockchain
func comprarBoosterSigned() {
	color.Yellow("Iniciando transação de compra...")

	// 1. cria o payload
	payloadObj := models.PurchasePayload{Intent: "buy_booster_standard"}
	payloadBytes, _ := json.Marshal(payloadObj)

	// 2. monta a request
	req := models.TransactionRequest{
		Type:      models.TxPurchase,
		UserID:    idPessoal,
		Timestamp: time.Now().Unix(),
		Payload:   string(payloadBytes),
	}

	// 3. assina digitalmente
	if err := assinarRequest(&req); err != nil {
		color.Red("Erro ao assinar transação: %v", err)
		return
	}

	// 4. manda para o cluster de servidores
	url := fmt.Sprintf("http://%s/cards/buy", serverAPI)
	body, _ := json.Marshal(req)
	resp, err := httpClient.Post(url, "application/json", strings.NewReader(string(body)))
	if err != nil {
		color.Red("Erro de conexão: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusAccepted {
		color.Green("✅ Transação enviada para a Mempool! Aguardando mineração...")
		// nao bloqueia o usuario
	} else {
		color.Red("Erro na compra: Status %d", resp.StatusCode)
	}
}

// função para ber o ledger
func verBlockchain() {
	url := fmt.Sprintf("http://%s/blockchain/", serverAPI)
	resp, err := httpClient.Get(url)
	if err != nil {
		color.Red("Erro: %v", err)
		return
	}
	defer resp.Body.Close()

	var data map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&data)

	formatted, _ := json.MarshalIndent(data, "", "  ")
	fmt.Println(string(formatted))
	fmt.Println("Pressione Enter para voltar.")
	bufio.NewReader(os.Stdin).ReadString('\n')
}

// função auxiliar para ver a mempool
func verMempool() {
	url := fmt.Sprintf("http://%s/blockchain/mempool", serverAPI)
	resp, err := httpClient.Get(url)
	if err != nil {
		color.Red("Erro: %v", err)
		return
	}
	defer resp.Body.Close()

	var data map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&data)
	formatted, _ := json.MarshalIndent(data, "", "  ")
	fmt.Println(string(formatted))
	fmt.Println("Pressione Enter para voltar.")
	bufio.NewReader(os.Stdin).ReadString('\n')
}

// função para começar a ouvir respostas do server
func listenRedis() {
	pubsub := redisClient.Subscribe(ctx, canalRedisResposta)
	defer pubsub.Close()

	for {
		msg, err := pubsub.ReceiveMessage(ctx)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		var envelope map[string]interface{} // envelope generico pra receber os dados
		json.Unmarshal([]byte(msg.Payload), &envelope)

		tipo := envelope["tipo"].(string)

		// converte payload para json e depois para struct
		payloadBytes, _ := json.Marshal(envelope["payload"])

		switch tipo {

		// respostas vindas da blockchain
		case "Compra_Sucesso":
			var dados struct {
				Mensagem string         `json:"mensagem"`
				Booster  models.Booster `json:"booster"`
				TxID     string         `json:"tx_id"`
			}
			json.Unmarshal(payloadBytes, &dados)
			color.Green("\n💰 [BLOCKCHAIN] %s", dados.Mensagem)
			color.White("Bloco Minerado! TxID: %s", dados.TxID)
			color.Yellow("Você recebeu %d cartas:", len(dados.Booster.Cards))
			for _, c := range dados.Booster.Cards {
				fmt.Printf("- %s (%s)\n", c.Modelo, c.Raridade)
				minhasCartas = append(minhasCartas, c)
			}
			exibirMenu() // atualiza o menu

		case "Troca_Confirmada":
			var dados struct {
				Msg  string `json:"mensagem"`
				TxID string `json:"tx_id"`
			}
			json.Unmarshal(payloadBytes, &dados)
			color.Green("\n🤝 [BLOCKCHAIN] %s (Tx: %s)", dados.Msg, dados.TxID)
			estadoAtual = EstadoLivre
			exibirMenu()

		case "Rank_Update":
			color.Yellow("\n🏆 [BLOCKCHAIN] Vitória registrada no ledger!")

		case "Inicio_Batalha":
			var p models.RespostaInicioBatalha
			json.Unmarshal(payloadBytes, &p)
			color.Red("\n⚔️ Batalha iniciada contra %s!", p.Mensagem)
			idBatalha = p.IdBatalha
			estadoAtual = EstadoBatalhando
			go loopBatalha()

		case "Inicio_Troca":
			var p models.RespostaInicioTroca
			json.Unmarshal(payloadBytes, &p)
			color.Green("\n🤝 Troca iniciada com %s!", p.Mensagem)
			idTroca = p.IdTroca
			estadoAtual = EstadoTrocando
			go loopTroca()
		}
	}
}


func loopBatalha() {
	
	fmt.Println("Iniciando batalha")
	time.Sleep(2 * time.Second)
	registrarResultadoBatalha(idBatalha, idPessoal)
	estadoAtual = EstadoPareado
}

func registrarResultadoBatalha(battleID, winnerID string) {
	payloadObj := models.BattleResultPayload{
		BattleID: battleID,
		Winner:   winnerID,
	}
	payloadBytes, _ := json.Marshal(payloadObj)

	req := models.TransactionRequest{
		Type:      models.TxBattleResult,
		UserID:    idPessoal,
		Timestamp: time.Now().Unix(),
		Payload:   string(payloadBytes),
	}
	assinarRequest(&req)

	url := fmt.Sprintf("http://%s/battle/register", serverAPI)
	body, _ := json.Marshal(req)
	httpClient.Post(url, "application/json", strings.NewReader(string(body)))
	fmt.Println("Resultado enviado para Blockchain.")
}


// função auxiliar para ver inventário
func verCartas() {
	color.Cyan("Suas Cartas:")
	if len(minhasCartas) == 0 {
		fmt.Println("(Vazio)")
	}
	for i, c := range minhasCartas {
		fmt.Printf("%d. %s [%s] (Atk: %d | HP: %d)\n", i+1, c.Modelo, c.Raridade, c.Ataque, c.Vida)
	}
	fmt.Println("Pressione Enter.")
	bufio.NewReader(os.Stdin).ReadString('\n')
}

func registrarNoServidor() bool {
	// tenta conectar na api definida no serverAPI
	url := fmt.Sprintf("http://%s/players/connect", serverAPI)


	req := models.LeaderConnectRequest{
		PlayerID:     idPessoal,
		ServerID:     "client_node",
		ServerHost:   "client_ip",
		ReplyChannel: canalRedisResposta,
	}
	body, _ := json.Marshal(req)

	// manda pro server atual (assumindo que ele é lider ou repassa)
	resp, err := httpClient.Post(url, "application/json", strings.NewReader(string(body)))
	if err != nil {
		color.Red("Erro ao conectar no servidor %s: %v", serverAPI, err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		color.Red("Servidor rejeitou conexão")
		return false
	}

	color.Green("Conectado ao servidor %s com sucesso!", serverAPI)
	serverUDP = strings.Split(serverAPI, ":")[0] + ":8083"
	return true
}

// função auxiliar para pegar informações do server definidas no docker-compose.yml
func lerVariaveisAmbiente() {
	serverAPI = os.Getenv("SERVER_API")
	if serverAPI == "" {
		fmt.Print("Digite o IP:PORT do servidor (ex: localhost:9090): ")
		fmt.Scanln(&serverAPI)
	}

	addrRedis := os.Getenv("REDIS_ADDR")
	if addrRedis == "" {
		addrRedis = "localhost:6379"
	}
}

func conectarRedis() {
	addrs := os.Getenv("REDIS_ADDRS")
	if addrs == "" {
		addrs = "localhost:6379"
	} // fallback se nao tiver nada

	redisClient = redis.NewClusterClient(&redis.ClusterOptions{
		Addrs: strings.Split(addrs, ","),
	})

	if err := redisClient.Ping(ctx).Err(); err != nil {
		color.Red("Aviso: Não foi possível conectar ao Redis (%v). O jogo não funcionará corretamente.", err)
	} else {
		color.Green("Redis conectado.")
	}
}

func monitorarLatencia() {

	for {
		if cancelarHeartbeat.Load() {
			time.Sleep(1 * time.Second)
			continue
		}

		start := time.Now()
		conn, err := net.DialTimeout("udp", serverUDP, 2*time.Second)
		if err == nil {
			conn.Write([]byte("ping"))
			buffer := make([]byte, 1024)
			conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			_, err = conn.Read(buffer)
			conn.Close()
		}

		if err != nil {
			// color.Red("Latência: Timeout")
		} else {
			latenciaMedia = time.Since(start).Milliseconds()
			// color.Green("Latência: %d ms", latenciaMedia)
		}
		time.Sleep(5 * time.Second)
	}
}

// funcoes de gameplay p2p, seguindo a lógica anterior de uso de json
func parear(id string) { idParceiro = id; estadoAtual = EstadoPareado; fmt.Println("Pareado com", id) }
func desparear()       { idParceiro = ""; estadoAtual = EstadoLivre; fmt.Println("Despareado") }

func solicitarBatalha() {
	url := fmt.Sprintf("http://%s/battle/initiate", serverAPI)
	req := models.BattleInitiateRequest{IdBatalha: uuid.New().String(), IdJogadorLocal: idPessoal, IdOponente: idParceiro, HostServidor: serverAPI}
	body, _ := json.Marshal(req)
	httpClient.Post(url, "application/json", strings.NewReader(string(body)))
	fmt.Println("Solicitação de batalha enviada...")
}
func solicitarTroca() {
	url := fmt.Sprintf("http://%s/trade/initiate", serverAPI)
	req := models.BattleInitiateRequest{IdBatalha: uuid.New().String(), IdJogadorLocal: idPessoal, IdOponente: idParceiro, HostServidor: serverAPI}
	body, _ := json.Marshal(req)
	httpClient.Post(url, "application/json", strings.NewReader(string(body)))
	fmt.Println("Solicitação de troca enviada...")
}
func loopTroca() {
	time.Sleep(2 * time.Second)
	estadoAtual = EstadoPareado
}
