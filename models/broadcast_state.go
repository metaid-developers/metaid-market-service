package models

import (
	"errors"
	"metaid-market-service/major"
	"metaid-market-service/tool"

	"gorm.io/gorm"
)

func savePreparedOrderTx(tx *gorm.DB, id, version int64, model interface{}) error {
	result := tx.Where("id = ? AND version = ? AND orderState = ?", id, version, OrderStateCreate).Save(model)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("order was changed by another request")
	}
	return nil
}

func reserveDummyUtxoTx(tx *gorm.DB, utxo *MarketUtxoModel) error {
	version := utxo.Version
	utxo.Version++
	utxo.UpdateTime = tool.MakeTimestamp()
	result := tx.Where("id = ? AND version = ? AND usedState = ?", utxo.Id, version, UsedNo).Save(utxo)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("dummy UTXO was reserved by another request")
	}
	return nil
}

func createDummyUtxosTx(tx *gorm.DB, utxos []*MarketUtxoModel) error {
	for _, utxo := range utxos {
		if err := tx.Create(utxo).Error; err != nil {
			return err
		}
	}
	return nil
}

func PrepareMrc20OrderForBroadcast(q *MarketMrc20OrderModel, usedUtxoList, newUtxoList []*MarketUtxoModel) error {
	if q == nil || q.FinalPsbt == "" || q.TxId == "" {
		return errors.New("signed order transaction is missing")
	}
	return major.GetSqlDB().Transaction(func(tx *gorm.DB) error {
		now := tool.MakeTimestamp()
		q.DealTime = now
		q.UpdateTime = now
		q.OrderState = OrderStateFinish
		q.ConfirmationState = ConfirmationStateUnconfirmed
		q.TakerPsbt = ""
		version := q.Version
		q.Version++
		if err := savePreparedOrderTx(tx, q.Id, version, q); err != nil {
			return err
		}
		for _, utxo := range usedUtxoList {
			if err := reserveDummyUtxoTx(tx, utxo); err != nil {
				return err
			}
		}
		return createDummyUtxosTx(tx, newUtxoList)
	})
}

func PreparePinOrderForBroadcast(q *MarketOrderModel, usedUtxoList, newUtxoList []*MarketUtxoModel) error {
	if q == nil || q.FinalPsbt == "" || q.TxId == "" {
		return errors.New("signed order transaction is missing")
	}
	return major.GetSqlDB().Transaction(func(tx *gorm.DB) error {
		now := tool.MakeTimestamp()
		q.DealTime = now
		q.UpdateTime = now
		q.OrderState = OrderStateFinish
		q.ConfirmationState = ConfirmationStateUnconfirmed
		q.TakerPsbt = ""
		version := q.Version
		q.Version++
		if err := savePreparedOrderTx(tx, q.Id, version, q); err != nil {
			return err
		}
		for _, utxo := range usedUtxoList {
			if err := reserveDummyUtxoTx(tx, utxo); err != nil {
				return err
			}
		}
		return createDummyUtxosTx(tx, newUtxoList)
	})
}

func PrepareBatchPinOrdersForBroadcast(orders []*MarketOrderModel, usedUtxoList, newUtxoList []*MarketUtxoModel) error {
	if len(orders) == 0 {
		return errors.New("orders are empty")
	}
	return major.GetSqlDB().Transaction(func(tx *gorm.DB) error {
		now := tool.MakeTimestamp()
		for _, q := range orders {
			if q == nil || q.FinalPsbt == "" || q.TxId == "" {
				return errors.New("signed batch transaction is missing")
			}
			q.DealTime = now
			q.UpdateTime = now
			q.OrderState = OrderStateFinish
			q.ConfirmationState = ConfirmationStateUnconfirmed
			q.TakerPsbt = ""
			version := q.Version
			q.Version++
			if err := savePreparedOrderTx(tx, q.Id, version, q); err != nil {
				return err
			}
		}
		for _, utxo := range usedUtxoList {
			if err := reserveDummyUtxoTx(tx, utxo); err != nil {
				return err
			}
		}
		return createDummyUtxosTx(tx, newUtxoList)
	})
}

func releaseReservedDummyUtxosByTxIdTx(tx *gorm.DB, txId string) error {
	if txId == "" {
		return nil
	}
	now := tool.MakeTimestamp()
	if err := tx.Model(&MarketUtxoModel{}).
		Where("usedTxId = ? AND usedState = ?", txId, UsedYes).
		Updates(map[string]interface{}{
			"usedState":  UsedNo,
			"usedTxId":   "",
			"orderId":    "",
			"updateTime": now,
			"version":    gorm.Expr("version + 1"),
		}).Error; err != nil {
		return err
	}
	if err := tx.Where("txId = ? AND usedState = ?", txId, UsedNo).Delete(&MarketUtxoModel{}).Error; err != nil {
		return err
	}
	return nil
}

func RevertMrc20OrderBroadcast(q *MarketMrc20OrderModel) error {
	if q == nil {
		return errors.New("order is nil")
	}
	txId := q.TxId
	return major.GetSqlDB().Transaction(func(tx *gorm.DB) error {
		now := tool.MakeTimestamp()
		version := q.Version
		q.OrderState = OrderStateCreate
		q.ConfirmationState = ConfirmationStateUnconfirmed
		q.FinalPsbt = ""
		q.TakerPsbt = ""
		q.TxId = ""
		q.BuyerAddress = ""
		q.BuyerIp = ""
		q.DealTime = 0
		q.FeeAmount = 0
		q.FeeRate = 0
		q.UpdateTime = now
		q.Version++
		result := tx.Where("id = ? AND version = ? AND orderState = ?", q.Id, version, OrderStateFinish).Save(q)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("order revert conflict")
		}
		return releaseReservedDummyUtxosByTxIdTx(tx, txId)
	})
}

func CancelFailedMrc20OrderBroadcast(q *MarketMrc20OrderModel, reason string) error {
	if q == nil {
		return errors.New("order is nil")
	}
	txId := q.TxId
	return major.GetSqlDB().Transaction(func(tx *gorm.DB) error {
		now := tool.MakeTimestamp()
		version := q.Version
		q.OrderState = OrderStateCancel
		q.ConfirmationState = ConfirmationStateUnconfirmed
		q.FinalPsbt = ""
		q.TakerPsbt = ""
		q.MakerPsbt = ""
		q.TxId = ""
		q.BuyerAddress = ""
		q.BuyerIp = ""
		q.DealTime = 0
		q.Reason = reason
		q.UpdateTime = now
		q.Version++
		result := tx.Where("id = ? AND version = ? AND orderState = ?", q.Id, version, OrderStateFinish).Save(q)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("order cancel conflict")
		}
		return releaseReservedDummyUtxosByTxIdTx(tx, txId)
	})
}

func RevertPinOrderBroadcast(q *MarketOrderModel) error {
	if q == nil {
		return errors.New("order is nil")
	}
	return RevertBatchPinOrdersBroadcast([]*MarketOrderModel{q}, q.TxId)
}

func RevertBatchPinOrdersBroadcast(orders []*MarketOrderModel, txId string) error {
	if len(orders) == 0 {
		return errors.New("orders are empty")
	}
	return major.GetSqlDB().Transaction(func(tx *gorm.DB) error {
		now := tool.MakeTimestamp()
		for _, q := range orders {
			if q == nil {
				continue
			}
			version := q.Version
			q.OrderState = OrderStateCreate
			q.ConfirmationState = ConfirmationStateUnconfirmed
			q.FinalPsbt = ""
			q.TakerPsbt = ""
			q.TxId = ""
			q.BuyerAddress = ""
			q.BuyerIp = ""
			q.DealTime = 0
			q.FeeAmount = 0
			q.FeeRate = 0
			q.UpdateTime = now
			q.Version++
			result := tx.Where("id = ? AND version = ? AND orderState = ?", q.Id, version, OrderStateFinish).Save(q)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return errors.New("order revert conflict")
			}
		}
		return releaseReservedDummyUtxosByTxIdTx(tx, txId)
	})
}

func CancelFailedPinOrderBroadcast(q *MarketOrderModel) error {
	if q == nil {
		return errors.New("order is nil")
	}
	return CancelFailedBatchPinOrdersBroadcast([]*MarketOrderModel{q}, q.TxId)
}

func CancelFailedBatchPinOrdersBroadcast(orders []*MarketOrderModel, txId string) error {
	if len(orders) == 0 {
		return errors.New("orders are empty")
	}
	return major.GetSqlDB().Transaction(func(tx *gorm.DB) error {
		now := tool.MakeTimestamp()
		for _, q := range orders {
			if q == nil {
				continue
			}
			version := q.Version
			q.OrderState = OrderStateCancel
			q.ConfirmationState = ConfirmationStateUnconfirmed
			q.FinalPsbt = ""
			q.TakerPsbt = ""
			q.MakerPsbt = ""
			q.TxId = ""
			q.BuyerAddress = ""
			q.BuyerIp = ""
			q.DealTime = 0
			q.UpdateTime = now
			q.Version++
			result := tx.Where("id = ? AND version = ? AND orderState = ?", q.Id, version, OrderStateFinish).Save(q)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return errors.New("order cancel conflict")
			}
		}
		return releaseReservedDummyUtxosByTxIdTx(tx, txId)
	})
}
