package order_service

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/btcsuite/btcd/wire"
	"github.com/godaddy-x/freego/utils/decimal"
	"gorm.io/gorm"
	"metaid-market-service/common"
	"metaid-market-service/conf"
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
	"metaid-market-service/models"
	"metaid-market-service/service/man_service"
	"metaid-market-service/tool"
	"strings"
)

func BatchPushMarketOrder(req *request.BatchPushOrderReq, publicKey, ip string) (*respond.BatchPushOrderResp, error) {
	var (
		orderEntityList     []*models.MarketOrderModel = make([]*models.MarketOrderModel, 0)
		psbtBuilder         *common.PsbtBuilder
		err                 error
		sellerAddress       string                = req.Address
		sellerPriceDecimal  int64                 = 8
		sellerPriceCoin     string                = "BTC"
		assetType                                 = req.AssetType
		assetIds            []string              = req.AssetIds
		psbtRawTotal        string                = req.PsbtRaw
		psbtBuilderItemList []*common.PsbtBuilder = make([]*common.PsbtBuilder, 0)
		nowTime             int64                 = tool.MakeTimestamp()
		feeAmount           int64                 = 2000
		feeRate             int64                 = 1

		list []*respond.PushOrderResp = make([]*respond.PushOrderResp, 0)
	)

	psbtBuilder, err = common.NewPsbtBuilder(common.GetNetParams(conf.Net), psbtRawTotal)
	if err != nil {
		return nil, err
	}
	preOutList := psbtBuilder.GetInputs()
	if preOutList == nil || len(preOutList) == 0 {
		return nil, errors.New("Wrong Psbt: empty inputs. ")
	}
	outList := psbtBuilder.GetOutputs()
	if outList == nil || len(outList) == 0 {
		return nil, errors.New("Wrong Psbt: empty outputs. ")
	}
	if len(preOutList) != len(outList) && len(preOutList) != len(req.AssetIds) {
		return nil, errors.New("Wrong Psbt: inputs and outputs not match. ")
	}

	isLegacy, err := common.CheckLegacyAddressType(conf.Net, sellerAddress)
	if err != nil {
		return nil, err
	}
	if isLegacy {
		return nil, errors.New("Wrong Psbt: legacy address does not support batch. ")
	}

	//if !psbtBuilder.IsComplete() {
	//	return nil, errors.New("Wrong Psbt: incomplete. ")
	//}

	for i, v := range preOutList {
		if len(preOutList) > len(req.AssetIds) && i < 2 {
			continue
		}
		assetId := assetIds[i]

		out := outList[i]
		sellerPriceAmount := out.Value
		pkScript := hex.EncodeToString(out.PkScript)
		address, err := common.PkScriptToAddress(conf.Net, pkScript)
		if err != nil {
			return nil, err
		}
		if address != sellerAddress {
			return nil, errors.New(fmt.Sprintf("Wrong Psbt: address not match. "))
		}

		//sellerAddress check
		verified, err := common.CheckPublicKeyAddress(common.GetNetParams(conf.Net), publicKey, sellerAddress)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Check address err: %s. ", err.Error()))
		}
		if !verified {
			return nil, errors.New(fmt.Sprintf("Check address verified: %v. ", verified))
		}

		inputs := make([]common.Input, 0)
		outputs := make([]common.Output, 0)

		preTxId := v.PreviousOutPoint.Hash.String()
		preTxIndex := v.PreviousOutPoint.Index

		inputs = append(inputs, common.Input{
			OutTxId:  preTxId,
			OutIndex: uint32(preTxIndex),
		})
		outputs = append(outputs, common.Output{
			Address: sellerAddress,
			Amount:  uint64(sellerPriceAmount),
		})

		itembuilder, err := common.CreatePsbtBuilder(common.GetNetParams(conf.Net), inputs, outputs)
		if err != nil {
			return nil, err
		}

		finalScriptWitness := psbtBuilder.PsbtUpdater.Upsbt.Inputs[i].FinalScriptWitness
		witnessUtxo := psbtBuilder.PsbtUpdater.Upsbt.Inputs[i].WitnessUtxo
		finalScriptSig := psbtBuilder.PsbtUpdater.Upsbt.Inputs[i].FinalScriptSig
		sighashType := psbtBuilder.PsbtUpdater.Upsbt.Inputs[i].SighashType
		err = itembuilder.AddSigIn(witnessUtxo, sighashType, finalScriptWitness, finalScriptSig, 0)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("PSBT(Single): AddPartialSigIn err:%s", err.Error()))
		}

		psbtRaw, err := itembuilder.ToString()
		if err != nil {
			return nil, errors.New(fmt.Sprintf("PSBT(Single): ToString err:%s", err.Error()))
		}

		psbtBuilderItemList = append(psbtBuilderItemList, itembuilder)

		assetLocalUtxoId := fmt.Sprintf("%s_%d", preTxId, preTxIndex)

		pinInfo, err := man_service.FetchUtxoPinInfo(conf.Net, preTxId, int64(preTxIndex))
		if err != nil {
			return nil, errors.New(fmt.Sprintf("fetch asset info err:%s", err))
		}
		outValue := pinInfo.OutputValue
		content := pinInfo.Content
		preview := pinInfo.Preview
		assetNumber := pinInfo.Number
		assetLevel := pinInfo.PopLv
		assetPop := common.GenPopSummary(pinInfo.Pop)
		detailMap := map[string]interface{}{
			"pinId":              pinInfo.Id,
			"metaid":             pinInfo.Metaid,
			"createAddress":      pinInfo.CreateAddress,
			"operation":          pinInfo.Operation,
			"pinNumber":          pinInfo.Number,
			"pop":                pinInfo.Pop,
			"popSummary":         common.GenPopSummary(pinInfo.Pop),
			"popLv":              pinInfo.PopLv,
			"path":               pinInfo.Path,
			"originalPath":       pinInfo.OriginalPath,
			"version":            pinInfo.Version,
			"encryption":         pinInfo.Encryption,
			"outputValue":        pinInfo.OutputValue,
			"contentLength":      pinInfo.ContentLength,
			"contentTypeDetect":  pinInfo.ContentTypeDetect,
			"timestamp":          pinInfo.Timestamp,
			"genesisHeight":      pinInfo.GenesisHeight,
			"genesisTransaction": pinInfo.GenesisTransaction,
		}

		orderId := fmt.Sprintf("%s_%s_%s_%s", assetLocalUtxoId, assetId, assetType, sellerAddress)
		orderId = hex.EncodeToString(tool.SHA256([]byte(orderId)))
		orderEntity, err := models.MarketOrderModelDao().GetOne(&models.MarketOrderModel{
			OrderId: orderId,
		})
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		if orderEntity != nil {
			if orderEntity.OrderState == models.OrderStateCreate {
				return nil, errors.New("Order already exist. ")
			} else if orderEntity.OrderState == models.OrderStateFinish {
				return nil, errors.New("Order already finish. ")
			}
		}
		if orderEntity == nil {
			orderEntity = &models.MarketOrderModel{
				OrderId:       orderId,
				UtxoId:        assetLocalUtxoId,
				OutValue:      outValue,
				AssetId:       assetId,
				AssetType:     assetType,
				AssetNumber:   assetNumber,
				AssetLevel:    int64(assetLevel),
				AssetPop:      assetPop,
				OrderState:    models.OrderStateCreate,
				SellerAddress: sellerAddress,
				SellerIp:      ip,
				//BuyerAddress:      "",
				SellPriceAmount:  sellerPriceAmount,
				SellPriceDecimal: sellerPriceDecimal,
				SellPriceCoin:    sellerPriceCoin,
				FeeAmount:        feeAmount,
				FeeRate:          feeRate,
				Content:          content,
				Preview:          preview,
				Detail:           tool.AnyToStr(detailMap),
				MakerPsbt:        psbtRaw,
				//TakerPsbt:         "",
				//TxId:              "",
				//BlockHeight:       0,
				//ConfirmationState: 0,
				Timestamp:  nowTime,
				CreateTime: nowTime,
				//UpdateTime:        0,
				State: models.STATE_EXIST,
			}
			//err = models.MarketOrderModelDao().Set(orderEntity)
			//if err != nil {
			//	return nil, err
			//}
		} else {
			orderEntity.OrderState = models.OrderStateCreate
			orderEntity.SellerIp = ip
			orderEntity.SellPriceAmount = sellerPriceAmount
			orderEntity.SellPriceDecimal = sellerPriceDecimal
			orderEntity.SellPriceCoin = sellerPriceCoin
			orderEntity.FeeAmount = feeAmount
			orderEntity.FeeRate = feeRate
			orderEntity.Content = content
			orderEntity.Preview = preview
			orderEntity.Detail = tool.AnyToStr(detailMap)
			orderEntity.MakerPsbt = psbtRaw
			orderEntity.Timestamp = nowTime
			//err = models.MarketOrderModelDao().Update(orderEntity)
			//if err != nil {
			//	return nil, err
			//}
		}
		orderEntityList = append(orderEntityList, orderEntity)
		list = append(list, &respond.PushOrderResp{
			OrderId:    orderId,
			AssetType:  assetType,
			AssetId:    assetId,
			OrderState: orderEntity.OrderState,
		})
	}

	err = models.MarketOrderModelDao().BatchSaveList(orderEntityList)
	if err != nil {
		return nil, err

	}
	return &respond.BatchPushOrderResp{
		Total: int64(len(list)),
		List:  list,
	}, nil
}

func BatchFetchOrderPsbt(req *request.BatchFetchOrderPsbtReq, publicKey, ip string) (*respond.BatchOrderPsbtResp, error) {
	var (
		orderEntityList      []*models.MarketOrderModel = make([]*models.MarketOrderModel, 0)
		err                  error
		psbtRawList          []string = make([]string, 0)
		outValueList         []int64  = make([]int64, 0)
		takerPsbtRaw         string   = ""
		feeAmountForPlatform int64    = 0
	)
	for _, v := range req.OrderIds {
		entity, err := models.MarketOrderModelDao().GetOne(&models.MarketOrderModel{
			OrderId: v,
		})
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		if entity == nil {
			return nil, errors.New("Order is empty. ")
		}

		if entity != nil {
			isLegacy, err := common.CheckLegacyAddressType(conf.Net, entity.SellerAddress)
			if err != nil {
				return nil, err
			}
			if isLegacy {
				return nil, errors.New("legacy address does not support batch. ")
			}

			if entity.SellerAddress == req.BuyerAddress {
				return nil, errors.New("Buyer address is same as seller. ")
			}

			orderEntityList = append(orderEntityList, entity)
		}
	}

	for _, v := range orderEntityList {
		psbtRawList = append(psbtRawList, v.MakerPsbt)
		outValueList = append(outValueList, v.OutValue)
	}

	takerPsbtRaw, feeAmountForPlatform, err = common.BatchMakeAskTakerPsbtRaw(conf.Net, psbtRawList, outValueList, req.BuyerAddress, req.BuyerChangeAmount, feeAmountForPlatform, true)
	if err != nil {
		return nil, err
	}

	return &respond.BatchOrderPsbtResp{
		OrderIds: req.OrderIds,
		TakePsbt: takerPsbtRaw,
		Fee:      feeAmountForPlatform,
		FeeRate:  0,
	}, nil
}

func BatchTakeMarketOrder(req *request.BatchTakeOrderReq, publicKey, ip string) (*respond.BatchTakeOrderResp, error) {
	var (
		entityList             []*models.MarketOrderModel = make([]*models.MarketOrderModel, 0)
		err                    error
		takerAskPsbtBuilder    *common.PsbtBuilder
		usedUtxoList           []*models.MarketUtxoModel = make([]*models.MarketUtxoModel, 0)
		newUtxoList            []*models.MarketUtxoModel = make([]*models.MarketUtxoModel, 0)
		nowTime                int64                     = tool.MakeTimestamp()
		offsetDummyInputIndex  int                       = 2
		offsetDummyOutputIndex int                       = 2
		orderIdsStr            string                    = ""
		list                   []*respond.TakeOrderInfo  = make([]*respond.TakeOrderInfo, 0)
	)
	for _, v := range req.OrderIds {
		entity, err := models.MarketOrderModelDao().GetOne(&models.MarketOrderModel{
			OrderId: v,
		})
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		if entity == nil {
			return nil, errors.New("Order is empty. ")
		}
		if entity.OrderState != models.OrderStateCreate {
			return nil, errors.New("Order is closed. ")
		}
		entityList = append(entityList, entity)
		offsetDummyInputIndex++
		offsetDummyOutputIndex++
		orderIdsStr += v + ","
	}

	takerAskPsbtBuilder, err = common.NewPsbtBuilder(common.GetNetParams(conf.Net), req.TakerPsbtRaw)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("PSBT: NewPsbtBuilder err:%s", err.Error()))
	}
	err = common.BatchSignAskTakerPsbtRawInDummy(conf.Net, takerAskPsbtBuilder, len(req.OrderIds))
	if err != nil {
		return nil, errors.New(fmt.Sprintf("SignAskTakerPsbtRawInDummy err:%s", err.Error()))
	}
	finalAskPsbtRaw, err := takerAskPsbtBuilder.ToString()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("PSBT(X): ToString err:%s", err.Error()))
	}
	txRaw, err := takerAskPsbtBuilder.ExtractPsbtTransaction()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("PSBT: ExtractPsbtTransaction err:%s", err.Error()))
	}
	txRawByte, _ := hex.DecodeString(txRaw)
	txAsk := wire.NewMsgTx(2)
	err = txAsk.Deserialize(bytes.NewReader(txRawByte))
	if err != nil {
		return nil, errors.New(fmt.Sprintf("txAsk Deserialize err: %v. ", err.Error()))
	}
	txId := txAsk.TxHash().String()

	if len(takerAskPsbtBuilder.GetInputs()) < 5 {
		return nil, errors.New(fmt.Sprintf("PSBT: No match inputs length err"))
	}

	for i, v := range takerAskPsbtBuilder.GetInputs() {
		if i == 0 || i == 1 || i == offsetDummyInputIndex {
			utxoId := fmt.Sprintf("%s_%d", v.PreviousOutPoint.Hash.String(), v.PreviousOutPoint.Index)
			utxoEntity, err := models.MarketUtxoModelDao().GetOne(&models.MarketUtxoModel{
				UtxoId: utxoId,
			})
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, err
			}
			if utxoEntity == nil {
				return nil, errors.New(fmt.Sprintf("Utxo not found: %s. ", utxoId))
			}
			if utxoEntity.UsedState != models.UsedNo {
				return nil, errors.New(fmt.Sprintf("Utxo used: %s. ", utxoId))
			}
			utxoEntity.UsedState = models.UsedYes
			utxoEntity.UsedTxId = txId
			utxoEntity.OrderId = orderIdsStr
			usedUtxoList = append(usedUtxoList, utxoEntity)
		}
	}

	for i, v := range takerAskPsbtBuilder.GetOutputs() {
		if i == 0 || i == offsetDummyOutputIndex || i == offsetDummyOutputIndex+1 {
			utxoType := models.UtxoTypeDummy600
			if i == 0 {
				utxoType = models.UtxoTypeDummy1200
			}
			pkScriptHex := hex.EncodeToString(v.PkScript)
			address, err := common.PkScriptToAddress(conf.Net, pkScriptHex)
			if err != nil {
				return nil, errors.New(fmt.Sprintf("PkScriptToAddress err: %s. ", err.Error()))
			}

			newUtxo := &models.MarketUtxoModel{
				UtxoId:   fmt.Sprintf("%s_%d", txId, i),
				UtxoType: utxoType,
				Amount:   uint64(v.Value),
				Address:  address,
				//PrivateKeyHex:  "",
				TxId:      txId,
				Index:     int64(i),
				PkScript:  pkScriptHex,
				UsedState: models.UsedNo,
				//UsedTxId:       "",
				//OrderId:        "",
				//SortIndex:      0,
				//ConfirmStatus:  0,
				FromOrderId:    orderIdsStr,
				NetworkFeeRate: req.NetworkFeeRate,
				Timestamp:      nowTime,
				//Version:        0,
				CreateTime: nowTime,
				//UpdateTime:     0,
				State: models.STATE_EXIST,
			}
			newUtxoList = append(newUtxoList, newUtxo)
		}
	}

	buyerAddress := ""
	buyerInput := takerAskPsbtBuilder.GetInputs()[4]
	buyerInputTxId := buyerInput.PreviousOutPoint.Hash.String()
	buyerInputIndex := buyerInput.PreviousOutPoint.Index
	utxoInfo := common.GetUtxoInfo(conf.Net, buyerInputTxId, int64(buyerInputIndex))
	if utxoInfo == nil || !utxoInfo.IsExist || utxoInfo.SpendStatus == "spend" {
		return nil, errors.New(fmt.Sprintf("PSBT(take): Utxo is spend. Please select a different utxo. [%s_%d]", buyerInputTxId, int64(buyerInputIndex)))
	}
	buyerAddress = utxoInfo.Address

	for _, entity := range entityList {
		entity.FinalPsbt = finalAskPsbtRaw
		entity.TxId = txId
		entity.BuyerAddress = buyerAddress
		entity.BuyerIp = ip
		entity.OrderState = models.OrderStateFinish
		entity.ConfirmationState = models.ConfirmationStateUnconfirmed

		list = append(list, &respond.TakeOrderInfo{
			OrderId:    entity.OrderId,
			AssetType:  entity.AssetType,
			AssetId:    entity.AssetId,
			OrderState: entity.OrderState,
		})
	}

	verified, err := common.CheckPublicKeyAddress(common.GetNetParams(conf.Net), publicKey, buyerAddress)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Check address err: %s. ", err.Error()))
	}
	if !verified {
		return nil, errors.New(fmt.Sprintf("Check address verified: %v. ", verified))
	}
	err = models.MarketOrderModelDao().BatchUpdateOrderEntityListForJobFunc(
		entityList,
		usedUtxoList, newUtxoList,
		txRaw,
		common.BroadcastTx)
	if err != nil {
		return nil, err
	}

	return &respond.BatchTakeOrderResp{
		TxId:  txId,
		Total: int64(len(list)),
		List:  list,
	}, nil
}

func PushMarketOrder(req *request.PushOrderReq, publicKey, ip string) (*respond.PushOrderResp, error) {
	var (
		orderEntity        *models.MarketOrderModel
		psbtBuilder        *common.PsbtBuilder
		err                error
		orderId            string                 = ""
		assetId            string                 = req.AssetId
		assetLocalUtxoId   string                 = ""
		assetType          models.AssetType       = req.AssetType
		sellerAddress      string                 = req.Address
		sellerPriceAmount  int64                  = 0
		sellerPriceDecimal int64                  = 8
		sellerPriceCoin    string                 = "BTC"
		psbtRaw            string                 = req.PsbtRaw
		nowTime            int64                  = tool.MakeTimestamp()
		outValue           int64                  = 0
		assetNumber        int64                  = 0
		assetLevel         int                    = 0
		assetPop           string                 = ""
		content            string                 = ""
		preview            string                 = ""
		detailMap          map[string]interface{} = map[string]interface{}{}

		feeAmount int64 = 2000
		feeRate   int64 = 1
	)
	if req.PsbtRaw == "" {
		return nil, errors.New("Wrong Psbt: empty. ")
	}

	verified, err := common.CheckPublicKeyAddress(common.GetNetParams(conf.Net), publicKey, sellerAddress)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Check address err: %s. ", err.Error()))
	}
	if !verified {
		return nil, errors.New(fmt.Sprintf("Check address verified: %v. ", verified))
	}

	isLegacy, err := common.CheckLegacyAddressType(conf.Net, sellerAddress)
	if err != nil {
		return nil, err
	}

	//check psbt
	psbtBuilder, err = common.NewPsbtBuilder(common.GetNetParams(conf.Net), psbtRaw)
	if err != nil {
		return nil, err
	}
	preOutList := psbtBuilder.GetInputs()
	if preOutList == nil || len(preOutList) == 0 {
		return nil, errors.New("Wrong Psbt: empty inputs. ")
	}
	outList := psbtBuilder.GetOutputs()
	if outList == nil || len(outList) == 0 {
		return nil, errors.New("Wrong Psbt: empty outputs. ")
	}
	if len(preOutList) != len(outList) {
		return nil, errors.New("Wrong Psbt: inputs and outputs not match. ")
	}
	if isLegacy && len(preOutList) != 3 {
		return nil, errors.New("Wrong Psbt: inputs not match. ")
	}
	//else if !isLegacy && len(preOutList) != 1 {
	//	return nil, errors.New("Wrong Psbt: inputs not match. ")
	//}

	//check inputs and asset
	for i, v := range preOutList {
		if len(preOutList) == 3 && i != 2 {
			continue
		}
		preTxId := v.PreviousOutPoint.Hash.String()
		preTxIndex := v.PreviousOutPoint.Index
		assetLocalUtxoId = fmt.Sprintf("%s_%d", preTxId, preTxIndex)

		pinInfo, err := man_service.FetchUtxoPinInfo(conf.Net, preTxId, int64(preTxIndex))
		if err != nil {
			return nil, errors.New(fmt.Sprintf("fetch asset info err:%s", err))
		}
		outValue = pinInfo.OutputValue
		content = pinInfo.Content
		preview = pinInfo.Preview
		assetNumber = pinInfo.Number
		assetLevel = pinInfo.PopLv
		assetPop = common.GenPopSummary(pinInfo.Pop)
		detailMap = map[string]interface{}{
			"pinId":              pinInfo.Id,
			"metaid":             pinInfo.Metaid,
			"createAddress":      pinInfo.CreateAddress,
			"operation":          pinInfo.Operation,
			"pinNumber":          pinInfo.Number,
			"pop":                pinInfo.Pop,
			"popSummary":         common.GenPopSummary(pinInfo.Pop),
			"popLv":              pinInfo.PopLv,
			"path":               pinInfo.Path,
			"originalPath":       pinInfo.OriginalPath,
			"version":            pinInfo.Version,
			"encryption":         pinInfo.Encryption,
			"outputValue":        pinInfo.OutputValue,
			"contentLength":      pinInfo.ContentLength,
			"contentTypeDetect":  pinInfo.ContentTypeDetect,
			"timestamp":          pinInfo.Timestamp,
			"genesisHeight":      pinInfo.GenesisHeight,
			"genesisTransaction": pinInfo.GenesisTransaction,
		}
	}

	for i, v := range outList {
		if len(preOutList) == 3 && i != 2 {
			continue
		}
		pkScript := hex.EncodeToString(v.PkScript)
		address, err := common.PkScriptToAddress(conf.Net, pkScript)
		if err != nil {
			return nil, err
		}
		if address != sellerAddress {
			return nil, errors.New("Address does not match in output. ")
		}
		sellerPriceAmount = v.Value
	}

	//if !psbtBuilder.IsComplete() {
	//	return nil, errors.New("Wrong Psbt: incomplete. ")
	//}

	orderId = fmt.Sprintf("%s_%s_%s_%s", assetLocalUtxoId, assetId, assetType, sellerAddress)
	orderId = hex.EncodeToString(tool.SHA256([]byte(orderId)))
	orderEntity, err = models.MarketOrderModelDao().GetOne(&models.MarketOrderModel{
		OrderId: orderId,
	})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if orderEntity != nil {
		if orderEntity.OrderState == models.OrderStateCreate {
			return nil, errors.New("Order already exist. ")
		} else if orderEntity.OrderState == models.OrderStateFinish {
			return nil, errors.New("Order already finish. ")
		}
	}
	if orderEntity == nil {
		orderEntity = &models.MarketOrderModel{
			OrderId:       orderId,
			UtxoId:        assetLocalUtxoId,
			OutValue:      outValue,
			AssetId:       assetId,
			AssetType:     assetType,
			AssetNumber:   assetNumber,
			AssetLevel:    int64(assetLevel),
			AssetPop:      assetPop,
			OrderState:    models.OrderStateCreate,
			SellerAddress: sellerAddress,
			SellerIp:      ip,
			//BuyerAddress:      "",
			SellPriceAmount:  sellerPriceAmount,
			SellPriceDecimal: sellerPriceDecimal,
			SellPriceCoin:    sellerPriceCoin,
			FeeAmount:        feeAmount,
			FeeRate:          feeRate,
			Content:          content,
			Preview:          preview,
			Detail:           tool.AnyToStr(detailMap),
			MakerPsbt:        psbtRaw,
			//TakerPsbt:         "",
			//TxId:              "",
			//BlockHeight:       0,
			//ConfirmationState: 0,
			Timestamp:  nowTime,
			CreateTime: nowTime,
			//UpdateTime:        0,
			State: models.STATE_EXIST,
		}
		err = models.MarketOrderModelDao().Set(orderEntity)
		if err != nil {
			return nil, err
		}
	} else {
		orderEntity.OrderState = models.OrderStateCreate
		orderEntity.SellerIp = ip
		orderEntity.SellPriceAmount = sellerPriceAmount
		orderEntity.SellPriceDecimal = sellerPriceDecimal
		orderEntity.SellPriceCoin = sellerPriceCoin
		orderEntity.FeeAmount = feeAmount
		orderEntity.FeeRate = feeRate
		orderEntity.Content = content
		orderEntity.Preview = preview
		orderEntity.Detail = tool.AnyToStr(detailMap)
		orderEntity.MakerPsbt = psbtRaw
		orderEntity.Timestamp = nowTime
		err = models.MarketOrderModelDao().Update(orderEntity)
		if err != nil {
			return nil, err
		}
	}

	return &respond.PushOrderResp{
		OrderId:    orderId,
		AssetType:  assetType,
		AssetId:    assetId,
		OrderState: orderEntity.OrderState,
	}, nil
}

func FetchOrderPsbt(req *request.FetchOrderPsbtReq, publicKey, ip string) (*respond.OrderInfo, error) {
	var (
		entity               *models.MarketOrderModel
		err                  error
		resp                 *respond.OrderInfo
		takerPsbtRaw         string = ""
		feeAmountForPlatform int64  = 0
		psbtBuilder          *common.PsbtBuilder
	)

	verified, err := common.CheckPublicKeyAddress(common.GetNetParams(conf.Net), publicKey, req.BuyerAddress)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Check address err: %s. ", err.Error()))
	}
	if !verified {
		return nil, errors.New(fmt.Sprintf("Check address verified: %v. ", verified))
	}

	_ = feeAmountForPlatform
	entity, err = models.MarketOrderModelDao().GetOne(&models.MarketOrderModel{
		OrderId: req.OrderId,
	})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if entity == nil {
		return nil, errors.New("Order is empty. ")
	}
	if entity.OrderState != models.OrderStateCreate {
		return nil, errors.New("Order is closed. ")
	}

	if entity.FeeRate > 0 {
		feeRateDe := decimal.New(int64(entity.FeeRate), -2)
		feeAmountForPlatform = decimal.New(entity.SellPriceAmount, 0).Mul(feeRateDe).IntPart()
		if feeAmountForPlatform < 2000 {
			feeAmountForPlatform = 2000
		}
	} else if entity.FeeAmount > 0 {
		feeAmountForPlatform = entity.FeeAmount
	}

	if entity.SellerAddress == req.BuyerAddress {
		return nil, errors.New("Buyer address is same as seller. ")
	}

	isLegacy, err := common.CheckLegacyAddressType(conf.Net, entity.SellerAddress)
	if err != nil {
		return nil, err
	}

	psbtBuilder, err = common.NewPsbtBuilder(common.GetNetParams(conf.Net), entity.MakerPsbt)
	if err != nil {
		return nil, err
	}

	if isLegacy && len(psbtBuilder.GetInputs()) != 3 {
		return nil, errors.New("Wrong Psbt: This orders was not matched. ")
	}

	if len(psbtBuilder.GetInputs()) == 3 {
		takerPsbtRaw, feeAmountForPlatform, err = common.MakeAskTakerPsbtRawForPreMake(conf.Net, entity.OrderId, entity.MakerPsbt, entity.OutValue, req.BuyerAddress, req.BuyerChangeAmount, feeAmountForPlatform, true)
		if err != nil {
			return nil, err
		}
	} else {
		takerPsbtRaw, feeAmountForPlatform, err = common.MakeAskTakerPsbtRaw(conf.Net, entity.OrderId, entity.MakerPsbt, entity.OutValue, req.BuyerAddress, req.BuyerChangeAmount, feeAmountForPlatform, true)
		if err != nil {
			return nil, err
		}
	}

	resp = &respond.OrderInfo{
		OrderId:          entity.OrderId,
		UtxoId:           entity.UtxoId,
		AssetType:        entity.AssetType,
		AssetId:          entity.AssetId,
		AssetNumber:      entity.AssetNumber,
		OrderState:       entity.OrderState,
		SellerAddress:    entity.SellerAddress,
		Seller:           nil,
		BuyerAddress:     entity.BuyerAddress,
		Buyer:            nil,
		SellPriceAmount:  entity.SellPriceAmount,
		SellPriceDecimal: entity.SellPriceDecimal,
		SellPriceCoin:    entity.SellPriceCoin,
		Fee:              entity.FeeAmount,
		//Fee:      feeAmountForPlatform,
		FeeRate:  entity.FeeRate,
		Content:  entity.Content,
		Preview:  entity.Preview,
		Detail:   entity.Detail,
		TakePsbt: takerPsbtRaw,
	}
	return resp, nil
}

func TakeMarketOrder(req *request.TakeOrderReq, publicKey, ip string) (*respond.TakeOrderResp, error) {
	var (
		entity              *models.MarketOrderModel
		err                 error
		takerAskPsbtBuilder *common.PsbtBuilder
		usedUtxoList        []*models.MarketUtxoModel = make([]*models.MarketUtxoModel, 0)
		newUtxoList         []*models.MarketUtxoModel = make([]*models.MarketUtxoModel, 0)
		nowTime             int64                     = tool.MakeTimestamp()
	)
	entity, err = models.MarketOrderModelDao().GetOne(&models.MarketOrderModel{
		OrderId: req.OrderId,
	})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if entity == nil {
		return nil, errors.New("Order is empty. ")
	}
	if entity.OrderState != models.OrderStateCreate {
		return nil, errors.New("Order is closed. ")
	}

	takerAskPsbtBuilder, err = common.NewPsbtBuilder(common.GetNetParams(conf.Net), req.TakerPsbtRaw)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("PSBT: NewPsbtBuilder err:%s", err.Error()))
	}
	err = common.SignAskTakerPsbtRawInDummy(conf.Net, takerAskPsbtBuilder)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("SignAskTakerPsbtRawInDummy err:%s", err.Error()))
	}
	finalAskPsbtRaw, err := takerAskPsbtBuilder.ToString()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("PSBT(X): ToString err:%s", err.Error()))
	}

	txRaw, err := takerAskPsbtBuilder.ExtractPsbtTransaction()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("PSBT: ExtractPsbtTransaction err:%s", err.Error()))
	}
	txRawByte, _ := hex.DecodeString(txRaw)
	txAsk := wire.NewMsgTx(2)
	err = txAsk.Deserialize(bytes.NewReader(txRawByte))
	if err != nil {
		return nil, errors.New(fmt.Sprintf("txAsk Deserialize err: %v. ", err.Error()))
	}
	txId := txAsk.TxHash().String()

	entity.FinalPsbt = finalAskPsbtRaw
	entity.TxId = txId

	if len(takerAskPsbtBuilder.GetInputs()) < 5 {
		return nil, errors.New(fmt.Sprintf("PSBT: No match inputs length err"))
	}

	for i, v := range takerAskPsbtBuilder.GetInputs() {
		if i == 0 || i == 1 || i == 3 {
			utxoId := fmt.Sprintf("%s_%d", v.PreviousOutPoint.Hash.String(), v.PreviousOutPoint.Index)
			utxoEntity, err := models.MarketUtxoModelDao().GetOne(&models.MarketUtxoModel{
				UtxoId: utxoId,
			})
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, err
			}
			if utxoEntity == nil {
				return nil, errors.New(fmt.Sprintf("Utxo not found: %s. ", utxoId))
			}
			if utxoEntity.UsedState != models.UsedNo {
				return nil, errors.New(fmt.Sprintf("Utxo used: %s. ", utxoId))
			}
			utxoEntity.UsedState = models.UsedYes
			utxoEntity.UsedTxId = txId
			utxoEntity.OrderId = entity.OrderId
			usedUtxoList = append(usedUtxoList, utxoEntity)
		}
	}

	for i, v := range takerAskPsbtBuilder.GetOutputs() {
		if i == 0 || i == 3 || i == 4 {
			utxoType := models.UtxoTypeDummy600
			if i == 0 {
				utxoType = models.UtxoTypeDummy1200
			}
			pkScriptHex := hex.EncodeToString(v.PkScript)
			address, err := common.PkScriptToAddress(conf.Net, pkScriptHex)
			if err != nil {
				return nil, errors.New(fmt.Sprintf("PkScriptToAddress err: %s. ", err.Error()))
			}

			newUtxo := &models.MarketUtxoModel{
				UtxoId:   fmt.Sprintf("%s_%d", txId, i),
				UtxoType: utxoType,
				Amount:   uint64(v.Value),
				Address:  address,
				//PrivateKeyHex:  "",
				TxId:      txId,
				Index:     int64(i),
				PkScript:  pkScriptHex,
				UsedState: models.UsedNo,
				//UsedTxId:       "",
				//OrderId:        "",
				//SortIndex:      0,
				//ConfirmStatus:  0,
				FromOrderId:    entity.OrderId,
				NetworkFeeRate: req.NetworkFeeRate,
				Timestamp:      nowTime,
				//Version:        0,
				CreateTime: nowTime,
				//UpdateTime:     0,
				State: models.STATE_EXIST,
			}
			newUtxoList = append(newUtxoList, newUtxo)
		}
	}

	buyerAddress := ""
	buyerInput := takerAskPsbtBuilder.GetInputs()[4]
	buyerInputTxId := buyerInput.PreviousOutPoint.Hash.String()
	buyerInputIndex := buyerInput.PreviousOutPoint.Index
	utxoInfo := common.GetUtxoInfo(conf.Net, buyerInputTxId, int64(buyerInputIndex))
	if utxoInfo == nil || !utxoInfo.IsExist || utxoInfo.SpendStatus == "spend" {
		return nil, errors.New(fmt.Sprintf("PSBT(take): Utxo is spend. Please select a different utxo. [%s_%d]", buyerInputTxId, int64(buyerInputIndex)))
	}
	buyerAddress = utxoInfo.Address
	entity.BuyerAddress = buyerAddress
	entity.BuyerIp = ip
	entity.OrderState = models.OrderStateFinish
	entity.ConfirmationState = models.ConfirmationStateUnconfirmed

	verified, err := common.CheckPublicKeyAddress(common.GetNetParams(conf.Net), publicKey, buyerAddress)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Check address err: %s. ", err.Error()))
	}
	if !verified {
		return nil, errors.New(fmt.Sprintf("Check address verified: %v. ", verified))
	}
	err = models.MarketOrderModelDao().UpdateOrderEntityListForJobFunc(
		entity,
		usedUtxoList, newUtxoList,
		txRaw,
		common.BroadcastTx)
	if err != nil {
		return nil, err
	}

	return &respond.TakeOrderResp{
		OrderId:    entity.OrderId,
		AssetType:  entity.AssetType,
		AssetId:    entity.AssetId,
		OrderState: entity.OrderState,
		TxId:       txId,
	}, nil
}

func CancelMarketOrder(req *request.CancelOrderReq, publicKey, ip string) (*respond.CancelOrderResp, error) {
	var (
		entity *models.MarketOrderModel
		err    error
	)
	entity, err = models.MarketOrderModelDao().GetOne(&models.MarketOrderModel{
		OrderId: req.OrderId,
	})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if entity == nil {
		return nil, errors.New("Order is empty. ")
	}
	verified, err := common.CheckPublicKeyAddress(common.GetNetParams(conf.Net), publicKey, entity.SellerAddress)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Check address err: %s. ", err.Error()))
	}
	if !verified {
		return nil, errors.New(fmt.Sprintf("Check address verified: %v. ", verified))
	}
	if entity.OrderState != models.OrderStateCreate {
		return nil, errors.New("Order state is not create. ")
	}
	entity.OrderState = models.OrderStateCancel
	err = models.MarketOrderModelDao().Update(entity)
	if err != nil {
		return nil, err
	}
	return &respond.CancelOrderResp{
		OrderId:    entity.OrderId,
		AssetType:  entity.AssetType,
		AssetId:    entity.AssetId,
		OrderState: entity.OrderState,
	}, nil

}

func FetchMarketOrders(req *request.FetchMarketOrdersReq, publicKey, ip string) (*respond.OrderListResp, error) {
	var (
		total      int64 = 0
		entityList []*models.MarketOrderModel
		list       []*respond.OrderInfo     = make([]*respond.OrderInfo, 0)
		filter     *models.MarketOrderModel = &models.MarketOrderModel{}
		sortKey    string                   = "timestamp"
		sortType   string                   = "desc"
	)
	if req.OrderState != 0 {
		filter.OrderState = req.OrderState
	}
	if req.AssetType != "" {
		filter.AssetType = req.AssetType
	}
	if req.Address != "" {
		filter.SellerAddress = req.Address
	}
	if req.SortKey != "" {
		sortKey = req.SortKey
	}
	if req.SortType != 0 {
		if req.SortType > 0 {
			sortType = "asc"
		} else {
			sortType = "desc"
		}
	}
	if req.Size <= 0 || req.Size >= 50 {
		req.Size = 50
	}
	total, _ = models.MarketOrderModelDao().CountByState(filter, req.Address)
	entityList, _ = models.MarketOrderModelDao().GetListByState(filter, req.Address, req.Cursor, req.Size, sortKey, sortType)
	for _, v := range entityList {
		item := &respond.OrderInfo{
			OrderId:           v.OrderId,
			UtxoId:            v.UtxoId,
			OutValue:          v.OutValue,
			AssetType:         v.AssetType,
			AssetId:           v.AssetId,
			AssetNumber:       v.AssetNumber,
			AssetPop:          v.AssetPop,
			AssetLevel:        v.AssetLevel,
			OrderState:        v.OrderState,
			SellerAddress:     v.SellerAddress,
			Seller:            common.FetchMetaIDUserInfo(v.SellerAddress),
			BuyerAddress:      v.BuyerAddress,
			Buyer:             common.FetchMetaIDUserInfo(v.BuyerAddress),
			SellPriceAmount:   v.SellPriceAmount,
			SellPriceDecimal:  v.SellPriceDecimal,
			SellPriceCoin:     v.SellPriceCoin,
			Fee:               v.FeeAmount,
			FeeRate:           v.FeeRate,
			Content:           v.Content,
			Preview:           v.Preview,
			Detail:            v.Detail,
			BlockHeight:       v.BlockHeight,
			ConfirmationState: v.ConfirmationState,
			DealTime:          v.DealTime,
			TxId:              v.TxId,
		}
		list = append(list, item)
	}
	return &respond.OrderListResp{
		Total: total,
		List:  list,
	}, nil
}

func FetchMarketOneOrder(req *request.FetchMarketOneOrderReq, publicKey, ip string) (*respond.OrderInfo, error) {
	var (
		entity *models.MarketOrderModel
		err    error
		resp   *respond.OrderInfo
	)
	entity, err = models.MarketOrderModelDao().GetOne(&models.MarketOrderModel{
		OrderId: req.OrderId,
	})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if entity != nil {
		resp = &respond.OrderInfo{
			OrderId:           entity.OrderId,
			UtxoId:            entity.UtxoId,
			AssetType:         entity.AssetType,
			AssetId:           entity.AssetId,
			AssetNumber:       entity.AssetNumber,
			AssetPop:          entity.AssetPop,
			AssetLevel:        entity.AssetLevel,
			OrderState:        entity.OrderState,
			SellerAddress:     entity.SellerAddress,
			Seller:            common.FetchMetaIDUserInfo(entity.SellerAddress),
			BuyerAddress:      entity.BuyerAddress,
			Buyer:             common.FetchMetaIDUserInfo(entity.BuyerAddress),
			SellPriceAmount:   entity.SellPriceAmount,
			SellPriceDecimal:  entity.SellPriceDecimal,
			SellPriceCoin:     entity.SellPriceCoin,
			Fee:               entity.FeeAmount,
			FeeRate:           entity.FeeRate,
			Content:           entity.Content,
			Preview:           entity.Preview,
			Detail:            entity.Detail,
			BlockHeight:       entity.BlockHeight,
			ConfirmationState: entity.ConfirmationState,
			DealTime:          entity.DealTime,
			TxId:              entity.TxId,
		}
	}
	return resp, nil
}

func FetchAssetDetail(req *request.FetchAssetDetailReq, publicKey, ip string) (*respond.OrderInfo, error) {
	var (
		orderEntity *models.MarketOrderModel
		assetInfo   *man_service.PinInfo
		err         error
	)
	assetInfo, err = man_service.FetchPinInfo(conf.Net, req.AssetId)
	if err != nil {
		return nil, err
	}
	location := assetInfo.Location
	locationStrs := strings.Split(location, ":")
	utxoId := ""
	if len(locationStrs) >= 2 {
		utxoId = fmt.Sprintf("%s_%s", locationStrs[0], locationStrs[1])
	}

	detailMap := map[string]interface{}{
		"pinId":              assetInfo.Id,
		"metaid":             assetInfo.Metaid,
		"createAddress":      assetInfo.CreateAddress,
		"operation":          assetInfo.Operation,
		"pinNumber":          assetInfo.Number,
		"pop":                assetInfo.Pop,
		"popSummary":         common.GenPopSummary(assetInfo.Pop),
		"popLv":              assetInfo.PopLv,
		"path":               assetInfo.Path,
		"originalPath":       assetInfo.OriginalPath,
		"version":            assetInfo.Version,
		"encryption":         assetInfo.Encryption,
		"outputValue":        assetInfo.OutputValue,
		"contentLength":      assetInfo.ContentLength,
		"contentTypeDetect":  assetInfo.ContentTypeDetect,
		"timestamp":          assetInfo.Timestamp,
		"genesisHeight":      assetInfo.GenesisHeight,
		"genesisTransaction": assetInfo.GenesisTransaction,
	}
	resp := &respond.OrderInfo{
		OrderId:          "",
		UtxoId:           utxoId,
		OutValue:         assetInfo.OutputValue,
		AssetType:        models.AssetTypePins,
		AssetId:          assetInfo.Id,
		AssetNumber:      assetInfo.Number,
		AssetLevel:       int64(assetInfo.PopLv),
		AssetPop:         common.GenPopSummary(assetInfo.Pop),
		OrderState:       0,
		HolderAddress:    assetInfo.Address,
		Holder:           nil,
		SellerAddress:    "",
		Seller:           nil,
		BuyerAddress:     "",
		Buyer:            nil,
		PinStatus:        assetInfo.Status,
		SellPriceAmount:  0,
		SellPriceDecimal: 0,
		SellPriceCoin:    "",
		Fee:              0,
		FeeRate:          0,
		Content:          assetInfo.Content,
		Preview:          assetInfo.Preview,
		Detail:           tool.AnyToStr(detailMap),
	}

	orderEntity, _ = models.MarketOrderModelDao().GetOne(&models.MarketOrderModel{
		UtxoId:     utxoId,
		OrderState: models.OrderStateCreate,
	})
	if orderEntity != nil {
		resp.OrderId = orderEntity.OrderId
		resp.OrderState = orderEntity.OrderState
		resp.SellerAddress = orderEntity.SellerAddress
		resp.Seller = common.FetchMetaIDUserInfo(orderEntity.SellerAddress)
		resp.SellPriceAmount = orderEntity.SellPriceAmount
		resp.SellPriceDecimal = orderEntity.SellPriceDecimal
		resp.SellPriceCoin = orderEntity.SellPriceCoin
		resp.Fee = orderEntity.FeeAmount
		resp.FeeRate = orderEntity.FeeRate
	}
	return resp, nil
}

func FetchAddressAssetList(req *request.FetchAddressAssetListReq, publicKey, ip string) (*respond.AddressAssetListResp, error) {
	var (
		pinListResp *man_service.PinInfoList
		err         error
		list        []*respond.OrderInfo = make([]*respond.OrderInfo, 0)
		total       int64                = 0
	)

	verified, err := common.CheckPublicKeyAddress(common.GetNetParams(conf.Net), publicKey, req.Address)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Check address err: %s. ", err.Error()))
	}
	if !verified {
		return nil, errors.New(fmt.Sprintf("Check address verified: %v. ", verified))
	}

	pinListResp, err = man_service.FetchPins(conf.Net, "owner", req.Address, req.Cursor, req.Size, true)
	if err != nil {
		return nil, err
	}

	total = pinListResp.Total
	for _, v := range pinListResp.List {
		location := v.Location
		locationStrs := strings.Split(location, ":")
		utxoId := ""
		if len(locationStrs) >= 2 {
			utxoId = fmt.Sprintf("%s_%s", locationStrs[0], locationStrs[1])
		}

		orderEntityFinish, _ := models.MarketOrderModelDao().GetOne(&models.MarketOrderModel{
			UtxoId:        utxoId,
			SellerAddress: req.Address,
			OrderState:    models.OrderStateFinish,
		})
		if orderEntityFinish != nil {
			continue
		}

		detailMap := map[string]interface{}{
			"pinId":              v.Id,
			"metaid":             v.Metaid,
			"createAddress":      v.CreateAddress,
			"operation":          v.Operation,
			"pinNumber":          v.Number,
			"pop":                v.Pop,
			"popSummary":         common.GenPopSummary(v.Pop),
			"popLv":              v.PopLv,
			"path":               v.Path,
			"originalPath":       v.OriginalPath,
			"version":            v.Version,
			"encryption":         v.Encryption,
			"outputValue":        v.OutputValue,
			"contentLength":      v.ContentLength,
			"contentTypeDetect":  v.ContentTypeDetect,
			"timestamp":          v.Timestamp,
			"genesisHeight":      v.GenesisHeight,
			"genesisTransaction": v.GenesisTransaction,
		}
		item := &respond.OrderInfo{
			OrderId:          "",
			UtxoId:           utxoId,
			OutValue:         v.OutputValue,
			AssetType:        models.AssetTypePins,
			AssetId:          v.Id,
			AssetNumber:      v.Number,
			AssetLevel:       int64(v.PopLv),
			AssetPop:         common.GenPopSummary(v.Pop),
			OrderState:       0,
			HolderAddress:    v.Address,
			Holder:           nil,
			SellerAddress:    "",
			Seller:           nil,
			BuyerAddress:     "",
			Buyer:            nil,
			PinStatus:        v.Status,
			SellPriceAmount:  0,
			SellPriceDecimal: 0,
			SellPriceCoin:    "",
			Fee:              0,
			FeeRate:          0,
			Content:          v.Content,
			Preview:          v.Preview,
			Detail:           tool.AnyToStr(detailMap),
		}

		orderEntity, _ := models.MarketOrderModelDao().GetOne(&models.MarketOrderModel{
			UtxoId:        utxoId,
			SellerAddress: req.Address,
			OrderState:    models.OrderStateCreate,
		})
		if orderEntity != nil {
			item.OrderId = orderEntity.OrderId
			item.OrderState = orderEntity.OrderState
			item.SellerAddress = orderEntity.SellerAddress
			item.Seller = common.FetchMetaIDUserInfo(orderEntity.SellerAddress)
			item.SellPriceAmount = orderEntity.SellPriceAmount
			item.SellPriceDecimal = orderEntity.SellPriceDecimal
			item.SellPriceCoin = orderEntity.SellPriceCoin
			item.Fee = orderEntity.FeeAmount
			item.FeeRate = orderEntity.FeeRate
		}

		list = append(list, item)
	}
	return &respond.AddressAssetListResp{
		Total: total,
		List:  list,
	}, nil
}

func TestAuth(req *request.TestAuthReq, publicKey string) (string, error) {
	verified, err := common.CheckPublicKeyAddress(common.GetNetParams(conf.Net), publicKey, req.Address)
	if err != nil {
		return "", errors.New(fmt.Sprintf("Check address err: %s. ", err.Error()))
	}
	if !verified {
		return "", errors.New(fmt.Sprintf("Check address verified: %v. ", verified))
	}
	return "success", nil
}
