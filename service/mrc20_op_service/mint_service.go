package mrc20_op_service

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"gorm.io/gorm"
	"metaid-market-service/common"
	"metaid-market-service/conf"
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
	"metaid-market-service/models"
	"metaid-market-service/service/common_service"
	"metaid-market-service/service/mrc20_service"
	"metaid-market-service/tool"
	"strconv"
	"strings"
)

func Mrc20MintPre(req *request.Mrc20MintPreRequest, publicKey, ip string) (*respond.Mrc20MintPreResp, error) {
	var (
		orderId   string = ""
		mintOrder *models.Mrc20MintOrderModel
		err       error

		totalFee   int64 = 0
		minerFee   int64 = 0
		serviceFee int64 = 0

		mrc20Builder   *mrc20_service.Mrc20Builder
		mrc20OpRequest *mrc20_service.Mrc20OpRequest
		tickId         string                   = req.TickerId
		payload        string                   = fmt.Sprintf(`{"id":"%s"}`, tickId)
		pinUtxoIds     string                   = ""
		mintPinsStr    string                   = ""
		mintPins       []*mrc20_service.MintPin = make([]*mrc20_service.MintPin, 0)
		payTos         []*mrc20_service.PayTo   = make([]*mrc20_service.PayTo, 0)
		//payToAddress        string                   = ""
		//payToAmount         int64                    = 0
		mrc20OutValue       int64    = req.OutValue
		mrc20OutAddressList []string = []string{req.OutAddress, req.OutAddress}
		changeAddress       string   = req.OutAddress
		feeRate             int64    = req.NetworkFeeRate
		revealPrePsbtRaw    string   = ""
		revealAddress       string   = ""
		revealInputIndex    int64    = 0

		nowTime int64 = tool.MakeTimestamp()

		extra map[string]interface{} = make(map[string]interface{})

		tickInfo *common_service.TickInfo
	)

	tickInfo, err = common_service.GetMrc20TickInfo(tickId, "")
	if err != nil {
		return nil, err
	}
	if tickInfo.PayCheck != nil && tickInfo.PayCheck.PayTo != "" && tickInfo.PayCheck.PayAmount != "" {
		//payToAddress = tickInfo.PayCheck.PayTo
		payTo := &mrc20_service.PayTo{
			Address: tickInfo.PayCheck.PayTo,
		}
		payTo.Amount, _ = strconv.ParseInt(tickInfo.PayCheck.PayAmount, 10, 64)
		if payTo.Amount <= 546 {
			payTo.Amount = 546
		}
		//payToAmount = payTo.Amount
		payTos = append(payTos, payTo)
	}

	for _, pin := range req.MintPins {
		if pin.PinUtxoTxId == "" || pin.PinUtxoOutValue <= 0 {
			return nil, errors.New("mint pin parameter error")
		}
		mintPinsStr += "," + pin.PinId
		pinUtxoIds += pinUtxoIds + "," + fmt.Sprintf("%s:%d", pin.PinUtxoTxId, pin.PinUtxoIndex)
		mintPin := &mrc20_service.MintPin{
			PinId:           pin.PinId,
			PinUtxoTxId:     pin.PinUtxoTxId,
			PinUtxoIndex:    pin.PinUtxoIndex,
			PinUtxoOutValue: pin.PinUtxoOutValue,
			Address:         pin.Address,
			PkScript:        pin.PkScript,
		}
		addressClass, err := common.CheckAddressClass(common.GetNetParams(conf.Net), pin.Address)
		if err != nil {
			return nil, err
		}
		if addressClass == txscript.PubKeyHashTy {
			mintPin.OutRaw, err = common.FetchTxHex(pin.PinUtxoTxId)
			if err != nil {
				return nil, err
			}
			//fmt.Printf("mintPin.OutRaw: %s\n", mintPin.OutRaw)
			//mintPin.PkScript = ""
		}
		mintPins = append(mintPins, mintPin)
		revealInputIndex++
	}
	payload = fmt.Sprintf(`{"id":"%s", "vout":"%d"}`, tickId, revealInputIndex+1)
	mintPinsStr = strings.Trim(mintPinsStr, ",")
	mrc20OpRequest = &mrc20_service.Mrc20OpRequest{
		Net:                 common.GetNetParams(conf.Net),
		MetaIdFlag:          common.GetMetaIdFlag(),
		Op:                  "mint",
		OpPayload:           payload,
		MintPins:            mintPins,
		PayTos:              payTos,
		TransferMrc20s:      nil,
		Mrc20OutValue:       mrc20OutValue,
		Mrc20OutAddressList: mrc20OutAddressList,
		ChangeAddress:       changeAddress,
	}

	mrc20Builder, minerFee, err = mrc20_service.Mrc20MintBuilder(mrc20OpRequest, feeRate)
	if err != nil {
		return nil, err
	}
	revealPrePsbtRaw, err = mrc20Builder.RevealPsbtBuilder.ToString()
	if err != nil {
		return nil, err
	}
	revealAddress, err = common.PkScriptToAddress(conf.Net, hex.EncodeToString(mrc20Builder.TxCtxData.CommitTxAddressPkScript))
	if err != nil {
		return nil, err
	}
	totalFee = minerFee + serviceFee

	orderId = fmt.Sprintf("mrc20_mint_%s_%s_%s_%d", req.TickerId, req.OutAddress, pinUtxoIds, nowTime)
	orderId = hex.EncodeToString(tool.SHA256([]byte(orderId)))

	mintOrder, err = models.Mrc20MintOrderModelDao().GetOne(&models.Mrc20MintOrderModel{OrderId: orderId})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if mintOrder != nil && (mintOrder.InscribeState == models.InscribeStateFinish) {
		return nil, errors.New("mint order already exists")
	}
	if mintOrder == nil {
		mintOrder = &models.Mrc20MintOrderModel{
			OrderId:             orderId,
			InscribeState:       models.InscribeStatePending,
			TickId:              tickId,
			MintPins:            mintPinsStr,
			TotalFee:            totalFee,
			MinerFee:            minerFee,
			ServiceFee:          serviceFee,
			RedeemScript:        hex.EncodeToString(mrc20Builder.TxCtxData.InscriptionScript),
			ControlBlockWitness: hex.EncodeToString(mrc20Builder.TxCtxData.ControlBlockWitness),
			RevealTxPrivateKey:  mrc20Builder.TxCtxData.RecoveryPrivateKeyHex,
			RevealTxAddress:     revealAddress,
			CommitTxRaw:         "",
			RevealOutValue:      mrc20OutValue,
			RevealInputIndex:    revealInputIndex,
			RevealPrePsbtRaw:    revealPrePsbtRaw,
			RevealMidPsbtRaw:    "",
			RevealFinalPsbtRaw:  "",
			CommitTxId:          "",
			TxId:                "",
			BlockHeight:         0,
			ConfirmationState:   models.ConfirmationStateUnconfirmed,
			Timestamp:           nowTime,
			Version:             0,
			CreateTime:          nowTime,
			UpdateTime:          0,
			State:               models.STATE_EXIST,
		}
		err = models.Mrc20MintOrderModelDao().Set(mintOrder)
		if err != nil {
			return nil, err
		}
	} else {
		mintOrder.MintPins = mintPinsStr
		mintOrder.TotalFee = totalFee
		mintOrder.MinerFee = minerFee
		mintOrder.ServiceFee = serviceFee
		mintOrder.RedeemScript = hex.EncodeToString(mrc20Builder.TxCtxData.InscriptionScript)
		mintOrder.ControlBlockWitness = hex.EncodeToString(mrc20Builder.TxCtxData.ControlBlockWitness)
		mintOrder.RevealOutValue = mrc20OutValue
		mintOrder.RevealTxPrivateKey = mrc20Builder.TxCtxData.RecoveryPrivateKeyHex
		mintOrder.RevealTxAddress = revealAddress
		mintOrder.RevealInputIndex = revealInputIndex
		mintOrder.RevealPrePsbtRaw = revealPrePsbtRaw
		mintOrder.UpdateTime = nowTime
		err = models.Mrc20MintOrderModelDao().Update(mintOrder)
		if err != nil {
			return nil, err
		}
	}

	return &respond.Mrc20MintPreResp{
		OrderId:          mintOrder.OrderId,
		TotalFee:         mintOrder.TotalFee,
		RevealFee:        mintOrder.MinerFee,
		RevealAddress:    mintOrder.RevealTxAddress,
		ServiceFee:       mintOrder.ServiceFee,
		ServiceAddress:   "",
		RevealPrePsbtRaw: mintOrder.RevealPrePsbtRaw,
		RevealInputIndex: revealInputIndex,
		Extra:            extra,
	}, nil
}

func Mrc20MintCommit(req *request.Mrc20MintCommitRequest, publicKey, ip string) (*respond.Mrc20MintCommitResp, error) {
	var (
		orderId      string = req.OrderId
		commitTxRaw  string = req.CommitTxRaw
		commitTxId   string = ""
		revealTxRaw  string = ""
		prePsbtRaw   string = req.RevealPrePsbtRaw
		finalPsbtRaw string = ""

		address string = ""

		txId      string = ""
		mintOrder *models.Mrc20MintOrderModel
		err       error

		txRawByte        []byte
		commitTx         *wire.MsgTx = wire.NewMsgTx(2)
		commitTxOutIndex int64       = req.CommitTxOutIndex
		revealTx         *wire.MsgTx = wire.NewMsgTx(2)
		revealTxRawByte  []byte

		psbtBuilder          *common.PsbtBuilder
		taprootInSigner      *common.InputSign
		revealTaprootSigners []*common.InputSign = make([]*common.InputSign, 0)
		pkScript             string              = ""

		nowTime int64 = tool.MakeTimestamp()
	)

	mintOrder, err = models.Mrc20MintOrderModelDao().GetOne(&models.Mrc20MintOrderModel{OrderId: orderId})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if mintOrder == nil {
		return nil, errors.New("mint order not exists")

	}
	if mintOrder.InscribeState != models.InscribeStatePending {
		return nil, errors.New("mint order state error")
	}
	txRawByte, _ = hex.DecodeString(commitTxRaw)
	err = commitTx.Deserialize(bytes.NewReader(txRawByte))
	if err != nil {
		return nil, err
	}
	commitTxId = commitTx.TxHash().String()
	if len(commitTx.TxOut) <= int(commitTxOutIndex) {
		return nil, errors.New("commitTxOutIndex error")
	}

	txIn := commitTx.TxIn[0]
	utxoInfo := common.GetUtxoInfo(conf.Net, txIn.PreviousOutPoint.Hash.String(), int64(txIn.PreviousOutPoint.Index))
	if utxoInfo == nil {
		return nil, errors.New("commitTx utxoInfo not exists")
	}
	address = utxoInfo.Address

	psbtBuilder, err = common.NewPsbtBuilder(common.GetNetParams(conf.Net), prePsbtRaw)
	if err != nil {
		return nil, err
	}
	pkScript, err = common.AddressToPkScript(conf.Net, mintOrder.RevealTxAddress)
	taprootInSigner = &common.InputSign{
		UtxoType: common.Taproot,
		Index:    int(mintOrder.RevealInputIndex),
		//OutRaw:         "",
		PkScript:            pkScript,
		RedeemScript:        mintOrder.RedeemScript,
		ControlBlockWitness: mintOrder.ControlBlockWitness,
		Amount:              uint64(mintOrder.MinerFee),
		SighashType:         txscript.SigHashAll,
		PriHex:              mintOrder.RevealTxPrivateKey,
		//MultiSigScript: "",
		//PreSigScript:   "",
	}
	revealTaprootSigners = append(revealTaprootSigners, taprootInSigner)
	err = psbtBuilder.UpdateAndSignTaprootInput(revealTaprootSigners)
	if err != nil {
		return nil, err
	}
	finalPsbtRaw, err = psbtBuilder.ToString()
	if err != nil {
		return nil, err
	}

	revealTxRaw, err = psbtBuilder.ExtractPsbtTransaction()
	if err != nil {
		return nil, err
	}
	revealTxRawByte, _ = hex.DecodeString(revealTxRaw)
	err = revealTx.Deserialize(bytes.NewReader(revealTxRawByte))
	if err != nil {
		return nil, err
	}
	txId = revealTx.TxHash().String()

	mintOrder.Address = address
	mintOrder.CommitTxRaw = commitTxRaw
	mintOrder.RevealMidPsbtRaw = prePsbtRaw
	mintOrder.RevealFinalPsbtRaw = finalPsbtRaw
	mintOrder.CommitTxId = commitTxId
	mintOrder.TxId = txId
	mintOrder.InscribeState = models.InscribeStatePaid
	mintOrder.UpdateTime = nowTime
	err = models.Mrc20MintOrderModelDao().Update(mintOrder)
	if err != nil {
		return nil, err
	}

	err = models.Mrc20MintOrderModelDao().UpdateEntityListForInscribing(mintOrder, []string{commitTxRaw, revealTxRaw}, common.BroadcastTx)
	if err != nil {
		return nil, err
	}
	return &respond.Mrc20MintCommitResp{
		OrderId:    mintOrder.OrderId,
		CommitTxId: commitTxId,
		RevealTxId: txId,
	}, nil
}
