package orders_exchange_service

import "metaid-market-service/models"

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
	MinerGas       int64  `json:"minerGas"`
	MinerOutValue  int64  `json:"minerOutValue"`
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
	OrderId                string      `json:"orderId"`
	TotalFee               int64       `json:"totalFee"`
	RevealInscribeFee      int64       `json:"revealInscribeFee"`
	RevealInscribeGas      int64       `json:"revealInscribeGas"`
	RevealInscribeOutValue int64       `json:"revealInscribeOutValue"`
	RevealMintFee          int64       `json:"revealMintFee"`
	RevealMintGas          int64       `json:"revealMintGas"`
	RevealMintOutValue     int64       `json:"revealMintOutValue"`
	RevealInscribeAddress  string      `json:"revealInscribeAddress"`
	RevealMintAddress      string      `json:"revealMintAddress"`
	ServiceFee             int64       `json:"serviceFee"`
	PayToAmount            int64       `json:"payToAmount"`
	Extra                  interface{} `json:"extra"`
}

type IdCoinsMintCommitResp struct {
	OrderId            string `json:"orderId"`
	CommitTxId         string `json:"commitTxId"`
	RevealInscribeTxId string `json:"revealInscribeTxId"`
	RevealMintTxId     string `json:"revealMintTxId"`
}

type FetchIdCoinsListRequest struct {
	Address         string `json:"address"`
	Cursor          int64  `json:"cursor"`
	Size            int64  `json:"size"`
	OrderBy         string `json:"order"`
	SortType        int    `json:"sortType"`
	FollowerAddress string `json:"followerAddress"`
	SearchTick      string `json:"searchTick"`
	Completed       string `json:"completed"`
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

type FetchIdCoinsOpOrdersResp struct {
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
	FollowersLimit   string `json:"followersLimit"`
	LiquidityPerMint int64  `json:"liquidityPerMint"`
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
	RefundTxId        string                   `json:"refundTxId"`
	BlockHeight       int64                    `json:"blockHeight"`
	ConfirmationState models.ConfirmationState `json:"confirmationState"`
	Timestamp         int64                    `json:"timestamp"`
	DeployerAddress   string                   `json:"deployerAddress"`
	DeployerMetaId    string                   `json:"deployerMetaId"`
	DeployerUserInfo  *UserInfo                `json:"deployerUserInfo"`
	MetaData          string                   `json:"metaData"`
}

type UserInfo struct {
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

type FetchIdCoinsListResp struct {
	Total int64              `json:"total"`
	List  []*IdCoinsInfoResp `json:"list"`
}

type IdCoinsInfoResp struct {
	TickId           string      `json:"tickId"`
	Tick             string      `json:"tick"`
	TokenName        string      `json:"tokenName"`
	Decimals         string      `json:"decimals"`
	AmtPerMint       string      `json:"amtPerMint"`
	FollowersLimit   string      `json:"followersLimit"`
	MintCount        string      `json:"mintCount"`
	LiquidityPerMint int64       `json:"liquidityPerMint"`
	PremineCount     string      `json:"premineCount"`
	TotalMinted      string      `json:"totalMinted"`
	BlockHeight      string      `json:"blockHeight"`
	MetaData         string      `json:"metaData"`
	Type             string      `json:"type"`
	Qual             interface{} `json:"qual"`
	PinCheck         interface{} `json:"pinCheck"`
	PayCheck         interface{} `json:"payCheck"`
	Mrc20Id          string      `json:"mrc20Id"`
	PinNumber        int64       `json:"pinNumber"`
	Holders          int64       `json:"holders"`
	DeployerMetaId   string      `json:"deployerMetaId"`
	DeployerAddress  string      `json:"deployerAddress"`
	DeployerUserInfo *UserInfo   `json:"deployerUserInfo"`
	DeployTime       int64       `json:"deployTime"`
	Price            string      `json:"price"`
	PriceUsd         string      `json:"priceUsd"`
	Pool             int64       `json:"pool"`
	TotalSupply      string      `json:"totalSupply"`
	Supply           string      `json:"supply"`
	Mintable         bool        `json:"mintable"`
	Remaining        string      `json:"remaining"`
	IsFollowing      bool        `json:"isFollowing"`
	FollowersCount   int64       `json:"followersCount"`
	OrdersPrice      string      `json:"ordersPrice"`
	OrdersPool       int64       `json:"ordersPool"`
}

type FetchIdCoinsMintOrderRequest struct {
	Address string `json:"address"`
	TickId  string `json:"tickId"`
}

type FetchOneIdCoinsMintOrderResp struct {
	AddressMintState  int64                    `json:"addressMintState"` //0-unminted, 1-minted
	OpOrderType       string                   `json:"opOrderType"`
	OrderId           string                   `json:"orderId"`
	TickId            string                   `json:"tickId"`
	Tick              string                   `json:"tick"`
	TickName          string                   `json:"tickName"`
	Decimals          string                   `json:"decimals"`
	DeployState       int                      `json:"deployState"` //0-pending, 1-success, 2-fail
	MintState         int                      `json:"mintState"`   //0-pending, 1-success, 2-fail
	FollowersLimit    string                   `json:"followersLimit"`
	LiquidityPerMint  int64                    `json:"liquidityPerMint"`
	AmtPerMint        string                   `json:"amtPerMint"`
	MintCount         string                   `json:"mintCount"`
	PremineCount      string                   `json:"premineCount"`
	TotalMinted       string                   `json:"totalMinted"`
	StartBlockHeight  string                   `json:"startBlockHeight"`
	Qual              string                   `json:"qual"`
	PinCheck          string                   `json:"pinCheck"`
	PayCheck          string                   `json:"payCheck"`
	UsedPins          []string                 `json:"usedPins"`
	TxId              string                   `json:"txId"`
	BlockHeight       int64                    `json:"blockHeight"`
	ConfirmationState models.ConfirmationState `json:"confirmationState"`
	Timestamp         int64                    `json:"timestamp"`
	DeployerAddress   string                   `json:"deployerAddress"`
	DeployerMetaId    string                   `json:"deployerMetaId"`
	DeployerUserInfo  *UserInfo                `json:"deployerUserInfo"`
	MetaData          string                   `json:"metaData"`
}

type FetchIdCoinsTickIdsResp struct {
	TickIds []string `json:"tickIds"`
}

type FetchIdCoinsDeployCheckRequest struct {
	Address string `json:"address"`
}

type FetchIdCoinsDeployCheckResp struct {
	CanDeploy bool   `json:"canDeploy"`
	Msg       string `json:"msg"`
}

type RefundIdCoinsValidPreRequest struct {
	Address string `json:"address"`
	OrderId string `json:"orderId"`
}

type RefundIdCoinsValidCommitRequest struct {
	OrderId string `json:"orderId"`
	PsbtRaw string `json:"psbtRaw"`
}

type RefundIdCoinsValidPreResp struct {
	OrderId       string `json:"orderId"`
	RefundAddress string `json:"refundAddress"`
	RefundAmount  int64  `json:"refundAmount"`
	PsbtRaw       string `json:"psbtRaw"`
}

type RefundIdCoinsValidCommitResp struct {
	OrderId       string `json:"orderId"`
	RefundAddress string `json:"refundAddress"`
	RefundAmount  int64  `json:"refundAmount"`
	TxId          string `json:"txId"`
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

type BookMintOrderTakePreviewResp struct {
	AssetCoinList []string `json:"assetCoinList"`
}

type BookMintOrderTakePreResp struct {
	OrderId          string `json:"orderId"`
	TotalAmount      int64  `json:"totalAmount"`
	ReceiveAddress   string `json:"receiveAddress"`
	PriceAmount      int64  `json:"priceAmount"`
	TotalFee         int64  `json:"totalFee"`
	MinerFee         int64  `json:"minerFee"`
	ServiceFee       int64  `json:"serviceFee"`
	PsbtRaw          string `json:"psbtRaw"`
	RevealInputIndex int64  `json:"revealInputIndex"`
}

type BookMintOrderTakeCommitResp struct {
	OrderId    string `json:"orderId"`
	TxId       string `json:"txId"`
	CommitTxId string `json:"commitTxId"`
	RevealTxId string `json:"revealTxId"`
	TickId     string `json:"tickId"`
}

type AdminAddAutoBridgeReq struct {
	Mrc20Id string `json:"mrc20Id"`
}

type AdminAddAutoBridgeResp struct {
	Message string `json:"message"`
	OrderId string `json:"orderId"`
}

type AdminAutoBridgeInfoReq struct {
	Mrc20Id string `json:"mrc20Id"`
}

type BridgeBuildStatus int

const (
	BridgeBuildStatusInit BridgeBuildStatus = iota
	BridgeBuildStatusSuccess
	BridgeBuildStatusRechargeBridgeSuccess
	BridgeBuildStatusBuildSwapPoolSuccess
	BridgeBuildStatusUpdateSwapPoolSuccess
	BridgeBuildStatusBuildSwapSpacePoolSuccess
	BridgeBuildStatusUpdateSwapSpacePoolSuccess
	BridgeBuildStatusFail
	BridgeBuildStatusRechargeBridgeFail
	BridgeBuildStatusBuildSwapPoolFail
	BridgeBuildStatusUpdateSwapPoolFail
	BridgeBuildStatusBuildSwapSpacePoolFail
	BridgeBuildStatusUpdateSwapSpacePoolFail
)

type AdminAutoBridgeInfoResp struct {
	OrderId           string            `json:"orderId"`
	TickId            string            `json:"tickId"`
	Protocol          string            `gorm:"column:protocol" json:"protocol"`
	Tick              string            `gorm:"column:tick" json:"tick"`
	BridgeBuildStatus BridgeBuildStatus `json:"bridgeBuildStatus"`
}
