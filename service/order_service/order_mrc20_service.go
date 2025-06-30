package order_service

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"metaid-market-service/common"
	"metaid-market-service/conf"
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
	"metaid-market-service/models"
	"metaid-market-service/service/common_service"
	"metaid-market-service/service/man_service"
	"metaid-market-service/tool"
	"strconv"
	"strings"

	"github.com/btcsuite/btcd/wire"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func PushMarketMrc20Order(req *request.PushMrc20OrderReq, publicKey, ip string) (*respond.PushMrc20OrderResp, error) {
	var (
		orderEntity *models.MarketMrc20OrderModel
		psbtBuilder *common.PsbtBuilder
		err         error
		orderId     string = ""
		tickId      string = req.TickId
		mrc20UtxoId string = ""
		outValue    int64  = 0

		tick              string  = ""
		tokenName         string  = ""
		decimals          int64   = 0
		chain             string  = ""
		amount            int64   = 0
		amountStr         string  = ""
		tokenPriceRate    float64 = 0
		tokenPriceRateStr string  = ""
		priceAmount       int64   = 0
		priceDecimal      int64   = 8
		priceCoin         string  = "BTC"

		assetType        models.AssetType = req.AssetType
		sellerAddress    string           = req.Address
		psbtRaw          string           = req.PsbtRaw
		askType          models.AskType   = req.AskType
		reqCoinAmountStr string           = req.CoinAmountStr
		reqUtxoOutValue  int64            = req.UtxoOutValue
		nowTime          int64            = tool.MakeTimestamp()

		feeAmount int64 = 2000
		feeRate   int64 = 1

		supply string = "0"
	)
	if req.PsbtRaw == "" {
		return nil, errors.New("Wrong Psbt: empty. ")
	}
	if assetType != models.AssetTypeMrc20 {
		return nil, errors.New("Wrong AssetType. ")
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
	if isLegacy && len(preOutList) != 2 {
		return nil, errors.New("Wrong Psbt: inputs not match. ")
	}

	//check inputs and asset
	for i, v := range preOutList {
		if len(preOutList) == 2 && i != 1 {
			continue
		}
		preTxId := v.PreviousOutPoint.Hash.String()
		preTxIndex := v.PreviousOutPoint.Index
		mrc20UtxoId = fmt.Sprintf("%s_%d", preTxId, preTxIndex)

		var tickInfo *common_service.TickInfo
		if askType == models.AskTypeNone {
			mrc20InfoList, mrc20InfoTotal, err := common_service.FetchTxPointInfo(preTxId, int64(preTxIndex), 0, 100)
			if err != nil {
				return nil, err
			}
			if mrc20InfoTotal > 1 {
				return nil, errors.New("Wrong Psbt: this mrc20 info is not unique. ")
			}

			mrc20Info := mrc20InfoList[0]

			tick = mrc20Info.Tick
			if tickId != mrc20Info.Mrc20Id {
				return nil, errors.New("Wrong Psbt: tickId not match. ")
			}
			tickInfo, err = common_service.GetMrc20TickInfo(tickId, "")
			if err != nil {
				return nil, err
			}
			tokenName = tickInfo.TokenName
			decimals, _ = strconv.ParseInt(tickInfo.Decimals, 10, 64)
			chain = mrc20Info.Chain

			amountStr = mrc20Info.AmtChange
			amount, _ = strconv.ParseInt(amountStr, 10, 64)
			outValue = mrc20Info.PointValue
		} else if askType == models.AskTypePreTransfer {
			if reqCoinAmountStr == "" || reqUtxoOutValue == 0 {
				return nil, errors.New("Wrong Psbt: empty coinAmountStr or outValue if askType is PreTransfer. ")
			}
			tickInfo, err = common_service.GetMrc20TickInfo(tickId, "")
			if err != nil {
				return nil, err
			}
			tick = tickInfo.Tick
			tokenName = tickInfo.TokenName
			decimals, _ = strconv.ParseInt(tickInfo.Decimals, 10, 64)
			chain = tickInfo.Chain

			amountStr = reqCoinAmountStr
			//amount, _ = strconv.ParseInt(amountStr, 10, 64)
			amountDe, _ := decimal.NewFromString(amountStr)
			amount = amountDe.IntPart()
			outValue = reqUtxoOutValue
		} else {
			return nil, errors.New("Wrong AskType. ")
		}

		totalMintDe, _ := decimal.NewFromString(tickInfo.TotalMinted)
		amtPerMintDe, _ := decimal.NewFromString(tickInfo.AmtPerMint)
		if totalMintDe.GreaterThan(decimal.Zero) {
			supplyDe := totalMintDe.Mul(amtPerMintDe)
			supply = supplyDe.String()
		}
	}

	for i, v := range outList {
		if len(preOutList) == 2 && i != 1 {
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
		priceAmount = v.Value
		priceDecimal = 8
		priceCoin = "BTC"
	}
	//if !psbtBuilder.IsComplete() {
	//	return nil, errors.New("Wrong Psbt: incomplete. ")
	//}
	amountDe, _ := decimal.NewFromString(amountStr)
	priceAmountDe := decimal.New(priceAmount, 0)
	tokenPriceRateDe := priceAmountDe.Div(amountDe)
	tokenPriceRateStr = tokenPriceRateDe.StringFixed(6)
	tokenPriceRate, _ = tokenPriceRateDe.Float64()
	//if tokenPriceRate < 1 {
	//	return nil, errors.New("Wrong Psbt: token price rate < 1. ")
	//}

	orderId = fmt.Sprintf("%s_%s_%s_%s", mrc20UtxoId, tickId, assetType, sellerAddress)
	orderId = hex.EncodeToString(tool.SHA256([]byte(orderId)))
	orderEntity, err = models.MarketMrc20OrderModelDao().GetOne(&models.MarketMrc20OrderModel{
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
		orderEntity = &models.MarketMrc20OrderModel{
			OrderId:           orderId,
			UtxoId:            mrc20UtxoId,
			AssetType:         assetType,
			OutValue:          outValue,
			TickId:            tickId,
			Tick:              tick,
			TokenName:         tokenName,
			Decimals:          decimals,
			Chain:             chain,
			Amount:            amount,
			AmountStr:         amountStr,
			TokenPriceRate:    tokenPriceRate,
			TokenPriceRateStr: tokenPriceRateStr,
			PriceAmount:       priceAmount,
			PriceDecimal:      priceDecimal,
			PriceCoin:         priceCoin,
			AskType:           askType,
			OrderState:        models.OrderStateCreate,
			SellerAddress:     sellerAddress,
			SellerIp:          ip,
			//BuyerAddress:      "",
			//BuyerIp:           "",
			FeeAmount: feeAmount,
			FeeRate:   feeRate,
			MakerPsbt: psbtRaw,
			//TakerPsbt:         "",
			//FinalPsbt:         "",
			//TxId:              "",
			//DealTime:          0,
			//BlockHeight:       0,
			ConfirmationState: models.ConfirmationStateUnconfirmed,
			Timestamp:         nowTime,
			Version:           0,
			CreateTime:        nowTime,
			UpdateTime:        0,
			State:             models.STATE_EXIST,
		}
		//err = models.MarketMrc20OrderModelDao().Set(orderEntity)
		err = models.MarketMrc20OrderModelDao().SetForPushOrder(orderEntity, supply)
		if err != nil {
			return nil, err
		}
	} else {
		orderEntity.OutValue = outValue
		orderEntity.AssetType = assetType
		orderEntity.TickId = tickId
		orderEntity.Tick = tick
		orderEntity.Chain = chain
		orderEntity.Amount = amount
		orderEntity.AmountStr = amountStr
		orderEntity.TokenPriceRate = tokenPriceRate
		orderEntity.TokenPriceRateStr = tokenPriceRateStr
		orderEntity.PriceAmount = priceAmount
		orderEntity.PriceDecimal = priceDecimal
		orderEntity.PriceCoin = priceCoin
		orderEntity.AskType = askType
		orderEntity.OrderState = models.OrderStateCreate
		orderEntity.SellerIp = ip
		orderEntity.FeeAmount = feeAmount
		orderEntity.FeeRate = feeRate
		orderEntity.MakerPsbt = psbtRaw
		orderEntity.Timestamp = nowTime
		//err = models.MarketMrc20OrderModelDao().Update(orderEntity)
		err = models.MarketMrc20OrderModelDao().UpdateForPushAndCancel(orderEntity, supply)
		if err != nil {
			return nil, err
		}
	}

	return &respond.PushMrc20OrderResp{
		OrderId:    orderId,
		AssetType:  assetType,
		TickId:     tickId,
		OrderState: orderEntity.OrderState,
	}, nil
}

func FetchMrc20OrderPsbt(req *request.FetchMrc20OrderPsbtReq, publicKey, ip string) (*respond.Mrc20OrderInfo, error) {
	var (
		entity               *models.MarketMrc20OrderModel
		err                  error
		resp                 *respond.Mrc20OrderInfo
		takerPsbtRaw         string = ""
		feeAmountForPlatform int64  = 0
		feeRateStr           string = ""
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
	entity, err = models.MarketMrc20OrderModelDao().GetOne(&models.MarketMrc20OrderModel{
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

	if entity.AskType == models.AskTypePreTransfer {
		return nil, errors.New("The asset-utxo of this order is waiting for confirmation. " +
			"Please wait for the confirmation or select a different order. ")
	}

	feeAmountForPlatform, _, feeRateStr, _ = common.GetPlatformMrc20TradeServiceFee(int64(entity.PriceAmount))

	//if entity.FeeRate > 0 {
	//	feeRateDe := decimal.New(int64(entity.FeeRate), -2)
	//	feeAmountForPlatform = decimal.New(entity.PriceAmount, 0).Mul(feeRateDe).IntPart()
	//	if feeAmountForPlatform < 2000 {
	//		feeAmountForPlatform = 2000
	//	}
	//} else if entity.FeeAmount > 0 {
	//	feeAmountForPlatform = entity.FeeAmount
	//}

	if entity.SellerAddress == req.BuyerAddress {
		return nil, errors.New("Buyer address is same as seller. ")
	}

	sellUtxoIdStrs := strings.Split(entity.UtxoId, "_")
	sellUtxoTxId := sellUtxoIdStrs[0]
	sellUtxoIndex, _ := strconv.ParseInt(sellUtxoIdStrs[1], 10, 64)
	utxoInfo := common.GetUtxoInfo(conf.Net, sellUtxoTxId, sellUtxoIndex)
	if utxoInfo == nil {
		return nil, errors.New(fmt.Sprintf("sell Utxo not found. [%s]", entity.UtxoId))
	}
	if !utxoInfo.IsExist || utxoInfo.SpendStatus == "spend" {
		entity.OrderState = models.OrderStateCancel
		err := models.MarketMrc20OrderModelDao().Update(entity)
		if err != nil {
			fmt.Printf("[%s] UpdateEntityForConfirm error: %v\n", entity.OrderId, err)
			return nil, err
		}
		return nil, errors.New(fmt.Sprintf("sell Utxo is spend. Please select a different order. [%s]", entity.UtxoId))
	}

	isLegacy, err := common.CheckLegacyAddressType(conf.Net, entity.SellerAddress)
	if err != nil {
		return nil, err
	}

	psbtBuilder, err = common.NewPsbtBuilder(common.GetNetParams(conf.Net), entity.MakerPsbt)
	if err != nil {
		return nil, err
	}

	if isLegacy && len(psbtBuilder.GetInputs()) != 2 {
		return nil, errors.New("Wrong Psbt: This orders was not matched. ")
	}

	if len(psbtBuilder.GetInputs()) == 2 {
		takerPsbtRaw, feeAmountForPlatform, err = common.MakeMrc20AskTakerPsbtRawForPreMake(conf.Net, entity.OrderId, entity.MakerPsbt, entity.OutValue, req.BuyerAddress, req.BuyerChangeAmount, feeAmountForPlatform, true)
		if err != nil {
			return nil, err
		}
	} else {
		takerPsbtRaw, feeAmountForPlatform, err = common.MakeMrc20AskTakerPsbtRaw(conf.Net, entity.OrderId, entity.MakerPsbt, entity.OutValue, req.BuyerAddress, req.BuyerChangeAmount, feeAmountForPlatform, true)
		if err != nil {
			return nil, err
		}
	}

	resp = &respond.Mrc20OrderInfo{
		OrderId:           entity.OrderId,
		UtxoId:            entity.UtxoId,
		OutValue:          entity.OutValue,
		AssetType:         entity.AssetType,
		OrderState:        entity.OrderState,
		SellerAddress:     entity.SellerAddress,
		Seller:            nil,
		BuyerAddress:      entity.BuyerAddress,
		Buyer:             nil,
		TickId:            entity.TickId,
		Tick:              entity.Tick,
		TokenName:         entity.TokenName,
		Decimals:          entity.Decimals,
		Chain:             entity.Chain,
		Amount:            entity.Amount,
		AmountStr:         entity.AmountStr,
		TokenPriceRate:    entity.TokenPriceRate,
		TokenPriceRateStr: entity.TokenPriceRateStr,
		PriceAmount:       entity.PriceAmount,
		PriceDecimal:      entity.PriceDecimal,
		PriceCoin:         entity.PriceCoin,
		Fee:               feeAmountForPlatform,
		//Fee:      feeAmountForPlatform,
		FeeRate:    entity.FeeRate,
		FeeRateStr: feeRateStr,
		TakePsbt:   takerPsbtRaw,
	}
	return resp, nil
}

func TakeMarketMrc20Order(req *request.TakeMrc20OrderReq, publicKey, ip string) (*respond.TakeMrc20OrderResp, error) {
	var (
		entity              *models.MarketMrc20OrderModel
		err                 error
		takerAskPsbtBuilder *common.PsbtBuilder
		usedUtxoList        []*models.MarketUtxoModel = make([]*models.MarketUtxoModel, 0)
		newUtxoList         []*models.MarketUtxoModel = make([]*models.MarketUtxoModel, 0)
		nowTime             int64                     = tool.MakeTimestamp()
		tickInfo            *man_service.Mrc20TickInfo
		supply              = "0"
	)
	entity, err = models.MarketMrc20OrderModelDao().GetOne(&models.MarketMrc20OrderModel{
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

	tickInfo, err = man_service.FetchMrc20TickInfo(entity.TickId, "")
	if err != nil {
		return nil, err
	}
	if tickInfo == nil || tickInfo.Mrc20Id == "" {
		return nil, errors.New("mrc20 not found")
	}
	if tickInfo.AmtPerMint != "" && tickInfo.MintCount != 0 {
		mintCountDe := decimal.New(tickInfo.MintCount, 0)
		totalMintedDe := decimal.New(tickInfo.TotalMinted, 0)
		amtPerMintDe, _ := decimal.NewFromString(tickInfo.AmtPerMint)
		if mintCountDe.GreaterThan(totalMintedDe) {
			supplyDe := totalMintedDe.Mul(amtPerMintDe)
			supply = supplyDe.String()
		} else {
			supplyDe := mintCountDe.Mul(amtPerMintDe)
			supply = supplyDe.String()
		}
	}

	takerAskPsbtBuilder, err = common.NewPsbtBuilder(common.GetNetParams(conf.Net), req.TakerPsbtRaw)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("PSBT: NewPsbtBuilder err:%s", err.Error()))
	}
	err = common.SignMrc20AskTakerPsbtRawInDummy(conf.Net, takerAskPsbtBuilder)
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

	if len(takerAskPsbtBuilder.GetInputs()) < 3 {
		return nil, errors.New(fmt.Sprintf("PSBT: No match inputs length err"))
	}

	for i, v := range takerAskPsbtBuilder.GetInputs() {
		if i == 0 {
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
		if i == 2 {
			utxoType := models.UtxoTypeDummy600
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
	buyerInput := takerAskPsbtBuilder.GetInputs()[2]
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

	feeAmountForPlatform, feeRate, _, _ := common.GetPlatformMrc20TradeServiceFee(int64(entity.PriceAmount))
	entity.FeeRate = feeRate
	entity.FeeAmount = feeAmountForPlatform

	verified, err := common.CheckPublicKeyAddress(common.GetNetParams(conf.Net), publicKey, buyerAddress)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Check address err: %s. ", err.Error()))
	}
	if !verified {
		return nil, errors.New(fmt.Sprintf("Check address verified: %v. ", verified))
	}
	err = models.MarketMrc20OrderModelDao().UpdateOrderEntityListForJobFunc(
		entity,
		usedUtxoList, newUtxoList,
		supply,
		txRaw,
		common.BroadcastTx)
	if err != nil {
		return nil, err
	}

	return &respond.TakeMrc20OrderResp{
		OrderId:    entity.OrderId,
		AssetType:  entity.AssetType,
		TickId:     entity.TickId,
		OrderState: entity.OrderState,
		TxId:       txId,
	}, nil
}

func CancelMarketMrc20Order(req *request.CancelMrc20OrderReq, publicKey, ip string) (*respond.CancelMrc20OrderResp, error) {
	var (
		entity *models.MarketMrc20OrderModel
		err    error
	)
	entity, err = models.MarketMrc20OrderModelDao().GetOne(&models.MarketMrc20OrderModel{
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

	supply := "0"
	tickInfo, err := common_service.GetMrc20TickInfo(entity.TickId, "")
	if err != nil {
		return nil, err
	}
	totalMintDe, _ := decimal.NewFromString(tickInfo.TotalMinted)
	amtPerMintDe, _ := decimal.NewFromString(tickInfo.AmtPerMint)
	if totalMintDe.GreaterThan(decimal.Zero) {
		supplyDe := totalMintDe.Mul(amtPerMintDe)
		supply = supplyDe.String()
	}

	err = models.MarketMrc20OrderModelDao().UpdateForPushAndCancel(entity, supply)
	if err != nil {
		return nil, err
	}
	return &respond.CancelMrc20OrderResp{
		OrderId:    entity.OrderId,
		AssetType:  entity.AssetType,
		TickId:     entity.TickId,
		OrderState: entity.OrderState,
	}, nil

}

func FetchMarketMrc20Orders(req *request.FetchMarketMrc20OrdersReq, publicKey, ip string) (*respond.Mrc20OrderListResp, error) {
	var (
		total      int64 = 0
		entityList []*models.MarketMrc20OrderModel
		list       []*respond.Mrc20OrderInfo     = make([]*respond.Mrc20OrderInfo, 0)
		filter     *models.MarketMrc20OrderModel = &models.MarketMrc20OrderModel{}
		sortKey    string                        = "timestamp"
		sortType   string                        = "desc"
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
	if req.TickId != "" {
		filter.TickId = req.TickId
	}
	total, _ = models.MarketMrc20OrderModelDao().CountByState(filter, req.Address)
	entityList, _ = models.MarketMrc20OrderModelDao().GetListByState(filter, req.Address, req.Cursor, req.Size, sortKey, sortType)
	for _, v := range entityList {
		item := &respond.Mrc20OrderInfo{
			OrderId:           v.OrderId,
			UtxoId:            v.UtxoId,
			OutValue:          v.OutValue,
			AskType:           v.AskType,
			AssetType:         v.AssetType,
			OrderState:        v.OrderState,
			SellerMetaId:      common.GetMetaIdByAddress(v.SellerAddress),
			SellerAddress:     v.SellerAddress,
			Seller:            common.FetchMetaIDUserInfo(v.SellerAddress),
			BuyerMetaId:       common.GetMetaIdByAddress(v.BuyerAddress),
			BuyerAddress:      v.BuyerAddress,
			Buyer:             common.FetchMetaIDUserInfo(v.BuyerAddress),
			TickId:            v.TickId,
			Tick:              v.Tick,
			TokenName:         v.TokenName,
			Decimals:          v.Decimals,
			Chain:             v.Chain,
			Amount:            v.Amount,
			AmountStr:         v.AmountStr,
			TokenPriceRate:    v.TokenPriceRate,
			TokenPriceRateStr: v.TokenPriceRateStr,
			PriceAmount:       v.PriceAmount,
			PriceDecimal:      v.PriceDecimal,
			PriceCoin:         v.PriceCoin,
			Fee:               v.FeeAmount,
			FeeRate:           v.FeeRate,
			TakePsbt:          v.MakerPsbt,
			BlockHeight:       v.BlockHeight,
			ConfirmationState: v.ConfirmationState,
			DealTime:          v.DealTime,
			TxId:              v.TxId,
		}
		list = append(list, item)
	}
	return &respond.Mrc20OrderListResp{
		Total: total,
		List:  list,
	}, nil
}

func FetchMarketMrc20OneOrder(req *request.FetchMarketMrc20OneOrderReq, publicKey, ip string) (*respond.Mrc20OrderInfo, error) {
	var (
		entity *models.MarketMrc20OrderModel
		err    error
		resp   *respond.Mrc20OrderInfo
	)
	entity, err = models.MarketMrc20OrderModelDao().GetOne(&models.MarketMrc20OrderModel{
		OrderId: req.OrderId,
	})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if entity != nil {
		resp = &respond.Mrc20OrderInfo{
			OrderId:           entity.OrderId,
			UtxoId:            entity.UtxoId,
			OutValue:          entity.OutValue,
			AssetType:         entity.AssetType,
			OrderState:        entity.OrderState,
			SellerMetaId:      common.GetMetaIdByAddress(entity.SellerAddress),
			SellerAddress:     entity.SellerAddress,
			Seller:            common.FetchMetaIDUserInfo(entity.SellerAddress),
			BuyerMetaId:       common.GetMetaIdByAddress(entity.BuyerAddress),
			BuyerAddress:      entity.BuyerAddress,
			Buyer:             common.FetchMetaIDUserInfo(entity.BuyerAddress),
			TickId:            entity.TickId,
			Tick:              entity.Tick,
			TokenName:         entity.TokenName,
			Decimals:          entity.Decimals,
			Chain:             entity.Chain,
			Amount:            entity.Amount,
			AmountStr:         entity.AmountStr,
			TokenPriceRate:    entity.TokenPriceRate,
			TokenPriceRateStr: entity.TokenPriceRateStr,
			PriceAmount:       entity.PriceAmount,
			PriceDecimal:      entity.PriceDecimal,
			PriceCoin:         entity.PriceCoin,
			Fee:               entity.FeeAmount,
			FeeRate:           entity.FeeRate,
			//TakePsbt:          "",
			BlockHeight:       entity.BlockHeight,
			ConfirmationState: entity.ConfirmationState,
			DealTime:          entity.DealTime,
			TxId:              entity.TxId,
		}
	}
	return resp, nil
}

// get mrc20 market hot list
func FetchMarketMrc20HotList(req *request.FetchMarketMrc20HotListReq, publicKey, ip string) (*respond.Mrc20HotListResp, error) {
	var (
		timeRange int64 = 7 * 24 * 60 * 60 * 1000 // 默认7天
		offset    int64 = 0
		limit     int64 = 20
	)

	// 如果请求中指定了时间范围，使用请求中的值
	if req.TimeRange > 0 {
		timeRange = req.TimeRange
	}

	// 如果请求中指定了分页参数，使用请求中的值
	if req.Cursor > 0 {
		offset = req.Cursor
	}
	if req.Size > 0 && req.Size <= 50 {
		limit = req.Size
	} else if req.Size > 50 {
		limit = 50
	}

	// 获取热门币种列表
	hotList, err := models.MarketMrc20InfoModelDao().GetHotMrc20CoreInfo(timeRange, offset, limit)
	if err != nil {
		return nil, err
	}

	// 转换为响应格式
	var list []*respond.Mrc20HotInfo = make([]*respond.Mrc20HotInfo, 0)
	for _, item := range hotList {
		change24h := "+0.00%"
		change24HDe := decimal.New(item.Change24H, 0)
		change24h = change24HDe.Div(decimal.New(100, 0)).String() + "%"
		cacheTickInfo := common_service.GetCacheMrc20TickInfo(item.TickId)

		hotInfo := &respond.Mrc20HotInfo{
			TickId:     item.TickId,
			Tick:       item.Tick,
			TokenName:  item.TokenName,
			MarketCap:  item.MarketCap,
			LastPrice:  item.LastPrice,
			Change24H:  change24h,
			TradeCount: item.TradeCount,
		}
		if cacheTickInfo != nil {
			hotInfo.Holders = cacheTickInfo.Holders
			hotInfo.DeployerUserInfo = cacheTickInfo.DeployerUserInfo
			hotInfo.MetaData = cacheTickInfo.MetaData
			hotInfo.Tag = cacheTickInfo.Tag
		}
		list = append(list, hotInfo)
	}

	return &respond.Mrc20HotListResp{
		TimeRange: timeRange,
		Total:     int64(len(list)),
		List:      list,
	}, nil
}

// get mrc20 market newest list
func FetchMarketMrc20NewestList(req *request.FetchMarketMrc20NewestListReq, publicKey, ip string) (*respond.Mrc20NewestListResp, error) {
	var (
		offset int64                      = 0
		limit  int64                      = 20
		list   []*respond.Mrc20NewestInfo = make([]*respond.Mrc20NewestInfo, 0)
	)

	if req.Cursor > 0 {
		offset = req.Cursor
	}
	if req.Size > 0 && req.Size <= 50 {
		limit = req.Size
	} else if req.Size > 50 {
		limit = 50
	}

	newestList, err := models.MarketMrc20InfoModelDao().GetLatestTradeMrc20CoreInfo(offset, limit)
	if err != nil {
		return nil, err
	}
	fmt.Printf("FetchMarketMrc20NewestList: newestList: %v\n", newestList)

	for _, item := range newestList {
		//fmt.Printf("FetchMarketMrc20NewestList: item: %+v\n", item)
		change24h := "+0.00%"
		change24HDe := decimal.New(item.Change24H, 0)
		change24h = change24HDe.Div(decimal.New(100, 0)).String() + "%"
		cacheTickInfo := common_service.GetCacheMrc20TickInfo(item.TickId)

		newestInfo := &respond.Mrc20NewestInfo{
			TickId:     item.TickId,
			Tick:       item.Tick,
			TokenName:  item.TokenName,
			MarketCap:  item.MarketCap,
			LastPrice:  item.LastPrice,
			Change24H:  change24h,
			TradeCount: item.TradeCount,
		}
		if cacheTickInfo != nil {
			newestInfo.Holders = cacheTickInfo.Holders
			newestInfo.DeployerUserInfo = cacheTickInfo.DeployerUserInfo
			newestInfo.MetaData = cacheTickInfo.MetaData
			newestInfo.Tag = cacheTickInfo.Tag
		}
		list = append(list, newestInfo)
	}

	return &respond.Mrc20NewestListResp{
		Total: int64(len(list)),
		List:  list,
	}, nil
}
