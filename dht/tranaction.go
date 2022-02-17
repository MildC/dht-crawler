package dht

// Transaction implements transaction.
type Transaction struct {
	*query
	id       string
	response chan struct{}
}
