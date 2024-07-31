package ticket_service

type Message struct {
	Code           int         `json:"code"`
	Message        string      `json:"message"`
	ProcessingTime int64       `json:"processingTime"`
	Data           interface{} `json:"data"`
}

type FetchClubTicketPriceInfoResp struct {
	TicketId       string `json:"ticketId"`
	Tick           string `json:"tick"`
	TokenName      string `json:"tokenName"`
	Decimals       string `json:"decimals"`
	TicketPriceStr string `json:"ticketPriceStr"`
}

type FetchClubTicketPriceInfoRequest struct {
	Tick     string `json:"tick"`
	TicketId string `json:"ticketId"`
}
