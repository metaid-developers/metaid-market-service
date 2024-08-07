package common_service

import (
	"errors"
	"fmt"
	"github.com/godaddy-x/freego/utils/decimal"
	"gorm.io/gorm"
	"metaid-market-service/common"
	"metaid-market-service/conf"
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
	"metaid-market-service/models"
	"metaid-market-service/protobuf/mrc20_holders_service"
	"metaid-market-service/protobuf/mrc20_utxo_service"
	"metaid-market-service/service/grpc_service"
	"metaid-market-service/service/man_service"
	"metaid-market-service/service/mvcscan_service"
	"metaid-market-service/service/orders_exchange_service"
	"metaid-market-service/service/own_service"
	"metaid-market-service/service/ticket_service"
	"strconv"
	"strings"
)

func FetchMrc20TickList(req *request.FetchMrc20TickListReq) (*respond.Mrc20TickListResp, error) {
	if req.SearchTick != "" || req.OrderBy == "totalSupply" || req.OrderBy == "progress" || req.OrderBy == "deployTime" {
		return FetchMrc20TickListByGrpc(req)
	}
	switch req.OrderBy {
	case "change24H", "lastPrice", "marketCap":
		return FetchMrc20TickListByMarket(req)
	default:
		req.OrderBy = strings.ToLower(req.OrderBy)
		return FetchMrc20TickListByMan(req)
	}
}

func FetchMrc20TickListByGrpc(req *request.FetchMrc20TickListReq) (*respond.Mrc20TickListResp, error) {
	var (
		grpcResp *mrc20_utxo_service.Mrc20DeployListResponse
		err      error
		list     []*respond.Mrc20TickInfo = make([]*respond.Mrc20TickInfo, 0)
		total    int64                    = 0
		sortType                          = "desc"
	)
	if req.SortType > 0 {
		sortType = "asc"
	}
	client, err := grpc_service.GetMrc20BaseConn()
	if err != nil {
		return nil, err
	}
	grpcResp, err = client.FetchMrc20DeployList(req.SearchTick, req.Completed, req.OrderBy, sortType, req.Cursor, req.Size)
	if err != nil {
		return nil, err
	}
	if grpcResp == nil {
		return nil, errors.New("grpc response is empty")
	}

	if grpcResp != nil {
		for _, v := range grpcResp.GetDetail() {
			totalSupply := "0"
			mintable := false
			remaining := "0"
			supply := "0"
			if v.AmtPerMint != "" && v.MintCount != 0 {
				totalMintedDe := decimal.New(v.TotalMinted, 0)
				//premineCountDe := decimal.New(v.PremineCount, 0)
				amtPerMintDe, _ := decimal.NewFromString(v.AmtPerMint)
				mintCountDe := decimal.New(v.MintCount, 0)

				if totalMintedDe.GreaterThan(mintCountDe) {
					supplyDe := mintCountDe.Mul(amtPerMintDe)
					supply = supplyDe.String()
				} else {
					supplyDe := totalMintedDe.Mul(amtPerMintDe)
					supply = supplyDe.String()
				}

				totalSupplyDe := mintCountDe.Mul(amtPerMintDe)
				totalSupply = totalSupplyDe.String()

				remainingDe := mintCountDe.Sub(totalMintedDe).Mul(amtPerMintDe)
				remaining = remainingDe.String()
				if remainingDe.GreaterThan(decimal.Zero) {
					mintable = true
				}
			}

			price := "0"
			priceUsd := "0.00"
			change24h := "+0.00%"
			marketCap := "0"
			marketCapUsd := "0.00"
			totalVolume := int64(0)

			marketInfo, _ := models.MarketMrc20InfoModelDao().GetOne(&models.MarketMrc20InfoModel{
				TickId: v.Mrc20Id,
			})
			if marketInfo != nil {
				price = strconv.FormatFloat(marketInfo.LastPrice, 'f', -1, 64)
				marketCap = strconv.FormatInt(marketInfo.MarketCap, 10)
				change24HDe := decimal.New(marketInfo.Change24H, 0)
				change24h = change24HDe.Div(decimal.New(100, 0)).String() + "%"

				btcUsd := GetPriceForUsd("BTC")
				btcUsdDe, _ := decimal.NewFromString(btcUsd)
				satUsdDe := btcUsdDe.Div(decimal.New(100000000, 0))

				priceDe, _ := decimal.NewFromString(price)
				priceUsd = priceDe.Mul(satUsdDe).StringFixed(3)
				marketCapDe, _ := decimal.NewFromString(marketCap)
				marketCapUsd = marketCapDe.Mul(satUsdDe).StringFixed(3)

				totalVolume = marketInfo.TotalVolume
			}

			item := &respond.Mrc20TickInfo{
				Tick:             v.Tick,
				TokenName:        v.TokenName,
				Decimals:         v.Decimals,
				AmtPerMint:       v.AmtPerMint,
				MintCount:        strconv.FormatInt(v.MintCount, 10),
				PremineCount:     strconv.FormatInt(v.PremineCount, 10),
				BeginHeight:      v.BeginHeight,
				EndHeight:        v.EndHeight,
				MetaData:         v.Metadata,
				Type:             v.DeployType,
				Qual:             v.PinCheck,
				PinCheck:         v.PinCheck,
				PayCheck:         v.PayCheck,
				TotalMinted:      strconv.FormatInt(v.TotalMinted, 10),
				Mrc20Id:          v.Mrc20Id,
				PinNumber:        v.PinNumber,
				Holders:          v.Holders,
				TxCount:          v.TxCount,
				DeployerMetaId:   v.MetaId,
				DeployerAddress:  v.Address,
				DeployTime:       v.DeployTime,
				DeployerUserInfo: common.FetchMetaIDUserInfo(v.Address),
				Price:            price,
				PriceUsd:         priceUsd,
				Change24h:        change24h,
				MarketCap:        marketCap,
				MarketCapUsd:     marketCapUsd,
				TotalVolume:      totalVolume,
				TotalSupply:      totalSupply,
				Supply:           supply,
				Mintable:         mintable,
				Remaining:        remaining,
				Progress:         float64(v.Progress),
			}

			list = append(list, item)
		}
		total = grpcResp.Total
	}
	return &respond.Mrc20TickListResp{
		Total: total,
		List:  list,
	}, nil
}

func FetchMrc20TickListByMan(req *request.FetchMrc20TickListReq) (*respond.Mrc20TickListResp, error) {
	var (
		mrc20Resp *man_service.Mrc20TickListResp
		err       error
		list      []*respond.Mrc20TickInfo = make([]*respond.Mrc20TickInfo, 0)
		total     int64                    = 0
		sortType                           = "desc"
	)
	if req.SortType > 0 {
		sortType = "asc"
	}
	mrc20Resp, err = man_service.FetchMrc20TickList(req.Cursor, req.Size, req.Completed, req.OrderBy, sortType)
	if err != nil {
		return nil, err
	}
	if mrc20Resp != nil {
		for _, v := range mrc20Resp.List {
			totalSupply := "0"
			mintable := false
			remaining := "0"
			supply := "0"
			if v.AmtPerMint != "" && v.MintCount != 0 {
				totalMintedDe := decimal.New(v.TotalMinted, 0)
				//premineCountDe := decimal.New(v.PremineCount, 0)
				amtPerMintDe, _ := decimal.NewFromString(v.AmtPerMint)
				mintCountDe := decimal.New(v.MintCount, 0)

				if totalMintedDe.GreaterThan(mintCountDe) {
					supplyDe := mintCountDe.Mul(amtPerMintDe)
					supply = supplyDe.String()
				} else {
					supplyDe := totalMintedDe.Mul(amtPerMintDe)
					supply = supplyDe.String()
				}

				totalSupplyDe := mintCountDe.Mul(amtPerMintDe)
				totalSupply = totalSupplyDe.String()

				remainingDe := mintCountDe.Sub(totalMintedDe).Mul(amtPerMintDe)
				remaining = remainingDe.String()
				if remainingDe.GreaterThan(decimal.Zero) {
					mintable = true
				}
			}

			price := "0"
			priceUsd := "0.00"
			change24h := "+0.00%"
			marketCap := "0"
			marketCapUsd := "0.00"
			totalVolume := int64(0)

			marketInfo, _ := models.MarketMrc20InfoModelDao().GetOne(&models.MarketMrc20InfoModel{
				TickId: v.Mrc20Id,
			})
			if marketInfo != nil {
				price = strconv.FormatFloat(marketInfo.LastPrice, 'f', -1, 64)
				marketCap = strconv.FormatInt(marketInfo.MarketCap, 10)
				change24HDe := decimal.New(marketInfo.Change24H, 0)
				change24h = change24HDe.Div(decimal.New(100, 0)).String() + "%"

				btcUsd := GetPriceForUsd("BTC")
				btcUsdDe, _ := decimal.NewFromString(btcUsd)
				satUsdDe := btcUsdDe.Div(decimal.New(100000000, 0))

				priceDe, _ := decimal.NewFromString(price)
				priceUsd = priceDe.Mul(satUsdDe).StringFixed(3)
				marketCapDe, _ := decimal.NewFromString(marketCap)
				marketCapUsd = marketCapDe.Mul(satUsdDe).StringFixed(3)

				totalVolume = marketInfo.TotalVolume
			}

			item := &respond.Mrc20TickInfo{
				Tick:             v.Tick,
				TokenName:        v.TokenName,
				Decimals:         v.Decimals,
				AmtPerMint:       v.AmtPerMint,
				MintCount:        strconv.FormatInt(v.MintCount, 10),
				PremineCount:     strconv.FormatInt(v.PremineCount, 10),
				BeginHeight:      v.BlockHeight,
				EndHeight:        v.EndHeight,
				MetaData:         v.Metadata,
				Type:             v.Type,
				Qual:             v.PinCheck,
				PinCheck:         v.PinCheck,
				PayCheck:         v.PayCheck,
				TotalMinted:      strconv.FormatInt(v.TotalMinted, 10),
				Mrc20Id:          v.Mrc20Id,
				PinNumber:        v.PinNumber,
				Holders:          v.Holders,
				TxCount:          v.TxCount,
				DeployerMetaId:   v.MetaId,
				DeployerAddress:  v.Address,
				DeployTime:       v.DeployTime,
				DeployerUserInfo: common.FetchMetaIDUserInfo(v.Address),
				Price:            price,
				PriceUsd:         priceUsd,
				Change24h:        change24h,
				MarketCap:        marketCap,
				MarketCapUsd:     marketCapUsd,
				TotalVolume:      totalVolume,
				TotalSupply:      totalSupply,
				Supply:           supply,
				Mintable:         mintable,
				Remaining:        remaining,
			}

			list = append(list, item)
		}
		total = mrc20Resp.Total
	}
	return &respond.Mrc20TickListResp{
		Total: total,
		List:  list,
	}, nil
}

func FetchMrc20TickListByMarket(req *request.FetchMrc20TickListReq) (*respond.Mrc20TickListResp, error) {
	var (
		entityList       []*models.MarketMrc20InfoModel
		list             []*respond.Mrc20TickInfo = make([]*respond.Mrc20TickInfo, 0)
		total            int64                    = 0
		sortType         string                   = "desc"
		marketTickIdList []string                 = make([]string, 0)

		mrc20Resp *man_service.Mrc20TickListResp
		err       error
	)
	if req.SortType > 0 {
		sortType = "asc"
	}
	entityList, _ = models.MarketMrc20InfoModelDao().GetListByOrder(&models.MarketMrc20InfoModel{}, req.Cursor, req.Size, req.OrderBy, sortType)
	for _, v := range entityList {
		totalSupply := "0"
		mintable := false
		remaining := "0"
		supply := "0"

		price := "0"
		priceUsd := "0.00"
		floorPrice := "0"
		floorPriceUsd := "0.00"
		change24h := "+0.00%"
		marketCap := "0"
		marketCapUsd := "0.00"
		totalVolume := int64(0)
		floorPrice = strconv.FormatFloat(v.FloorPrice, 'f', -1, 64)
		price = strconv.FormatFloat(v.LastPrice, 'f', -1, 64)
		marketCap = strconv.FormatInt(v.MarketCap, 10)
		change24HDe := decimal.New(v.Change24H, 0)
		change24h = change24HDe.Div(decimal.New(100, 0)).String() + "%"

		btcUsd := GetPriceForUsd("BTC")
		btcUsdDe, _ := decimal.NewFromString(btcUsd)
		satUsdDe := btcUsdDe.Div(decimal.New(100000000, 0))

		priceDe, _ := decimal.NewFromString(price)
		priceUsd = priceDe.Mul(satUsdDe).StringFixed(3)
		floorPriceDe, _ := decimal.NewFromString(floorPrice)
		floorPriceUsd = floorPriceDe.Mul(satUsdDe).StringFixed(3)
		marketCapDe, _ := decimal.NewFromString(marketCap)
		marketCapUsd = marketCapDe.Mul(satUsdDe).StringFixed(3)

		totalVolume = v.TotalVolume

		tickInfo, _ := GetMrc20TickInfo(v.TickId, "")
		if tickInfo == nil {
			continue
		}

		if tickInfo.AmtPerMint != "" && tickInfo.MintCount != "0" {
			totalMintedDe, _ := decimal.NewFromString(tickInfo.TotalMinted)
			//premineCountDe, _ := decimal.NewFromString(tickInfo.PremineCount)
			amtPerMintDe, _ := decimal.NewFromString(tickInfo.AmtPerMint)
			mintCountDe, _ := decimal.NewFromString(tickInfo.MintCount)
			if totalMintedDe.GreaterThan(mintCountDe) {
				supplyDe := mintCountDe.Mul(amtPerMintDe)
				supply = supplyDe.String()
			} else {
				supplyDe := totalMintedDe.Mul(amtPerMintDe)
				supply = supplyDe.String()
			}

			totalSupplyDe := mintCountDe.Mul(amtPerMintDe)
			totalSupply = totalSupplyDe.String()

			remainingDe := mintCountDe.Sub(totalMintedDe).Mul(amtPerMintDe)
			remaining = remainingDe.String()
			if remainingDe.GreaterThan(decimal.Zero) {
				mintable = true
			}
		}
		fmt.Printf("Completed:%t, mintable:%t\n", req.Completed, mintable)
		if req.Completed == "ture" && mintable {
			continue
		}

		marketTickIdList = append(marketTickIdList, v.TickId)

		item := &respond.Mrc20TickInfo{
			Tick:             tickInfo.Tick,
			TokenName:        tickInfo.TokenName,
			Decimals:         tickInfo.Decimals,
			AmtPerMint:       tickInfo.AmtPerMint,
			MintCount:        tickInfo.MintCount,
			PremineCount:     tickInfo.PremineCount,
			BeginHeight:      tickInfo.BeginHeight,
			EndHeight:        tickInfo.EndHeight,
			MetaData:         tickInfo.MetaData,
			Type:             tickInfo.Type,
			Qual:             tickInfo.Qual,
			TotalMinted:      tickInfo.TotalMinted,
			Mrc20Id:          tickInfo.Mrc20Id,
			PinNumber:        tickInfo.PinNumber,
			Holders:          tickInfo.Holders,
			TxCount:          tickInfo.TxCount,
			DeployerMetaId:   tickInfo.MetaId,
			DeployerAddress:  tickInfo.Address,
			DeployTime:       tickInfo.DeployTime,
			DeployerUserInfo: common.FetchMetaIDUserInfo(tickInfo.Address),
			Price:            price,
			PriceUsd:         priceUsd,
			FloorPrice:       floorPrice,
			FloorPriceUsd:    floorPriceUsd,
			Change24h:        change24h,
			MarketCap:        marketCap,
			MarketCapUsd:     marketCapUsd,
			TotalVolume:      totalVolume,
			TotalSupply:      totalSupply,
			Supply:           supply,
			Mintable:         mintable,
			Remaining:        remaining,
		}
		list = append(list, item)
	}

	mrc20Resp, err = man_service.FetchMrc20TickList(req.Cursor, req.Size, req.Completed, "txcount", sortType)
	if err != nil {
		return nil, err
	}
	if mrc20Resp != nil {
		for _, v := range mrc20Resp.List {

			isMarket := false
			for _, tickId := range marketTickIdList {
				if v.Mrc20Id == tickId {
					isMarket = true
					break
				}
			}
			if isMarket {
				continue
			}

			totalSupply := "0"
			mintable := false
			remaining := "0"
			supply := "0"
			if v.AmtPerMint != "" && v.MintCount != 0 {
				totalMintedDe := decimal.New(v.TotalMinted, 0)
				//premineCountDe := decimal.New(v.PremineCount, 0)
				amtPerMintDe, _ := decimal.NewFromString(v.AmtPerMint)
				mintCountDe := decimal.New(v.MintCount, 0)

				if totalMintedDe.GreaterThan(mintCountDe) {
					supplyDe := mintCountDe.Mul(amtPerMintDe)
					supply = supplyDe.String()
				} else {
					supplyDe := totalMintedDe.Mul(amtPerMintDe)
					supply = supplyDe.String()
				}

				totalSupplyDe := mintCountDe.Mul(amtPerMintDe)
				totalSupply = totalSupplyDe.String()

				remainingDe := mintCountDe.Sub(totalMintedDe).Mul(amtPerMintDe)
				remaining = remainingDe.String()
				if remainingDe.GreaterThan(decimal.Zero) {
					mintable = true
				}
			}

			price := "0"
			priceUsd := "0.00"
			change24h := "+0.00%"
			marketCap := "0"
			marketCapUsd := "0.00"
			totalVolume := int64(0)

			marketInfo, _ := models.MarketMrc20InfoModelDao().GetOne(&models.MarketMrc20InfoModel{
				TickId: v.Mrc20Id,
			})
			if marketInfo != nil {
				price = strconv.FormatFloat(marketInfo.LastPrice, 'f', -1, 64)
				marketCap = strconv.FormatInt(marketInfo.MarketCap, 10)
				change24HDe := decimal.New(marketInfo.Change24H, 0)
				change24h = change24HDe.Div(decimal.New(100, 0)).String() + "%"

				btcUsd := GetPriceForUsd("BTC")
				btcUsdDe, _ := decimal.NewFromString(btcUsd)
				satUsdDe := btcUsdDe.Div(decimal.New(100000000, 0))

				priceDe, _ := decimal.NewFromString(price)
				priceUsd = priceDe.Mul(satUsdDe).StringFixed(3)
				marketCapDe, _ := decimal.NewFromString(marketCap)
				marketCapUsd = marketCapDe.Mul(satUsdDe).StringFixed(3)

				totalVolume = marketInfo.TotalVolume
			}

			item := &respond.Mrc20TickInfo{
				Tick:             v.Tick,
				TokenName:        v.TokenName,
				Decimals:         v.Decimals,
				AmtPerMint:       v.AmtPerMint,
				MintCount:        strconv.FormatInt(v.MintCount, 10),
				PremineCount:     strconv.FormatInt(v.PremineCount, 10),
				BeginHeight:      v.BeginHeight,
				EndHeight:        v.EndHeight,
				MetaData:         v.Metadata,
				Type:             v.Type,
				Qual:             v.PinCheck,
				PinCheck:         v.PinCheck,
				PayCheck:         v.PayCheck,
				TotalMinted:      strconv.FormatInt(v.TotalMinted, 10),
				Mrc20Id:          v.Mrc20Id,
				PinNumber:        v.PinNumber,
				Holders:          v.Holders,
				TxCount:          v.TxCount,
				DeployerMetaId:   v.MetaId,
				DeployerAddress:  v.Address,
				DeployTime:       v.DeployTime,
				DeployerUserInfo: common.FetchMetaIDUserInfo(v.Address),
				Price:            price,
				PriceUsd:         priceUsd,
				Change24h:        change24h,
				MarketCap:        marketCap,
				MarketCapUsd:     marketCapUsd,
				TotalVolume:      totalVolume,
				TotalSupply:      totalSupply,
				Supply:           supply,
				Mintable:         mintable,
				Remaining:        remaining,
			}

			list = append(list, item)
		}
		total = mrc20Resp.Total
	}
	if int64(len(list)) > req.Size {
		list = list[:req.Size]
	}

	return &respond.Mrc20TickListResp{
		Total: total,
		List:  list,
	}, nil
}

func FetchMrc20TickInfo(req *request.FetchMrc20TickInfoReq) (*respond.Mrc20TickInfo, error) {
	var (
		mrc20Resp *man_service.Mrc20TickInfo
		err       error
	)
	if req.TickId == "" && req.Tick == "" {
		return nil, errors.New("tickId or tick is required")
	}
	mrc20Resp, err = man_service.FetchMrc20TickInfo(req.TickId, req.Tick)
	if err != nil {
		return nil, err
	}
	if mrc20Resp == nil {
		return nil, errors.New("mrc20 not found")
	}
	totalSupply := "0"
	mintable := false
	remaining := "0"
	supply := "0"
	if mrc20Resp.AmtPerMint != "" && mrc20Resp.MintCount != 0 {
		totalMintedDe := decimal.New(mrc20Resp.TotalMinted, 0)
		//premineCountDe := decimal.New(mrc20Resp.PremineCount, 0)
		amtPerMintDe, _ := decimal.NewFromString(mrc20Resp.AmtPerMint)
		mintCountDe := decimal.New(mrc20Resp.MintCount, 0)
		if totalMintedDe.GreaterThan(mintCountDe) {
			supplyDe := mintCountDe.Mul(amtPerMintDe)
			supply = supplyDe.String()
		} else {
			supplyDe := totalMintedDe.Mul(amtPerMintDe)
			supply = supplyDe.String()
		}

		totalSupplyDe := mintCountDe.Mul(amtPerMintDe)
		totalSupply = totalSupplyDe.String()

		remainingDe := mintCountDe.Sub(totalMintedDe).Mul(amtPerMintDe)
		remaining = remainingDe.String()
		if remainingDe.GreaterThan(decimal.Zero) {
			mintable = true
		}
	}

	price := "0"
	priceUsd := "0.00"
	floorPrice := "0"
	floorPriceUsd := "0.00"
	change24h := "+0.00%"
	marketCap := "0"
	marketCapUsd := "0.00"
	totalVolume := int64(0)

	marketInfo, _ := models.MarketMrc20InfoModelDao().GetOne(&models.MarketMrc20InfoModel{
		TickId: mrc20Resp.Mrc20Id,
	})
	if marketInfo != nil {
		floorPrice = strconv.FormatFloat(marketInfo.FloorPrice, 'f', -1, 64)
		price = strconv.FormatFloat(marketInfo.LastPrice, 'f', -1, 64)
		marketCap = strconv.FormatInt(marketInfo.MarketCap, 10)
		change24HDe := decimal.New(marketInfo.Change24H, 0)
		change24h = change24HDe.Div(decimal.New(100, 0)).String() + "%"

		btcUsd := GetPriceForUsd("BTC")
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

	return &respond.Mrc20TickInfo{
		Tick:             mrc20Resp.Tick,
		TokenName:        mrc20Resp.TokenName,
		Decimals:         mrc20Resp.Decimals,
		AmtPerMint:       mrc20Resp.AmtPerMint,
		MintCount:        strconv.FormatInt(mrc20Resp.MintCount, 10),
		PremineCount:     strconv.FormatInt(mrc20Resp.PremineCount, 10),
		TotalMinted:      strconv.FormatInt(mrc20Resp.TotalMinted, 10),
		BeginHeight:      mrc20Resp.BeginHeight,
		EndHeight:        mrc20Resp.EndHeight,
		MetaData:         mrc20Resp.Metadata,
		Type:             mrc20Resp.Type,
		Qual:             mrc20Resp.PinCheck,
		PinCheck:         mrc20Resp.PinCheck,
		PayCheck:         mrc20Resp.PayCheck,
		Mrc20Id:          mrc20Resp.Mrc20Id,
		PinNumber:        mrc20Resp.PinNumber,
		Holders:          mrc20Resp.Holders,
		TxCount:          mrc20Resp.TxCount,
		DeployerMetaId:   mrc20Resp.MetaId,
		DeployerAddress:  mrc20Resp.Address,
		DeployerUserInfo: common.FetchMetaIDUserInfo(mrc20Resp.Address),
		DeployTime:       mrc20Resp.DeployTime,
		Price:            price,
		PriceUsd:         priceUsd,
		FloorPrice:       floorPrice,
		FloorPriceUsd:    floorPriceUsd,
		Change24h:        change24h,
		MarketCap:        marketCap,
		MarketCapUsd:     marketCapUsd,
		TotalVolume:      totalVolume,
		TotalSupply:      totalSupply,
		Supply:           supply,
		Mintable:         mintable,
		Remaining:        remaining,
	}, nil
}

func FetchMrc20TickAddressShovels(req *request.Mrc20AddressShovelsReq) (*respond.Mrc20ShovelResp, error) {
	var (
		mrc20Resp *man_service.Mrc20ShovelResp
		err       error
		list      []*respond.ShovelInfo = make([]*respond.ShovelInfo, 0)
		total     int64                 = 0
	)
	mrc20Resp, err = man_service.FetchMrc20AddressShovelList(req.Address, req.TickId, req.Cursor, req.Size)
	if err != nil {
		return nil, err
	}
	if mrc20Resp != nil {
		for _, v := range mrc20Resp.List {
			list = append(list, &respond.ShovelInfo{
				Id:                 v.Id,
				Number:             v.Number,
				Metaid:             v.Metaid,
				Address:            v.Address,
				Creator:            v.Creator,
				InitialOwner:       v.InitialOwner,
				Output:             v.Output,
				OutputValue:        v.OutputValue,
				Timestamp:          v.Timestamp,
				GenesisFee:         v.GenesisFee,
				GenesisHeight:      v.GenesisHeight,
				GenesisTransaction: v.GenesisTransaction,
				TxIndex:            v.TxIndex,
				TxInIndex:          v.TxInIndex,
				Offset:             v.Offset,
				Location:           v.Location,
				Operation:          v.Operation,
				Path:               v.Path,
				ParentPath:         v.ParentPath,
				OriginalPath:       v.OriginalId,
				Encryption:         v.Encryption,
				Version:            v.Version,
				ContentType:        v.Content,
				ContentTypeDetect:  v.ContentTypeDetect,
				ContentBody:        v.ContentBody,
				ContentLength:      v.ContentLength,
				ContentSummary:     v.ContentSummary,
				Status:             v.Status,
				OriginalId:         v.OriginalId,
				IsTransfered:       v.IsTransfered,
				Preview:            v.Preview,
				Content:            v.Content,
				Pop:                v.Pop,
				PopLv:              v.PopLv,
				ChainName:          v.ChainName,
				DataValue:          v.DataValue,
				Mrc20Minted:        v.Mrc20Minted,
				Mrc20MintPin:       v.Mrc20MintPin,
			})
		}
		total = mrc20Resp.Total
	}
	return &respond.Mrc20ShovelResp{
		Total: total,
		List:  list,
	}, nil
}

func FetchMrc20TickAddressBalances(req *request.Mrc20AddressBalancesReq) (*respond.Mrc20BalanceInfoResp, error) {
	var (
		mrc20Resp *man_service.Mrc20BalanceResp
		err       error
		list      []*respond.Mrc20BalanceInfo = make([]*respond.Mrc20BalanceInfo, 0)
		total     int64                       = 0
	)
	mrc20Resp, err = man_service.FetchMrc20AddressBalanceList(req.Address, req.Cursor, req.Size)
	if err != nil {
		return nil, err
	}
	if mrc20Resp != nil {
		for _, v := range mrc20Resp.List {
			tickInfo, err := man_service.FetchMrc20TickInfo(v.Id, "")
			if err != nil {
				return nil, err
			}
			if tickInfo == nil || tickInfo.Mrc20Id == "" {
				return nil, errors.New("mrc20 not found")
			}

			item := &respond.Mrc20BalanceInfo{
				Tick:      tickInfo.Tick,
				TokenName: tickInfo.TokenName,
				Mrc20Id:   v.Id,
				Balance:   v.Balance,
				Decimals:  tickInfo.Decimals,
			}
			list = append(list, item)
		}
		total = mrc20Resp.Total
	}
	return &respond.Mrc20BalanceInfoResp{
		Total: total,
		List:  list,
	}, nil
}

func FetchMrc20TickAddressUtxos(req *request.Mrc20AddressUtxosReq) (*respond.Mrc20UtxoResp, error) {
	var (
		tickInfo  *man_service.Mrc20TickInfo
		mrc20Resp *man_service.Mrc20UtxoResp
		err       error
		list      []*respond.Mrc20Utxo = make([]*respond.Mrc20Utxo, 0)
		total     int64                = 0
		tag       string               = ""
	)
	tickInfo, err = man_service.FetchMrc20TickInfo(req.TickId, "")
	if err != nil {
		return nil, err
	}
	if tickInfo == nil || tickInfo.Mrc20Id == "" {
		return nil, errors.New("mrc20 not found")
	}

	idCoins, _ := orders_exchange_service.FetchOneIdCoinsInfo(&orders_exchange_service.FetchOneIdCoinsRequest{
		TickId:        req.TickId,
		Tick:          "",
		IssuerAddress: "",
	}, nil)
	if idCoins != nil {
		tag = "id-coins"
	}

	mrc20Resp, err = man_service.FetchMrc20AddressUtxoList(req.Address, req.TickId, req.Cursor, req.Size)
	if err != nil {
		return nil, err
	}
	if mrc20Resp != nil {
		for _, v := range mrc20Resp.List {

			if !v.Verify {
				continue
			}
			if v.AmtChange == "0" || v.AmtChange == "" {
				continue
			}
			utxoId := strings.Replace(v.TxPoint, ":", "_", -1)
			orderEntityFinish, _ := models.MarketMrc20OrderModelDao().GetOne(&models.MarketMrc20OrderModel{
				UtxoId:        utxoId,
				SellerAddress: req.Address,
				OrderState:    models.OrderStateFinish,
			})
			if orderEntityFinish != nil {
				continue
			}

			address := v.ToAddress
			txPoint := v.TxPoint
			txPointStrs := strings.Split(txPoint, ":")
			txId := ""
			vout := int64(0)
			if len(txPointStrs) == 2 {
				txId = txPointStrs[0]
				vout, _ = strconv.ParseInt(txPointStrs[1], 10, 64)
			}
			pkScript, _ := common.AddressToPkScript(conf.Net, address)
			satoshi := int64(546)
			if v.PointValue > 0 {
				satoshi = v.PointValue
			}

			mrc20s := make([]*respond.Mrc20Info, 0)
			mrc20s = append(mrc20s, &respond.Mrc20Info{
				Tick:     v.Tick,
				Mrc20Id:  v.Mrc20Id,
				TxPoint:  v.TxPoint,
				Amount:   v.AmtChange,
				Decimals: tickInfo.Decimals,
			})

			txPointInfo, txPointTotal, err := FetchTxPointInfo(txId, vout, 0, 100)
			if err != nil {
				return nil, err
			}
			if txPointTotal > 1 {
				for _, p := range txPointInfo {
					if p.Mrc20Id == v.Mrc20Id {
						continue
					}
					mrc20s = append(mrc20s, &respond.Mrc20Info{
						Tick:     p.Tick,
						Mrc20Id:  p.Mrc20Id,
						TxPoint:  p.TxPoint,
						Amount:   v.AmtChange,
						Decimals: tickInfo.Decimals,
					})
				}
			}

			item := &respond.Mrc20Utxo{
				Chain:       v.Chain,
				BlockHeight: v.BlockHeight,
				Address:     address,
				Satoshi:     satoshi,
				Satoshis:    satoshi,
				ScriptPk:    pkScript,
				TxId:        txId,
				Vout:        vout,
				OutputIndex: vout,
				Mrc20s:      mrc20s,
				Timestamp:   v.Timestamp,
				OrderId:     "",
				Tag:         tag,
			}
			orderEntity, _ := models.MarketMrc20OrderModelDao().GetOne(&models.MarketMrc20OrderModel{
				UtxoId:        utxoId,
				SellerAddress: req.Address,
				OrderState:    models.OrderStateCreate,
			})
			if orderEntity != nil {
				item.OrderId = orderEntity.OrderId
			}
			list = append(list, item)
		}
		total = mrc20Resp.Total
	}
	return &respond.Mrc20UtxoResp{
		Total: total,
		List:  list,
	}, nil
}

func FetchMrc20TickMarketPrice(req *request.FetchMrc20TickMarketPriceResp) (*respond.Mrc20TickMarketPriceResp, error) {
	var (
		marketInfo    *models.MarketMrc20InfoModel
		err           error
		price         string = "0"
		priceUsd      string = "0.00"
		floorPrice    string = "0"
		floorPriceUsd string = "0.00"
		marketCap     string = "0"
		marketCapUsd  string = "0.00"
		totalVolume   int64  = 0

		ticketPriceInfo *ticket_service.FetchClubTicketPriceInfoResp
	)

	if req.TickId == conf.MetaCoinMrc20Id {
		priceInfo, _ := mvcscan_service.FetchMetaContractFtSummaryInfo(conf.MetaCoinCodehash, conf.MetaCoinGenesis, nil)
		if priceInfo == nil || len(priceInfo.Data) == 0 {
			return nil, errors.New("meta coin price info not found")
		}
		mcPriceInfo := priceInfo.Data[0]
		priceUsd = mcPriceInfo.MarketPriceUsdt
		priceUsdDe, _ := decimal.NewFromString(priceUsd)

		btcUsd := GetPriceForUsd("BTC")
		btcUsdDe, _ := decimal.NewFromString(btcUsd)
		satUsdDe := btcUsdDe.Div(decimal.New(100000000, 0))

		priceDe := priceUsdDe.Div(satUsdDe)
		price = priceDe.StringFixed(6)
		return &respond.Mrc20TickMarketPriceResp{
			TickId:    req.TickId,
			Tick:      "",
			TokenName: "",
			Decimals:  0,
			Supply:    "",
			//TotalVolume:   totalVolume,
			//MarketCap:     marketCap,
			//MarketCapUsd:  marketCapUsd,
			//LastPrice:     price,
			//LastPriceUsd:  priceUsd,
			Price:    price,
			PriceUsd: priceUsd,
			//FloorPrice:    floorPrice,
			//FloorPriceUsd: floorPriceUsd,
		}, nil
	}

	ticketPriceInfo, _ = ticket_service.FetchClubTicketPriceInfo(&ticket_service.FetchClubTicketPriceInfoRequest{
		TicketId: req.TickId,
	}, nil)
	if ticketPriceInfo != nil {
		decimals, _ := strconv.ParseInt(ticketPriceInfo.Decimals, 10, 64)

		btcUsd := GetPriceForUsd("BTC")
		btcUsdDe, _ := decimal.NewFromString(btcUsd)
		satUsdDe := btcUsdDe.Div(decimal.New(100000000, 0))

		price = ticketPriceInfo.TicketPriceStr
		priceDe, _ := decimal.NewFromString(price)
		priceUsd = priceDe.Mul(satUsdDe).StringFixed(3)

		return &respond.Mrc20TickMarketPriceResp{
			TickId:        ticketPriceInfo.TicketId,
			Tick:          ticketPriceInfo.Tick,
			TokenName:     ticketPriceInfo.TokenName,
			Decimals:      decimals,
			Supply:        "",
			TotalVolume:   totalVolume,
			MarketCap:     marketCap,
			MarketCapUsd:  marketCapUsd,
			LastPrice:     price,
			LastPriceUsd:  priceUsd,
			Price:         price,
			PriceUsd:      priceUsd,
			FloorPrice:    floorPrice,
			FloorPriceUsd: floorPriceUsd,
		}, nil
	}

	marketInfo, err = models.MarketMrc20InfoModelDao().GetOne(&models.MarketMrc20InfoModel{
		TickId: req.TickId,
	})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if marketInfo == nil {
		return nil, errors.New("mrc20 market info not found")

	}
	price = strconv.FormatFloat(marketInfo.LastPrice, 'f', -1, 64)
	floorPrice = strconv.FormatFloat(marketInfo.FloorPrice, 'f', -1, 64)
	marketCap = strconv.FormatInt(marketInfo.MarketCap, 10)

	btcUsd := GetPriceForUsd("BTC")
	btcUsdDe, _ := decimal.NewFromString(btcUsd)
	satUsdDe := btcUsdDe.Div(decimal.New(100000000, 0))

	priceDe, _ := decimal.NewFromString(price)
	priceUsd = priceDe.Mul(satUsdDe).StringFixed(3)
	floorPriceDe, _ := decimal.NewFromString(floorPrice)
	floorPriceUsd = floorPriceDe.Mul(satUsdDe).StringFixed(3)
	marketCapDe, _ := decimal.NewFromString(marketCap)
	marketCapUsd = marketCapDe.Mul(satUsdDe).StringFixed(3)

	totalVolume = marketInfo.TotalVolume
	return &respond.Mrc20TickMarketPriceResp{
		TickId:        marketInfo.TickId,
		Tick:          marketInfo.Tick,
		TokenName:     marketInfo.TokenName,
		Decimals:      marketInfo.Decimals,
		Supply:        marketInfo.Supply,
		TotalVolume:   totalVolume,
		MarketCap:     marketCap,
		MarketCapUsd:  marketCapUsd,
		LastPrice:     price,
		LastPriceUsd:  priceUsd,
		Price:         price,
		PriceUsd:      priceUsd,
		FloorPrice:    floorPrice,
		FloorPriceUsd: floorPriceUsd,
	}, nil
}

func FetchMrc20IdCoinsTickAddressUtxos(req *request.Mrc20IdCoinsAddressUtxosReq) (*respond.Mrc20UtxoResp, error) {
	var (
		err            error
		list           []*respond.Mrc20Utxo = make([]*respond.Mrc20Utxo, 0)
		total          int64                = 0
		idCoinsTickIds []string             = make([]string, 0)

		grpcResp *mrc20_utxo_service.Mrc20UtxoResponse
	)
	if req.TickId != "" {
		idCoinsTickIds = append(idCoinsTickIds, req.TickId)
	} else {
		respOrders, _ := orders_exchange_service.FetchIdCoinsTickIds(nil)
		if respOrders != nil {
			idCoinsTickIds = respOrders.TickIds
		}
	}

	client, err := grpc_service.GetMrc20BaseConn()
	if err != nil {
		return nil, err
	}
	grpcResp, err = client.FetchMrc20AddressUtxoList(idCoinsTickIds, req.Address, req.Cursor, req.Size)
	if err != nil {
		return nil, err
	}
	if grpcResp == nil {
		return nil, errors.New("grpc response is empty")
	}

	if grpcResp != nil {
		for _, v := range grpcResp.GetDetail() {

			mrc20InfoList := make([]*respond.Mrc20Info, 0)
			for _, m := range v.GetMrc20S() {
				mrc20InfoList = append(mrc20InfoList, &respond.Mrc20Info{
					Tick:     m.GetTick(),
					Mrc20Id:  m.GetMrc20Id(),
					TxPoint:  m.GetTxPoint(),
					Amount:   m.GetAmount(),
					Decimals: m.GetDecimals(),
				})
			}

			item := &respond.Mrc20Utxo{
				Chain:       v.GetChain(),
				BlockHeight: v.GetBlockHeight(),
				Address:     v.GetAddress(),
				Satoshi:     v.GetSatoshi(),
				Satoshis:    v.GetSatoshis(),
				ScriptPk:    v.GetScriptPk(),
				TxId:        v.GetTxId(),
				Vout:        v.GetVout(),
				OutputIndex: v.GetOutputIndex(),
				Mrc20s:      mrc20InfoList,
				Timestamp:   v.GetTimestamp(),
				OrderId:     "",
			}
			utxoId := fmt.Sprintf("%s_%d", v.GetTxId(), v.GetVout())
			orderEntity, _ := models.MarketMrc20OrderModelDao().GetOne(&models.MarketMrc20OrderModel{
				UtxoId:        utxoId,
				SellerAddress: req.Address,
				OrderState:    models.OrderStateCreate,
			})
			if orderEntity != nil {
				item.OrderId = orderEntity.OrderId
			}
			list = append(list, item)
		}
		total = grpcResp.Total
	}
	return &respond.Mrc20UtxoResp{
		Total: total,
		List:  list,
	}, nil
}

func FetchMrc20Holders(req *request.Mrc20TickHoldersRequest) (*respond.Mrc20TickHolderResp, error) {
	var (
		total int64                 = 0
		list  []*respond.HolderInfo = make([]*respond.HolderInfo, 0)

		tickInfo *man_service.Mrc20TickInfo
		grpcResp *mrc20_holders_service.Mrc20TickHoldersResponse
		err      error
	)
	if req.TickId == "" && req.Tick == "" {
		return nil, errors.New("tickId and tick are empty")
	}
	tickInfo, err = man_service.FetchMrc20TickInfo(req.TickId, req.Tick)
	if err != nil {
		return nil, err
	}
	if tickInfo == nil || tickInfo.Mrc20Id == "" {
		return nil, errors.New("mrc20 not found")
	}
	supply := "0"
	if tickInfo.AmtPerMint != "" && tickInfo.MintCount != 0 {
		totalMintedDe := decimal.New(tickInfo.TotalMinted, 0)
		//premineCountDe := decimal.New(v.PremineCount, 0)
		amtPerMintDe, _ := decimal.NewFromString(tickInfo.AmtPerMint)
		mintCountDe := decimal.New(tickInfo.MintCount, 0)

		if totalMintedDe.GreaterThan(mintCountDe) {
			supplyDe := mintCountDe.Mul(amtPerMintDe)
			supply = supplyDe.String()
		} else {
			supplyDe := totalMintedDe.Mul(amtPerMintDe)
			supply = supplyDe.String()
		}
	}
	supplyDe, _ := decimal.NewFromString(supply)

	client, err := grpc_service.GetMrc20BaseConn()
	if err != nil {
		return nil, err
	}
	grpcResp, err = client.FetchMrc20TickHolders(tickInfo.Mrc20Id, req.Cursor, req.Size)
	if err != nil {
		return nil, err
	}
	if grpcResp == nil {
		return nil, errors.New("grpc response is empty")
	}
	total = grpcResp.Total
	for _, v := range grpcResp.Detail {
		balanceDe, _ := decimal.NewFromString(v.GetBalance())
		proportion := balanceDe.Div(supplyDe).Mul(decimal.New(100, 0)).StringFixed(2)

		list = append(list, &respond.HolderInfo{
			TickId:     tickInfo.Mrc20Id,
			Tick:       tickInfo.Tick,
			TokenName:  tickInfo.TokenName,
			MetaId:     common.GetMetaIdByAddress(v.GetAddress()),
			Address:    v.GetAddress(),
			UserInfo:   common.FetchMetaIDUserInfo(v.GetAddress()),
			Balance:    v.GetBalance(),
			Proportion: proportion,
		})
	}
	return &respond.Mrc20TickHolderResp{
		Total: total,
		List:  list,
	}, nil
}

func CheckUtxoInfo(req *request.CheckUtxoInfoReq) (map[string]*own_service.OwnUtxoInfo, error) {
	var (
		err      error
		utxoInfo map[string]*own_service.OwnUtxoInfo
	)
	utxoInfo, err = own_service.CheckUtxoInfo("", req.OutPoints)
	if err != nil {
		return nil, err
	}
	if utxoInfo == nil {
		return nil, errors.New("utxo not found")
	}
	return utxoInfo, nil
}
