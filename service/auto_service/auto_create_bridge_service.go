package auto_service

import (
	"errors"
	"fmt"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"metaid-market-service/conf"
	"metaid-market-service/models"
	"metaid-market-service/service/common_service"
	"metaid-market-service/service/orders_exchange_service"
	"metaid-market-service/tool"
)

func MakeAutoCreateBridge(marketTickInfo *models.MarketMrc20InfoModel, mrc20Id string, orderCount, marketCapSat, marketCapUsdt int64) error {
	var (
		tickId string = mrc20Id

		entity *models.MarketAutoBridgeCreateModel
		err    error

		nowTime int64 = tool.MakeTimestamp()
	)

	entity, err = models.MarketAutoBridgeCreateModelDao().GetOne(&models.MarketAutoBridgeCreateModel{
		TickId: tickId,
	})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return errors.New("[MakeAutoCreateBridge]Get autoBridge" + err.Error())
	}
	if entity != nil {
		if entity.AutoStatus == models.AutoStatusFinish {
			return errors.New("[MakeAutoCreateBridge]autoBridge already exists and finished")
		} else if entity.AutoStatus == models.AutoStatusCreate {
			return errors.New("[MakeAutoCreateBridge]autoBridge already exists")
		} else {
			return errors.New("[MakeAutoCreateBridge]autoBridge already exists but status error")
		}
	}

	entity = &models.MarketAutoBridgeCreateModel{
		TickId:        tickId,
		Tick:          marketTickInfo.Tick,
		TokenName:     marketTickInfo.TokenName,
		Decimals:      marketTickInfo.Decimals,
		OrderCount:    orderCount,
		MarketCapSat:  marketCapSat,
		MarketCapUsdt: marketCapUsdt,
		AutoStatus:    models.AutoStatusCreate,
		Timestamp:     nowTime,
		Version:       0,
		CreateTime:    nowTime,
		UpdateTime:    0,
		State:         models.STATE_EXIST,
	}

	job := func() error {
		respInfo, err := orders_exchange_service.AddBridgeBuild(&orders_exchange_service.AdminAddAutoBridgeReq{
			Mrc20Id: tickId,
		}, nil)
		if err != nil {
			return err
		}
		_ = respInfo
		return nil
	}

	err = models.MarketAutoBridgeCreateModelDao().SetAndFunc(entity, marketTickInfo, job)
	if err != nil {
		return errors.New("[MakeAutoCreateBridge]Set autoBridge" + err.Error())
	}

	return nil
}

func CheckMarketTickInfo() {
	var (
		entityList []*models.MarketMrc20InfoModel

		orderCountLimit    int64 = conf.AutoBridgeRuleConfigData.OrderCountLimit
		marketCapUsdtLimit int64 = conf.AutoBridgeRuleConfigData.MarketCapUsdtLimit

		cursor int64 = 0
		limit  int64 = 1000
	)

	btcUsd := common_service.GetPriceForUsd("BTC")
	btcUsdDe, _ := decimal.NewFromString(btcUsd)
	satUsdDe := btcUsdDe.Div(decimal.New(100000000, 0))

	entityList, _ = models.MarketMrc20InfoModelDao().GetCanAutoBridgeList(&models.MarketMrc20InfoModel{}, orderCountLimit, cursor, limit)
	fmt.Printf("CheckMarketTickInfo entityList len:%d\n", len(entityList))
	for _, entity := range entityList {
		if entity.OrderCount < orderCountLimit {
			continue
		}

		marketCapSat := entity.MarketCap
		marketCapSatDe := decimal.New(marketCapSat, 0)
		marketCapUsdtDe := marketCapSatDe.Mul(satUsdDe)

		marketCapUsdtLimitDe := decimal.New(marketCapUsdtLimit, 0)

		if marketCapUsdtDe.LessThan(marketCapUsdtLimitDe) {
			continue
		}
		marketCapUsdt := marketCapUsdtDe.IntPart()

		err := MakeAutoCreateBridge(entity, entity.TickId, entity.OrderCount, marketCapSat, marketCapUsdt)
		if err != nil {
			continue
		}
		fmt.Printf("CheckMarketTickInfo MakeAutoCreateBridge success:%s\n", entity.TickId)
	}
}

func SyncAutoBridgeInfo() {
	var (
		entityList []*models.MarketAutoBridgeCreateModel

		cursor int64 = 0
		limit  int64 = 1000
	)

	entityList, _ = models.MarketAutoBridgeCreateModelDao().GetList(&models.MarketAutoBridgeCreateModel{
		AutoStatus: models.AutoStatusCreate,
	}, cursor, limit)
	fmt.Printf("SyncAutoBridgeInfo entityList len:%d\n", len(entityList))
	for _, entity := range entityList {
		if entity.AutoStatus != models.AutoStatusCreate {
			continue
		}

		respInfo, err := orders_exchange_service.GetBridgeBuildInfo(&orders_exchange_service.AdminAutoBridgeInfoReq{
			Mrc20Id: entity.TickId,
		}, nil)
		if err != nil {
			fmt.Printf("SyncAutoBridgeInfo GetBridgeBuildInfo error:%s\n", err.Error())
			continue
		}
		if respInfo == nil {
			fmt.Printf("SyncAutoBridgeInfo GetBridgeBuildInfo respInfo nil\n")
			continue
		}
		if respInfo.BridgeBuildStatus == orders_exchange_service.BridgeBuildStatusUpdateSwapPoolSuccess {
			marketInfo, err := models.MarketMrc20InfoModelDao().GetOne(&models.MarketMrc20InfoModel{
				TickId: entity.TickId,
			})
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				fmt.Printf("SyncAutoBridgeInfo GetOne error:%s\n", err.Error())
				continue
			}
			if marketInfo == nil {
				fmt.Printf("SyncAutoBridgeInfo marketInfo nil\n")
				continue
			}

			entity.AutoStatus = models.AutoStatusFinish
			entity.UpdateTime = tool.MakeTimestamp()
			err = models.MarketAutoBridgeCreateModelDao().UpdateAndMarketInfo(entity, marketInfo)
			if err != nil {
				fmt.Printf("SyncAutoBridgeInfo Set error:%s\n", err.Error())
				continue
			}
		}

		fmt.Printf("SyncAutoBridgeInfo MakeAutoCreateBridge success:%s\n", entity.TickId)
	}

}
