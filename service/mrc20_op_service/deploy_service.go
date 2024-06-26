package mrc20_op_service

import (
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/btcsuite/btcd/wire"
	"metaid-market-service/common"
	"metaid-market-service/conf"
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
	"metaid-market-service/models"
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
	Blockheight  string      `json:"blockheight"`
	Metadata     string      `json:"metadata"`
	Qual         interface{} `json:"qual"`
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
		StartBlockHeight:  data.Blockheight,
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
