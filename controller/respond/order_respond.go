package respond

import "metaid-market-service/models"

type BatchPushOrderResp struct {
	Total int64            `json:"total"`
	List  []*PushOrderResp `json:"list"`
}

type PushOrderResp struct {
	OrderId    string            `json:"orderId"`
	AssetType  models.AssetType  `json:"assetType"`
	AssetId    string            `json:"assetId"`
	OrderState models.OrderState `json:"orderState"`
}

type OrderListResp struct {
	Total int64        `json:"total"`
	List  []*OrderInfo `json:"list"`
}

type OrderInfo struct {
	OrderId           string                   `json:"orderId"`
	UtxoId            string                   `json:"utxoId"`
	OutValue          int64                    `json:"outValue"`
	AssetType         models.AssetType         `json:"assetType"`
	AssetId           string                   `json:"assetId"`
	AssetNumber       int64                    `json:"assetNumber"`
	AssetLevel        int64                    `json:"assetLevel"`
	AssetPop          string                   `json:"assetPop"`
	OrderState        models.OrderState        `json:"orderState"`
	HolderAddress     string                   `json:"holderAddress,omitempty"`
	Holder            *UserInfo                `json:"holder,omitempty"`
	SellerAddress     string                   `json:"sellerAddress"`
	Seller            *UserInfo                `json:"seller"`
	BuyerAddress      string                   `json:"buyerAddress"`
	Buyer             *UserInfo                `json:"buyer"`
	SellPriceAmount   int64                    `json:"sellPriceAmount"`
	SellPriceDecimal  int64                    `json:"sellPriceDecimal"`
	SellPriceCoin     string                   `json:"sellPriceCoin"`
	PinStatus         int64                    `json:"pinStatus"` //0:transfer-confirm, -9:transfer-unconfirm
	Fee               int64                    `json:"fee"`
	FeeRate           int64                    `json:"feeRate"`
	FeeRateStr        string                   `json:"feeRateStr"`
	Content           string                   `json:"content"`
	Preview           string                   `json:"preview"`
	Detail            interface{}              `json:"detail"`
	TakePsbt          string                   `json:"takePsbt,omitempty"`
	BlockHeight       int64                    `json:"blockHeight"`
	ConfirmationState models.ConfirmationState `json:"confirmationState"`
	DealTime          int64                    `json:"dealTime"`
	TxId              string                   `json:"txId"`
}

type UserInfo struct {
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

type TakeOrderResp struct {
	OrderId    string            `json:"orderId"`
	TxId       string            `json:"txId"`
	AssetType  models.AssetType  `json:"assetType"`
	AssetId    string            `json:"assetId"`
	OrderState models.OrderState `json:"orderState"`
}

type CancelOrderResp struct {
	OrderId    string            `json:"orderId"`
	AssetType  models.AssetType  `json:"assetType"`
	AssetId    string            `json:"assetId"`
	OrderState models.OrderState `json:"orderState"`
}

type AddressAssetListResp struct {
	Total int64        `json:"total"`
	List  []*OrderInfo `json:"list"`
}

type BatchOrderPsbtResp struct {
	OrderIds []string `json:"orderIds"`
	TakePsbt string   `json:"takePsbt,omitempty"`
	Fee      int64    `json:"fee"`
	FeeRate  int64    `json:"feeRate"`
}

type BatchTakeOrderResp struct {
	TxId  string           `json:"txId"`
	Total int64            `json:"total"`
	List  []*TakeOrderInfo `json:"list"`
}
type TakeOrderInfo struct {
	OrderId    string            `json:"orderId"`
	AssetType  models.AssetType  `json:"assetType"`
	AssetId    string            `json:"assetId"`
	OrderState models.OrderState `json:"orderState"`
}

// mrc20 orders
type PushMrc20OrderResp struct {
	OrderId    string            `json:"orderId"`
	AssetType  models.AssetType  `json:"assetType"`
	TickId     string            `json:"tickId"`
	OrderState models.OrderState `json:"orderState"`
}

type Mrc20OrderListResp struct {
	Total int64             `json:"total"`
	List  []*Mrc20OrderInfo `json:"list"`
}

type Mrc20OrderInfo struct {
	OrderId           string                   `json:"orderId"`
	UtxoId            string                   `json:"utxoId"`
	OutValue          int64                    `json:"outValue"`
	AskType           models.AskType           `json:"askType"`
	AssetType         models.AssetType         `json:"assetType"`
	OrderState        models.OrderState        `json:"orderState"`
	SellerMetaId      string                   `json:"sellerMetaId"`
	SellerAddress     string                   `json:"sellerAddress"`
	Seller            *UserInfo                `json:"seller"`
	BuyerMetaId       string                   `json:"buyerMetaId"`
	BuyerAddress      string                   `json:"buyerAddress"`
	Buyer             *UserInfo                `json:"buyer"`
	TickId            string                   `json:"tickId"`
	Tick              string                   `json:"tick"`
	TokenName         string                   `json:"tokenName"`
	Decimals          int64                    `json:"decimals"`
	Chain             string                   `json:"chain"`
	Amount            int64                    `json:"amount"`
	AmountStr         string                   `json:"amountStr"`
	TokenPriceRate    float64                  `json:"tokenPriceRate"`
	TokenPriceRateStr string                   `json:"tokenPriceRateStr"`
	PriceAmount       int64                    `json:"priceAmount"`
	PriceDecimal      int64                    `json:"priceDecimal"`
	PriceCoin         string                   `json:"priceCoin"`
	Fee               int64                    `json:"fee"`
	FeeRate           int64                    `json:"feeRate"`
	FeeRateStr        string                   `json:"feeRateStr"`
	TakePsbt          string                   `json:"takePsbt,omitempty"`
	BlockHeight       int64                    `json:"blockHeight"`
	ConfirmationState models.ConfirmationState `json:"confirmationState"`
	DealTime          int64                    `json:"dealTime"`
	TxId              string                   `json:"txId"`
}

type TakeMrc20OrderResp struct {
	OrderId    string            `json:"orderId"`
	TxId       string            `json:"txId"`
	AssetType  models.AssetType  `json:"assetType"`
	TickId     string            `json:"tickId"`
	OrderState models.OrderState `json:"orderState"`
}

type CancelMrc20OrderResp struct {
	OrderId    string            `json:"orderId"`
	AssetType  models.AssetType  `json:"assetType"`
	TickId     string            `json:"tickId"`
	OrderState models.OrderState `json:"orderState"`
}

// 热门币种信息
type Mrc20HotInfo struct {
	TickId           string    `json:"tickId"`     // 币种ID
	Tick             string    `json:"tick"`       // 币种符号
	TokenName        string    `json:"tokenName"`  // 币种名称
	MarketCap        int64     `json:"marketCap"`  // 市值
	LastPrice        float64   `json:"lastPrice"`  // 最新价格
	Change24H        string    `json:"change24H"`  // 24小时涨跌幅（基点）
	TradeCount       int64     `json:"tradeCount"` // 交易数量
	Holders          int64     `json:"holders"`
	DeployerUserInfo *UserInfo `json:"deployerUserInfo"`
	MetaData         string    `json:"metaData"`
	Tag              string    `json:"tag"`
}

// 热门币种列表响应
type Mrc20HotListResp struct {
	TimeRange int64           `json:"timeRange"` // 查询的时间范围（毫秒）
	Total     int64           `json:"total"`     // 总数
	List      []*Mrc20HotInfo `json:"list"`      // 热门币种列表
}

// 最新交易币种信息
type Mrc20NewestInfo struct {
	TickId           string    `json:"tickId"`     // 币种ID
	Tick             string    `json:"tick"`       // 币种符号
	TokenName        string    `json:"tokenName"`  // 币种名称
	MarketCap        int64     `json:"marketCap"`  // 市值
	LastPrice        float64   `json:"lastPrice"`  // 最新价格
	Change24H        string    `json:"change24H"`  // 24小时涨跌幅（基点）
	TradeCount       int64     `json:"tradeCount"` // 交易数量
	Holders          int64     `json:"holders"`
	DeployerUserInfo *UserInfo `json:"deployerUserInfo"`
	MetaData         string    `json:"metaData"`
	Tag              string    `json:"tag"`
}

// 最新交易币种列表响应
type Mrc20NewestListResp struct {
	Total int64              `json:"total"` // 总数
	List  []*Mrc20NewestInfo `json:"list"`  // 最新交易币种列表
}
