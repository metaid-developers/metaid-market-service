package respond

type Mrc20MintPreResp struct {
	OrderId          string      `json:"orderId"`
	TotalFee         int64       `json:"totalFee"`
	RevealFee        int64       `json:"revealFee"`
	RevealAddress    string      `json:"revealAddress"`
	ServiceFee       int64       `json:"serviceFee"`
	ServiceAddress   string      `json:"serviceAddress"`
	RevealPrePsbtRaw string      `json:"revealPrePsbtRaw"`
	RevealInputIndex int64       `json:"revealInputIndex"`
	Extra            interface{} `json:"extra"`
}

type Mrc20MintCommitResp struct {
	OrderId    string `json:"orderId"`
	CommitTxId string `json:"commitTxId"`
	RevealTxId string `json:"revealTxId"`
}

type Mrc20TransferPreResp struct {
	OrderId          string      `json:"orderId"`
	TotalFee         int64       `json:"totalFee"`
	RevealFee        int64       `json:"revealFee"`
	RevealAddress    string      `json:"revealAddress"`
	ServiceFee       int64       `json:"serviceFee"`
	ServiceAddress   string      `json:"serviceAddress"`
	RevealPrePsbtRaw string      `json:"revealPrePsbtRaw"`
	RevealInputIndex int64       `json:"revealInputIndex"`
	Extra            interface{} `json:"extra"`
}

type Mrc20TransferCommitResp struct {
	OrderId    string `json:"orderId"`
	CommitTxId string `json:"commitTxId"`
	RevealTxId string `json:"revealTxId"`
}

type Mrc20DeployResp struct {
	OrderId    string `json:"orderId"`
	TickId     string `json:"tickId"`
	CommitTxId string `json:"commitTxId"`
	RevealTxId string `json:"revealTxId"`
}
