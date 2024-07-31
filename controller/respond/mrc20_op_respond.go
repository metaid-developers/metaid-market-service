package respond

import "metaid-market-service/models"

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

type Mrc20DeployPreResp struct {
	OrderId       string `json:"orderId"`
	TotalFee      int64  `json:"totalFee"`
	MinerFee      int64  `json:"minerFee"`
	ServiceFee    int64  `json:"serviceFee"`
	RevealAddress string `json:"revealAddress"`
	//RevealPrePsbtRaw string      `json:"revealPrePsbtRaw"`
	//RevealInputIndex int64       `json:"revealInputIndex"`
	Extra interface{} `json:"extra"`
}
type Mrc20DeployCommitResp struct {
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

type FetchMrc20OpOrdersResp struct {
	Total int64              `json:"total"`
	List  []*OpOrderInfoResp `json:"list"`
}

type OpOrderInfoResp struct {
	OpOrderType      string `json:"opOrderType"`
	OrderId          string `json:"orderId"`
	TickId           string `json:"tickId"`
	Tick             string `json:"tick"`
	TickName         string `json:"tickName"`
	Decimals         string `json:"decimals"`
	DeployState      int    `json:"deployState"` //0-pending, 1-success, 2-fail
	MintState        int    `json:"mintState"`   //0-pending, 1-success, 2-fail
	AmtPerMint       string `json:"amtPerMint"`
	MintCount        string `json:"mintCount"`
	PremineCount     string `json:"premineCount"`
	TotalMinted      string `json:"totalMinted"`
	StartBlockHeight string `json:"startBlockHeight"`
	EndBlockHeight   string `json:"endBlockHeight"`
	Qual             string `json:"qual"`
	PinCheck         string `json:"pinCheck"`
	PayCheck         string `json:"payCheck"`
	//QualPath          string                   `json:"qualPath"`
	//QualLevel         int64                    `json:"qualLevel"`
	//QualCount         int64                    `json:"qualCount"`
	UsedPins          []string                 `json:"usedPins"`
	Holders           int64                    `json:"holders"`
	TxId              string                   `json:"txId"`
	BlockHeight       int64                    `json:"blockHeight"`
	ConfirmationState models.ConfirmationState `json:"confirmationState"`
	Timestamp         int64                    `json:"timestamp"`
	DeployerAddress   string                   `json:"deployerAddress"`
	DeployerMetaId    string                   `json:"deployerMetaId"`
	DeployerUserInfo  *UserInfo                `json:"deployerUserInfo"`
	MetaData          string                   `json:"metaData"`
}
