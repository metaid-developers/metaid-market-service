package models

import (
	"errors"
	"fmt"
	"gorm.io/gorm"
	"metaid-market-service/major"
	"metaid-market-service/tool"
	"sync"
)

type AssetType string

const (
	AssetTypePins     AssetType = "pins"
	AssetTypeOrdinals AssetType = "ordinals"
)

type OrderState int64

const (
	OrderStateCreate OrderState = 1
	OrderStateCancel OrderState = 2
	OrderStateFinish OrderState = 3
)

type ConfirmationState int64

const (
	ConfirmationStateUnconfirmed ConfirmationState = 1
	ConfirmationStateConfirmed   ConfirmationState = 2
)

type MarketOrderModel struct {
	Id                int64             `gorm:"column:id" json:"id"`
	OrderId           string            `gorm:"column:orderId" json:"orderId"`
	UtxoId            string            `gorm:"column:utxoId" json:"utxoId"`
	OutValue          int64             `gorm:"column:outValue" json:"outValue"`
	AssetId           string            `gorm:"column:assetId" json:"assetId"`
	AssetType         AssetType         `gorm:"column:assetType" json:"assetType"`
	AssetNumber       int64             `gorm:"column:assetNumber" json:"assetNumber"`
	AssetLevel        int64             `gorm:"column:assetLevel" json:"assetLevel"`
	AssetPath         string            `gorm:"column:assetPath" json:"assetPath"`
	AssetPop          string            `gorm:"column:assetPop" json:"assetPop"`
	OrderState        OrderState        `gorm:"column:orderState" json:"orderState"`
	SellerAddress     string            `gorm:"column:sellerAddress" json:"sellerAddress"`
	SellerIp          string            `gorm:"column:sellerIp" json:"sellerIp"`
	BuyerAddress      string            `gorm:"column:buyerAddress" json:"buyerAddress"`
	BuyerIp           string            `gorm:"column:buyerIp" json:"buyerIp"`
	SellPriceAmount   int64             `gorm:"column:sellPriceAmount" json:"sellPriceAmount"`
	SellPriceDecimal  int64             `gorm:"column:sellPriceDecimal" json:"sellPriceDecimal"`
	SellPriceCoin     string            `gorm:"column:sellPriceCoin" json:"sellPriceCoin"`
	FeeAmount         int64             `gorm:"column:feeAmount" json:"feeAmount"`
	FeeRate           int64             `gorm:"column:feeRate" json:"feeRate"`
	Content           string            `gorm:"column:content" json:"content"`
	Preview           string            `gorm:"column:preview" json:"preview"`
	Detail            string            `gorm:"column:detail" json:"detail"`
	MakerPsbt         string            `gorm:"column:makerPsbt" json:"makerPsbt"`
	TakerPsbt         string            `gorm:"column:takerPsbt" json:"takerPsbt"`
	FinalPsbt         string            `gorm:"column:finalPsbt" json:"finalPsbt"`
	TxId              string            `gorm:"column:txId" json:"txId"`
	DealTime          int64             `gorm:"column:dealTime" json:"dealTime"`
	BlockHeight       int64             `gorm:"column:blockHeight" json:"blockHeight"`
	ConfirmationState ConfirmationState `gorm:"column:confirmationState" json:"confirmationState"`
	Timestamp         int64             `gorm:"column:timestamp" json:"timestamp"`
	Version           int64             `gorm:"column:version" json:"version"`
	CreateTime        int64             `gorm:"column:createTime" json:"createTime"`
	UpdateTime        int64             `gorm:"column:updateTime" json:"updateTime"`
	State             int64             `gorm:"column:state" json:"state"`
}

func (MarketOrderModel) TableName() string {
	return "tb_market_order"
}

var _marketOrderModelOnce sync.Once
var _marketOrderModelManager *marketOrderModelDao

type marketOrderModelDao struct {
}

func MarketOrderModelDao() *marketOrderModelDao {
	_marketOrderModelOnce.Do(func() {
		_marketOrderModelManager = &marketOrderModelDao{}
	})
	return _marketOrderModelManager
}

func (_ *marketOrderModelDao) Set(model *MarketOrderModel) error {
	if model == nil {
		return errors.New("model is nil")
	}
	tx := major.GetSqlDB().Create(model)
	if tx.Error != nil {
		return tx.Error
	}
	return nil
}

func (_ *marketOrderModelDao) GetOne(qo *MarketOrderModel) (*MarketOrderModel, error) {
	model := &MarketOrderModel{}
	tx := major.GetSqlDB().Where(qo).First(model)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return model, nil
}

func (_ *marketOrderModelDao) GetLastOne(qo *MarketOrderModel) (*MarketOrderModel, error) {
	model := &MarketOrderModel{}
	tx := major.GetSqlDB().Where(qo).Order("blockHeight desc").First(model)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return model, nil
}

func (_ *marketOrderModelDao) GetList(qo *MarketOrderModel, offset, limit int64) ([]*MarketOrderModel, error) {
	var models []*MarketOrderModel
	tx := major.GetSqlDB().Where(qo).Limit(int(limit)).Offset(int(offset)).Order("timestamp asc").Find(&models)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return models, nil
}

func (_ *marketOrderModelDao) CountByState(qo *MarketOrderModel, address, prefixPath string) (int64, error) {
	var count int64
	filter := ""
	if qo.OrderState == OrderStateFinish {
		qo.SellerAddress = ""
		filter = fmt.Sprintf("sellerAddress = '%s' or buyerAddress = '%s'", address, address)
	}
	if prefixPath != "" {
		if filter != "" {
			filter += fmt.Sprintf(" and assetPath like '%s%%'", prefixPath)
		} else {
			filter = fmt.Sprintf("assetPath like '%s%%'", prefixPath)
		}
	}
	tx := major.GetSqlDB().Model(&MarketOrderModel{}).Where(qo).Where(filter).Count(&count)
	if tx.Error != nil {
		return 0, tx.Error
	}
	return count, nil
}

func (_ *marketOrderModelDao) GetListByState(qo *MarketOrderModel, address string, offset, limit int64, sortKey, sortType, prefixPath string) ([]*MarketOrderModel, error) {
	var models []*MarketOrderModel
	filter := ""
	if qo.OrderState == OrderStateFinish {
		qo.SellerAddress = ""
		filter = fmt.Sprintf("sellerAddress = '%s' or buyerAddress = '%s'", address, address)
	}
	if prefixPath != "" {
		if filter != "" {
			filter += fmt.Sprintf(" and assetPath like '%s%%'", prefixPath)
		} else {
			filter = fmt.Sprintf("assetPath like '%s%%'", prefixPath)
		}
	}
	tx := major.GetSqlDB().Where(qo).Where(filter).Limit(int(limit)).Offset(int(offset)).Order(fmt.Sprintf("%s %s", sortKey, sortType)).Find(&models)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return models, nil
}

func (_ *marketOrderModelDao) Count(qo *MarketOrderModel) (int64, error) {
	var count int64
	filter := ""
	tx := major.GetSqlDB().Model(&MarketOrderModel{}).Where(qo).Where(filter).Count(&count)
	if tx.Error != nil {
		return 0, tx.Error
	}
	return count, nil
}

func (_ *marketOrderModelDao) GetAll(qo *MarketOrderModel) ([]MarketOrderModel, error) {
	var models []MarketOrderModel
	tx := major.GetSqlDB().Where(qo).Find(&models)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return models, nil
}

func (_ *marketOrderModelDao) Update(q *MarketOrderModel) error {
	if q == nil {
		return errors.New("model is nil")
	}

	sv := q.Version
	q.Version += 1
	q.UpdateTime = tool.MakeTimestamp()
	tx := major.GetSqlDB().Where(map[string]interface{}{"version": sv, "id": q.Id}).Updates(q)
	if tx.Error != nil {
		return tx.Error
	}
	return nil
}

func (_ *marketOrderModelDao) BatchSaveList(list []*MarketOrderModel) error {
	err := major.GetSqlDB().Transaction(func(tx *gorm.DB) error {
		for _, model := range list {
			if model.Id == 0 {
				if err := tx.Create(model).Error; err != nil {
					return err
				}
			} else {
				sv := model.Version
				model.Version += 1
				model.UpdateTime = tool.MakeTimestamp()
				if err := tx.Save(model).Where(map[string]interface{}{"version": sv, "id": model.Id}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (_ *marketOrderModelDao) UpdateOrderEntityListForJobFunc(q *MarketOrderModel, usedUtxoList, newUtxoList []*MarketUtxoModel, txRaw string, jobFunc func(txRaw string) (string, error)) error {
	err := major.GetSqlDB().Transaction(func(tx *gorm.DB) error {
		nowTime := tool.MakeTimestamp()

		updateTime := nowTime
		q.DealTime = nowTime
		sv := q.Version
		q.Version += 1
		q.UpdateTime = updateTime
		q.TakerPsbt = ""
		q.MakerPsbt = ""
		q.FinalPsbt = ""
		if err := tx.Save(q).Where(map[string]interface{}{"version": sv, "id": q.Id}).Error; err != nil {
			return err
		}
		for _, utxo := range usedUtxoList {
			av := utxo.Version
			utxo.Version += 1
			utxo.UpdateTime = updateTime
			if err := tx.Save(utxo).Where(map[string]interface{}{"version": av, "id": utxo.Id}).Error; err != nil {
				return err
			}
		}
		for _, utxo := range newUtxoList {
			utxo.Timestamp = nowTime
			utxo.CreateTime = nowTime
			if err := tx.Create(utxo).Error; err != nil {
				return err
			}
		}

		_, err := jobFunc(txRaw)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (_ *marketOrderModelDao) BatchUpdateOrderEntityListForJobFunc(qList []*MarketOrderModel, usedUtxoList, newUtxoList []*MarketUtxoModel, txRaw string, jobFunc func(txRaw string) (string, error)) error {
	err := major.GetSqlDB().Transaction(func(tx *gorm.DB) error {
		nowTime := tool.MakeTimestamp()

		updateTime := nowTime
		for _, q := range qList {
			q.DealTime = nowTime
			sv := q.Version
			q.Version += 1
			q.UpdateTime = updateTime
			q.TakerPsbt = ""
			q.MakerPsbt = ""
			q.FinalPsbt = ""
			if err := tx.Save(q).Where(map[string]interface{}{"version": sv, "id": q.Id}).Error; err != nil {
				return err
			}
		}
		for _, utxo := range usedUtxoList {
			av := utxo.Version
			utxo.Version += 1
			utxo.UpdateTime = updateTime
			if err := tx.Save(utxo).Where(map[string]interface{}{"version": av, "id": utxo.Id}).Error; err != nil {
				return err
			}
		}
		for _, utxo := range newUtxoList {
			utxo.Timestamp = nowTime
			utxo.CreateTime = nowTime
			if err := tx.Create(utxo).Error; err != nil {
				return err
			}
		}

		_, err := jobFunc(txRaw)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}
