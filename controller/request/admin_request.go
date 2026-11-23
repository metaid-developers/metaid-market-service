package request

import (
	"metaid-market-service/common"
	"metaid-market-service/models"
)

type ColdDownDummyUtxoRequest struct {
	Chain         string          `json:"chain"`
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
	SignMode      common.SignMode `json:"signMode"`
}

type BlacklistUpsertItem struct {
	EntryType   string `json:"entryType"`
	EntryValue  string `json:"entryValue"`
	Reason      string `json:"reason"`
	ParentValue string `json:"parentValue"`
	State       int64  `json:"state"`
}

type BlacklistUpsertRequest struct {
	Entries     []BlacklistUpsertItem `json:"entries"`
	EntryType   string                `json:"entryType"`
	EntryValue  string                `json:"entryValue"`
	Reason      string                `json:"reason"`
	ParentValue string                `json:"parentValue"`
	State       int64                 `json:"state"`
}
