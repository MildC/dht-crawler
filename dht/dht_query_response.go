package dht

type DHTErrorResponse struct {
	TransactionID string
	ErrorCode     int
	ErrorMessage  string
}

func (r *DHTErrorResponse) ToPayload() map[string]interface{} {
	return map[string]interface{}{
		"t": r.TransactionID,
		"y": "e",
		"e": []interface{}{r.ErrorCode, r.ErrorMessage},
	}
}

func NewDHTErrorResponse(transID string, errCode int, errMsg string) *DHTErrorResponse {
	return &DHTErrorResponse{
		TransactionID: transID,
		ErrorCode:     errCode,
		ErrorMessage:  errMsg,
	}
}

type DHTQueryResponse struct {
	TransactionID string
	Arguments     map[string]interface{}
}

func (r *DHTQueryResponse) ToPayload() map[string]interface{} {
	return map[string]interface{}{
		"t": r.TransactionID,
		"y": "r",
		"e": r.Arguments,
	}
}

func NewDHTQueryResponse(transID string, args map[string]interface{}) *DHTQueryResponse {
	return &DHTQueryResponse{
		TransactionID: transID,
		Arguments:     args,
	}
}
