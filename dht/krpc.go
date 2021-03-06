package dht

import (
	"errors"
	"net"
	"strings"
	"time"
)

const (
	generalError = 201 + iota
	serverError
	protocolError
	unknownError
)

// packet represents the information receive from udp.
type packet struct {
	data  []byte
	raddr *net.UDPAddr
}

// token represents the token when response getPeers request.
type token struct {
	data       string
	createTime time.Time
}

// tokenManager managers the tokens.
type tokenManager struct {
	*syncedMap
	expiredAfter time.Duration
	dht          *DHT
}

// newTokenManager returns a new tokenManager.
func newTokenManager(expiredAfter time.Duration, dht *DHT) *tokenManager {
	return &tokenManager{
		syncedMap:    newSyncedMap(),
		expiredAfter: expiredAfter,
		dht:          dht,
	}
}

// token returns a token. If it doesn't exist or is expired, it will add a
// new token.
func (tm *tokenManager) token(addr *net.UDPAddr) string {
	v, ok := tm.Get(addr.IP.String())
	tk, _ := v.(token)

	if !ok || time.Since(tk.createTime) > tm.expiredAfter {
		tk = token{
			data:       randomString(5),
			createTime: time.Now(),
		}

		tm.Set(addr.IP.String(), tk)
	}

	return tk.data
}

// clear removes expired tokens.
func (tm *tokenManager) clear() {
	for range time.Tick(time.Minute * 3) {
		keys := make([]interface{}, 0, 100)

		for item := range tm.Iter() {
			if time.Since(item.val.(token).createTime) > tm.expiredAfter {
				keys = append(keys, item.key)
			}
		}

		tm.DeleteMulti(keys)
	}
}

// check returns whether the token is valid.
func (tm *tokenManager) check(addr *net.UDPAddr, tokenString string) bool {
	key := addr.IP.String()
	v, ok := tm.Get(key)
	tk, _ := v.(token)

	if ok {
		tm.Delete(key)
	}

	return ok && tokenString == tk.data
}

// send sends data to the udp.
func send(dht *DHT, addr *net.UDPAddr, q DHTPayload) error {
	dht.conn.SetWriteDeadline(time.Now().Add(time.Second * 15))

	_, err := dht.conn.WriteToUDP([]byte(Encode(q.ToPayload())), addr)
	if err != nil {
		dht.blackList.insert(addr.IP.String(), -1)
	}
	return err
}

// ParseKey parses the key in dict data. `t` is type of the keyed value.
// It's one of "int", "string", "map", "list".
func ParseKey(data map[string]interface{}, key string, t string) error {
	val, ok := data[key]
	if !ok {
		return errors.New("lack of key")
	}

	switch t {
	case "string":
		_, ok = val.(string)
	case "int":
		_, ok = val.(int)
	case "map":
		_, ok = val.(map[string]interface{})
	case "list":
		_, ok = val.([]interface{})
	default:
		panic("invalid type")
	}

	if !ok {
		return errors.New("invalid key type")
	}

	return nil
}

// ParseKeys parses keys. It just wraps ParseKey.
func ParseKeys(data map[string]interface{}, pairs [][]string) error {
	for _, args := range pairs {
		key, t := args[0], args[1]
		if err := ParseKey(data, key, t); err != nil {
			return err
		}
	}
	return nil
}

// parseMessage parses the basic data received from udp.
// It returns a map value.
func parseMessage(data interface{}) (map[string]interface{}, error) {
	response, ok := data.(map[string]interface{})
	if !ok {
		return nil, errors.New("response is not dict")
	}

	if err := ParseKeys(
		response, [][]string{{"t", "string"}, {"y", "string"}}); err != nil {
		return nil, err
	}

	return response, nil
}

// handleRequest handles the requests received from udp.
func handleRequest(dht *DHT, addr *net.UDPAddr, payload map[string]interface{}) (success bool) {
	q := NewDHTQueryFromPayload(payload)

	if err := ParseKeys(payload, [][]string{{"q", "string"}, {"a", "map"}}); err != nil {
		send(dht, addr, NewDHTErrorResponse(q.TransactionID, protocolError, err.Error()))
		return
	}

	if err := ParseKey(q.Arguments, "id", "string"); err != nil {
		send(dht, addr, NewDHTErrorResponse(q.TransactionID, protocolError, err.Error()))
		return
	}

	id := q.Arguments["id"].(string)
	if id == dht.node.IDRawString() {
		return
	}

	if len(id) != 20 {
		send(dht, addr, NewDHTErrorResponse(q.TransactionID, protocolError, "invalid id"))
		return
	}

	if no, ok := dht.routingTable.GetNodeByAddress(addr.String()); ok &&
		no.IDRawString() != id {

		dht.blackList.insert(addr.IP.String(), addr.Port)
		dht.routingTable.RemoveByAddr(addr.String())

		send(dht, addr, NewDHTErrorResponse(q.TransactionID, protocolError, "invalid id"))
		return
	}

	switch q.QueryType {
	case DHTQueryTypePing:
		send(dht, addr, NewDHTQueryResponse(q.TransactionID, map[string]interface{}{
			"id": dht.id(id),
		}))
	case DHTQueryTypeFindNode:
		if dht.IsStandardMode() {
			if err := ParseKey(q.Arguments, "target", "string"); err != nil {
				send(dht, addr, NewDHTErrorResponse(q.TransactionID, protocolError, err.Error()))
				return
			}

			target := q.Arguments["target"].(string)
			if len(target) != 20 {
				send(dht, addr, NewDHTErrorResponse(q.TransactionID, protocolError, "invalid target"))
				return
			}

			var nodes string
			targetID := newBitmapFromString(target)

			no, _ := dht.routingTable.GetNodeKBucktByID(targetID)
			if no != nil {
				nodes = no.CompactNodeInfo()
			} else {
				nodes = strings.Join(
					dht.routingTable.GetNeighborCompactInfos(targetID, dht.K),
					"",
				)
			}

			send(dht, addr, NewDHTQueryResponse(q.TransactionID, map[string]interface{}{
				"id":    dht.id(target),
				"nodes": nodes,
			}))
		}
	case DHTQueryTypeGetPeers:
		if err := ParseKey(q.Arguments, "info_hash", "string"); err != nil {
			send(dht, addr, NewDHTErrorResponse(q.TransactionID, protocolError, err.Error()))
			return
		}

		infoHash := q.Arguments["info_hash"].(string)

		if len(infoHash) != 20 {
			send(dht, addr, NewDHTErrorResponse(q.TransactionID, protocolError, "invalid info_hash"))
			return
		}

		if dht.IsCrawlMode() {
			send(dht, addr, NewDHTQueryResponse(q.TransactionID, map[string]interface{}{
				"id":    dht.id(infoHash),
				"token": dht.tokenManager.token(addr),
				"nodes": "",
			}))
		} else if peers := dht.peersManager.GetPeers(
			infoHash, dht.K); len(peers) > 0 {

			values := make([]interface{}, len(peers))
			for i, p := range peers {
				values[i] = p.CompactIPPortInfo()
			}

			send(dht, addr, NewDHTQueryResponse(q.TransactionID, map[string]interface{}{
				"id":     dht.id(infoHash),
				"values": values,
				"token":  dht.tokenManager.token(addr),
			}))
		} else {
			send(dht, addr, NewDHTQueryResponse(q.TransactionID, map[string]interface{}{
				"id":    dht.id(infoHash),
				"token": dht.tokenManager.token(addr),
				"nodes": strings.Join(dht.routingTable.GetNeighborCompactInfos(
					newBitmapFromString(infoHash), dht.K), ""),
			}))
		}

		if dht.OnGetPeers != nil {
			dht.OnGetPeers(infoHash, addr.IP.String(), addr.Port)
		}
	case DHTQueryTypeAnnouncePeer:
		if err := ParseKeys(q.Arguments, [][]string{
			{"info_hash", "string"},
			{"port", "int"},
			{"token", "string"},
		}); err != nil {

			send(dht, addr, NewDHTErrorResponse(q.TransactionID, protocolError, err.Error()))
			return
		}

		infoHash := q.Arguments["info_hash"].(string)
		port := q.Arguments["port"].(int)
		token := q.Arguments["token"].(string)

		if !dht.tokenManager.check(addr, token) {
			//			send(dht, addr, makeError(t, protocolError, "invalid token"))
			return
		}

		if impliedPort, ok := q.Arguments["implied_port"]; ok &&
			impliedPort.(int) != 0 {

			port = addr.Port
		}

		if dht.IsStandardMode() {
			dht.peersManager.Insert(infoHash, NewPeer(addr.IP, port, token))

			send(dht, addr, NewDHTQueryResponse(q.TransactionID, map[string]interface{}{
				"id": dht.id(id),
			}))
		}

		if dht.OnAnnouncePeer != nil {
			dht.OnAnnouncePeer(infoHash, addr.IP.String(), port)
		}
	default:
		//		send(dht, addr, makeError(t, protocolError, "invalid q"))
		return
	}

	no := NewNode(id, addr)
	dht.routingTable.Insert(no)
	return true
}

// findOn puts nodes in the response to the routingTable, then if target is in
// the nodes or all nodes are in the routingTable, it stops. Otherwise it
// continues to findNode or getPeers.
func findOn(dht *DHT, r map[string]interface{}, target *bitmap, queryType DHTQueryType) error {
	if err := ParseKey(r, "nodes", "string"); err != nil {
		return err
	}

	nodes := r["nodes"].(string)
	if len(nodes)%26 != 0 {
		return errors.New("the length of nodes should can be divided by 26")
	}

	hasNew, found := false, false
	for i := 0; i < len(nodes)/26; i++ {
		no, _ := NewNodeFromCompactInfo(
			string(nodes[i*26:(i+1)*26]), dht.Network)

		if no.IDRawString() == target.RawString() {
			found = true
		}

		if dht.routingTable.Insert(no) {
			hasNew = true
		}
	}

	if found || !hasNew {
		return nil
	}

	targetID := target.RawString()
	for _, no := range dht.routingTable.GetNeighbors(target, dht.K) {
		switch queryType {
		case DHTQueryTypeFindNode:
			dht.transactionManager.findNode(no, targetID)
		case DHTQueryTypeGetPeers:
			dht.transactionManager.getPeers(no, targetID)
		default:
			panic("invalid find type")
		}
	}
	return nil
}

// handleResponse handles responses received from udp.
func handleResponse(dht *DHT, addr *net.UDPAddr, response map[string]interface{}) (success bool) {
	t := response["t"].(string)

	trans := dht.transactionManager.filterOne(t, addr)
	if trans == nil {
		return
	}

	// inform transManager to delete the transaction.
	if err := ParseKey(response, "r", "map"); err != nil {
		return
	}

	r := response["r"].(map[string]interface{})

	if err := ParseKey(r, "id", "string"); err != nil {
		return
	}

	id := r["id"].(string)

	// If response's node id is not the same with the node id in the
	// transaction, raise error.
	if trans.Node.ID() != nil && trans.Node.IDRawString() != r["id"].(string) {
		dht.blackList.insert(addr.IP.String(), addr.Port)
		dht.routingTable.RemoveByAddr(addr.String())
		return
	}

	node := NewNode(id, addr)

	switch trans.Data.QueryType {
	case DHTQueryTypePing:
	case DHTQueryTypeFindNode:
		if trans.Data.QueryType != DHTQueryTypeFindNode {
			return
		}

		target := trans.Data.Arguments["target"].(string)
		if findOn(dht, r, newBitmapFromString(target), DHTQueryTypeFindNode) != nil {
			return
		}
	case DHTQueryTypeGetPeers:
		if err := ParseKey(r, "token", "string"); err != nil {
			return
		}

		token := r["token"].(string)
		infoHash := trans.Data.Arguments["info_hash"].(string)

		if err := ParseKey(r, "values", "list"); err == nil {
			values := r["values"].([]interface{})
			for _, v := range values {
				p, err := NewPeerFromCompactIPPortInfo(v.(string), token)
				if err != nil {
					continue
				}
				dht.peersManager.Insert(infoHash, p)
				if dht.OnGetPeersResponse != nil {
					dht.OnGetPeersResponse(infoHash, p)
				}
			}
		} else if findOn(dht, r, newBitmapFromString(infoHash), DHTQueryTypeGetPeers) != nil {
			return
		}
	case DHTQueryTypeAnnouncePeer:
	default:
		return
	}

	// inform transManager to delete transaction.
	trans.Response <- struct{}{}

	dht.blackList.delete(addr.IP.String(), addr.Port)
	dht.routingTable.Insert(node)

	return true
}

// handleError handles errors received from udp.
func handleError(dht *DHT, addr *net.UDPAddr,
	response map[string]interface{}) (success bool) {

	if err := ParseKey(response, "e", "list"); err != nil {
		return
	}

	if e := response["e"].([]interface{}); len(e) != 2 {
		return
	}

	if trans := dht.transactionManager.filterOne(
		response["t"].(string), addr); trans != nil {

		trans.Response <- struct{}{}
	}

	return true
}

var handlers = map[string]func(*DHT, *net.UDPAddr, map[string]interface{}) bool{
	"q": handleRequest,
	"r": handleResponse,
	"e": handleError,
}

// handle handles packets received from udp.
func handle(dht *DHT, pkt packet) {
	if len(dht.workerTokens) == dht.PacketWorkerLimit {
		return
	}

	dht.workerTokens <- struct{}{}

	go func() {
		defer func() {
			<-dht.workerTokens
		}()

		if dht.blackList.in(pkt.raddr.IP.String(), pkt.raddr.Port) {
			return
		}

		data, err := Decode(pkt.data)
		if err != nil {
			return
		}

		response, err := parseMessage(data)
		if err != nil {
			return
		}

		if f, ok := handlers[response["y"].(string)]; ok {
			f(dht, pkt.raddr, response)
		}
	}()
}
