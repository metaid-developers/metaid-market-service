package mrc20_op_service

import (
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
	"metaid-market-service/service/orders_exchange_service"
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
			OpOrderType: req.OpOrderType,
			Address:     req.Address,
			TickId:      req.TickId,
			Cursor:      req.Cursor,
			Size:        req.Size,
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
			TickId: req.TickId,
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
	}
	return resp, nil
}
