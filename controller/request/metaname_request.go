package request

import "metaid-market-service/models"

type RegisterMetaNamePreRequest struct {
	MetaName       string `json:"metaName"`
	Address        string `json:"address"`
	NetworkFeeRate int64  `json:"networkFeeRate"`
}

type RegisterMetaNameCommitRequest struct {
	OrderId          string `json:"orderId"`
	CommitTxRaw      string `json:"commitTxRaw"`
	CommitTxOutIndex int64  `json:"commitTxOutIndex"`
}

type FetchMetaNameOpOrdersRequest struct {
	Address      string                   `json:"address"`
	Cursor       int64                    `json:"cursor"`
	Size         int64                    `json:"size"`
	Confirmation models.ConfirmationState `json:"confirmation"`
}
