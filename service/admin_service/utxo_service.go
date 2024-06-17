package admin_service

import (
	"fmt"
	"github.com/btcsuite/btcd/chaincfg"
	"metaid-market-service/common"
	"metaid-market-service/conf"
	"metaid-market-service/controller/request"
	"metaid-market-service/models"
	"metaid-market-service/tool"
)

func ColdDownDummyUtxo(req *request.ColdDownDummyUtxoRequest) (string, error) {
	var (
		netParams *chaincfg.Params = common.GetNetParams(conf.Net)
		err       error
		coldTxId  string = ""
		txRaw     string = ""
		//latestUtxo *model.OrderUtxoModel
		utxoList  []*models.MarketUtxoModel = make([]*models.MarketUtxoModel, 0)
		perAmount uint64                    = req.PerAmount

		_, dummyAddress string = common.GetPlatformKeyAndAddressForDummyAsk()
		nowTime         int64  = tool.MakeTimestamp()
	)

	inputs := make([]*common.TxInputUtxo, 0)
	inputs = append(inputs, &common.TxInputUtxo{
		TxId:     req.TxId,
		TxIndex:  req.Index,
		PkScript: req.PkScript,
		Amount:   req.Amount,
		PriHex:   req.PriKeyHex,
		SignMode: common.SignModeSegwit,
	})

	outputs := make([]*common.TxOutput, 0)
	for i := int64(0); i < req.Count; i++ {
		outputs = append(outputs, &common.TxOutput{
			Address: dummyAddress,
			Amount:  int64(perAmount),
		})

		pkScript, err := common.AddressToPkScript(conf.Net, dummyAddress)
		if err != nil {
			return "", err
		}

		utxoList = append(utxoList, &models.MarketUtxoModel{
			//UtxoId:         "",
			UtxoType:      req.UtxoType,
			Amount:        perAmount,
			Address:       dummyAddress,
			PrivateKeyHex: "",
			//TxId:           "",
			Index:     i,
			PkScript:  pkScript,
			UsedState: models.UsedNo,
			//UsedTxId:       "",
			//OrderId:        "",
			//SortIndex:      0,
			//ConfirmStatus:  0,
			//FromOrderId:    "",
			NetworkFeeRate: req.FeeRate,
			Timestamp:      nowTime,
			//Version:        0,
			CreateTime: nowTime,
			//UpdateTime:     0,
			State: models.STATE_EXIST,
		})
	}

	if req.ChangeAddress == "" {
		req.ChangeAddress = req.Address
	}
	tx, err := common.BuildCommonTx(netParams, inputs, outputs, req.ChangeAddress, req.FeeRate, false)
	if err != nil {
		fmt.Printf("BuildCommonTx err:%s\n", err.Error())
		return "", err
	}
	txRaw, err = common.ToRaw(tx)
	if err != nil {
		return "", err
	}

	for _, u := range utxoList {
		u.TxId = tx.TxHash().String()
		u.UtxoId = fmt.Sprintf("%s_%d", u.TxId, u.Index)
	}

	coldTxId, err = models.MarketUtxoModelDao().ColdDownUtxoEntityListForJobFunc(utxoList, txRaw, common.BroadcastTx)
	if err != nil {
		return "", err
	}

	return coldTxId, nil
}
