package mempool_space_service

type TxDetail struct {
	TxId          string        `json:"txid"`
	Height        string        `json:"height"`
	OutputDetails []*OutputItem `json:"outputDetails"`
}

type OutputItem struct {
	OutputHash string `json:"outputHash"`
	Tag        string `json:"tag"`
	Amount     string `json:"amount"`
}

type FeeRecommended struct {
	FastestFee  int64 `json:"fastestFee"`
	HalfHourFee int64 `json:"halfHourFee"`
	HourFee     int64 `json:"hourFee"`
	EconomyFee  int64 `json:"economyFee"`
	MinimumFee  int64 `json:"minimumFee"`
}
