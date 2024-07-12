package orders_exchange_service

type Message struct {
	Code           int         `json:"code"`
	Message        string      `json:"message"`
	ProcessingTime int64       `json:"processingTime"`
	Data           interface{} `json:"data"`
}

type BuildIdCoinsPreRequest struct {
	Tick             string `json:"tick"`
	TokenName        string `json:"ticker"`
	IssuerMetaId     string `json:"issuerMetaId"`
	IssuerAddress    string `json:"issuerAddress"`
	IssuerSign       string `json:"issuerSign"`
	Message          string `json:"message"`
	FollowersNum     int64  `json:"followersNum"`
	AmountPerMint    int64  `json:"amountPerMint"`
	LiquidityPerMint int64  `json:"liquidityPerMint"`
	NetworkFeeRate   int64  `json:"networkFeeRate"`
}

type BuildIdCoinsCommitRequest struct {
	OrderId          string `json:"orderId"`
	CommitTxRaw      string `json:"commitTxRaw"`
	CommitTxOutIndex int64  `json:"commitTxOutIndex"`
}

type IdCoinsMintPreRequest struct {
	NetworkFeeRate int64  `json:"networkFeeRate"`
	TickId         string `json:"tickId"`
	OutAddress     string `json:"outAddress"`
	OutValue       int64  `json:"outValue"`
}

type IdCoinsMintCommitRequest struct {
	OrderId                  string `json:"orderId"`
	CommitTxRaw              string `json:"commitTxRaw"`
	CommitTxOutInscribeIndex int64  `json:"commitTxOutInscribeIndex"`
	CommitTxOutMintIndex     int64  `json:"commitTxOutMintIndex"`
}

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
	OrderId               string      `json:"orderId"`
	TotalFee              int64       `json:"totalFee"`
	RevealInscribeFee     int64       `json:"revealInscribeFee"`
	RevealMintFee         int64       `json:"revealMintFee"`
	RevealInscribeAddress string      `json:"revealInscribeAddress"`
	RevealMintAddress     string      `json:"revealMintAddress"`
	ServiceFee            int64       `json:"serviceFee"`
	ServiceAddress        string      `json:"serviceAddress"`
	Extra                 interface{} `json:"extra"`
}

type IdCoinsMintCommitResp struct {
	OrderId            string `json:"orderId"`
	CommitTxId         string `json:"commitTxId"`
	RevealInscribeTxId string `json:"revealInscribeTxId"`
	RevealMintTxId     string `json:"revealMintTxId"`
}
