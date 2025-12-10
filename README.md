# 🎖️ PlanoZeta - Jogo de Tanques Multiplayer Distribuído

> *"Em PlanoZ, você comanda um pelotão de tanques históricos em batalhas estratégicas PvP. Colecione tanques lendários, negocie com outros comandantes e prove sua superioridade tática em combates turn-based distribuídos."*

## 📖 Sobre o Jogo

PlanoZeta é um jogo de cartas multiplayer baseado em tanques onde dois jogadores batalham usando decks montados com veículos blindados históricos. O sistema utiliza uma arquitetura distribuída com múltiplos servidores, Redis Cluster e eleição automática de líder. Agora, conta com uma Blockchain Customizada para garantir a integridade, propriedade e histórico de todas as ações críticas do jogo.

## 🚀 O Que Mudou? (Novas Features)

- Blockchain & Ledger Imutável:

Todas as compras, trocas e resultados de batalha são registrados em blocos.

O histórico é público e verificável por qualquer nó da rede.

- Proof of Work (Mineração):

Os servidores agora atuam como Mineradores.

Eles competem para resolver um desafio criptográfico (PoW) com dificuldade ajustável (targetBits), garantindo a segurança da rede contra spam e fraudes.

- Criptografia & Segurança (ECDSA):

Sua Chave, Seus Tanques: Cada jogador possui um par de chaves (Pública/Privada).

Assinatura Digital: O servidor não aceita "ordens". Ele valida transações assinadas. Você assina o pedido de compra ou troca no cliente, e o servidor apenas valida e transmite para a Mempool.

- Consenso Distribuído:

Além da eleição de líder para orquestração (via Redis), os nós propagam blocos minerados via P2P. Se um bloco é válido, ele é anexado à cadeia local de cada servid

### 🎮 Mecânicas de Jogo

- **Vida dos Tanques**: Cada tanque possui vida e ataque únicos
- **Sistema de Batalha**: Turnos simultâneos onde ambos jogadores escolhem cartas
- **Pareamento**: Conecte-se com outro jogador antes de batalhar
- **Troca de Cartas**: Negocie tanques com jogadores pareados
- **Compra de Boosters**: Adquira pacotes com 3 cartas aleatórias

### 🚜 Categorias de Tanques

- **Light**: Tanques leves e ágeis (M22, BMP, Fox, AMX13)
- **Medium**: Tanques médios balanceados (Sherman, T-34, Panther, M47)
- **Heavy**: Tanques pesados devastadores (Tiger II, IS-6, KV-2, Maus)

## 📋 Pré-requisitos

- **Docker**: 20.10 ou superior
- **Docker Compose**: 2.0 ou superior
- **Portas Livres**: 6379-6381 (Redis), 9090-9092 (API), 8081-8083 (UDP)
```

### 🔧 Tecnologias

- **Backend**: Go 1.21
- **Banco de Dados em memória**: Redis Cluster (3 nós)
- **Comunicação**: REST API + Pub/Sub Redis + UDP
- **Containerização**: Docker multi-stage builds
- **Eleição de Líder**: Algoritmo baseado em health checks e menor ID alfabético

## 🚀 Como Executar

### 📦 Preparação Inicial
Antes de iniciar os servidores, compile as imagens Docker:
### 1) Passo: Abrir em um terminal_1 e digitar
```
bash
docker compose build --no-cache
```
### 2) Passo: Mesmo terminal_1 e digitar
```
bash
docker compose up -d redis-node-1 redis-node-2 redis-node-3 redis-cluster-init
```
### 3) Passo: Abrir outro terminal_2 e digitar
```
bash
docker compose run --service-ports --name server1 server1
```
### 4) Passo: Abrir outro terminal_3 e digitar
```
bash
docker compose run --service-ports --no-deps --name server2 server2
```
### 5) Passo: Abrir outro terminal_4 e digitar
```
bash
docker compose run --service-ports --no-deps --name server3 server3
```
### 6) Passo: Ir em cada terminal dos servers (terminal_2, _3, _4) e dar enter para fazer eleição
### 7) Passo: Abrir outro terminal_5 (um para rodar cada cliente diferente) e digitar:
```
bash
docker compose run --rm client
```

## 🎯 Como Jogar

### 📝 Comandos Disponíveis

#### Estado Livre (após conectar)
- `Parear <id_jogador>` - Parear com outro jogador
- `Abrir` - Comprar pacote de cartas (3 cartas aleatórias)
- `Ping` - Medir latência UDP com o servidor
- `Ver Blockchain`- Apresenta os blocos atuais da Blockchain
- `Sair` - Desconectar

#### Estado Pareado
- `Batalhar` - Iniciar batalha (requer 5+ cartas no inventário)
- `Trocar` - Propor troca de cartas
- `Abrir` - Comprar mais cartas
- `Ping` - Testar conexão

#### Durante Troca
- `list` - Ver suas cartas
- `ofertar <número>` - Ofertar carta específica (1 a N)
- `cancelar` - Cancelar troca

#### Durante Batalha
- O servidor escolhe automaticamente 5 cartas aleatórias do seu deck
- Aguarde o servidor solicitar sua jogada
- O resultado é calculado automaticamente

## 🌐 Portas Utilizadas

### Redis Cluster
- `6379` - redis-node-1
- `6380` - redis-node-2
- `6381` - redis-node-3

### Servidores de Jogo
- `9090` - Server1 API REST
- `9091` - Server2 API REST
- `9092` - Server3 API REST
- `8081/UDP` - Server1 Ping
- `8082/UDP` - Server2 Ping
- `8083/UDP` - Server3 Ping

## 🏆 Sistema de Eleição de Líder

O sistema utiliza eleição automática baseada em:
- **Health Checks**: Verificação periódica (a cada 5s)
- **Critério de Eleição**: Menor ID alfabético entre servidores vivos
- **Failover Automático**: Se o líder cai, nova eleição é iniciada
- **Reconexão de Clientes**: Clientes detectam queda e reconectam automaticamente

### Estados do Servidor
```
✓ server1 está ONLINE
✓ server2 está ONLINE  
✓ server3 está ONLINE
🎖️  NOVO LÍDER ELEITO: server1
```

## 🔍 Monitoramento

### Verificar Status do Cluster Redis
```bash
docker exec redis-node-1 redis-cli -h SEU_IP -p 6379 cluster info
```

### Verificar Containers Ativos
```bash
docker compose ps
```

## 🐛 Troubleshooting

### Problema: "Port already allocated"
**Solução:**
```bash
docker compose down --remove-orphans
docker volume prune -f
```

### Problema: "CLUSTERDOWN Hash slot not served"
**Causa**: Cluster Redis não está pronto

**Solução:**
```bash
# Aguarde mais tempo após iniciar o redis-cluster-init
# Ou verifique o status:
docker exec redis-node-1 redis-cli cluster info
```

### Problema: Cliente não recebe respostas
**Soluções:**
1. Verifique se pressionou ENTER nos servidores
2. Confirme que um líder foi eleito (veja os logs)
3. Teste conectividade com o Redis

## 🧹 Limpeza

```bash
# Parar todos os containers
docker compose down --remove-orphans

# Limpar volumes do Redis
docker volume prune -f

# Limpar imagens não utilizadas
docker image prune -a
```

## 📚 Comandos Úteis

```bash
# Ver todos os containers (incluindo parados)
docker ps -a

# Reiniciar apenas servidores
docker compose restart server1 server2 server3

# Ver uso de recursos
docker stats

# Acessar logs em tempo real
docker compose logs -f
```

---

*Assuma o comando e domine o campo de batalha em PlanoZ! 🎖️🚜*
