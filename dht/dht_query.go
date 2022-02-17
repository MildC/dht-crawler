package dht

type DHTPayload interface {
	ToPayload() map[string]interface{}
}

type DHTQueryType string

const (
	DHTQueryTypePing         DHTQueryType = "ping"
	DHTQueryTypeFindNode     DHTQueryType = "find_node"
	DHTQueryTypeGetPeers     DHTQueryType = "get_peers"
	DHTQueryTypeAnnouncePeer DHTQueryType = "announce_peer"
)

func (q DHTQueryType) String() string {
	return string(q)
}

type DHTQuery struct {
	TransactionID string
	QueryType     DHTQueryType
	Arguments     map[string]interface{}
}

func (q *DHTQuery) ToPayload() map[string]interface{} {
	return map[string]interface{}{
		"t": q.TransactionID,
		"y": "q",
		"q": string(q.QueryType),
		"a": q.Arguments,
	}
}

func NewDHTQuery(transID string, queryType DHTQueryType, args map[string]interface{}) *DHTQuery {
	return &DHTQuery{
		TransactionID: transID,
		QueryType:     queryType,
		Arguments:     args,
	}
}

func NewDHTQueryFromPayload(payload map[string]interface{}) *DHTQuery {
	args, _ := payload["a"].(map[string]interface{})
	return &DHTQuery{
		TransactionID: payload["t"].(string),
		QueryType:     DHTQueryType(payload["q"].(string)),
		Arguments:     args,
	}
}
