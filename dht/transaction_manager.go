package dht

import (
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

// transactionManager represents the manager of transactions.
type transactionManager struct {
	*sync.RWMutex
	transactions *syncedMap
	index        *syncedMap
	cursor       uint64
	maxCursor    uint64
	queryChan    chan *query
	dht          *DHT
}

// newTransactionManager returns new transactionManager pointer.
func newTransactionManager(maxCursor uint64, dht *DHT) *transactionManager {
	return &transactionManager{
		RWMutex:      &sync.RWMutex{},
		transactions: newSyncedMap(),
		index:        newSyncedMap(),
		maxCursor:    maxCursor,
		queryChan:    make(chan *query, 1024),
		dht:          dht,
	}
}

// genTransID generates a transaction id and returns it.
func (tm *transactionManager) genTransID() string {
	tm.Lock()
	defer tm.Unlock()

	tm.cursor = (tm.cursor + 1) % tm.maxCursor
	return string(int2bytes(tm.cursor))
}

// newTransaction creates a new transaction.
func (tm *transactionManager) newTransaction(id string, q *query) *transaction {
	return &transaction{
		id:       id,
		query:    q,
		response: make(chan struct{}, tm.dht.Try+1),
	}
}

// genIndexKey generates an indexed key which consists of queryType and
// address.
func (tm *transactionManager) genIndexKey(queryType, address string) string {
	return strings.Join([]string{queryType, address}, ":")
}

// genIndexKeyByTrans generates an indexed key by a transaction.
func (tm *transactionManager) genIndexKeyByTrans(trans *transaction) string {
	return tm.genIndexKey(trans.data["q"].(string), trans.node.Address().String())
}

// insert adds a transaction to transactionManager.
func (tm *transactionManager) insert(trans *transaction) {
	tm.Lock()
	defer tm.Unlock()

	tm.transactions.Set(trans.id, trans)
	tm.index.Set(tm.genIndexKeyByTrans(trans), trans)
}

// delete removes a transaction from transactionManager.
func (tm *transactionManager) delete(transID string) {
	v, ok := tm.transactions.Get(transID)
	if !ok {
		return
	}

	tm.Lock()
	defer tm.Unlock()

	trans := v.(*transaction)
	tm.transactions.Delete(trans.id)
	tm.index.Delete(tm.genIndexKeyByTrans(trans))
}

// len returns how many transactions are requesting now.
func (tm *transactionManager) len() int {
	return tm.transactions.Len()
}

// transaction returns a transaction. keyType should be one of 0, 1 which
// represents transId and index each.
func (tm *transactionManager) transaction(
	key string, keyType int) *transaction {

	sm := tm.transactions
	if keyType == 1 {
		sm = tm.index
	}

	v, ok := sm.Get(key)
	if !ok {
		return nil
	}

	return v.(*transaction)
}

// getByTransID returns a transaction by transID.
func (tm *transactionManager) getByTransID(transID string) *transaction {
	return tm.transaction(transID, 0)
}

// getByIndex returns a transaction by indexed key.
func (tm *transactionManager) getByIndex(index string) *transaction {
	return tm.transaction(index, 1)
}

// transaction gets the proper transaction with whose id is transId and
// address is addr.
func (tm *transactionManager) filterOne(
	transID string, addr *net.UDPAddr) *transaction {

	trans := tm.getByTransID(transID)
	if trans == nil || trans.node.Address().String() != addr.String() {
		return nil
	}

	return trans
}

// query sends the query-formed data to udp and wait for the response.
// When timeout, it will retry `try - 1` times, which means it will query
// `try` times totally.
func (tm *transactionManager) query(q *query, try int) {
	log.Printf("query %v %v\n", q.node.ID(), q.data)
	transID := q.data["t"].(string)
	trans := tm.newTransaction(transID, q)

	tm.insert(trans)
	defer tm.delete(trans.id)

	success := false
	for i := 0; i < try; i++ {
		if err := send(tm.dht, q.node.Address(), q.data); err != nil {
			break
		}

		select {
		case <-trans.response:
			success = true
			break
		case <-time.After(time.Second * 15):
		}
	}

	if !success && q.node.ID() != nil {
		tm.dht.blackList.insert(q.node.Address().IP.String(), q.node.Address().Port)
		tm.dht.routingTable.RemoveByAddr(q.node.Address().String())
	}
}

// run starts to listen and consume the query chan.
func (tm *transactionManager) run() {
	for q := range tm.queryChan {
		go tm.query(q, tm.dht.Try)
	}
}

// sendQuery send query-formed data to the chan.
func (tm *transactionManager) sendQuery(
	no Node, queryType string, a map[string]interface{}) {

	// If the target is self, then stop.
	if no.ID() != nil && no.IDRawString() == tm.dht.node.IDRawString() ||
		tm.getByIndex(tm.genIndexKey(queryType, no.Address().String())) != nil ||
		tm.dht.blackList.in(no.Address().IP.String(), no.Address().Port) {
		return
	}

	data := makeQuery(tm.genTransID(), queryType, a)
	tm.queryChan <- &query{
		node: no,
		data: data,
	}
}

// ping sends ping query to the chan.
func (tm *transactionManager) ping(no Node) {
	tm.sendQuery(no, pingType, map[string]interface{}{
		"id": tm.dht.id(no.IDRawString()),
	})
}

// findNode sends find_node query to the chan.
func (tm *transactionManager) findNode(no Node, target string) {
	tm.sendQuery(no, findNodeType, map[string]interface{}{
		"id":     tm.dht.id(target),
		"target": target,
	})
}

// getPeers sends get_peers query to the chan.
func (tm *transactionManager) getPeers(no Node, infoHash string) {
	tm.sendQuery(no, getPeersType, map[string]interface{}{
		"id":        tm.dht.id(infoHash),
		"info_hash": infoHash,
	})
}

// announcePeer sends announce_peer query to the chan.
func (tm *transactionManager) announcePeer(
	no Node, infoHash string, impliedPort, port int, token string) {

	tm.sendQuery(no, announcePeerType, map[string]interface{}{
		"id":           tm.dht.id(no.IDRawString()),
		"info_hash":    infoHash,
		"implied_port": impliedPort,
		"port":         port,
		"token":        token,
	})
}
