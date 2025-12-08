package blockchain

import (
	"errors"
	"log/slog"
	"pbl-2-redes/internal/models"
	"sync"
	"time"
)

type Blockchain struct {
	Height         int
	Ledger         []*Block
	MPool          []models.Transaction
	IncomingBlocks chan BlockTask // possivelmente desnecessário
	StateChan      *chan int
	MX             sync.Mutex // mutex local para quando for mexer na pool
	blockOK        int        // 0 idle, 1 ok, 2 inválido
}

func New() *Blockchain {
	channel := make(chan int)
	return &Blockchain{Height: 1, Ledger: []*Block{Genesis()}, MPool: []models.Transaction{}, IncomingBlocks: make(chan BlockTask), StateChan: &channel, MX: sync.Mutex{}}
}

func (b *Blockchain) AddTransaction(transaction models.Transaction) error {
	if !VerifySignature(transaction.PublicKey, transaction.UserData, transaction.Signature) {
		slog.Error("invalid signature")
		return errors.New("invalid signature")
	}

	if !b.AntiReplay(transaction.PublicKey, transaction.Signature) {
		slog.Error("duplicated signature")
		return errors.New("duplicated signature")
	}

	err := b.GreedyCheck(transaction)

	if err != nil {
		slog.Error(err.Error())
		return err
	}

	b.MPool = append(b.MPool, transaction)
	return nil
}

// Pega último hash
func (b *Blockchain) GetLastHash() []byte {
	return b.Ledger[len(b.Ledger)].Hash
}

// Função que adiciona bloco na blockchain
func (b *Blockchain) AddBlock(block *Block) {
	b.RecheckMempool(block.Transactions)
	// verificar antireplay etc ADICIONAD DPS
	b.Ledger = append(b.Ledger, block)
}

func Genesis() *Block {
	baseTransaction := models.Transaction{Type: models.NONE, Data: []string{}}
	channel := make(chan int)
	return NewBlock([]byte{}, []*models.Transaction{&baseTransaction}, &channel)
}

// Função que atualiza ledger a depender do resultado de comparação de height no cluster
func (b *Blockchain) UpdateLedger(l []*Block) error {
	if len(b.Ledger) < len(l) {
		b.Ledger = l
		return nil
	}
	return errors.New("ledger já atualizado")
}

// Função que verifica ilegalidade de transação (uso de recurso já utilizado / transação duplicada sobre mesmo recurso)
func (b *Blockchain) GreedyCheck(t models.Transaction) error {
	// verifica situações de ilegalidade no ledger
	for _, block := range b.Ledger {
		for _, transaction := range block.Transactions {
			// na compra, se booster já foi comprado, desiste
			if t.Type == models.PC && t.Data[1] == transaction.Data[1] {
				return errors.New("illegal action, booster already purchased")
			}
			// na troca, se usuário 1 já trocou aquela carta, não troca
			if t.Type == models.TD && t.Data[0] == transaction.Data[0] && t.Data[1] == transaction.Data[1] {
				return errors.New("illegal action, card already traded by user")
			}
			// na troca, se usuário 2 já trocou aquela carta, não troca
			if t.Type == models.TD && t.Data[1] == transaction.Data[1] && t.Data[3] == transaction.Data[3] {
				return errors.New("illegal action, card already traded by user")
			}
			// na troca, se batalha já tiver registrada, desiste
			if t.Type == models.BR && t.Data[0] == transaction.Data[0] {
				return errors.New("illegal action, battle already exists")
			}
		}
	}

	// verifica situações de ilegalidade na mempool
	for _, transaction := range b.MPool {
		// na compra, se booster já tem intenção de compra
		if t.Type == models.PC && t.Data[1] == transaction.Data[1] {
			return errors.New("illegal action, booster cannot be purchased")
		}
		// na troca, se usuário 1 já está tentando trocar aquela carta, não troca
		if t.Type == models.TD && t.Data[0] == transaction.Data[0] && t.Data[1] == transaction.Data[1] {
			return errors.New("illegal action, card already traded by user")
		}
		// na troca, se usuário 2 já está tentando trocar aquela carta, não troca
		if t.Type == models.TD && t.Data[1] == transaction.Data[1] && t.Data[3] == transaction.Data[3] {
			return errors.New("illegal action, card already traded by user")
		}
		// na troca, se batalha já tiver registrada, desiste
		if t.Type == models.BR && t.Data[0] == transaction.Data[0] {
			return errors.New("illegal action, battle already exists")
		}
	}

	return nil
}

// Verifica situações de ilegalidade ao adicionar bloco
func (b *Blockchain) IllegalCheck(t models.Transaction) bool {
	// verifica situações de ilegalidade no ledger
	for _, block := range b.Ledger {
		for _, transaction := range block.Transactions {
			// na compra, se booster já foi comprado, desiste
			if t.Type == models.PC && t.Data[1] == transaction.Data[1] {
				return false
			}
			// na troca, se usuário 1 já trocou aquela carta, não troca
			if t.Type == models.TD && t.Data[0] == transaction.Data[0] && t.Data[1] == transaction.Data[1] {
				return false
			}
			// na troca, se usuário 2 já trocou aquela carta, não troca
			if t.Type == models.TD && t.Data[1] == transaction.Data[1] && t.Data[3] == transaction.Data[3] {
				return false
			}
			// na troca, se batalha já tiver registrada, desiste
			if t.Type == models.BR && t.Data[0] == transaction.Data[0] {
				return false
			}
		}
	}
	return true
}

// Minera blocos
// Uma solução mais simples seria fazer as funções de forma atômica e conjunta (no laço for, se precisar checar bloco, guarda nonce
// e checa novo nó. se não for adicionado, continua. se for, cancela mineração.)
func (b *Blockchain) MineBlock(length int) {
	// cria slice temp de transações para serem adicionadas no bloco
	t := []*models.Transaction{}

	// guarda as transações selecionadas
	for i := range length {
		t = append(t, &b.MPool[i])
		// remove da pool
		//
	}

	// pega o último hash
	previousHash := b.GetLastHash()

	newBlock := NewBlock(previousHash, t, b.StateChan)

	// se deu tudo certo
	if newBlock != nil {
		b.AddBlock(newBlock)
	} else {
		slog.Error("failed while mining a new block.")
	}
}

// Verifica se precisa minerar novo bloco
func (b *Blockchain) RunBlockchain() {
	// verifica se tem x quantidade de transações ou se passou timeout
	// IMPORTANTE: essa lógica tem que ser feita junto a logica de verificar a chegada de novos blocos
	ticker := time.NewTicker(2 * time.Second)

	// variável de contexto que controla a mineração
	// ela serve para comunicar os sinais de cancelamento ou pausa quando for necessário
	//var ctx context.Context
	//var cancelF func()

	// estado que guarda o momento atual da blockchain (idle, mining, validating, cancel)
	state := idle
	b.blockOK = 0

	for {
		select {
		// CASO TIMEOUT, MINERA
		case <-ticker.C:
			b.MX.Lock()
			// guarda tamanho da pool naquele instante
			poolLength := len(b.MPool)
			b.MX.Unlock()

			// se a pool tiver pelo menos 1 transação
			// e tiver menos que 5 e nn tiver minerando
			// tenta criar bloco
			if poolLength >= 1 && poolLength < 5 && state == idle {
				//ctx, cancelF = context.WithCancel(context.Background())
				b.MX.Lock()
				state = mining
				b.MX.Unlock()

				go b.MineBlock(poolLength)
			}

		// caso tenha chegado bloco
		// AJEITAR
		case incomingTask := <-b.IncomingBlocks:
			// se tiver minerado para
			state = validating
			err := b.CheckNewBlock(incomingTask.Block)

			if err != nil {
				incomingTask.OnFinish(err)
			} else {
				*b.StateChan <- cancel
				b.AddBlock(incomingTask.Block)
				incomingTask.OnFinish(nil)
			}

		}
		// se chegou numa quantidade de transações (ou seja, não passou do timeout)
		// caso tenha mais q 5 transações
		b.MX.Lock()
		if len(b.MPool) >= 5 && state == idle {
			//ctx, cancelF = context.WithCancel(context.Background())
			b.MX.Lock()
			state = mining
			b.MX.Unlock()

			go b.MineBlock(5)
		}
		b.MX.Unlock()
	}
}

// Verifica novos blocos
func (b *Blockchain) CheckNewBlock(block *Block) error {
	pow := NewProofOfWork(block)
	// faz validação por assinatura
	if !pow.Validate() {
		return errors.New("invalid signature")
	}

	// passa por todas as transações e vê se tem alguma ilegal
	for _, t := range block.Transactions {
		if !b.IllegalCheck(*t) {
			return errors.New("invalid transactions")
		}
	}
	return nil
}

// Checa mempool pelas transações do bloco
func (b *Blockchain) RecheckMempool(transactions []*models.Transaction) {
	for _, t := range transactions {
		for index, p := range b.MPool {
			if p.Equals(*(t)) {
				// remove se as transações forem iguais e já existirem lá na mempool
				b.MPool = append(b.MPool[:index], b.MPool[index+1:]...)
			}
		}
	}
}
