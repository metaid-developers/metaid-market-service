package mrc20_service

import (
	"fmt"
	"github.com/btcsuite/btcd/chaincfg"
	"metaid-market-service/common"
	"metaid-market-service/conf"
)

type Mrc20OpRequest struct {
	Net                   *chaincfg.Params
	MetaIdFlag            string
	Op                    string //deploy, mint, transfer
	OpPayload             string
	MintPins              []*MintPin
	TransferMrc20s        []*TransferMrc20
	Mrc20Outs             []*Mrc20OutInfo
	PayTos                []*PayTo
	OtherOuts             []*OtherOut
	Mrc20OutValue         int64
	Mrc20OutAddressList   []string
	ChangeAddress         string
	DeployPinOutAddress   string
	DeployMrc20OutAddress string
	TransferAddress       string
}

type Mrc20OutInfo struct {
	Amount   string `json:"amount"`
	Address  string `json:"address"`
	PkScript string `json:"pkScript"`
	OutValue int64  `json:"outValue"`
}

type OtherOut struct {
	Address string
	Amount  int64
	Script  string
}

type OtherIn struct {
	UtxoTxId      string
	UtxoIndex     uint32
	UtxoOutValue  int64
	Address       string
	PrivateKeyHex string
	RedeemScript  string
	PkScript      string
	OutRaw        string
}

func Mrc20DeployBuilder(opRep *Mrc20OpRequest, feeRate int64) (*Mrc20Builder, int64, error) {
	var (
		err          error
		mrc20Builder *Mrc20Builder
		fee          int64 = 0

		content                = opRep.OpPayload
		path                   = "/ft/mrc20/deploy"
		metaIdData *MetaIdData = &MetaIdData{
			MetaIDFlag:  opRep.MetaIdFlag,
			Operation:   "create",
			Path:        path,
			Content:     []byte(content),
			Encryption:  "",
			Version:     "",
			ContentType: "application/json",
		}

		host string = conf.Host
	)
	if host != "" {
		metaIdData.Path = fmt.Sprintf("%s:%s", host, path)
	}

	mrc20Builder = &Mrc20Builder{
		Net:            opRep.Net,
		MetaIdData:     metaIdData,
		MintPins:       opRep.MintPins,
		TransferMrc20s: opRep.TransferMrc20s,
		FeeRate:        feeRate,
		op:             opRep.Op,

		mrc20OutValue:       opRep.Mrc20OutValue,
		mrc20OutAddressList: opRep.Mrc20OutAddressList,

		mrc20PinOutAddress:     opRep.DeployPinOutAddress,
		mrc20PremineOutAddress: opRep.DeployMrc20OutAddress,
		transferAddress:        opRep.TransferAddress,

		OtherOuts: opRep.OtherOuts,
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

func Mrc20MintBuilder(opRep *Mrc20OpRequest, feeRate int64) (*Mrc20Builder, int64, error) {
	var (
		err          error
		mrc20Builder *Mrc20Builder
		fee          int64 = 0

		content                = opRep.OpPayload
		path                   = "/ft/mrc20/mint"
		metaIdData *MetaIdData = &MetaIdData{
			MetaIDFlag: opRep.MetaIdFlag,
			//Operation:   "hide",
			Operation:   "create",
			Path:        path,
			Content:     []byte(content),
			Encryption:  "",
			Version:     "",
			ContentType: "application/json",
		}
		host string = conf.Host
	)

	if host != "" {
		metaIdData.Path = fmt.Sprintf("%s:%s", host, path)
	}
	mrc20Builder = &Mrc20Builder{
		Net:            opRep.Net,
		MetaIdData:     metaIdData,
		MintPins:       opRep.MintPins,
		TransferMrc20s: opRep.TransferMrc20s,
		FeeRate:        feeRate,
		op:             opRep.Op,
		PayTos:         opRep.PayTos,
		OtherOuts:      opRep.OtherOuts,

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

func Mrc20TransferBuilder(opRep *Mrc20OpRequest, feeRate int64) (*Mrc20Builder, int64, error) {
	var (
		err          error
		mrc20Builder *Mrc20Builder
		fee          int64 = 0

		content                = opRep.OpPayload
		path                   = "/ft/mrc20/transfer"
		metaIdData *MetaIdData = &MetaIdData{
			MetaIDFlag:  opRep.MetaIdFlag,
			Operation:   "hide",
			Path:        path,
			Content:     []byte(content),
			Encryption:  "",
			Version:     "",
			ContentType: "application/json",
		}
		host string = conf.Host
	)
	if host != "" {
		metaIdData.Path = fmt.Sprintf("%s:%s", host, path)
	}
	mrc20Builder = &Mrc20Builder{
		Net:                opRep.Net,
		MetaIdData:         metaIdData,
		MintPins:           opRep.MintPins,
		TransferMrc20s:     opRep.TransferMrc20s,
		Mrc20Outs:          opRep.Mrc20Outs,
		FeeRate:            feeRate,
		op:                 opRep.Op,
		mrc20ChangeAddress: opRep.ChangeAddress,
		OtherOuts:          opRep.OtherOuts,

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
