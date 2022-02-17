package dht

import (
	"net"
	"sync"
	"time"
)

// transactionManager represents the manager of transactions.
type transactionManager struct {
	*sync.RWMutex
	transactions *TransactionMap
	index        *TransactionMap
	cursor       uint64
	maxCursor    uint64
	queryChan    chan *Query
	dht          *DHT
}

// newTransactionManager returns new transactionManager pointer.
func newTransactionManager(maxCursor uint64, dht *DHT) *transactionManager {
	return &transactionManager{
		RWMutex:      &sync.RWMutex{},
		transactions: NewTransactionMap(),
		index:        NewTransactionMap(),
		maxCursor:    maxCursor,
		queryChan:    make(chan *Query, 1024),
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

// genIndexKey generates an indexed key which consists of queryType and
// address.
func (tm *transactionManager) genIndexKey(queryType DHTQueryType, address string) string {
	return queryType.String() + ":" + address
}

// genIndexKeyByTrans generates an indexed key by a transaction.
func (tm *transactionManager) genIndexKeyByTrans(trans *Transaction) string {
	return tm.genIndexKey(trans.Data.QueryType, trans.Node.Address().String())
}

// insert adds a transaction to transactionManager.
func (tm *transactionManager) insert(trans *Transaction) {
	tm.Lock()
	defer tm.Unlock()

	tm.transactions.Store(trans.ID, trans)
	tm.index.Store(tm.genIndexKeyByTrans(trans), trans)
}

// delete removes a transaction from transactionManager.
func (tm *transactionManager) delete(transID string) {
	trans, ok := tm.transactions.GetTransaction(transID)
	if !ok {
		return
	}

	tm.Lock()
	defer tm.Unlock()

	tm.transactions.Delete(trans.ID)
	tm.index.Delete(tm.genIndexKeyByTrans(trans))
}

// len returns how many transactions are requesting now.
func (tm *transactionManager) len() int {
	return tm.transactions.Len()
}

// transaction returns a transaction. keyType should be one of 0, 1 which
// represents transId and index each.
func (tm *transactionManager) transaction(
	key string, keyType int) *Transaction {

	sm := tm.transactions
	if keyType == 1 {
		sm = tm.index
	}

	trans, _ := sm.GetTransaction(key)
	return trans
}

// getByTransID returns a transaction by transID.
func (tm *transactionManager) getByTransID(transID string) *Transaction {
	return tm.transaction(transID, 0)
}

// getByIndex returns a transaction by indexed key.
func (tm *transactionManager) getByIndex(index string) *Transaction {
	return tm.transaction(index, 1)
}

// transaction gets the proper transaction with whose id is transId and
// address is addr.
func (tm *transactionManager) filterOne(
	transID string, addr *net.UDPAddr) *Transaction {

	trans := tm.getByTransID(transID)
	if trans == nil || trans.Node.Address().String() != addr.String() {
		return nil
	}

	return trans
}

// query sends the query-formed data to udp and wait for the response.
// When timeout, it will retry `try - 1` times, which means it will query
// `try` times totally.
func (tm *transactionManager) query(q *Query, try int) {
	// tm.dht.logger.Sugar().Debugf("query %v:%v", q.Node.Address().IP, q.Node.Address().Port)
	transID := q.Data.TransactionID
	trans := tm.newTransaction(transID, q)

	tm.insert(trans)
	defer tm.delete(trans.ID)

	success := false
	for i := 0; i < try; i++ {
		if err := send(tm.dht, q.Node.Address(), q.Data); err != nil {
			break
		}

		select {
		case <-trans.Response:
			success = true
			break
		case <-time.After(time.Second * 15):
		}
	}

	if !success && q.Node.ID() != nil {
		tm.dht.blackList.insert(q.Node.Address().IP.String(), q.Node.Address().Port)
		tm.dht.routingTable.RemoveByAddr(q.Node.Address().String())
	}
}

// run starts to listen and consume the query chan.
func (tm *transactionManager) run() {
	for q := range tm.queryChan {
		go tm.query(q, tm.dht.Try)
	}
}

// sendQuery send query-formed data to the chan.
func (tm *transactionManager) sendQuery(no Node, queryType DHTQueryType, a map[string]interface{}) {
	// If the target is self, then stop.
	if no.ID() != nil && no.IDRawString() == tm.dht.node.IDRawString() ||
		tm.getByIndex(tm.genIndexKey(queryType, no.Address().String())) != nil ||
		tm.dht.blackList.in(no.Address().IP.String(), no.Address().Port) {
		return
	}

	tm.queryChan <- &Query{
		Node: no,
		Data: NewDHTQuery(tm.genTransID(), queryType, a),
	}
}

// ping sends ping query to the chan.
func (tm *transactionManager) ping(no Node) {
	tm.sendQuery(no, DHTQueryTypePing, map[string]interface{}{
		"id": tm.dht.id(no.IDRawString()),
	})
}

// findNode sends find_node query to the chan.
func (tm *transactionManager) findNode(no Node, target string) {
	tm.sendQuery(no, DHTQueryTypeFindNode, map[string]interface{}{
		"id":     tm.dht.id(target),
		"target": target,
	})
}

// getPeers sends get_peers query to the chan.
func (tm *transactionManager) getPeers(no Node, infoHash string) {
	tm.sendQuery(no, DHTQueryTypeGetPeers, map[string]interface{}{
		"id":        tm.dht.id(infoHash),
		"info_hash": infoHash,
	})
}

// announcePeer sends announce_peer query to the chan.
func (tm *transactionManager) AnnouncePeer(no Node, infoHash string, impliedPort, port int, token string) {
	tm.sendQuery(no, DHTQueryTypeAnnouncePeer, map[string]interface{}{
		"id":           tm.dht.id(no.IDRawString()),
		"info_hash":    infoHash,
		"implied_port": impliedPort,
		"port":         port,
		"token":        token,
	})
}
