package models

import (
	"errors"
	"fmt"
	"metaid-market-service/major"
	"metaid-market-service/tool"
	"strings"
	"sync"

	"gorm.io/gorm"
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
	Chain             string            `gorm:"column:chain" json:"chain"`
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

func buildMarketOrderFilter(qo *MarketOrderModel, address, prefixPath string, notInAssetIds []string) (string, []interface{}) {
	conditions := make([]string, 0, 3)
	args := make([]interface{}, 0, 2+len(notInAssetIds))
	if qo.OrderState == OrderStateFinish {
		qo.SellerAddress = ""
		conditions = append(conditions, "(sellerAddress = ? OR buyerAddress = ?)")
		args = append(args, address, address)
	}
	if prefixPath != "" {
		conditions = append(conditions, "assetPath LIKE ?")
		args = append(args, prefixPath+"%")
	}
	if len(notInAssetIds) > 0 {
		placeholders := make([]string, len(notInAssetIds))
		for i, assetId := range notInAssetIds {
			placeholders[i] = "?"
			args = append(args, assetId)
		}
		conditions = append(conditions, fmt.Sprintf("assetId NOT IN (%s)", strings.Join(placeholders, ",")))
	}
	return strings.Join(conditions, " AND "), args
}

func (_ *marketOrderModelDao) CountByState(qo *MarketOrderModel, address, prefixPath string, notInAssetIds []string) (int64, error) {
	var count int64
	filter, args := buildMarketOrderFilter(qo, address, prefixPath, notInAssetIds)
	tx := major.GetSqlDB().Model(&MarketOrderModel{}).Where(qo).Where(filter, args...).Count(&count)
	if tx.Error != nil {
		return 0, tx.Error
	}
	return count, nil
}

func (_ *marketOrderModelDao) GetListByState(qo *MarketOrderModel, address string, offset, limit int64, sortKey, sortType, prefixPath string, notInAssetIds []string) ([]*MarketOrderModel, error) {
	var models []*MarketOrderModel
	filter, args := buildMarketOrderFilter(qo, address, prefixPath, notInAssetIds)
	tx := major.GetSqlDB().Where(qo).Where(filter, args...).Limit(int(limit)).Offset(int(offset)).Order(fmt.Sprintf("%s %s", sortKey, sortType)).Find(&models)
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
	if tx.RowsAffected != 1 {
		return errors.New("order update conflict")
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

func (_ *marketOrderModelDao) UpdateOrderEntityListForJobFunc(chain string, q *MarketOrderModel, usedUtxoList, newUtxoList []*MarketUtxoModel, txRaw string, jobFunc func(chain, txRaw string) (string, error)) error {
	if jobFunc != nil {
		return errors.New("broadcast must run outside the database transaction")
	}
	if q == nil || q.FinalPsbt == "" {
		return errors.New("prepared order transaction is missing")
	}
	return major.GetSqlDB().Transaction(func(tx *gorm.DB) error {
		nowTime := tool.MakeTimestamp()
		q.DealTime = nowTime
		q.OrderState = OrderStateFinish
		q.ConfirmationState = ConfirmationStateUnconfirmed
		sv := q.Version
		q.Version++
		q.UpdateTime = nowTime
		q.TakerPsbt = ""
		q.MakerPsbt = ""
		q.FinalPsbt = ""
		result := tx.Where("id = ? AND version = ? AND orderState = ? AND finalPsbt <> ''", q.Id, sv, OrderStateFinish).Save(q)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("order finalization conflict")
		}
		for _, utxo := range usedUtxoList {
			av := utxo.Version
			utxo.Version++
			utxo.UpdateTime = nowTime
			result := tx.Where("id = ? AND version = ?", utxo.Id, av).Save(utxo)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return errors.New("dummy UTXO finalization conflict")
			}
		}
		for _, utxo := range newUtxoList {
			utxo.Timestamp = nowTime
			utxo.CreateTime = nowTime
			if err := tx.Create(utxo).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (_ *marketOrderModelDao) BatchUpdateOrderEntityListForJobFunc(chain string, qList []*MarketOrderModel, usedUtxoList, newUtxoList []*MarketUtxoModel, txRaw string, jobFunc func(chain, txRaw string) (string, error)) error {
	if jobFunc != nil {
		return errors.New("broadcast must run outside the database transaction")
	}
	return major.GetSqlDB().Transaction(func(tx *gorm.DB) error {
		nowTime := tool.MakeTimestamp()
		for _, q := range qList {
			if q == nil || q.FinalPsbt == "" {
				return errors.New("prepared batch transaction is missing")
			}
			q.DealTime = nowTime
			q.OrderState = OrderStateFinish
			q.ConfirmationState = ConfirmationStateUnconfirmed
			sv := q.Version
			q.Version++
			q.UpdateTime = nowTime
			q.TakerPsbt = ""
			q.MakerPsbt = ""
			q.FinalPsbt = ""
			result := tx.Where("id = ? AND version = ? AND orderState = ? AND finalPsbt <> ''", q.Id, sv, OrderStateFinish).Save(q)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return errors.New("batch order finalization conflict")
			}
		}
		for _, utxo := range usedUtxoList {
			av := utxo.Version
			utxo.Version++
			utxo.UpdateTime = nowTime
			result := tx.Where("id = ? AND version = ?", utxo.Id, av).Save(utxo)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return errors.New("dummy UTXO finalization conflict")
			}
		}
		for _, utxo := range newUtxoList {
			utxo.Timestamp = nowTime
			utxo.CreateTime = nowTime
			if err := tx.Create(utxo).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
