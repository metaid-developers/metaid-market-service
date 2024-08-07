package request

import "metaid-market-service/models"

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

type FetchIdCoinsListRequest struct {
	Address         string `json:"address"`
	Cursor          int64  `json:"cursor"`
	Size            int64  `json:"size"`
	OrderBy         string `json:"order"`
	SortType        int    `json:"sortType"`
	FollowerAddress string `json:"followerAddress"`
	SearchTick      string `json:"searchTick"`
}

type FetchOneIdCoinsRequest struct {
	TickId        string `json:"tickId"`
	Tick          string `json:"tick"`
	IssuerAddress string `json:"issuerAddress"`
	Address       string `json:"address"`
}

type FetchIdCoinsOpOrdersRequest struct {
	OpOrderType  string                   `json:"opOrderType"`
	Address      string                   `json:"address"`
	TickId       string                   `json:"tickId"`
	Cursor       int64                    `json:"cursor"`
	Size         int64                    `json:"size"`
	Confirmation models.ConfirmationState `json:"confirmation"`
}

type FetchIdCoinsMintOrderRequest struct {
	Address string `json:"address"`
	TickId  string `json:"tickId"`
}

type FetchIdCoinsDeployCheckRequest struct {
	Address string `json:"address"`
}

type RefundIdCoinsValidPreRequest struct {
	Address string `json:"address"`
	OrderId string `json:"orderId"`
}

type RefundIdCoinsValidCommitRequest struct {
	OrderId string `json:"orderId"`
	PsbtRaw string `json:"psbtRaw"`
}

type BookTakeMintBidPreviewReq struct {
	TickId         string   `json:"tickId"`
	AssetUtxoIds   []string `json:"assetUtxoIds"`
	SellerAddress  string   `json:"sellerAddress"`
	NetworkFeeRate int64    `json:"networkFeeRate"`
}

type BookTakeMintBidPreReq struct {
	TickId         string   `json:"tickId"`
	AssetUtxoIds   []string `json:"assetUtxoIds"`
	SellCoinAmount string   `json:"sellCoinAmount"`
	SellerAddress  string   `json:"sellerAddress"`
	NetworkFeeRate int64    `json:"networkFeeRate"`
}

type BookTakeMintBidCommitReq struct {
	OrderId          string `json:"orderId"`
	CommitTxRaw      string `json:"commitTxRaw"`
	CommitTxOutIndex int64  `json:"commitTxOutIndex"`
	RevealPrePsbtRaw string `json:"revealPrePsbtRaw"`
}
