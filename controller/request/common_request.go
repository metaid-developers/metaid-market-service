package request

type FetchMrc20TickListReq struct {
	Cursor    int64  `json:"cursor"`
	Size      int64  `json:"size"`
	Completed bool   `json:"completed"`
	OrderBy   string `json:"order"`
	SortType  int    `json:"sortType"`
}

type FetchMrc20TickInfoReq struct {
	TickId string `json:"tickId"`
	Tick   string `json:"tick"`
}

type Mrc20AddressShovelsReq struct {
	Address string `json:"address"`
	TickId  string `json:"tickId"`
	Cursor  int64  `json:"cursor"`
	Size    int64  `json:"size"`
}

type Mrc20AddressUtxosReq struct {
	Address string `json:"address"`
	TickId  string `json:"tickId"`
	Cursor  int64  `json:"cursor"`
	Size    int64  `json:"size"`
}

type Mrc20AddressBalancesReq struct {
	Address string `json:"address"`
	Cursor  int64  `json:"cursor"`
	Size    int64  `json:"size"`
}

type BroadcastTxReq struct {
	TxHex string `json:"txHex"`
}

type FetchMrc20TickMarketPriceResp struct {
	TickId string `json:"tickId"`
}
