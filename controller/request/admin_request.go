package request

import "metaid-market-service/models"

type ColdDownDummyUtxoRequest struct {
	UtxoType      models.UtxoType `json:"utxoType"`
	TxId          string          `json:"txId"`
	Index         int64           `json:"index"`
	Amount        uint64          `json:"amount"`
	PkScript      string          `json:"pkScript"`
	Address       string          `json:"address"`
	PriKeyHex     string          `json:"priKeyHex"`
	PerAmount     uint64          `json:"perAmount"`
	Count         int64           `json:"count"`
	ChangeAddress string          `json:"changeAddress"`
	FeeRate       int64           `json:"feeRate"`
}
