package respond

import "metaid-market-service/models"

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

type FetchIdCoinsOpOrdersResp struct {
	Total int64                     `json:"total"`
	List  []*IdCoinsOpOrderInfoResp `json:"list"`
}

type IdCoinsOpOrderInfoResp struct {
	OpOrderType       string                   `json:"opOrderType"`
	OrderId           string                   `json:"orderId"`
	TickId            string                   `json:"tickId"`
	Tick              string                   `json:"tick"`
	TickName          string                   `json:"tickName"`
	Decimals          string                   `json:"decimals"`
	DeployState       int                      `json:"deployState"` //0-pending, 1-success, 2-fail
	MintState         int                      `json:"mintState"`   //0-pending, 1-success, 2-fail
	AmtPerMint        string                   `json:"amtPerMint"`
	MintCount         string                   `json:"mintCount"`
	PremineCount      string                   `json:"premineCount"`
	TotalMinted       string                   `json:"totalMinted"`
	FollowersLimit    string                   `json:"followersLimit"`
	LiquidityPerMint  int64                    `json:"liquidityPerMint"`
	StartBlockHeight  string                   `json:"startBlockHeight"`
	EndBlockHeight    string                   `json:"endBlockHeight"`
	Qual              string                   `json:"qual"`
	PinCheck          string                   `json:"pinCheck"`
	PayCheck          string                   `json:"payCheck"`
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
	MarketPrice      string      `json:"marketPrice"`
	MarketPriceUsd   string      `json:"marketPriceUsd"`
	FloorPrice       string      `json:"floorPrice"`
	FloorPriceUsd    string      `json:"floorPriceUsd"`
	Change24h        string      `json:"change24h"`
	MarketCap        string      `json:"marketCap"`
	MarketCapUsd     string      `json:"marketCapUsd"`
	TotalVolume      int64       `json:"totalVolume"`
	OrdersPrice      string      `json:"ordersPrice"`
	OrdersPool       int64       `json:"ordersPool"`
}

type FetchIdCoinsDeployCheckResp struct {
	CanDeploy bool   `json:"canDeploy"`
	Msg       string `json:"msg"`
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
