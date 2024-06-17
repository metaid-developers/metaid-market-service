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
