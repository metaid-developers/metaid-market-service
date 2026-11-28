package order_service

import (
	"errors"
	"metaid-market-service/common"
	"metaid-market-service/conf"
	"metaid-market-service/models"
	"metaid-market-service/service/common_service"
	"strings"

	"github.com/shopspring/decimal"
)

func IsDustBroadcastRejection(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "dust")
}

func IsDeterministicBroadcastRejection(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	alreadyAccepted := []string{
		"already in block chain",
		"txn-already-known",
		"txn-already-in-mempool",
		"already known",
	}
	for _, s := range alreadyAccepted {
		if strings.Contains(msg, s) {
			return false
		}
	}
	rejections := []string{
		"dust",
		"min relay fee not met",
		"insufficient fee",
		"bad-txns-inputs-missingorspent",
		"missing inputs",
		"bad-txns-inputs-duplicate",
		"bad-txns-nonfinal",
		"non-final",
		"non-mandatory-script-verify-flag",
		"mandatory-script-verify-flag-failed",
		"txn-mempool-conflict",
		"tx-size",
	}
	for _, s := range rejections {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}

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
		if IsDeterministicBroadcastRejection(err) {
			txId := entity.TxId
			orders := []*models.MarketOrderModel{entity}
			if txId != "" {
				if peers, listErr := models.MarketOrderModelDao().GetList(&models.MarketOrderModel{TxId: txId, OrderState: models.OrderStateFinish}, 0, 100); listErr == nil && len(peers) > 0 {
					orders = peers
				}
			}
			canRevert := !IsDustBroadcastRejection(err)
			for _, o := range orders {
				if o.MakerPsbt == "" {
					canRevert = false
					break
				}
			}
			if canRevert {
				_ = models.RevertBatchPinOrdersBroadcast(orders, txId)
			} else {
				_ = models.CancelFailedBatchPinOrdersBroadcast(orders, txId)
			}
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
		if IsDeterministicBroadcastRejection(err) {
			if IsDustBroadcastRejection(err) || entity.MakerPsbt == "" {
				_ = models.CancelFailedMrc20OrderBroadcast(entity, "Broadcast rejected: "+err.Error())
			} else {
				_ = models.RevertMrc20OrderBroadcast(entity)
			}
		}
		return err
	}
	if broadcastTxId != entity.TxId {
		return errors.New("recovery broadcast transaction id mismatch")
	}
	return finalize()
}
