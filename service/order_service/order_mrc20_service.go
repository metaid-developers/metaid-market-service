package order_service

//
//func PushMarketMrc20Order(req *request.PushOrderReq, publicKey, ip string) (*respond.PushOrderResp, error) {
//	var (
//		orderEntity        *models.MarketOrderModel
//		psbtBuilder        *common.PsbtBuilder
//		err                error
//		orderId            string                 = ""
//		assetId            string                 = req.AssetId
//		assetLocalUtxoId   string                 = ""
//		assetType          models.AssetType       = req.AssetType
//		sellerAddress      string                 = req.Address
//		sellerPriceAmount  int64                  = 0
//		sellerPriceDecimal int64                  = 8
//		sellerPriceCoin    string                 = "BTC"
//		psbtRaw            string                 = req.PsbtRaw
//		nowTime            int64                  = tool.MakeTimestamp()
//		outValue           int64                  = 0
//		assetNumber        int64                  = 0
//		assetLevel         int                    = 0
//		assetPop           string                 = ""
//		content            string                 = ""
//		preview            string                 = ""
//		detailMap          map[string]interface{} = map[string]interface{}{}
//
//		feeAmount int64 = 2000
//		feeRate   int64 = 1
//	)
//	if req.PsbtRaw == "" {
//		return nil, errors.New("Wrong Psbt: empty. ")
//	}
//
//	verified, err := common.CheckPublicKeyAddress(common.GetNetParams(conf.Net), publicKey, sellerAddress)
//	if err != nil {
//		return nil, errors.New(fmt.Sprintf("Check address err: %s. ", err.Error()))
//	}
//	if !verified {
//		return nil, errors.New(fmt.Sprintf("Check address verified: %v. ", verified))
//	}
//
//	isLegacy, err := common.CheckLegacyAddressType(conf.Net, sellerAddress)
//	if err != nil {
//		return nil, err
//	}
//
//	//check psbt
//	psbtBuilder, err = common.NewPsbtBuilder(common.GetNetParams(conf.Net), psbtRaw)
//	if err != nil {
//		return nil, err
//	}
//	preOutList := psbtBuilder.GetInputs()
//	if preOutList == nil || len(preOutList) == 0 {
//		return nil, errors.New("Wrong Psbt: empty inputs. ")
//	}
//	outList := psbtBuilder.GetOutputs()
//	if outList == nil || len(outList) == 0 {
//		return nil, errors.New("Wrong Psbt: empty outputs. ")
//	}
//	if len(preOutList) != len(outList) {
//		return nil, errors.New("Wrong Psbt: inputs and outputs not match. ")
//	}
//	if isLegacy && len(preOutList) != 3 {
//		return nil, errors.New("Wrong Psbt: inputs not match. ")
//	}
//	//else if !isLegacy && len(preOutList) != 1 {
//	//	return nil, errors.New("Wrong Psbt: inputs not match. ")
//	//}
//
//	//check inputs and asset
//	for i, v := range preOutList {
//		if len(preOutList) == 3 && i != 2 {
//			continue
//		}
//		preTxId := v.PreviousOutPoint.Hash.String()
//		preTxIndex := v.PreviousOutPoint.Index
//		assetLocalUtxoId = fmt.Sprintf("%s_%d", preTxId, preTxIndex)
//
//		pinInfo, err := man_service.FetchUtxoPinInfo(conf.Net, preTxId, int64(preTxIndex))
//		if err != nil {
//			return nil, errors.New(fmt.Sprintf("fetch asset info err:%s", err))
//		}
//		outValue = pinInfo.OutputValue
//		content = pinInfo.Content
//		preview = pinInfo.Preview
//		assetNumber = pinInfo.Number
//		assetLevel = pinInfo.PopLv
//		assetPop = common.GenPopSummary(pinInfo.Pop)
//		detailMap = map[string]interface{}{
//			"pinId":              pinInfo.Id,
//			"metaid":             pinInfo.Metaid,
//			"createAddress":      pinInfo.CreateAddress,
//			"operation":          pinInfo.Operation,
//			"pinNumber":          pinInfo.Number,
//			"pop":                pinInfo.Pop,
//			"popSummary":         common.GenPopSummary(pinInfo.Pop),
//			"popLv":              pinInfo.PopLv,
//			"path":               pinInfo.Path,
//			"originalPath":       pinInfo.OriginalPath,
//			"version":            pinInfo.Version,
//			"encryption":         pinInfo.Encryption,
//			"outputValue":        pinInfo.OutputValue,
//			"contentLength":      pinInfo.ContentLength,
//			"contentTypeDetect":  pinInfo.ContentTypeDetect,
//			"timestamp":          pinInfo.Timestamp,
//			"genesisHeight":      pinInfo.GenesisHeight,
//			"genesisTransaction": pinInfo.GenesisTransaction,
//		}
//	}
//
//	for i, v := range outList {
//		if len(preOutList) == 3 && i != 2 {
//			continue
//		}
//		pkScript := hex.EncodeToString(v.PkScript)
//		address, err := common.PkScriptToAddress(conf.Net, pkScript)
//		if err != nil {
//			return nil, err
//		}
//		if address != sellerAddress {
//			return nil, errors.New("Address does not match in output. ")
//		}
//		sellerPriceAmount = v.Value
//	}
//
//	//if !psbtBuilder.IsComplete() {
//	//	return nil, errors.New("Wrong Psbt: incomplete. ")
//	//}
//
//	orderId = fmt.Sprintf("%s_%s_%s_%s", assetLocalUtxoId, assetId, assetType, sellerAddress)
//	orderId = hex.EncodeToString(tool.SHA256([]byte(orderId)))
//	orderEntity, err = models.MarketOrderModelDao().GetOne(&models.MarketOrderModel{
//		OrderId: orderId,
//	})
//	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
//		return nil, err
//	}
//	if orderEntity != nil {
//		if orderEntity.OrderState == models.OrderStateCreate {
//			return nil, errors.New("Order already exist. ")
//		} else if orderEntity.OrderState == models.OrderStateFinish {
//			return nil, errors.New("Order already finish. ")
//		}
//	}
//	if orderEntity == nil {
//		orderEntity = &models.MarketOrderModel{
//			OrderId:       orderId,
//			UtxoId:        assetLocalUtxoId,
//			OutValue:      outValue,
//			AssetId:       assetId,
//			AssetType:     assetType,
//			AssetNumber:   assetNumber,
//			AssetLevel:    int64(assetLevel),
//			AssetPop:      assetPop,
//			OrderState:    models.OrderStateCreate,
//			SellerAddress: sellerAddress,
//			SellerIp:      ip,
//			//BuyerAddress:      "",
//			SellPriceAmount:  sellerPriceAmount,
//			SellPriceDecimal: sellerPriceDecimal,
//			SellPriceCoin:    sellerPriceCoin,
//			FeeAmount:        feeAmount,
//			FeeRate:          feeRate,
//			Content:          content,
//			Preview:          preview,
//			Detail:           tool.AnyToStr(detailMap),
//			MakerPsbt:        psbtRaw,
//			//TakerPsbt:         "",
//			//TxId:              "",
//			//BlockHeight:       0,
//			//ConfirmationState: 0,
//			Timestamp:  nowTime,
//			CreateTime: nowTime,
//			//UpdateTime:        0,
//			State: models.STATE_EXIST,
//		}
//		err = models.MarketOrderModelDao().Set(orderEntity)
//		if err != nil {
//			return nil, err
//		}
//	} else {
//		orderEntity.OrderState = models.OrderStateCreate
//		orderEntity.SellerIp = ip
//		orderEntity.SellPriceAmount = sellerPriceAmount
//		orderEntity.SellPriceDecimal = sellerPriceDecimal
//		orderEntity.SellPriceCoin = sellerPriceCoin
//		orderEntity.FeeAmount = feeAmount
//		orderEntity.FeeRate = feeRate
//		orderEntity.Content = content
//		orderEntity.Preview = preview
//		orderEntity.Detail = tool.AnyToStr(detailMap)
//		orderEntity.MakerPsbt = psbtRaw
//		orderEntity.Timestamp = nowTime
//		err = models.MarketOrderModelDao().Update(orderEntity)
//		if err != nil {
//			return nil, err
//		}
//	}
//
//	return &respond.PushOrderResp{
//		OrderId:    orderId,
//		AssetType:  assetType,
//		AssetId:    assetId,
//		OrderState: orderEntity.OrderState,
//	}, nil
//}
//
//func FetchMrc20OrderPsbt(req *request.FetchOrderPsbtReq, publicKey, ip string) (*respond.OrderInfo, error) {
//	var (
//		entity               *models.MarketOrderModel
//		err                  error
//		resp                 *respond.OrderInfo
//		takerPsbtRaw         string = ""
//		feeAmountForPlatform int64  = 0
//		psbtBuilder          *common.PsbtBuilder
//	)
//
//	verified, err := common.CheckPublicKeyAddress(common.GetNetParams(conf.Net), publicKey, req.BuyerAddress)
//	if err != nil {
//		return nil, errors.New(fmt.Sprintf("Check address err: %s. ", err.Error()))
//	}
//	if !verified {
//		return nil, errors.New(fmt.Sprintf("Check address verified: %v. ", verified))
//	}
//
//	_ = feeAmountForPlatform
//	entity, err = models.MarketOrderModelDao().GetOne(&models.MarketOrderModel{
//		OrderId: req.OrderId,
//	})
//	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
//		return nil, err
//	}
//	if entity == nil {
//		return nil, errors.New("Order is empty. ")
//	}
//	if entity.OrderState != models.OrderStateCreate {
//		return nil, errors.New("Order is closed. ")
//	}
//
//	if entity.FeeRate > 0 {
//		feeRateDe := decimal.New(int64(entity.FeeRate), -2)
//		feeAmountForPlatform = decimal.New(entity.SellPriceAmount, 0).Mul(feeRateDe).IntPart()
//		if feeAmountForPlatform < 2000 {
//			feeAmountForPlatform = 2000
//		}
//	} else if entity.FeeAmount > 0 {
//		feeAmountForPlatform = entity.FeeAmount
//	}
//
//	if entity.SellerAddress == req.BuyerAddress {
//		return nil, errors.New("Buyer address is same as seller. ")
//	}
//
//	isLegacy, err := common.CheckLegacyAddressType(conf.Net, entity.SellerAddress)
//	if err != nil {
//		return nil, err
//	}
//
//	psbtBuilder, err = common.NewPsbtBuilder(common.GetNetParams(conf.Net), entity.MakerPsbt)
//	if err != nil {
//		return nil, err
//	}
//
//	if isLegacy && len(psbtBuilder.GetInputs()) != 3 {
//		return nil, errors.New("Wrong Psbt: This orders was not matched. ")
//	}
//
//	if len(psbtBuilder.GetInputs()) == 3 {
//		takerPsbtRaw, feeAmountForPlatform, err = common.MakeAskTakerPsbtRawForPreMake(conf.Net, entity.OrderId, entity.MakerPsbt, entity.OutValue, req.BuyerAddress, req.BuyerChangeAmount, feeAmountForPlatform, true)
//		if err != nil {
//			return nil, err
//		}
//	} else {
//		takerPsbtRaw, feeAmountForPlatform, err = common.MakeAskTakerPsbtRaw(conf.Net, entity.OrderId, entity.MakerPsbt, entity.OutValue, req.BuyerAddress, req.BuyerChangeAmount, feeAmountForPlatform, true)
//		if err != nil {
//			return nil, err
//		}
//	}
//
//	resp = &respond.OrderInfo{
//		OrderId:          entity.OrderId,
//		UtxoId:           entity.UtxoId,
//		AssetType:        entity.AssetType,
//		AssetId:          entity.AssetId,
//		AssetNumber:      entity.AssetNumber,
//		OrderState:       entity.OrderState,
//		SellerAddress:    entity.SellerAddress,
//		Seller:           nil,
//		BuyerAddress:     entity.BuyerAddress,
//		Buyer:            nil,
//		SellPriceAmount:  entity.SellPriceAmount,
//		SellPriceDecimal: entity.SellPriceDecimal,
//		SellPriceCoin:    entity.SellPriceCoin,
//		Fee:              entity.FeeAmount,
//		//Fee:      feeAmountForPlatform,
//		FeeRate:  entity.FeeRate,
//		Content:  entity.Content,
//		Preview:  entity.Preview,
//		Detail:   entity.Detail,
//		TakePsbt: takerPsbtRaw,
//	}
//	return resp, nil
//}
//
//func TakeMarketMrc20Order(req *request.TakeOrderReq, publicKey, ip string) (*respond.TakeOrderResp, error) {
//	var (
//		entity              *models.MarketOrderModel
//		err                 error
//		takerAskPsbtBuilder *common.PsbtBuilder
//		usedUtxoList        []*models.MarketUtxoModel = make([]*models.MarketUtxoModel, 0)
//		newUtxoList         []*models.MarketUtxoModel = make([]*models.MarketUtxoModel, 0)
//		nowTime             int64                     = tool.MakeTimestamp()
//	)
//	entity, err = models.MarketOrderModelDao().GetOne(&models.MarketOrderModel{
//		OrderId: req.OrderId,
//	})
//	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
//		return nil, err
//	}
//	if entity == nil {
//		return nil, errors.New("Order is empty. ")
//	}
//	if entity.OrderState != models.OrderStateCreate {
//		return nil, errors.New("Order is closed. ")
//	}
//
//	takerAskPsbtBuilder, err = common.NewPsbtBuilder(common.GetNetParams(conf.Net), req.TakerPsbtRaw)
//	if err != nil {
//		return nil, errors.New(fmt.Sprintf("PSBT: NewPsbtBuilder err:%s", err.Error()))
//	}
//	err = common.SignAskTakerPsbtRawInDummy(conf.Net, takerAskPsbtBuilder)
//	if err != nil {
//		return nil, errors.New(fmt.Sprintf("SignAskTakerPsbtRawInDummy err:%s", err.Error()))
//	}
//	finalAskPsbtRaw, err := takerAskPsbtBuilder.ToString()
//	if err != nil {
//		return nil, errors.New(fmt.Sprintf("PSBT(X): ToString err:%s", err.Error()))
//	}
//
//	txRaw, err := takerAskPsbtBuilder.ExtractPsbtTransaction()
//	if err != nil {
//		return nil, errors.New(fmt.Sprintf("PSBT: ExtractPsbtTransaction err:%s", err.Error()))
//	}
//	txRawByte, _ := hex.DecodeString(txRaw)
//	txAsk := wire.NewMsgTx(2)
//	err = txAsk.Deserialize(bytes.NewReader(txRawByte))
//	if err != nil {
//		return nil, errors.New(fmt.Sprintf("txAsk Deserialize err: %v. ", err.Error()))
//	}
//	txId := txAsk.TxHash().String()
//
//	entity.FinalPsbt = finalAskPsbtRaw
//	entity.TxId = txId
//
//	if len(takerAskPsbtBuilder.GetInputs()) < 5 {
//		return nil, errors.New(fmt.Sprintf("PSBT: No match inputs length err"))
//	}
//
//	for i, v := range takerAskPsbtBuilder.GetInputs() {
//		if i == 0 || i == 1 || i == 3 {
//			utxoId := fmt.Sprintf("%s_%d", v.PreviousOutPoint.Hash.String(), v.PreviousOutPoint.Index)
//			utxoEntity, err := models.MarketUtxoModelDao().GetOne(&models.MarketUtxoModel{
//				UtxoId: utxoId,
//			})
//			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
//				return nil, err
//			}
//			if utxoEntity == nil {
//				return nil, errors.New(fmt.Sprintf("Utxo not found: %s. ", utxoId))
//			}
//			if utxoEntity.UsedState != models.UsedNo {
//				return nil, errors.New(fmt.Sprintf("Utxo used: %s. ", utxoId))
//			}
//			utxoEntity.UsedState = models.UsedYes
//			utxoEntity.UsedTxId = txId
//			utxoEntity.OrderId = entity.OrderId
//			usedUtxoList = append(usedUtxoList, utxoEntity)
//		}
//	}
//
//	for i, v := range takerAskPsbtBuilder.GetOutputs() {
//		if i == 0 || i == 3 || i == 4 {
//			utxoType := models.UtxoTypeDummy600
//			if i == 0 {
//				utxoType = models.UtxoTypeDummy1200
//			}
//			pkScriptHex := hex.EncodeToString(v.PkScript)
//			address, err := common.PkScriptToAddress(conf.Net, pkScriptHex)
//			if err != nil {
//				return nil, errors.New(fmt.Sprintf("PkScriptToAddress err: %s. ", err.Error()))
//			}
//
//			newUtxo := &models.MarketUtxoModel{
//				UtxoId:   fmt.Sprintf("%s_%d", txId, i),
//				UtxoType: utxoType,
//				Amount:   uint64(v.Value),
//				Address:  address,
//				//PrivateKeyHex:  "",
//				TxId:      txId,
//				Index:     int64(i),
//				PkScript:  pkScriptHex,
//				UsedState: models.UsedNo,
//				//UsedTxId:       "",
//				//OrderId:        "",
//				//SortIndex:      0,
//				//ConfirmStatus:  0,
//				FromOrderId:    entity.OrderId,
//				NetworkFeeRate: req.NetworkFeeRate,
//				Timestamp:      nowTime,
//				//Version:        0,
//				CreateTime: nowTime,
//				//UpdateTime:     0,
//				State: models.STATE_EXIST,
//			}
//			newUtxoList = append(newUtxoList, newUtxo)
//		}
//	}
//
//	buyerAddress := ""
//	buyerInput := takerAskPsbtBuilder.GetInputs()[4]
//	buyerInputTxId := buyerInput.PreviousOutPoint.Hash.String()
//	buyerInputIndex := buyerInput.PreviousOutPoint.Index
//	utxoInfo := common.GetUtxoInfo(conf.Net, buyerInputTxId, int64(buyerInputIndex))
//	if utxoInfo == nil || !utxoInfo.IsExist || utxoInfo.SpendStatus == "spend" {
//		return nil, errors.New(fmt.Sprintf("PSBT(take): Utxo is spend. Please select a different utxo. [%s_%d]", buyerInputTxId, int64(buyerInputIndex)))
//	}
//	buyerAddress = utxoInfo.Address
//	entity.BuyerAddress = buyerAddress
//	entity.BuyerIp = ip
//	entity.OrderState = models.OrderStateFinish
//	entity.ConfirmationState = models.ConfirmationStateUnconfirmed
//
//	verified, err := common.CheckPublicKeyAddress(common.GetNetParams(conf.Net), publicKey, buyerAddress)
//	if err != nil {
//		return nil, errors.New(fmt.Sprintf("Check address err: %s. ", err.Error()))
//	}
//	if !verified {
//		return nil, errors.New(fmt.Sprintf("Check address verified: %v. ", verified))
//	}
//	err = models.MarketOrderModelDao().UpdateOrderEntityListForJobFunc(
//		entity,
//		usedUtxoList, newUtxoList,
//		txRaw,
//		common.BroadcastTx)
//	if err != nil {
//		return nil, err
//	}
//
//	return &respond.TakeOrderResp{
//		OrderId:    entity.OrderId,
//		AssetType:  entity.AssetType,
//		AssetId:    entity.AssetId,
//		OrderState: entity.OrderState,
//		TxId:       txId,
//	}, nil
//}
//
//func CancelMarketMrc20Order(req *request.CancelOrderReq, publicKey, ip string) (*respond.CancelOrderResp, error) {
//	var (
//		entity *models.MarketOrderModel
//		err    error
//	)
//	entity, err = models.MarketOrderModelDao().GetOne(&models.MarketOrderModel{
//		OrderId: req.OrderId,
//	})
//	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
//		return nil, err
//	}
//	if entity == nil {
//		return nil, errors.New("Order is empty. ")
//	}
//	verified, err := common.CheckPublicKeyAddress(common.GetNetParams(conf.Net), publicKey, entity.SellerAddress)
//	if err != nil {
//		return nil, errors.New(fmt.Sprintf("Check address err: %s. ", err.Error()))
//	}
//	if !verified {
//		return nil, errors.New(fmt.Sprintf("Check address verified: %v. ", verified))
//	}
//	if entity.OrderState != models.OrderStateCreate {
//		return nil, errors.New("Order state is not create. ")
//	}
//	entity.OrderState = models.OrderStateCancel
//	err = models.MarketOrderModelDao().Update(entity)
//	if err != nil {
//		return nil, err
//	}
//	return &respond.CancelOrderResp{
//		OrderId:    entity.OrderId,
//		AssetType:  entity.AssetType,
//		AssetId:    entity.AssetId,
//		OrderState: entity.OrderState,
//	}, nil
//
//}
