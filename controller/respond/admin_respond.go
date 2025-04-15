package respond

import "metaid-market-service/models"

type FetchAutoCreateBridgeInfoRequest struct {
	TickId        string            `json:"tickId"`
	Tick          string            `json:"tick"`
	TokenName     string            `json:"tokenName"`
	Decimals      int64             `json:"decimals"`
	OrderCount    int64             `json:"orderCount"`
	MarketCapSat  int64             `json:"marketCapSat"`
	MarketCapUsdt int64             `json:"marketCapUsdt"`
	AutoStatus    models.AutoStatus `json:"autoStatus"` //1-Create 2-Finish
}
