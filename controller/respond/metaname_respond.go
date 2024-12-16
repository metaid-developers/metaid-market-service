package respond

import "metaid-market-service/models"

type RegisterMetaNamePreResp struct {
	OrderId        string `json:"orderId"`
	TotalFee       int64  `json:"totalFee"`
	ReceiveAddress string `json:"receiveAddress"`
	MinerFee       int64  `json:"minerFee"`
	MinerGas       int64  `json:"minerGas"`
	MinerOutValue  int64  `json:"minerOutValue"`
	ServiceFee     int64  `json:"serviceFee"`
	MetaName       string `json:"metaName"`
}

type RegisterMetaNameCommitResp struct {
	OrderId    string `json:"orderId"`
	CommitTxId string `json:"commitTxId"`
	RevealTxId string `json:"revealTxId"`
	PinId      string `json:"pinId"`
	TxId       string `json:"txId"`
	MetaName   string `json:"metaName"`
}

type FetchMetaNameOpOrdersResp struct {
	Total int64                      `json:"total"`
	List  []*MetaNameOpOrderInfoResp `json:"list"`
}

type MetaNameOpOrderInfoResp struct {
	OpOrderType       string                   `json:"opOrderType"`
	OrderId           string                   `json:"orderId"`
	MetaName          string                   `json:"metaName"`
	Name              string                   `json:"name"`
	Namespace         string                   `json:"namespace"`
	ReceiveAddress    string                   `json:"receiveAddress"`
	RegisterAddress   string                   `json:"registerAddress"`
	RegisterState     int                      `json:"registerState"` //0-pending, 1-success, 2-fail
	TxId              string                   `json:"txId"`
	BlockHeight       int64                    `json:"blockHeight"`
	ConfirmationState models.ConfirmationState `json:"confirmationState"`
	Timestamp         int64                    `json:"timestamp"`
	MetaData          string                   `json:"metaData"`
}
