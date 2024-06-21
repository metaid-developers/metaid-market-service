package common_service

import (
	"metaid-market-service/service/man_service"
)

func FetchTxPointInfo(txId string, index, cursor, size int64) ([]*man_service.Mrc20Utxo, int64, error) {
	var (
		txPointInfo *man_service.Mrc20UtxoResp
		err         error
	)
	txPointInfo, err = man_service.FetchMrc20txPointList(txId, index, cursor, size)
	if err != nil {
		return nil, 0, err
	}
	return txPointInfo.List, txPointInfo.Total, nil
}

type TickInfo struct {
	Tick        string      `json:"tick"`
	TokenName   string      `json:"tokenName"`
	Decimals    string      `json:"decimals"`
	AmtPerMint  string      `json:"amtPerMint"`
	MintCount   string      `json:"mintCount"`
	BlockHeight string      `json:"blockheight"`
	MetaData    string      `json:"metadata"`
	Type        string      `json:"type"`
	Qual        interface{} `json:"qual"`
	TotalMinted int64       `json:"totalMinted"`
	Mrc20Id     string      `json:"mrc20Id"`
	PinNumber   int64       `json:"pinNumber"`
}

func GetMrc20TickInfo(tickId string) (*TickInfo, error) {
	var (
		mrc20Resp *man_service.Mrc20TickInfo
		err       error
	)
	mrc20Resp, err = man_service.FetchMrc20TickInfo(tickId)
	if err != nil {
		return nil, err
	}
	return &TickInfo{
		Tick:        mrc20Resp.Tick,
		TokenName:   mrc20Resp.TokenName,
		Decimals:    mrc20Resp.Decimals,
		AmtPerMint:  mrc20Resp.AmtPerMint,
		MintCount:   mrc20Resp.MintCount,
		BlockHeight: mrc20Resp.BlockHeight,
		MetaData:    mrc20Resp.MetaData,
		Type:        mrc20Resp.Type,
		Qual:        mrc20Resp.Qual,
		TotalMinted: mrc20Resp.TotalMinted,
		Mrc20Id:     mrc20Resp.Mrc20Id,
		PinNumber:   mrc20Resp.PinNumber,
	}, nil
}
