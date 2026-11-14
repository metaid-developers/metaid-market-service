package order_service

import (
	"errors"
	"metaid-market-service/common"
	"metaid-market-service/conf"
	"metaid-market-service/models"
	"metaid-market-service/service/common_service"

	"github.com/shopspring/decimal"
)

func RetryPinOrderBroadcast(entity *models.MarketOrderModel) error {
	if entity == nil || entity.FinalPsbt == "" {
		return nil
	}
	if _, err := common_service.GetTxInfo(entity.Chain, entity.TxId); err == nil {
		return models.MarketOrderModelDao().UpdateOrderEntityListForJobFunc(entity.Chain, entity, nil, nil, "", nil)
	}
	builder, err := common.NewPsbtBuilder(common.GetNetParams(conf.Net, entity.Chain), entity.FinalPsbt)
	if err != nil {
		return err
	}
	raw, err := builder.ExtractPsbtTransaction()
	if err != nil {
		return err
	}
	broadcastTxId, err := common.BroadcastTx(entity.Chain, raw)
	if err != nil {
		if _, checkErr := common_service.GetTxInfo(entity.Chain, entity.TxId); checkErr == nil {
			return models.MarketOrderModelDao().UpdateOrderEntityListForJobFunc(entity.Chain, entity, nil, nil, "", nil)
		}
		return err
	}
	if broadcastTxId != entity.TxId {
		return errors.New("recovery broadcast transaction id mismatch")
	}
	return models.MarketOrderModelDao().UpdateOrderEntityListForJobFunc(entity.Chain, entity, nil, nil, "", nil)
}

func RetryMrc20OrderBroadcast(entity *models.MarketMrc20OrderModel) error {
	if entity == nil || entity.FinalPsbt == "" {
		return nil
	}
	finalize := func() error {
		supply := "0"
		tickInfo, err := common_service.GetMrc20TickInfo(entity.TickId, "")
		if err != nil {
			return err
		}
		totalMinted, _ := decimal.NewFromString(tickInfo.TotalMinted)
		amountPerMint, _ := decimal.NewFromString(tickInfo.AmtPerMint)
		if totalMinted.GreaterThan(decimal.Zero) {
			supply = totalMinted.Mul(amountPerMint).String()
		}
		return models.MarketMrc20OrderModelDao().UpdateOrderEntityListForJobFunc(entity.Chain, entity, nil, nil, supply, "", nil)
	}
	if _, err := common_service.GetTxInfo(entity.Chain, entity.TxId); err == nil {
		return finalize()
	}
	builder, err := common.NewPsbtBuilder(common.GetNetParams(conf.Net, entity.Chain), entity.FinalPsbt)
	if err != nil {
		return err
	}
	raw, err := builder.ExtractPsbtTransaction()
	if err != nil {
		return err
	}
	broadcastTxId, err := common.BroadcastTx(entity.Chain, raw)
	if err != nil {
		if _, checkErr := common_service.GetTxInfo(entity.Chain, entity.TxId); checkErr == nil {
			return finalize()
		}
		return err
	}
	if broadcastTxId != entity.TxId {
		return errors.New("recovery broadcast transaction id mismatch")
	}
	return finalize()
}
