package mrc20_service

import (
	"fmt"
	"github.com/btcsuite/btcd/chaincfg"
	"metaid-market-service/common"
)

type Mrc20OpRequest struct {
	Net                 *chaincfg.Params
	MetaIdFlag          string
	Op                  string // mint, transfer
	OpPayload           string
	MintPins            []*MintPin
	TransferMrc20s      []*TransferMrc20
	Mrc20OutValue       int64
	Mrc20OutAddressList []string
	ChangeAddress       string
}

func Mrc20MintBuilder(opRep *Mrc20OpRequest, feeRate int64) (*Mrc20Builder, int64, error) {
	var (
		err          error
		mrc20Builder *Mrc20Builder
		fee          int64 = 0

		content                = opRep.OpPayload
		path                   = "/ft/mrc20/mint"
		metaIdData *MetaIdData = &MetaIdData{
			MetaIDFlag:  opRep.MetaIdFlag,
			Operation:   "hide",
			Path:        path,
			Content:     []byte(content),
			Encryption:  "",
			Version:     "",
			ContentType: "application/json",
		}
	)
	mrc20Builder = &Mrc20Builder{
		Net:            opRep.Net,
		MetaIdData:     metaIdData,
		MintPins:       opRep.MintPins,
		TransferMrc20s: opRep.TransferMrc20s,
		FeeRate:        feeRate,

		mrc20OutValue:       opRep.Mrc20OutValue,
		mrc20OutAddressList: opRep.Mrc20OutAddressList,
	}

	txCtxData, err := createMetaIdTxCtxData(opRep.Net, mrc20Builder.MetaIdData)
	if err != nil {
		return nil, 0, err
	}
	mrc20Builder.TxCtxData = txCtxData

	err = mrc20Builder.buildEmptyRevealPsbt()
	if err != nil {
		return nil, 0, err
	}
	fee = mrc20Builder.CalRevealPsbtFee(feeRate)
	return mrc20Builder, fee, nil
}

func SignMrc20Mint(builder *Mrc20Builder, commitTxId string, commitTxOutIndex uint32, mintPins []*MintPin, taprootInSigner *common.InputSign) (*Mrc20Builder, error) {
	var (
		err error
	)
	if builder == nil {
		return nil, fmt.Errorf("builder is nil")
	}
	err = builder.completeRevealPsbt(commitTxId, commitTxOutIndex)
	if err != nil {
		return nil, err
	}
	err = builder.signRevealPsbt(mintPins, nil, taprootInSigner)
	if err != nil {
		return nil, err
	}
	return builder, nil
}
