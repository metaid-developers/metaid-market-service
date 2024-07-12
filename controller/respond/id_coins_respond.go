package respond

type BuildIdCoinsPreResp struct {
	OrderId        string `json:"orderId"`
	TotalFee       int64  `json:"totalFee"`
	ReceiveAddress string `json:"receiveAddress"`
	MinerFee       int64  `json:"minerFee"`
	ServiceFee     int64  `json:"serviceFee"`
}

type BuildIdCoinsCommitResp struct {
	OrderId    string `json:"orderId"`
	TickId     string `json:"tickId"`
	CommitTxId string `json:"commitTxId"`
	RevealTxId string `json:"revealTxId"`
	PinId      string `json:"pinId"`
	TxId       string `json:"txId"`
}

type IdCoinsMintPreResp struct {
	OrderId           string      `json:"orderId"`
	TotalFee          int64       `json:"totalFee"`
	RevealInscribeFee int64       `json:"revealInscribeFee"`
	RevealMintFee     int64       `json:"revealMintFee"`
	RevealAddress     string      `json:"revealAddress"`
	ServiceFee        int64       `json:"serviceFee"`
	ServiceAddress    string      `json:"serviceAddress"`
	Extra             interface{} `json:"extra"`
}

type IdCoinsMintCommitResp struct {
	OrderId            string `json:"orderId"`
	CommitTxId         string `json:"commitTxId"`
	RevealInscribeTxId string `json:"revealInscribeTxId"`
	RevealMintTxId     string `json:"revealMintTxId"`
}
