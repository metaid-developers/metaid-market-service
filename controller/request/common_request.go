package request

type FetchMrc20TickListReq struct {
	Cursor     int64  `json:"cursor"`
	Size       int64  `json:"size"`
	Completed  string `json:"completed"`
	OrderBy    string `json:"order"`
	SortType   int    `json:"sortType"`
	SearchTick string `json:"searchTick"`
	Tag        string `json:"tag"` //"id-coins" lists only the fixed id-coin snapshot
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
	Source  string `json:"source"`
}

type Mrc20AddressBalancesReq struct {
	Address string `json:"address"`
	Cursor  int64  `json:"cursor"`
	Size    int64  `json:"size"`
	Source  string `json:"source"`
}

type BroadcastTxReq struct {
	Chain string `json:"chain"`
	TxHex string `json:"txHex"`
}

type FetchMrc20TickMarketPriceResp struct {
	TickId string `json:"tickId"`
}

type FetchMrc20TickMarketPriceListReq struct {
	Cursor int64 `json:"cursor"`
	Size   int64 `json:"size"`
}

type Mrc20IdCoinsAddressUtxosReq struct {
	TickId  string `json:"tickId"`
	Address string `json:"address"`
	Cursor  int64  `json:"cursor"`
	Size    int64  `json:"size"`
	Source  string `json:"source"`
}

type Mrc20TickHoldersRequest struct {
	TickId string `json:"tickId"`
	Tick   string `json:"tick"`
	Cursor int64  `json:"cursor"`
	Size   int64  `json:"size"`
}

type CheckUtxoInfoReq struct {
	OutPoints []string `json:"outPoints"`
}
