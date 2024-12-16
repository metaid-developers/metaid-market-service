package metaname_service

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
	"metaid-market-service/service/mrc20_service"
	"metaid-market-service/tool"
	"strings"
)

func RegisterMetaNamePre(req *request.RegisterMetaNamePreRequest, publicKey, ip string) (*respond.RegisterMetaNamePreResp, error) {
	var (
		net                   string = conf.Net
		receiveAddress        string = req.Address
		registerAddress       string = req.Address
		changeAddress         string = req.Address
		metanameRegisterOrder *models.MetanameRegisterOrderModel
		err                   error
		orderId               string = ""
		nowTime               int64  = tool.MakeTimestamp()

		totalFee       int64  = 0
		minerFee       int64  = 0
		minerGas       int64  = 0
		minerOutValue  int64  = 0
		serviceFee     int64  = 0
		serviceAddress string = ""

		metanameStrs []string
		metaname     string                 = req.MetaName
		name         string                 = ""
		namespace    string                 = ""
		metaDataMap  map[string]interface{} = map[string]interface{}{
			"name":     "",
			"rev":      receiveAddress,
			"relay":    "",
			"metadata": "",
		}
		path string = "/metaname/" + namespace

		metaIdInscribeBuilder *mrc20_service.MetaIdBuilder
		metaIdOpRequest       *mrc20_service.MetaIdInscribeRequest
		otherOuts             []*mrc20_service.OtherOut = make([]*mrc20_service.OtherOut, 0)
		pinOutValue           int64                     = 546
		feeRate               int64                     = req.NetworkFeeRate
		revealPrePsbtRaw      string                    = ""
		revealAddress         string                    = ""
		revealInputIndex      int64                     = 0
	)
	metanameStrs = strings.Split(metaname, ".")
	if len(metanameStrs) != 2 {
		return nil, errors.New("metaname error")
	}
	name = metanameStrs[0]
	namespace = metanameStrs[1]

	metaDataMap["name"] = name
	path = fmt.Sprintf("/metaname/%s", namespace)

	serviceFee, serviceAddress = common.GetPlatformServiceFeeConfigData().MetaIdInscribeFee, common.GetPlatformServiceFeeConfigData().ServiceAddress
	if serviceFee >= 546 {
		otherOuts = append(otherOuts, &mrc20_service.OtherOut{
			Address: serviceAddress,
			Amount:  serviceFee,
		})
	}
	_ = serviceAddress

	verified, err := common.CheckPublicKeyAddress(common.GetNetParams(net), publicKey, registerAddress)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Check address err: %s. ", err.Error()))
	}
	if !verified {
		return nil, errors.New(fmt.Sprintf("Check address verified: %v. ", verified))
	}

	//check if metaname is exist
	metanameRegisterOrder, err = models.MetanameRegisterOrderModelDao().GetOne(&models.MetanameRegisterOrderModel{
		Metaname:        metaname,
		RegisterAddress: registerAddress,
		InscribeState:   models.InscribeStateFinish,
	})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if metanameRegisterOrder != nil {
		return nil, errors.New("metaname has been registered")
	}

	metaIdOpRequest = &mrc20_service.MetaIdInscribeRequest{
		Net:           common.GetNetParams(net),
		MetaIdFlag:    common.GetMetaIdFlag(),
		Path:          path,
		Payload:       tool.AnyToStr(metaDataMap),
		PinOutValue:   pinOutValue,
		PinOutAddress: receiveAddress,
		ChangeAddress: changeAddress,
		OtherOuts:     otherOuts,
	}

	metaIdInscribeBuilder, minerFee, err = mrc20_service.MetaIdInscribeBuilder(metaIdOpRequest, feeRate)
	if err != nil {
		return nil, err
	}
	revealPrePsbtRaw, err = metaIdInscribeBuilder.RevealPsbtBuilder.ToString()
	if err != nil {
		return nil, err
	}
	revealAddress, err = common.PkScriptToAddress(net, hex.EncodeToString(metaIdInscribeBuilder.TxCtxData.CommitTxAddressPkScript))
	if err != nil {
		return nil, err
	}
	totalFee = minerFee
	minerGas = minerFee - minerOutValue - serviceFee

	orderId = fmt.Sprintf("metaname_register_%s_%s", registerAddress, metaname)
	orderId = hex.EncodeToString(tool.SHA256([]byte(orderId)))

	metanameRegisterOrder, err = models.MetanameRegisterOrderModelDao().GetOne(&models.MetanameRegisterOrderModel{OrderId: orderId})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if metanameRegisterOrder != nil && metanameRegisterOrder.InscribeState == models.InscribeStateFinish {
		return nil, errors.New("register order already exists")
	}
	if metanameRegisterOrder == nil {
		metanameRegisterOrder = &models.MetanameRegisterOrderModel{
			OrderId:             orderId,
			InscribeState:       models.InscribeStatePending,
			RegisterAddress:     registerAddress,
			ReceiveAddress:      receiveAddress,
			Metaname:            metaname,
			Name:                name,
			Namespace:           namespace,
			Payload:             tool.AnyToStr(metaDataMap),
			Chain:               "btc",
			NetworkFeeRate:      feeRate,
			TotalFee:            totalFee,
			MinerFee:            minerFee,
			ServiceFee:          serviceFee,
			RedeemScript:        hex.EncodeToString(metaIdInscribeBuilder.TxCtxData.InscriptionScript),
			ControlBlockWitness: hex.EncodeToString(metaIdInscribeBuilder.TxCtxData.ControlBlockWitness),
			RevealTxPrivateKey:  metaIdInscribeBuilder.TxCtxData.RecoveryPrivateKeyHex,
			RevealTxAddress:     revealAddress,
			RevealInputIndex:    revealInputIndex,
			RevealPrePsbtRaw:    revealPrePsbtRaw,
			RevealFinalPsbtRaw:  "",
			CommitTxRaw:         "",
			RevealTxRaw:         "",
			CommitTxId:          "",
			RevealTxId:          "",
			TxId:                "",
			BlockHeight:         0,
			ConfirmationState:   models.ConfirmationStateUnconfirmed,
			Timestamp:           nowTime,
			Version:             0,
			CreateTime:          nowTime,
			UpdateTime:          0,
			State:               models.STATE_EXIST,
		}
		err = models.MetanameRegisterOrderModelDao().Set(metanameRegisterOrder)
		if err != nil {
			return nil, err
		}
	} else {
		metanameRegisterOrder.TotalFee = totalFee
		metanameRegisterOrder.MinerFee = minerFee
		metanameRegisterOrder.ServiceFee = serviceFee
		metanameRegisterOrder.RedeemScript = hex.EncodeToString(metaIdInscribeBuilder.TxCtxData.InscriptionScript)
		metanameRegisterOrder.ControlBlockWitness = hex.EncodeToString(metaIdInscribeBuilder.TxCtxData.ControlBlockWitness)
		metanameRegisterOrder.RevealTxPrivateKey = metaIdInscribeBuilder.TxCtxData.RecoveryPrivateKeyHex
		metanameRegisterOrder.RevealTxAddress = revealAddress
		metanameRegisterOrder.RevealInputIndex = revealInputIndex
		metanameRegisterOrder.RevealPrePsbtRaw = revealPrePsbtRaw
		metanameRegisterOrder.UpdateTime = nowTime
		err = models.MetanameRegisterOrderModelDao().Update(metanameRegisterOrder)
		if err != nil {
			return nil, err
		}
	}

	return &respond.RegisterMetaNamePreResp{
		OrderId:        metanameRegisterOrder.OrderId,
		TotalFee:       metanameRegisterOrder.TotalFee,
		ReceiveAddress: metanameRegisterOrder.RevealTxAddress,
		MinerFee:       metanameRegisterOrder.MinerFee,
		MinerGas:       minerGas,
		MinerOutValue:  minerOutValue,
		ServiceFee:     metanameRegisterOrder.ServiceFee,
		MetaName:       metanameRegisterOrder.Metaname,
	}, nil
}

func RegisterMetaNameCommit(req *request.RegisterMetaNameCommitRequest, publicKey, ip string) (*respond.RegisterMetaNameCommitResp, error) {
	var (
		net          string = conf.Net
		orderId      string = req.OrderId
		commitTxRaw  string = req.CommitTxRaw
		commitTxId   string = ""
		revealTxRaw  string = ""
		finalPsbtRaw string = ""

		address string = ""

		txId                  string = ""
		metanameRegisterOrder *models.MetanameRegisterOrderModel
		err                   error

		txRawByte        []byte
		commitTx         *wire.MsgTx = wire.NewMsgTx(2)
		commitTxOutIndex int64       = req.CommitTxOutIndex
		revealTx         *wire.MsgTx = wire.NewMsgTx(2)
		revealTxRawByte  []byte

		prePsbtRaw           string = ""
		psbtBuilder          *common.PsbtBuilder
		taprootInSigner      *common.InputSign
		revealTaprootSigners []*common.InputSign = make([]*common.InputSign, 0)
		pkScript             string              = ""

		txRawList []string = []string{commitTxRaw}

		nowTime int64 = tool.MakeTimestamp()
	)

	metanameRegisterOrder, err = models.MetanameRegisterOrderModelDao().GetOne(&models.MetanameRegisterOrderModel{OrderId: orderId})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if metanameRegisterOrder == nil {
		return nil, errors.New("deploy order not exists")
	}
	if metanameRegisterOrder.InscribeState != models.InscribeStatePending {
		return nil, errors.New("deploy order state error")
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
	utxoInfo := common.GetUtxoInfo(net, txIn.PreviousOutPoint.Hash.String(), int64(txIn.PreviousOutPoint.Index))
	if utxoInfo == nil {
		return nil, errors.New("commitTx utxoInfo not exists")
	}
	address = utxoInfo.Address
	if address != metanameRegisterOrder.RegisterAddress {
		return nil, errors.New("commitTx address not match")
	}

	prePsbtRaw = metanameRegisterOrder.RevealPrePsbtRaw
	if prePsbtRaw == "" {
		return nil, errors.New("prePsbtRaw is empty")
	}

	psbtBuilder, err = common.NewPsbtBuilder(common.GetNetParams(net), prePsbtRaw)
	if err != nil {
		return nil, err
	}

	txHash := commitTx.TxHash()
	commitPreOutPoint := wire.NewOutPoint(&txHash, uint32(commitTxOutIndex))
	psbtBuilder.PsbtUpdater.Upsbt.UnsignedTx.TxIn[metanameRegisterOrder.RevealInputIndex].PreviousOutPoint = *commitPreOutPoint

	pkScript, err = common.AddressToPkScript(net, metanameRegisterOrder.RevealTxAddress)
	taprootInSigner = &common.InputSign{
		UtxoType: common.Taproot,
		Index:    int(metanameRegisterOrder.RevealInputIndex),
		//OutRaw:         "",
		PkScript:            pkScript,
		RedeemScript:        metanameRegisterOrder.RedeemScript,
		ControlBlockWitness: metanameRegisterOrder.ControlBlockWitness,
		Amount:              uint64(metanameRegisterOrder.MinerFee),
		SighashType:         txscript.SigHashAll,
		PriHex:              metanameRegisterOrder.RevealTxPrivateKey,
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
	txRawList = append(txRawList, revealTxRaw)

	metanameRegisterOrder.RevealTxRaw = revealTxRaw
	metanameRegisterOrder.CommitTxRaw = commitTxRaw
	metanameRegisterOrder.RevealFinalPsbtRaw = finalPsbtRaw
	metanameRegisterOrder.CommitTxId = commitTxId
	metanameRegisterOrder.TxId = txId
	metanameRegisterOrder.PinId = fmt.Sprintf("%si0", txId)
	metanameRegisterOrder.InscribeState = models.InscribeStatePaid
	metanameRegisterOrder.UpdateTime = nowTime

	err = models.MetanameRegisterOrderModelDao().Update(metanameRegisterOrder)
	if err != nil {
		return nil, err
	}

	err = models.MetanameRegisterOrderModelDao().SaveEntityForInscribing(metanameRegisterOrder, txRawList, common.BroadcastTx)
	if err != nil {
		return nil, err
	}
	return &respond.RegisterMetaNameCommitResp{
		OrderId:    metanameRegisterOrder.OrderId,
		CommitTxId: commitTxId,
		RevealTxId: txId,
		PinId:      metanameRegisterOrder.PinId,
		TxId:       txId,
		MetaName:   metanameRegisterOrder.Metaname,
	}, nil
}
