package transactions

import (
	"errors"
	"pbl-2-redes/internal/models"
)

type Transactions struct {
	queue []models.Transaction
}

func New() *Transactions {
	return &Transactions{queue: make([]models.Transaction, 0)}
}

func (q Transactions) GetAll() []models.Transaction {
	return q.queue
}

func (q Transactions) GetFirstTransaction() models.Transaction {
	return q.queue[0]
}

func (q *Transactions) Enqueue(t models.Transaction) {
	q.queue = append(q.queue, t)
}

func (q *Transactions) Dequeue() error {
	if len(q.queue) >= 1 {
		q.queue = q.queue[1:]
		return nil
	}
	return errors.New("queue is empty")
}

func (q *Transactions) Length() int {
	return len(q.queue)
}
