package request

type FetchMrc20TickInfoReq struct {
	TickId string `json:"tickId"`
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
