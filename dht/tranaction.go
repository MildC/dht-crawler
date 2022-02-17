package dht

// transaction implements transaction.
type transaction struct {
	*query
	id       string
	response chan struct{}
}
