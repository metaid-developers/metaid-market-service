package request

type Mrc20MintPreRequest struct {
	NetworkFeeRate int64          `json:"networkFeeRate"`
	TickerId       string         `json:"tickerId"`
	MintPins       []*MintPinInfo `json:"mintPins"`
	OutAddress     string         `json:"outAddress"`
	OutValue       int64          `json:"outValue"`
}

type MintPinInfo struct {
	PinId           string `json:"pinId"`
	PinUtxoTxId     string `json:"pinUtxoTxId"`
	PinUtxoIndex    uint32 `json:"pinUtxoIndex"`
	PinUtxoOutValue int64  `json:"pinUtxoOutValue"`
	Address         string `json:"address"`
	PkScript        string `json:"pkScript"`
	OutRaw          string `json:"outRaw"`
}

type Mrc20MintCommitRequest struct {
	OrderId          string `json:"orderId"`
	CommitTxRaw      string `json:"commitTxRaw"`
	CommitTxOutIndex int64  `json:"commitTxOutIndex"`
	RevealPrePsbtRaw string `json:"revealPrePsbtRaw"`
}

type Mrc20TransferPreRequest struct {
	NetworkFeeRate int64                `json:"networkFeeRate"`
	TickerId       string               `json:"tickerId"`
	Transfers      []*TransferMrc20Utxo `json:"transfers"`
	ChangeAddress  string               `json:"changeAddress"`
	ChangeOutValue int64                `json:"changeOutValue"`
	Mrc20Outs      []*Mrc20OutInfo      `json:"mrc20Outs"`
}

type TransferMrc20Utxo struct {
	UtxoTxId     string `json:"utxoTxId"`
	UtxoIndex    uint32 `json:"utxoIndex"`
	UtxoOutValue int64  `json:"utxoOutValue"`
	TickerId     string `json:"tickerId"`
	Amount       string `json:"amount"`
	Address      string `json:"address"`
	PkScript     string `json:"pkScript"`
	OutRaw       string `json:"outRaw"`
}

type Mrc20OutInfo struct {
	Amount   string `json:"amount"`
	Address  string `json:"address"`
	PkScript string `json:"pkScript"`
	OutValue int64  `json:"outValue"`
}

type Mrc20TransferCommitRequest struct {
	OrderId          string `json:"orderId"`
	CommitTxRaw      string `json:"commitTxRaw"`
	CommitTxOutIndex int64  `json:"commitTxOutIndex"`
	RevealPrePsbtRaw string `json:"revealPrePsbtRaw"`
}
