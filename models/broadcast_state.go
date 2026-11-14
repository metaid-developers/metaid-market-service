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
		q.MakerPsbt = ""
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
		q.MakerPsbt = ""
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
			q.MakerPsbt = ""
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
