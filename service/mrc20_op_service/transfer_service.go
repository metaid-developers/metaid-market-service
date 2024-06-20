package mrc20_op_service

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"gorm.io/gorm"
	"metaid-market-service/common"
	"metaid-market-service/conf"
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
	"metaid-market-service/models"
	"metaid-market-service/service/mrc20_service"
	"metaid-market-service/tool"
)

func Mrc20TransferPre(req *request.Mrc20TransferPreRequest, publicKey, ip string) (*respond.Mrc20TransferPreResp, error) {
	var (
		orderId       string = ""
		transferOrder *models.Mrc20TransferOrderModel
		err           error

		totalFee   int64 = 0
		minerFee   int64 = 0
		serviceFee int64 = 0

		mrc20Builder      *mrc20_service.Mrc20Builder
		mrc20OpRequest    *mrc20_service.Mrc20OpRequest
		tickerId          string                         = req.TickerId
		payload           string                         = ""
		mrc20UtxoIds      string                         = ""
		mrc20OutAddresses string                         = ""
		transferMrc20s    []*mrc20_service.TransferMrc20 = make([]*mrc20_service.TransferMrc20, 0)
		mrc20Outs         []*mrc20_service.Mrc20OutInfo  = make([]*mrc20_service.Mrc20OutInfo, 0)
		changeAddress     string                         = req.ChangeAddress
		feeRate           int64                          = req.NetworkFeeRate
		revealPrePsbtRaw  string                         = ""
		revealAddress     string                         = ""
		revealInputIndex  int64                          = 0

		nowTime int64                  = tool.MakeTimestamp()
		extra   map[string]interface{} = make(map[string]interface{})
	)
	for _, v := range req.Transfers {
		if v.Amount == "" || v.UtxoOutValue <= 0 || v.UtxoTxId == "" {
			return nil, errors.New("transfer request parameter error")
		}
		transferMrc20 := &mrc20_service.TransferMrc20{
			Address:       v.Address,
			PkScript:      v.PkScript,
			UtxoTxId:      v.UtxoTxId,
			UtxoIndex:     v.UtxoIndex,
			UtxoOutValue:  v.UtxoOutValue,
			Mrc20Amount:   v.Amount,
			Mrc20TickerId: v.TickerId,
		}
		addressClass, err := common.CheckAddressClass(common.GetNetParams(conf.Net), v.Address)
		if err != nil {
			return nil, err
		}
		if addressClass == txscript.PubKeyHashTy {
			transferMrc20.OutRaw, err = common.FetchTxHex(v.UtxoTxId)
			if err != nil {
				return nil, err
			}
			fmt.Printf("transferMrc20.OutRaw: %s\n", transferMrc20.OutRaw)
			//mintPin.PkScript = ""
		}
		transferMrc20s = append(transferMrc20s, transferMrc20)
		mrc20UtxoIds += v.UtxoTxId + "_"
		revealInputIndex++
	}
	for _, v := range req.Mrc20Outs {
		mrc20Outs = append(mrc20Outs, &mrc20_service.Mrc20OutInfo{
			Amount:   v.Amount,
			Address:  v.Address,
			PkScript: v.PkScript,
			OutValue: v.OutValue,
		})
		mrc20OutAddresses += v.Address + "_"
	}
	payload, err = mrc20_service.MakeTransferPayload(tickerId, transferMrc20s, mrc20Outs)
	if err != nil {
		return nil, err
	}
	mrc20OpRequest = &mrc20_service.Mrc20OpRequest{
		Net:                 common.GetNetParams(conf.Net),
		MetaIdFlag:          common.GetMetaIdFlag(),
		Op:                  "transfer",
		OpPayload:           payload,
		MintPins:            nil,
		TransferMrc20s:      transferMrc20s,
		Mrc20OutValue:       0,
		Mrc20OutAddressList: nil,
		ChangeAddress:       changeAddress,
		Mrc20Outs:           mrc20Outs,
	}

	mrc20Builder, minerFee, err = mrc20_service.Mrc20TransferBuilder(mrc20OpRequest, feeRate)
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

	orderId = fmt.Sprintf("mrc20_transfer_%s_%s_%s_%d", req.TickerId, mrc20OutAddresses, mrc20UtxoIds, nowTime)
	orderId = hex.EncodeToString(tool.SHA256([]byte(orderId)))

	transferOrder, err = models.Mrc20TransferOrderModelDao().GetOne(&models.Mrc20TransferOrderModel{OrderId: orderId})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if transferOrder != nil && (transferOrder.InscribeState == models.InscribeStateFinish) {
		return nil, errors.New("transfer order already exists")
	}
	if transferOrder == nil {
		transferOrder = &models.Mrc20TransferOrderModel{
			OrderId:             orderId,
			Payload:             payload,
			InscribeState:       models.InscribeStatePending,
			TicketId:            tickerId,
			TotalFee:            totalFee,
			MinerFee:            minerFee,
			ServiceFee:          serviceFee,
			RedeemScript:        hex.EncodeToString(mrc20Builder.TxCtxData.InscriptionScript),
			ControlBlockWitness: hex.EncodeToString(mrc20Builder.TxCtxData.ControlBlockWitness),
			RevealTxPrivateKey:  mrc20Builder.TxCtxData.RecoveryPrivateKeyHex,
			RevealTxAddress:     revealAddress,
			CommitTxRaw:         "",
			RevealOutValue:      0,
			RevealInputIndex:    revealInputIndex,
			RevealPrePsbtRaw:    revealPrePsbtRaw,
			RevealMidPsbtRaw:    "",
			RevealFinalPsbtRaw:  "",
			CommitTxId:          "",
			TxId:                "",
			BlockHeight:         0,
			ConfirmationState:   models.ConfirmationStateUnconfirmed,
			Timestamp:           0,
			Version:             nowTime,
			CreateTime:          nowTime,
			UpdateTime:          0,
			State:               models.STATE_EXIST,
		}
		err = models.Mrc20TransferOrderModelDao().Set(transferOrder)
		if err != nil {
			return nil, err
		}
	} else {
		transferOrder.Payload = payload
		transferOrder.TotalFee = totalFee
		transferOrder.MinerFee = minerFee
		transferOrder.ServiceFee = serviceFee
		transferOrder.RedeemScript = hex.EncodeToString(mrc20Builder.TxCtxData.InscriptionScript)
		transferOrder.ControlBlockWitness = hex.EncodeToString(mrc20Builder.TxCtxData.ControlBlockWitness)
		transferOrder.RevealOutValue = 0
		transferOrder.RevealTxPrivateKey = mrc20Builder.TxCtxData.RecoveryPrivateKeyHex
		transferOrder.RevealTxAddress = revealAddress
		transferOrder.RevealInputIndex = revealInputIndex
		transferOrder.RevealPrePsbtRaw = revealPrePsbtRaw
		transferOrder.UpdateTime = nowTime
		err = models.Mrc20TransferOrderModelDao().Update(transferOrder)
		if err != nil {
			return nil, err
		}
	}

	privateKeyBytes, err := hex.DecodeString(transferOrder.RevealTxPrivateKey)
	if err != nil {
		return nil, err
	}
	privateKey, _ := btcec.PrivKeyFromBytes(privateKeyBytes)
	xOnlyPubKey := schnorr.SerializePubKey(privateKey.PubKey())
	extra["redeemScript"] = transferOrder.RedeemScript
	extra["controlBlockWitness"] = transferOrder.ControlBlockWitness
	extra["xOnlyPubKey"] = hex.EncodeToString(xOnlyPubKey)

	return &respond.Mrc20TransferPreResp{
		OrderId:          transferOrder.OrderId,
		TotalFee:         transferOrder.TotalFee,
		RevealFee:        transferOrder.MinerFee,
		RevealAddress:    transferOrder.RevealTxAddress,
		ServiceFee:       transferOrder.ServiceFee,
		ServiceAddress:   "",
		RevealPrePsbtRaw: transferOrder.RevealPrePsbtRaw,
		RevealInputIndex: revealInputIndex,
		Extra:            extra,
	}, nil
}

func Mrc20TransferCommit(req *request.Mrc20TransferCommitRequest, publicKey, ip string) (*respond.Mrc20TransferCommitResp, error) {
	var (
		orderId      string = req.OrderId
		commitTxRaw  string = req.CommitTxRaw
		commitTxId   string = ""
		revealTxRaw  string = ""
		prePsbtRaw   string = req.RevealPrePsbtRaw
		finalPsbtRaw string = ""

		txId          string = ""
		transferOrder *models.Mrc20TransferOrderModel
		err           error

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

	transferOrder, err = models.Mrc20TransferOrderModelDao().GetOne(&models.Mrc20TransferOrderModel{OrderId: orderId})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if transferOrder == nil {
		return nil, errors.New("transfer order not exists")

	}
	if transferOrder.InscribeState != models.InscribeStatePending {
		return nil, errors.New("transfer order state error")
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

	psbtBuilder, err = common.NewPsbtBuilder(common.GetNetParams(conf.Net), prePsbtRaw)
	if err != nil {
		return nil, err
	}
	pkScript, err = common.AddressToPkScript(conf.Net, transferOrder.RevealTxAddress)
	taprootInSigner = &common.InputSign{
		UtxoType: common.Taproot,
		Index:    int(transferOrder.RevealInputIndex),
		//OutRaw:         "",
		PkScript:            pkScript,
		RedeemScript:        transferOrder.RedeemScript,
		ControlBlockWitness: transferOrder.ControlBlockWitness,
		Amount:              uint64(transferOrder.MinerFee),
		SighashType:         txscript.SigHashAll,
		PriHex:              transferOrder.RevealTxPrivateKey,
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

	transferOrder.CommitTxRaw = commitTxRaw
	transferOrder.RevealMidPsbtRaw = prePsbtRaw
	transferOrder.RevealFinalPsbtRaw = finalPsbtRaw
	transferOrder.CommitTxId = commitTxId
	transferOrder.TxId = txId
	transferOrder.InscribeState = models.InscribeStatePaid
	transferOrder.UpdateTime = nowTime
	err = models.Mrc20TransferOrderModelDao().Update(transferOrder)
	if err != nil {
		return nil, err
	}

	err = models.Mrc20TransferOrderModelDao().UpdateEntityListForInscribing(transferOrder, []string{commitTxRaw, revealTxRaw}, common.BroadcastTx)
	if err != nil {
		return nil, err
	}
	return &respond.Mrc20TransferCommitResp{
		OrderId:    transferOrder.OrderId,
		CommitTxId: commitTxId,
		RevealTxId: txId,
	}, nil
}
