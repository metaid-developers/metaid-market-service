package admin_service

import (
	"errors"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
	"metaid-market-service/models"
	"metaid-market-service/service/auto_service"
	"metaid-market-service/service/common_service"
)

func AddAutoCreateBridge(req *request.AddAutoCreateBridgeRequest) (string, error) {
	var (
		entity *models.MarketMrc20InfoModel
		err    error
	)
	entity, err = models.MarketMrc20InfoModelDao().GetOne(&models.MarketMrc20InfoModel{
		TickId: req.TickId,
	})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	if entity == nil {
		return "", errors.New("mrc20 not exists")
	}

	btcUsd := common_service.GetPriceForUsd("BTC")
	btcUsdDe, _ := decimal.NewFromString(btcUsd)
	satUsdDe := btcUsdDe.Div(decimal.New(100000000, 0))
	marketCapSat := entity.MarketCap
	marketCapSatDe := decimal.New(marketCapSat, 0)
	marketCapUsdtDe := marketCapSatDe.Mul(satUsdDe)

	marketCapUsdt := marketCapUsdtDe.IntPart()

	err = auto_service.MakeAutoCreateBridge(entity, req.TickId, entity.OrderCount, marketCapSat, marketCapUsdt)
	if err != nil {
		return "", err
	}
	return "success", nil
}

func GetMarketAutoBridgeCreateInfo(req *request.FetchAutoCreateBridgeInfoRequest) (*respond.FetchAutoCreateBridgeInfoRequest, error) {
	var (
		entity *models.MarketAutoBridgeCreateModel
		err    error
	)
	entity, err = models.MarketAutoBridgeCreateModelDao().GetOne(&models.MarketAutoBridgeCreateModel{
		TickId: req.TickId,
	})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if entity == nil {
		return nil, errors.New("mrc20 not exists")
	}

	return &respond.FetchAutoCreateBridgeInfoRequest{
		TickId:        entity.TickId,
		Tick:          entity.Tick,
		TokenName:     entity.TokenName,
		Decimals:      entity.Decimals,
		OrderCount:    entity.OrderCount,
		MarketCapSat:  entity.MarketCapSat,
		MarketCapUsdt: entity.MarketCapUsdt,
		AutoStatus:    entity.AutoStatus,
	}, nil
}
