package request

import "metaid-market-service/models"

type BatchPushOrderReq struct {
	AssetType models.AssetType `json:"assetType"` //pins/ordinals
	AssetIds  []string         `json:"assetIds"`
	Address   string           `json:"address"`
	PsbtRaw   string           `json:"psbtRaw"`
}
type BatchFetchOrderPsbtReq struct {
	OrderIds          []string `json:"orderIds"`
	BuyerAddress      string   `json:"buyerAddress"`
	BuyerChangeAmount uint64   `json:"buyerChangeAmount"`
}
type BatchTakeOrderReq struct {
	OrderIds       []string `json:"orderIds"`
	TakerPsbtRaw   string   `json:"takerPsbtRaw"`
	NetworkFeeRate int64    `json:"networkFeeRate"`
}

type PushOrderReq struct {
	AssetType models.AssetType `json:"assetType"` //pins/ordinals
	AssetId   string           `json:"assetId"`
	Address   string           `json:"address"`
	PsbtRaw   string           `json:"psbtRaw"`
}

type FetchOrderPsbtReq struct {
	OrderId           string `json:"orderId"`
	BuyerAddress      string `json:"buyerAddress"`
	BuyerChangeAmount uint64 `json:"buyerChangeAmount"`
}

type TakeOrderReq struct {
	OrderId        string `json:"orderId"`
	TakerPsbtRaw   string `json:"takerPsbtRaw"`
	NetworkFeeRate int64  `json:"networkFeeRate"`
}

type CancelOrderReq struct {
	OrderId string `json:"orderId"`
}

type FetchMarketOrdersReq struct {
	OrderState models.OrderState `json:"orderState"` //1-create,2-finish,3-cancel
	AssetType  models.AssetType  `json:"assetType"`  //pins/ordinals
	Cursor     int64             `json:"cursor"`
	Size       int64             `json:"size"`
	Address    string            `json:"address"`
	SortKey    string            `json:"sortKey"`  //sellPriceAmount/timestamp/assetLevel
	SortType   int64             `json:"sortType"` //1/-1
}

type FetchMarketOneOrderReq struct {
	OrderId string `json:"orderId"`
}

type FetchAssetDetailReq struct {
	AssetType models.AssetType `json:"assetType"` //pins/ordinals
	AssetId   string           `json:"assetId"`
}

type FetchAddressAssetListReq struct {
	Address   string           `json:"address"`
	AssetType models.AssetType `json:"assetType"` //pins/ordinals
	Cursor    int64            `json:"cursor"`
	Size      int64            `json:"size"`
}

type TestAuthReq struct {
	Address string `json:"address"`
}

// mrc20 orders
type PushMrc20OrderReq struct {
	AssetType models.AssetType `json:"assetType"` //pins/ordinals
	TickId    string           `json:"tickId"`
	Address   string           `json:"address"`
	PsbtRaw   string           `json:"psbtRaw"`
}

type FetchMrc20OrderPsbtReq struct {
	OrderId           string `json:"orderId"`
	BuyerAddress      string `json:"buyerAddress"`
	BuyerChangeAmount uint64 `json:"buyerChangeAmount"`
}

type TakeMrc20OrderReq struct {
	OrderId        string `json:"orderId"`
	TakerPsbtRaw   string `json:"takerPsbtRaw"`
	NetworkFeeRate int64  `json:"networkFeeRate"`
}

type CancelMrc20OrderReq struct {
	OrderId string `json:"orderId"`
}

type FetchMarketMrc20OrdersReq struct {
	OrderState models.OrderState `json:"orderState"` //1-create,2-finish,3-cancel
	AssetType  models.AssetType  `json:"assetType"`  //mrc20
	Cursor     int64             `json:"cursor"`
	Size       int64             `json:"size"`
	Address    string            `json:"address"`
	SortKey    string            `json:"sortKey"`  //priceAmount/timestamp/tokenPriceRate
	SortType   int64             `json:"sortType"` //1/-1
}

type FetchMarketMrc20OneOrderReq struct {
	OrderId string `json:"orderId"`
}
