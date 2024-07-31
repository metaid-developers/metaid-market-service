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
	"metaid-market-service/service/parse_service"
	"metaid-market-service/tool"
	"strconv"
)

const (
	Mrc20DeployOperation   = "create"
	Mrc20DeployPath        = "/ft/mrc20/deploy"
	Mrc20DeployContentType = "application/json"
)

type Mrc20DeployData struct {
	Tick         string      `json:"tick"`
	TokenName    string      `json:"tokenName"`
	Decimals     string      `json:"decimals"`
	AmtPerMint   string      `json:"amtPerMint"`
	MintCount    string      `json:"mintCount"`
	PremineCount string      `json:"premineCount"`
	BeginHeight  string      `json:"beginHeight"`
	EndHeight    string      `json:"endHeight"`
	Metadata     string      `json:"metadata"`
	Qual         interface{} `json:"qual"`
}

func Mrc20DeployPre(req *request.Mrc20DeployPreRequest, publicKey, ip string) (*respond.Mrc20DeployPreResp, error) {
	var (
		payload        string = req.Payload
		networkFeeRate int64  = req.NetworkFeeRate
		address        string = req.Address

		mrc20DeployData *mrc20_service.Mrc20DeployData
		orderId         string = ""
		entity          *models.Mrc20DeployOrderModel
		err             error
		tick            string = ""

		mrc20OpRequest        *mrc20_service.Mrc20OpRequest
		mrc20Builder          *mrc20_service.Mrc20Builder
		mrc20OutValue         int64  = 546
		changeAddress         string = address
		deployPinOutAddress          = address
		deployMrc20OutAddress        = address
		revealInputIndex      int64  = 0
		revealPrePsbtRaw      string = ""
		revealAddress         string = ""

		totalFee   int64 = 0
		minerFee   int64 = 0
		serviceFee int64 = 0

		nowTime int64 = tool.MakeTimestamp()
	)

	if payload == "" {
		return nil, errors.New("pin content body is empty")
	}
	if err = tool.JsonToObject(payload, &mrc20DeployData); err != nil {
		return nil, err
	}

	tick = mrc20DeployData.Tick

	tickInfo, _ := common_service.GetMrc20TickInfo("", tick)
	if tickInfo != nil && tickInfo.Mrc20Id != "" {
		return nil, errors.New("tick already exist")
	}

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
		//DeployMrc20OutAddress: deployMrc20OutAddress,
	}
	if mrc20DeployData.PremineCount != "" {
		mrc20OpRequest.DeployMrc20OutAddress = deployMrc20OutAddress
	}

	mrc20Builder, minerFee, err = mrc20_service.Mrc20DeployBuilder(mrc20OpRequest, networkFeeRate)
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

	orderId = fmt.Sprintf("mrc20_deploy_%s_%s_%d", deployPinOutAddress, tick, nowTime)
	orderId = hex.EncodeToString(tool.SHA256([]byte(orderId)))

	entity, err = models.Mrc20DeployOrderModelDao().GetOne(&models.Mrc20DeployOrderModel{OrderId: orderId})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if entity != nil && (entity.InscribeState == models.InscribeStatePaid || entity.InscribeState == models.InscribeStateFinish) {
		return nil, errors.New("deploy order already exists")
	}
	if entity == nil {
		entity = &models.Mrc20DeployOrderModel{
			OrderId:             orderId,
			InscribeState:       models.InscribeStatePending,
			Address:             address,
			TickId:              "",
			Tick:                tick,
			TokenName:           mrc20DeployData.TokenName,
			Decimals:            mrc20DeployData.Decimals,
			AmtPerMint:          mrc20DeployData.AmtPerMint,
			MintCount:           mrc20DeployData.MintCount,
			PremineCount:        mrc20DeployData.PremineCount,
			StartBlockHeight:    mrc20DeployData.BeginHeight,
			EndBlockHeight:      mrc20DeployData.EndHeight,
			Qual:                tool.AnyToStr(mrc20DeployData.PayCheck),
			Payload:             payload,
			Chain:               "BTC",
			NetworkFeeRate:      networkFeeRate,
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
		err = models.Mrc20DeployOrderModelDao().Set(entity)
		if err != nil {
			return nil, err
		}
	} else {
		entity.Address = address
		entity.TokenName = mrc20DeployData.TokenName
		entity.Decimals = mrc20DeployData.Decimals
		entity.AmtPerMint = mrc20DeployData.AmtPerMint
		entity.MintCount = mrc20DeployData.MintCount
		entity.PremineCount = mrc20DeployData.PremineCount
		entity.StartBlockHeight = mrc20DeployData.BeginHeight
		entity.EndBlockHeight = mrc20DeployData.EndHeight
		entity.Qual = tool.AnyToStr(mrc20DeployData.PinCheck)
		entity.Chain = "BTC"
		entity.NetworkFeeRate = networkFeeRate
		entity.TotalFee = totalFee
		entity.MinerFee = minerFee
		entity.ServiceFee = serviceFee
		entity.RedeemScript = hex.EncodeToString(mrc20Builder.TxCtxData.InscriptionScript)
		entity.ControlBlockWitness = hex.EncodeToString(mrc20Builder.TxCtxData.ControlBlockWitness)
		entity.RevealTxPrivateKey = mrc20Builder.TxCtxData.RecoveryPrivateKeyHex
		entity.RevealTxAddress = revealAddress
		entity.RevealInputIndex = revealInputIndex
		entity.RevealPrePsbtRaw = revealPrePsbtRaw
		entity.UpdateTime = nowTime
		err = models.Mrc20DeployOrderModelDao().Update(entity)
		if err != nil {
			return nil, err
		}
	}
	return &respond.Mrc20DeployPreResp{
		OrderId:       orderId,
		TotalFee:      totalFee,
		MinerFee:      minerFee,
		ServiceFee:    serviceFee,
		RevealAddress: revealAddress,
		//RevealPrePsbtRaw: revealPrePsbtRaw,
		//RevealInputIndex: revealInputIndex,
	}, nil
}

func Mrc20DeployCommit(req *request.Mrc20DeployCommitRequest, publicKey, ip string) (*respond.Mrc20DeployCommitResp, error) {
	var (
		orderId      string = req.OrderId
		commitTxRaw  string = req.CommitTxRaw
		commitTxId   string = ""
		revealTxRaw  string = ""
		finalPsbtRaw string = ""

		txId        string = ""
		deployOrder *models.Mrc20DeployOrderModel
		err         error

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

	deployOrder, err = models.Mrc20DeployOrderModelDao().GetOne(&models.Mrc20DeployOrderModel{OrderId: orderId})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if deployOrder == nil {
		return nil, errors.New("deploy order not exists")
	}
	if deployOrder.InscribeState != models.InscribeStatePending {
		return nil, errors.New("deploy order state error")
	}
	txRawByte, _ = hex.DecodeString(commitTxRaw)
	err = commitTx.Deserialize(bytes.NewReader(txRawByte))
	if err != nil {
		return nil, errors.New("commitTx Deserialize error: " + err.Error())
	}
	commitTxId = commitTx.TxHash().String()
	if len(commitTx.TxOut) <= int(commitTxOutIndex) {
		return nil, errors.New("commitTxOutIndex error")
	}

	//txIn := commitTx.TxIn[0]
	//utxoInfo := common.GetUtxoInfo(conf.Net, txIn.PreviousOutPoint.Hash.String(), int64(txIn.PreviousOutPoint.Index))
	//if utxoInfo == nil {
	//	return nil, errors.New("commitTx utxoInfo not exists")
	//}
	//address = utxoInfo.Address
	//if address != deployOrder.IssuerAddress {
	//	return nil, errors.New("commitTx address not match")
	//}

	prePsbtRaw = deployOrder.RevealPrePsbtRaw
	if prePsbtRaw == "" {
		return nil, errors.New("prePsbtRaw is empty")
	}

	psbtBuilder, err = common.NewPsbtBuilder(common.GetNetParams(conf.Net), prePsbtRaw)
	if err != nil {
		return nil, err
	}

	txHash := commitTx.TxHash()
	commitPreOutPoint := wire.NewOutPoint(&txHash, uint32(commitTxOutIndex))
	psbtBuilder.PsbtUpdater.Upsbt.UnsignedTx.TxIn[deployOrder.RevealInputIndex].PreviousOutPoint = *commitPreOutPoint

	pkScript, err = common.AddressToPkScript(conf.Net, deployOrder.RevealTxAddress)
	taprootInSigner = &common.InputSign{
		UtxoType: common.Taproot,
		Index:    int(deployOrder.RevealInputIndex),
		//OutRaw:         "",
		PkScript:            pkScript,
		RedeemScript:        deployOrder.RedeemScript,
		ControlBlockWitness: deployOrder.ControlBlockWitness,
		Amount:              uint64(deployOrder.MinerFee),
		SighashType:         txscript.SigHashAll,
		PriHex:              deployOrder.RevealTxPrivateKey,
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
		return nil, errors.New("revealTx Deserialize error: " + err.Error())
	}
	txId = revealTx.TxHash().String()
	txRawList = append(txRawList, revealTxRaw)

	deployOrder.CommitTxRaw = commitTxRaw
	deployOrder.RevealTxRaw = revealTxRaw
	deployOrder.RevealFinalPsbtRaw = finalPsbtRaw
	deployOrder.CommitTxId = commitTxId
	deployOrder.RevealTxId = txId
	deployOrder.TxId = txId
	deployOrder.UpdateTime = nowTime
	deployOrder.TickId = fmt.Sprintf("%si0", txId)

	err = models.Mrc20DeployOrderModelDao().SaveEntityForInscribing(deployOrder, txRawList, common.BroadcastTx)
	if err != nil {
		return nil, err
	}
	return &respond.Mrc20DeployCommitResp{
		OrderId:    deployOrder.OrderId,
		CommitTxId: deployOrder.CommitTxId,
		RevealTxId: deployOrder.RevealTxId,
	}, nil
}

func Mrc20Deploy(req *request.Mrc20DeployRequest, publicKey, ip string) (*respond.Mrc20DeployResp, error) {
	var (
		orderId string = ""
		entity  *models.Mrc20DeployOrderModel
		err     error

		address string = ""

		isMetaIdPin bool
		pin         *parse_service.PersonalInformationNode
		txOuts      []*wire.TxOut
		payload     string = ""
		data        *Mrc20DeployData
		commitTx    *wire.MsgTx
		revealTx    *wire.MsgTx
		tickId      string
		txRawList   []string = []string{req.CommitTxRaw, req.RevealTxRaw}
		nowTime     int64    = tool.MakeTimestamp()
	)
	commitTx, err = common.TxRawToTx(req.CommitTxRaw)
	if err != nil {
		return nil, err
	}
	revealTx, err = common.TxRawToTx(req.RevealTxRaw)
	if err != nil {
		return nil, err
	}

	txIn := commitTx.TxIn[0]
	utxoInfo := common.GetUtxoInfo(conf.Net, txIn.PreviousOutPoint.Hash.String(), int64(txIn.PreviousOutPoint.Index))
	if utxoInfo == nil {
		return nil, errors.New("commitTx utxoInfo not exists")
	}
	address = utxoInfo.Address
	isMetaIdPin, pin, txOuts, err = parse_service.ParseTxPin(req.RevealTxRaw)
	if err != nil {
		return nil, err
	}
	if !isMetaIdPin {
		return nil, errors.New("not metaid pin tx")
	}
	if pin == nil {
		return nil, errors.New("pin is nil")
	}
	if pin.Operation != Mrc20DeployOperation {
		return nil, errors.New("pin operation is not create")
	}
	if pin.Path != Mrc20DeployPath {
		return nil, errors.New("pin path is not /ft/mrc20/deploy")
	}
	if pin.ContentType != Mrc20DeployContentType {
		return nil, errors.New("pin content type is not application/json")
	}
	payload = string(pin.ContentBody)
	if payload == "" {
		return nil, errors.New("pin content body is empty")
	}
	if err = tool.JsonToObject(payload, &data); err != nil {
		return nil, err
	}
	if data.PremineCount != "" {
		premineCount, _ := strconv.ParseInt(data.PremineCount, 10, 64)
		if premineCount > 0 && len(txOuts) < 2 {
			return nil, errors.New("tx out count less than 2 when premine count > 0")
		}
	}
	tickId = fmt.Sprintf("%si0", revealTx.TxHash().String())
	orderId = fmt.Sprintf("%s%s", tickId, data.Tick)
	orderId = hex.EncodeToString(tool.SHA256([]byte(orderId)))

	entity = &models.Mrc20DeployOrderModel{
		OrderId:           orderId,
		InscribeState:     models.InscribeStatePending,
		Address:           address,
		TickId:            tickId,
		Tick:              data.Tick,
		TokenName:         data.TokenName,
		Decimals:          data.Decimals,
		AmtPerMint:        data.AmtPerMint,
		MintCount:         data.MintCount,
		PremineCount:      data.PremineCount,
		StartBlockHeight:  data.BeginHeight,
		EndBlockHeight:    data.EndHeight,
		Qual:              tool.AnyToStr(data.Qual),
		Payload:           payload,
		Chain:             "BTC",
		CommitTxRaw:       req.CommitTxRaw,
		RevealTxRaw:       req.RevealTxRaw,
		CommitTxId:        commitTx.TxHash().String(),
		RevealTxId:        revealTx.TxHash().String(),
		TxId:              revealTx.TxHash().String(),
		BlockHeight:       0,
		ConfirmationState: models.ConfirmationStateUnconfirmed,
		Timestamp:         nowTime,
		Version:           0,
		CreateTime:        nowTime,
		UpdateTime:        0,
		State:             models.STATE_EXIST,
	}
	err = models.Mrc20DeployOrderModelDao().SaveEntityForInscribing(entity, txRawList, common.BroadcastTx)
	if err != nil {
		return nil, err
	}
	return &respond.Mrc20DeployResp{
		OrderId:    orderId,
		TickId:     tickId,
		CommitTxId: entity.CommitTxId,
		RevealTxId: entity.RevealTxId,
	}, nil
}
