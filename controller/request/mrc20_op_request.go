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
	PinUxtoTxId     string `json:"pinUxtoTxId"`
	PinUxtoIndex    uint32 `json:"pinUxtoIndex"`
	PinUtxoOutValue int64  `json:"pinUtxoOutValue"`
	Address         string `json:"address"`
	PkScript        string `json:"pkScript"`
}

type Mrc20MintCommitRequest struct {
	OrderId          string `json:"orderId"`
	CommitTxRaw      string `json:"commitTxRaw"`
	CommitTxOutIndex int64  `json:"commitTxOutIndex"`
	RevealPrePsbtRaw string `json:"revealPrePsbtRaw"`
}
