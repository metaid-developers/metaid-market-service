package mrc20_op_service

import (
	"github.com/godaddy-x/freego/utils/decimal"
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
	"metaid-market-service/models"
	"metaid-market-service/service/common_service"
	"metaid-market-service/service/orders_exchange_service"
	"strconv"
)

func FetchIdCoinsOpOrders(req *request.FetchIdCoinsOpOrdersRequest, publicKey, ip string) (*respond.FetchIdCoinsOpOrdersResp, error) {
	return fetchIdCoinsOpOrdersFromOrders(req, publicKey, ip)
}

func fetchIdCoinsOpOrdersFromOrders(req *request.FetchIdCoinsOpOrdersRequest, publicKey, ip string) (*respond.FetchIdCoinsOpOrdersResp, error) {
	var (
		headers map[string]string = map[string]string{
			"X-Public-Key": publicKey,
		}

		reqOrders *orders_exchange_service.FetchIdCoinsOpOrdersRequest = &orders_exchange_service.FetchIdCoinsOpOrdersRequest{
			OpOrderType:  req.OpOrderType,
			Address:      req.Address,
			TickId:       req.TickId,
			Cursor:       req.Cursor,
			Size:         req.Size,
			Confirmation: req.Confirmation,
		}
		respOrders *orders_exchange_service.FetchIdCoinsOpOrdersResp
		err        error

		list []*respond.IdCoinsOpOrderInfoResp = make([]*respond.IdCoinsOpOrderInfoResp, 0)
	)
	respOrders, err = orders_exchange_service.FetchIdCoinsOpOrders(reqOrders, headers)
	if err != nil {
		return nil, err
	}
	for _, v := range respOrders.List {
		deployerUserInfo := &respond.UserInfo{}
		if v.DeployerUserInfo != nil {
			deployerUserInfo = &respond.UserInfo{
				Name:   v.DeployerUserInfo.Name,
				Avatar: v.DeployerUserInfo.Avatar,
			}
		}
		list = append(list, &respond.IdCoinsOpOrderInfoResp{
			OpOrderType:       v.OpOrderType,
			OrderId:           v.OrderId,
			TickId:            v.TickId,
			Tick:              v.Tick,
			TickName:          v.TickName,
			Decimals:          v.Decimals,
			DeployState:       v.DeployState,
			MintState:         v.MintState,
			AmtPerMint:        v.AmtPerMint,
			MintCount:         v.MintCount,
			PremineCount:      v.PremineCount,
			TotalMinted:       v.TotalMinted,
			FollowersLimit:    v.FollowersLimit,
			LiquidityPerMint:  v.LiquidityPerMint,
			StartBlockHeight:  v.StartBlockHeight,
			Qual:              v.Qual,
			PinCheck:          v.PinCheck,
			PayCheck:          v.PayCheck,
			UsedPins:          v.UsedPins,
			TxId:              v.TxId,
			BlockHeight:       v.BlockHeight,
			ConfirmationState: v.ConfirmationState,
			Timestamp:         v.Timestamp,
			DeployerAddress:   v.DeployerAddress,
			DeployerMetaId:    v.DeployerMetaId,
			DeployerUserInfo:  deployerUserInfo,
			MetaData:          v.MetaData,
		})
	}

	return &respond.FetchIdCoinsOpOrdersResp{
		Total: respOrders.Total,
		List:  list,
	}, nil
}

func FetchIdCoinsAddressMintOrder(req *request.FetchIdCoinsMintOrderRequest, publicKey, ip string) (*respond.FetchOneIdCoinsMintOrderResp, error) {
	return fetchIdCoinsAddressMintOrder(req, publicKey, ip)
}

func fetchIdCoinsAddressMintOrder(req *request.FetchIdCoinsMintOrderRequest, publicKey, ip string) (*respond.FetchOneIdCoinsMintOrderResp, error) {
	var (
		headers map[string]string = map[string]string{
			"X-Public-Key": publicKey,
		}

		reqOrders *orders_exchange_service.FetchIdCoinsMintOrderRequest = &orders_exchange_service.FetchIdCoinsMintOrderRequest{
			TickId:  req.TickId,
			Address: req.Address,
		}
		respOrders *orders_exchange_service.FetchOneIdCoinsMintOrderResp
		err        error
	)
	respOrders, err = orders_exchange_service.FetchIdCoinsAddressMintOrder(reqOrders, headers)
	if err != nil {
		return nil, err
	}
	deployerUserInfo := &respond.UserInfo{}
	if respOrders.DeployerUserInfo != nil {
		deployerUserInfo = &respond.UserInfo{
			Name:   respOrders.DeployerUserInfo.Name,
			Avatar: respOrders.DeployerUserInfo.Avatar,
		}
	}
	resp := &respond.FetchOneIdCoinsMintOrderResp{
		AddressMintState:  respOrders.AddressMintState,
		OpOrderType:       respOrders.OpOrderType,
		OrderId:           respOrders.OrderId,
		TickId:            respOrders.TickId,
		Tick:              respOrders.Tick,
		TickName:          respOrders.TickName,
		Decimals:          respOrders.Decimals,
		DeployState:       respOrders.DeployState,
		MintState:         respOrders.MintState,
		FollowersLimit:    respOrders.FollowersLimit,
		LiquidityPerMint:  respOrders.LiquidityPerMint,
		AmtPerMint:        respOrders.AmtPerMint,
		MintCount:         respOrders.MintCount,
		PremineCount:      respOrders.PremineCount,
		TotalMinted:       respOrders.TotalMinted,
		StartBlockHeight:  respOrders.StartBlockHeight,
		Qual:              respOrders.Qual,
		PinCheck:          respOrders.PinCheck,
		PayCheck:          respOrders.PayCheck,
		UsedPins:          respOrders.UsedPins,
		TxId:              respOrders.OrderId,
		BlockHeight:       respOrders.BlockHeight,
		ConfirmationState: respOrders.ConfirmationState,
		Timestamp:         respOrders.Timestamp,
		DeployerAddress:   respOrders.DeployerAddress,
		DeployerMetaId:    respOrders.DeployerMetaId,
		DeployerUserInfo:  deployerUserInfo,
		MetaData:          respOrders.MetaData,
	}
	return resp, nil
}

func FetchIdCoinsList(req *request.FetchIdCoinsListRequest, publicKey, ip string) (*respond.FetchIdCoinsListResp, error) {
	return fetchIdCoinsListFromOrders(req, publicKey, ip)
}

func fetchIdCoinsListFromOrders(req *request.FetchIdCoinsListRequest, publicKey, ip string) (*respond.FetchIdCoinsListResp, error) {
	var (
		headers map[string]string = map[string]string{
			"X-Public-Key": publicKey,
		}

		reqOrders *orders_exchange_service.FetchIdCoinsListRequest = &orders_exchange_service.FetchIdCoinsListRequest{
			Address:         req.Address,
			Cursor:          req.Cursor,
			Size:            req.Size,
			OrderBy:         req.OrderBy,
			SortType:        req.SortType,
			FollowerAddress: req.FollowerAddress,
			SearchTick:      req.SearchTick,
		}
		respOrders *orders_exchange_service.FetchIdCoinsListResp
		err        error

		list []*respond.IdCoinsInfoResp = make([]*respond.IdCoinsInfoResp, 0)
	)
	respOrders, err = orders_exchange_service.FetchIdCoinsList(reqOrders, headers)
	if err != nil {
		return nil, err
	}
	for _, v := range respOrders.List {
		deployerUserInfo := &respond.UserInfo{}
		if v.DeployerUserInfo != nil {
			deployerUserInfo = &respond.UserInfo{
				Name:   v.DeployerUserInfo.Name,
				Avatar: v.DeployerUserInfo.Avatar,
			}
		}
		list = append(list, &respond.IdCoinsInfoResp{
			TickId:           v.TickId,
			Tick:             v.Tick,
			TokenName:        v.TokenName,
			Decimals:         v.Decimals,
			AmtPerMint:       v.AmtPerMint,
			FollowersLimit:   v.FollowersLimit,
			MintCount:        v.MintCount,
			LiquidityPerMint: v.LiquidityPerMint,
			PremineCount:     v.PremineCount,
			TotalMinted:      v.TotalMinted,
			BlockHeight:      v.BlockHeight,
			MetaData:         v.MetaData,
			Type:             v.Type,
			Qual:             v.Qual,
			PinCheck:         v.PinCheck,
			PayCheck:         v.PayCheck,
			Mrc20Id:          v.Mrc20Id,
			PinNumber:        v.PinNumber,
			Holders:          v.Holders,
			DeployerMetaId:   v.DeployerMetaId,
			DeployerAddress:  v.DeployerAddress,
			DeployerUserInfo: deployerUserInfo,
			DeployTime:       v.DeployTime,
			Price:            v.Price,
			PriceUsd:         v.PriceUsd,
			Pool:             v.Pool,
			TotalSupply:      v.TotalSupply,
			Supply:           v.Supply,
			Mintable:         v.Mintable,
			Remaining:        v.Remaining,
			IsFollowing:      v.IsFollowing,
		})
	}

	return &respond.FetchIdCoinsListResp{
		Total: respOrders.Total,
		List:  list,
	}, nil
}

func FetchOneIdCoinsInfo(req *request.FetchOneIdCoinsRequest, publicKey, ip string) (*respond.IdCoinsInfoResp, error) {
	return fetchOneIdCoinsInfoFromOrders(req, publicKey, ip)
}

func fetchOneIdCoinsInfoFromOrders(req *request.FetchOneIdCoinsRequest, publicKey, ip string) (*respond.IdCoinsInfoResp, error) {
	var (
		headers map[string]string = map[string]string{
			"X-Public-Key": publicKey,
		}

		reqOrders *orders_exchange_service.FetchOneIdCoinsRequest = &orders_exchange_service.FetchOneIdCoinsRequest{
			TickId:        req.TickId,
			Tick:          req.Tick,
			IssuerAddress: req.IssuerAddress,
			Address:       req.Address,
		}
		respOrders *orders_exchange_service.IdCoinsInfoResp
		err        error
	)
	respOrders, err = orders_exchange_service.FetchOneIdCoinsInfo(reqOrders, headers)
	if err != nil {
		return nil, err
	}
	deployerUserInfo := &respond.UserInfo{}
	if respOrders.DeployerUserInfo != nil {
		deployerUserInfo = &respond.UserInfo{
			Name:   respOrders.DeployerUserInfo.Name,
			Avatar: respOrders.DeployerUserInfo.Avatar,
		}
	}

	marketInfo, _ := models.MarketMrc20InfoModelDao().GetOne(&models.MarketMrc20InfoModel{
		TickId: respOrders.Mrc20Id,
	})
	price := "0"
	priceUsd := "0.00"
	floorPrice := "0"
	floorPriceUsd := "0.00"
	change24h := "+0.00%"
	marketCap := "0"
	marketCapUsd := "0.00"
	totalVolume := int64(0)
	if marketInfo != nil {
		floorPrice = strconv.FormatFloat(marketInfo.FloorPrice, 'f', -1, 64)
		price = strconv.FormatFloat(marketInfo.LastPrice, 'f', -1, 64)
		marketCap = strconv.FormatInt(marketInfo.MarketCap, 10)
		change24HDe := decimal.New(marketInfo.Change24H, 0)
		change24h = change24HDe.Div(decimal.New(100, 0)).String() + "%"

		btcUsd := common_service.GetPriceForUsd("BTC")
		btcUsdDe, _ := decimal.NewFromString(btcUsd)
		satUsdDe := btcUsdDe.Div(decimal.New(100000000, 0))

		priceDe, _ := decimal.NewFromString(price)
		priceUsd = priceDe.Mul(satUsdDe).StringFixed(3)
		floorPriceDe, _ := decimal.NewFromString(floorPrice)
		floorPriceUsd = floorPriceDe.Mul(satUsdDe).StringFixed(3)
		marketCapDe, _ := decimal.NewFromString(marketCap)
		marketCapUsd = marketCapDe.Mul(satUsdDe).StringFixed(3)

		totalVolume = marketInfo.TotalVolume
	}

	resp := &respond.IdCoinsInfoResp{
		TickId:           respOrders.TickId,
		Tick:             respOrders.Tick,
		TokenName:        respOrders.TokenName,
		Decimals:         respOrders.Decimals,
		AmtPerMint:       respOrders.AmtPerMint,
		FollowersLimit:   respOrders.FollowersLimit,
		MintCount:        respOrders.MintCount,
		LiquidityPerMint: respOrders.LiquidityPerMint,
		PremineCount:     respOrders.PremineCount,
		TotalMinted:      respOrders.TotalMinted,
		BlockHeight:      respOrders.BlockHeight,
		MetaData:         respOrders.MetaData,
		Type:             respOrders.Type,
		Qual:             respOrders.Qual,
		PinCheck:         respOrders.PinCheck,
		PayCheck:         respOrders.PayCheck,
		Mrc20Id:          respOrders.Mrc20Id,
		PinNumber:        respOrders.PinNumber,
		Holders:          respOrders.Holders,
		DeployerMetaId:   respOrders.DeployerMetaId,
		DeployerAddress:  respOrders.DeployerAddress,
		DeployerUserInfo: deployerUserInfo,
		DeployTime:       respOrders.DeployTime,
		Price:            respOrders.Price,
		PriceUsd:         respOrders.PriceUsd,
		Pool:             respOrders.Pool,
		TotalSupply:      respOrders.TotalSupply,
		Supply:           respOrders.Supply,
		Mintable:         respOrders.Mintable,
		Remaining:        respOrders.Remaining,
		IsFollowing:      respOrders.IsFollowing,
		FollowersCount:   respOrders.FollowersCount,
		MarketPrice:      price,
		MarketPriceUsd:   priceUsd,
		FloorPrice:       floorPrice,
		FloorPriceUsd:    floorPriceUsd,
		Change24h:        change24h,
		MarketCap:        marketCap,
		MarketCapUsd:     marketCapUsd,
		TotalVolume:      totalVolume,
		OrdersPrice:      respOrders.OrdersPrice,
		OrdersPool:       respOrders.OrdersPool,
	}
	return resp, nil
}

func FetchIdCoinsDeployCheckInfo(req *request.FetchIdCoinsDeployCheckRequest, publicKey, ip string) (*respond.FetchIdCoinsDeployCheckResp, error) {
	return fetchIdCoinsDeployCheckInfo(req, publicKey, ip)
}

func fetchIdCoinsDeployCheckInfo(req *request.FetchIdCoinsDeployCheckRequest, publicKey, ip string) (*respond.FetchIdCoinsDeployCheckResp, error) {
	var (
		headers map[string]string = map[string]string{
			"X-Public-Key": publicKey,
		}

		reqOrders *orders_exchange_service.FetchIdCoinsDeployCheckRequest = &orders_exchange_service.FetchIdCoinsDeployCheckRequest{
			Address: req.Address,
		}
		respOrders *orders_exchange_service.FetchIdCoinsDeployCheckResp
		err        error
	)
	respOrders, err = orders_exchange_service.FetchIdCoinsDeployCheckInfo(reqOrders, headers)
	if err != nil {
		return nil, err
	}

	resp := &respond.FetchIdCoinsDeployCheckResp{
		CanDeploy: respOrders.CanDeploy,
		Msg:       respOrders.Msg,
	}
	return resp, nil
}
