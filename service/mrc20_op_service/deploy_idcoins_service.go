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
	"metaid-market-service/controller/auth"
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
	"metaid-market-service/models"
	"metaid-market-service/service/mrc20_service"
	"metaid-market-service/service/orders_exchange_service"
	"metaid-market-service/service/wallet_service"
	"metaid-market-service/tool"
)

func BuildIdCoinsPre(req *request.BuildIdCoinsPreRequest, publicKey, ip string) (*respond.BuildIdCoinsPreResp, error) {
	return buildIdCoinsPreFromOrders(req, publicKey, ip)
}

func BuildIdCoinsCommit(req *request.BuildIdCoinsCommitRequest, publicKey, ip string) (*respond.BuildIdCoinsCommitResp, error) {
	return buildIdCoinsCommitFromOrder(req, publicKey, ip)
}

func buildIdCoinsPreFromOrders(req *request.BuildIdCoinsPreRequest, publicKey, ip string) (*respond.BuildIdCoinsPreResp, error) {
	var (
		headers map[string]string = map[string]string{
			"X-Public-Key": publicKey,
		}

		reqOrders *orders_exchange_service.BuildIdCoinsPreRequest = &orders_exchange_service.BuildIdCoinsPreRequest{
			Tick:             req.Tick,
			TokenName:        req.TokenName,
			IssuerMetaId:     req.IssuerMetaId,
			IssuerAddress:    req.IssuerAddress,
			IssuerSign:       req.IssuerSign,
			Message:          req.Message,
			FollowersNum:     req.FollowersNum,
			AmountPerMint:    req.AmountPerMint,
			LiquidityPerMint: req.LiquidityPerMint,
			NetworkFeeRate:   req.NetworkFeeRate,
		}
		respOrders *orders_exchange_service.BuildIdCoinsPreResp
		err        error
	)
	respOrders, err = orders_exchange_service.BuildIdCoinsPre(reqOrders, headers)
	if err != nil {
		return nil, err
	}
	return &respond.BuildIdCoinsPreResp{
		OrderId:        respOrders.OrderId,
		TotalFee:       respOrders.TotalFee,
		ReceiveAddress: respOrders.ReceiveAddress,
		MinerFee:       respOrders.MinerFee,
		MinerGas:       respOrders.MinerGas,
		MinerOutValue:  respOrders.MinerOutValue,
		ServiceFee:     respOrders.ServiceFee,
	}, nil
}

func buildIdCoinsCommitFromOrder(req *request.BuildIdCoinsCommitRequest, publicKey, ip string) (*respond.BuildIdCoinsCommitResp, error) {
	var (
		headers map[string]string = map[string]string{
			"X-Public-Key": publicKey,
		}

		reqOrders *orders_exchange_service.BuildIdCoinsCommitRequest = &orders_exchange_service.BuildIdCoinsCommitRequest{
			OrderId:          req.OrderId,
			CommitTxRaw:      req.CommitTxRaw,
			CommitTxOutIndex: req.CommitTxOutIndex,
		}
		respOrders *orders_exchange_service.BuildIdCoinsCommitResp
		err        error
	)
	respOrders, err = orders_exchange_service.BuildIdCoinsCommit(reqOrders, headers)
	if err != nil {
		return nil, err
	}
	return &respond.BuildIdCoinsCommitResp{
		OrderId:    respOrders.OrderId,
		TickId:     respOrders.TickId,
		CommitTxId: respOrders.CommitTxId,
		RevealTxId: respOrders.RevealTxId,
		PinId:      respOrders.PinId,
		TxId:       respOrders.TxId,
	}, nil
}

func buildIdCoinsPreFromSelf(req *request.BuildIdCoinsPreRequest, publicKey, ip string) (*respond.BuildIdCoinsPreResp, error) {
	var (
		idCoinsDeploy *models.IdCoinsDeployOrderModel
		err           error
		orderId       string = ""
		nowTime       int64  = tool.MakeTimestamp()

		totalFee   int64 = 0
		minerFee   int64 = 0
		serviceFee int64 = 0

		issuerMetaId     string                 = req.IssuerMetaId
		issuerAddress    string                 = req.IssuerAddress
		issuerSign       string                 = req.IssuerSign
		tick             string                 = req.Tick
		tokenName        string                 = req.TokenName
		amountPerMint    int64                  = req.AmountPerMint
		liquidityPerMint int64                  = req.LiquidityPerMint
		followersNum     int64                  = req.FollowersNum
		metaDataMap      map[string]interface{} = map[string]interface{}{
			//"issuerSign":  req.IssuerSign,
			"message":  req.Message,
			"tickSign": "",
		}

		mrc20Builder        *mrc20_service.Mrc20Builder
		mrc20OpRequest      *mrc20_service.Mrc20OpRequest
		deployPinOutAddress string = req.IssuerAddress
		payload             string = ""
		mrc20DeployData     *mrc20_service.Mrc20DeployData
		changeAddress       string = req.IssuerAddress
		mrc20OutValue       int64  = 546
		feeRate             int64  = req.NetworkFeeRate
		revealPrePsbtRaw    string = ""
		revealAddress       string = ""
		revealInputIndex    int64  = 0

		payToAddress   string = ""
		payToPublicKey string = ""

		signMsgPrivate, _ string = common.GetPlatformKeyForSignMsg()
		unsignMsg         string = ""
		signMsg           string = ""
	)
	payToPublicKey, payToAddress, err = getIdCoinsPoolKeyAndAddress(tick)
	if err != nil {
		return nil, err
	}

	unsignMsg = tick + payToPublicKey + payToAddress
	signMsg, err = auth.SignTextMessage(unsignMsg, signMsgPrivate)
	if err != nil {
		return nil, err
	}
	metaDataMap["tickSign"] = signMsg

	payload, mrc20DeployData, _ = mrc20_service.MakeDeployPayloadForIdCoins(
		tick, tokenName, issuerMetaId, tool.AnyToStr(metaDataMap), payToAddress, followersNum, amountPerMint, liquidityPerMint)

	mrc20OpRequest = &mrc20_service.Mrc20OpRequest{
		Net:                 common.GetNetParams(conf.Net),
		MetaIdFlag:          common.GetMetaIdFlag(),
		Op:                  "deploy",
		OpPayload:           payload,
		MintPins:            nil,
		TransferMrc20s:      nil,
		Mrc20Outs:           nil,
		Mrc20OutValue:       mrc20OutValue,
		Mrc20OutAddressList: nil,
		ChangeAddress:       changeAddress,
		DeployPinOutAddress: deployPinOutAddress,
	}

	mrc20Builder, minerFee, err = mrc20_service.Mrc20DeployBuilder(mrc20OpRequest, feeRate)
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

	orderId = fmt.Sprintf("id_coins_deploy_%s_%s_%s", deployPinOutAddress, tick, payToPublicKey)
	orderId = hex.EncodeToString(tool.SHA256([]byte(orderId)))

	idCoinsDeploy, err = models.IdCoinsDeployOrderModelDao().GetOne(&models.IdCoinsDeployOrderModel{OrderId: orderId})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if idCoinsDeploy != nil && idCoinsDeploy.InscribeState == models.InscribeStateFinish {
		return nil, errors.New("deploy order already exists")
	}
	if idCoinsDeploy == nil {
		idCoinsDeploy = &models.IdCoinsDeployOrderModel{
			OrderId:             orderId,
			DeployType:          models.DeployTypeIdCoins,
			InscribeState:       models.InscribeStatePending,
			IssuerMetaId:        issuerMetaId,
			IssuerAddress:       issuerAddress,
			IssuerPublicKey:     publicKey,
			IssuerSign:          issuerSign,
			TickId:              "",
			Tick:                tick,
			TokenName:           tokenName,
			Decimals:            mrc20DeployData.Decimals,
			AmtPerMint:          mrc20DeployData.AmtPerMint,
			MintCount:           mrc20DeployData.MintCount,
			PremineCount:        mrc20DeployData.PremineCount,
			StartBlockHeight:    mrc20DeployData.BeginHeight,
			EndBlockHeight:      mrc20DeployData.EndHeight,
			Metadata:            tool.AnyToStr(metaDataMap),
			TickSign:            signMsg,
			PinCheck:            tool.AnyToStr(mrc20DeployData.PinCheck),
			PayCheckPublicKey:   payToPublicKey,
			PayCheckAddress:     payToAddress,
			PayCheckAmount:      liquidityPerMint,
			Payload:             tool.AnyToStr(mrc20DeployData),
			Chain:               "btc",
			TotalFee:            totalFee,
			MinerFee:            minerFee,
			ServiceFee:          serviceFee,
			RedeemScript:        hex.EncodeToString(mrc20Builder.TxCtxData.InscriptionScript),
			ControlBlockWitness: hex.EncodeToString(mrc20Builder.TxCtxData.ControlBlockWitness),
			RevealTxPrivateKey:  mrc20Builder.TxCtxData.RecoveryPrivateKeyHex,
			RevealTxAddress:     revealAddress,
			RevealInputIndex:    revealInputIndex,
			RevealPrePsbtRaw:    revealPrePsbtRaw,
			RevealFinalPsbtRaw:  "",
			CommitTxRaw:         "",
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
		err = models.IdCoinsDeployOrderModelDao().Set(idCoinsDeploy)
		if err != nil {
			return nil, err
		}
	} else {
		idCoinsDeploy.TokenName = tokenName
		idCoinsDeploy.Decimals = mrc20DeployData.Decimals
		idCoinsDeploy.AmtPerMint = mrc20DeployData.AmtPerMint
		idCoinsDeploy.MintCount = mrc20DeployData.MintCount
		idCoinsDeploy.PremineCount = mrc20DeployData.PremineCount
		idCoinsDeploy.StartBlockHeight = mrc20DeployData.BeginHeight
		idCoinsDeploy.EndBlockHeight = mrc20DeployData.EndHeight
		idCoinsDeploy.Metadata = tool.AnyToStr(metaDataMap)
		idCoinsDeploy.TickSign = signMsg
		idCoinsDeploy.PinCheck = tool.AnyToStr(mrc20DeployData.PinCheck)
		idCoinsDeploy.PayCheckPublicKey = payToPublicKey
		idCoinsDeploy.PayCheckAddress = payToAddress
		idCoinsDeploy.PayCheckAmount = liquidityPerMint
		idCoinsDeploy.Payload = tool.AnyToStr(mrc20DeployData)
		idCoinsDeploy.TotalFee = totalFee
		idCoinsDeploy.MinerFee = minerFee
		idCoinsDeploy.ServiceFee = serviceFee
		idCoinsDeploy.RedeemScript = hex.EncodeToString(mrc20Builder.TxCtxData.InscriptionScript)
		idCoinsDeploy.ControlBlockWitness = hex.EncodeToString(mrc20Builder.TxCtxData.ControlBlockWitness)
		idCoinsDeploy.RevealTxPrivateKey = mrc20Builder.TxCtxData.RecoveryPrivateKeyHex
		idCoinsDeploy.RevealTxAddress = revealAddress
		idCoinsDeploy.RevealInputIndex = revealInputIndex
		idCoinsDeploy.RevealPrePsbtRaw = revealPrePsbtRaw
		idCoinsDeploy.UpdateTime = nowTime
		err = models.IdCoinsDeployOrderModelDao().Update(idCoinsDeploy)
		if err != nil {
			return nil, err
		}
	}

	return &respond.BuildIdCoinsPreResp{
		OrderId:        idCoinsDeploy.OrderId,
		TotalFee:       idCoinsDeploy.TotalFee,
		MinerFee:       idCoinsDeploy.MinerFee,
		ReceiveAddress: idCoinsDeploy.RevealTxAddress,
		ServiceFee:     idCoinsDeploy.ServiceFee,
	}, nil
}

func buildIdCoinsCommitFromSelf(req *request.BuildIdCoinsCommitRequest, publicKey, ip string) (*respond.BuildIdCoinsCommitResp, error) {
	var (
		orderId      string = req.OrderId
		commitTxRaw  string = req.CommitTxRaw
		commitTxId   string = ""
		revealTxRaw  string = ""
		finalPsbtRaw string = ""

		address string = ""

		txId          string = ""
		idCoinsDeploy *models.IdCoinsDeployOrderModel
		err           error

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

	idCoinsDeploy, err = models.IdCoinsDeployOrderModelDao().GetOne(&models.IdCoinsDeployOrderModel{OrderId: orderId})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if idCoinsDeploy == nil {
		return nil, errors.New("deploy order not exists")
	}
	if idCoinsDeploy.InscribeState != models.InscribeStatePending {
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
	utxoInfo := common.GetUtxoInfo(conf.Net, txIn.PreviousOutPoint.Hash.String(), int64(txIn.PreviousOutPoint.Index))
	if utxoInfo == nil {
		return nil, errors.New("commitTx utxoInfo not exists")
	}
	address = utxoInfo.Address
	if address != idCoinsDeploy.IssuerAddress {
		return nil, errors.New("commitTx address not match")
	}

	prePsbtRaw = idCoinsDeploy.RevealPrePsbtRaw
	if prePsbtRaw == "" {
		return nil, errors.New("prePsbtRaw is empty")
	}

	psbtBuilder, err = common.NewPsbtBuilder(common.GetNetParams(conf.Net), prePsbtRaw)
	if err != nil {
		return nil, err
	}

	txHash := commitTx.TxHash()
	commitPreOutPoint := wire.NewOutPoint(&txHash, uint32(commitTxOutIndex))
	psbtBuilder.PsbtUpdater.Upsbt.UnsignedTx.TxIn[idCoinsDeploy.RevealInputIndex].PreviousOutPoint = *commitPreOutPoint

	pkScript, err = common.AddressToPkScript(conf.Net, idCoinsDeploy.RevealTxAddress)
	taprootInSigner = &common.InputSign{
		UtxoType: common.Taproot,
		Index:    int(idCoinsDeploy.RevealInputIndex),
		//OutRaw:         "",
		PkScript:            pkScript,
		RedeemScript:        idCoinsDeploy.RedeemScript,
		ControlBlockWitness: idCoinsDeploy.ControlBlockWitness,
		Amount:              uint64(idCoinsDeploy.MinerFee),
		SighashType:         txscript.SigHashAll,
		PriHex:              idCoinsDeploy.RevealTxPrivateKey,
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

	idCoinsDeploy.CommitTxRaw = commitTxRaw
	idCoinsDeploy.RevealFinalPsbtRaw = finalPsbtRaw
	idCoinsDeploy.CommitTxId = commitTxId
	idCoinsDeploy.TxId = txId
	idCoinsDeploy.TickId = fmt.Sprintf("%si0", txId)
	idCoinsDeploy.InscribeState = models.InscribeStatePaid
	idCoinsDeploy.UpdateTime = nowTime

	err = models.IdCoinsDeployOrderModelDao().Update(idCoinsDeploy)
	if err != nil {
		return nil, err
	}

	err = models.IdCoinsDeployOrderModelDao().UpdateEntityForInscribing(idCoinsDeploy, txRawList, common.BroadcastTx)
	if err != nil {
		return nil, err
	}
	return &respond.BuildIdCoinsCommitResp{
		OrderId:    idCoinsDeploy.OrderId,
		TickId:     idCoinsDeploy.TickId,
		CommitTxId: commitTxId,
		RevealTxId: txId,
		PinId:      idCoinsDeploy.TickId,
		TxId:       txId,
	}, nil
}

func getIdCoinsPoolKeyAndAddress(tick string) (string, string, error) {
	var (
		err           error
		protocol      string = "idCoins"
		btcPubblicKey string = ""
		btcAddress    string = ""
	)

	res, err := wallet_service.FetchPoolKey("", tick, protocol, nil)
	btcAddress, err = common.GetSegwitAddressFromPublicKey(common.GetNetParams(conf.Net), res.Btc)
	if err != nil {
		return "", "", err
	}
	return btcPubblicKey, btcAddress, nil

}
