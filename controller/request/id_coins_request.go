package request

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
	OrderId          string `json:"orderId"`
	CommitTxRaw      string `json:"commitTxRaw"`
	CommitTxOutIndex int64  `json:"commitTxOutIndex"`
}
