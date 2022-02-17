package dht

import (
	"sync"
)

// Query represents the query data included queried node and query-formed data.
type Query struct {
	Node Node
	Data *DHTQuery
}

// Transaction implements transaction.
type Transaction struct {
	*Query
	ID       string
	Response chan struct{}
}

// newTransaction creates a new transaction.
func (tm *transactionManager) newTransaction(id string, q *Query) *Transaction {
	return &Transaction{
		ID:       id,
		Query:    q,
		Response: make(chan struct{}, tm.dht.Try+1),
	}
}

type TransactionMap struct {
	*sync.Map
}

func NewTransactionMap() *TransactionMap {
	return &TransactionMap{Map: &sync.Map{}}
}

func (m TransactionMap) Has(id string) bool {
	_, ok := m.Load(id)
	return ok
}

func (m TransactionMap) GetTransaction(id string) (t *Transaction, ok bool) {
	v, ok := m.Load(id)
	if v != nil {
		transaction := v.(*Transaction)
		return transaction, ok
	}
	return nil, ok
}

func (m TransactionMap) Len() (count int) {
	m.Range(func(_, v interface{}) bool {
		count += 1
		return true
	})
	return
}
