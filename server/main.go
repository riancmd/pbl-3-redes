package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"PlanoZ/internal/blockchain"
	"PlanoZ/internal/models"
	"PlanoZ/internal/utils/cardDB"

	"github.com/fatih/color"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// configs gerais
const (
	// topicos do redis
	TopicoConectar     = "conectar"
	TopicoComprarCarta = "comprar_carta"

	// configs de rede
	HealthCheckInterval = 5 * time.Second
	RequestTimeout      = 2 * time.Second
)

// struct principal do servidor
type Server struct {
	ID         string
	Host       string // ex: "server1:9090"
	UDPAddr    string // ex: "server1:8083"
	IsLeader   bool
	StartTimer time.Time

	// nova arquitetura
	Blockchain *blockchain.Blockchain
	CardDB     *cardDB.CardDB
	Boosters   []models.Booster // estoque local de boosters

	// redis
	redisClient *redis.ClusterClient
	ctx         context.Context

	// controle de liderança e cluster
	currentLeader string
	serverList    map[string]string // id -> host api
	liveServers   map[string]bool   // quem está vivo
	muLeader      sync.RWMutex
	muLiveServers sync.RWMutex

	// memória do jogo
	playerList map[string]models.PlayerInfo // lista de players online
	muPlayers  sync.RWMutex

	batalhas   map[string]*models.Batalha // batalhas rolando (apenas no host)
	muBatalhas sync.Mutex

	trades   map[string]*models.Troca // trocas rolando
	muTrades sync.Mutex

	// mapas pra comunicacao peer to peer (saber pra quem responder)
	batalhasPeer   map[string]models.PeerBattleInfo
	muBatalhasPeer sync.RWMutex

	tradesPeer   map[string]models.PeerTradeInfo
	muTradesPeer sync.RWMutex

	// api engine
	ginEngine *gin.Engine
}

// info básica de onde o player está (qual servidor)
type PlayerInfo struct {
	ServerID     string
	ServerHost   string
	ReplyChannel string // canal do redis para comunicar
}

func main() {
	// 1. pega configs do ambiente (docker compose)
	serverID := os.Getenv("SERVER_ID")
	apiPort := os.Getenv("API_PORT")
	udpPort := os.Getenv("UDP_PORT")
	redisAddrs := os.Getenv("REDIS_ADDRS")
	serverListEnv := os.Getenv("SERVER_LIST")

	if serverID == "" {
		serverID = "server-unknown-" + uuid.New().String()
	}

	// 2. conecta no redis cluster
	rdb := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs: strings.Split(redisAddrs, ","),
	})

	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		color.Red("Falha ao conectar no Redis: %v", err)
	} else {
		color.Green("Conectado ao Redis Cluster!")
	}

	// 3. parse da lista de servidores pra descoberta inicial
	sList := make(map[string]string)
	if serverListEnv != "" {
		parts := strings.Split(serverListEnv, ",")
		for _, p := range parts {
			// p = "server1:9090"
			hostParts := strings.Split(p, ":")
			if len(hostParts) >= 1 {
				sID := hostParts[0] // usa hostname como id
				sList[sID] = p
			}
		}
	}

	// 4. sobe o DB de cartas e blockchain
	cd := cardDB.New()

	// carrega o json 
	color.Cyan("Carregando banco de dados de cartas...")
	definitions, err := cd.InitializeCardsFromJSON("cardVault.json")
	var generatedBoosters []models.Booster

	if err != nil {
		color.Red("Erro crítico ao carregar cardVault.json: %v. O servidor iniciará sem cartas.", err)
		generatedBoosters = []models.Booster{}
	} else {
		// gera estoque inicial (100 boosters)
		copies := cd.CalculateCardCopies(definitions, 100)
		pool := cd.CreateCardPool(definitions, copies)
		generatedBoosters = cd.CreateBoosters(pool)
		color.Green("Estoque gerado: %d boosters disponíveis.", len(generatedBoosters))
	}

	bc := blockchain.New()

	// monta a struct do server
	s := &Server{
		ID:         serverID,
		Host:       serverID + ":" + apiPort,
		UDPAddr:    serverID + ":" + udpPort,
		StartTimer: time.Now(),

		Blockchain: bc,
		CardDB:     cd,
		Boosters:   generatedBoosters,

		redisClient:  rdb,
		ctx:          ctx,
		serverList:   sList,
		liveServers:  make(map[string]bool),
		playerList:   make(map[string]models.PlayerInfo),
		batalhas:     make(map[string]*models.Batalha),
		batalhasPeer: make(map[string]models.PeerBattleInfo),
		trades:       make(map[string]*models.Troca),
		tradesPeer:   make(map[string]models.PeerTradeInfo),
	}

	// 5. goroutines rodando paralelamente

	// A) loop da blockchain (processa blocos que chegam)
	go s.Blockchain.RunBlockchainLoop()

	// B) ouve redis (conexoes e compras globais)
	go s.listenRedisGlobal(TopicoConectar)
	go s.listenRedisGlobal(TopicoComprarCarta)

	// C) ping UDP
	go s.RunUDP(udpPort)

	// D) sobe api rest
	s.ginEngine = s.setupRouter()
	go s.RunAPI(apiPort)

	// minerador e listener de blocos
	go s.RunBlockListener()
	go s.RunMiner()

	// 6. logs 
	externalPort := os.Getenv("EXTERNAL_PORT")
	if externalPort == "" {
		externalPort = apiPort
	}

	color.Cyan("===========================================")
	color.Cyan("  PLANO Z - SERVIDOR BLOCKCHAIN")
	color.Cyan("===========================================")
	color.White("Server ID:      %s", s.ID)
	color.White("API Interna:    %s:%s", serverID, apiPort)
	color.White("API Externa:    localhost:%s", externalPort)
	color.White("UDP Interna:    %s:%s", serverID, udpPort)
	color.White("Servidores:     %v", s.serverList)
	color.Cyan("===========================================")

	// 7. espera sinal manual (ENTER) para eleição
	color.Yellow("\nPressione ENTER para iniciar Health Checks e Eleição...")
	bufio.NewReader(os.Stdin).ReadString('\n')

	// 8. heartbeating
	go s.RunHealthChecks()

	// 9. espera para detectar servers ativos
	color.Yellow("Aguardando descoberta de nós (3s)...")
	time.Sleep(3 * time.Second)

	// 10. mostrar servers conectados e vivos
	s.muLiveServers.RLock()
	color.Green("Nós vivos detectados: %d", len(s.liveServers))
	for id, alive := range s.liveServers {
		if alive {
			color.Green("  ✓ %s", id)
		}
	}
	s.muLiveServers.RUnlock()

	// 11. votação do líder
	color.Yellow("\nIniciando eleição de líder...")
	s.electNewLeader(nil)

	select {}
}

// funcoes de inicializacao

// sobe o gin
func (s *Server) RunAPI(port string) {
	color.Green("Iniciando servidor API Gin na porta :%s", port)
	// ouve em "0.0.0.0:port"
	if err := s.ginEngine.Run(":" + port); err != nil {
		panic(fmt.Sprintf("Falha ao iniciar Gin: %v", err))
	}
}

// sobe o udp
func (s *Server) RunUDP(port string) {
	// ouve em "0.0.0.0:port"
	udpAddr, err := net.ResolveUDPAddr("udp", ":"+port)
	if err != nil {
		color.Red("Erro ao resolver porta UDP %s: %v", port, err)
		return
	}
	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		color.Red("Erro na criação da porta UDP %s: %v", port, err)
		return
	}
	defer udpConn.Close()

	color.Green("Servidor UDP ouvindo em :%s", port)
	s.lidarPing(udpConn)
}
