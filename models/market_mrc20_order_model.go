package models

import (
	"errors"
	"fmt"
	"gorm.io/gorm"
	"metaid-market-service/major"
	"metaid-market-service/tool"
	"sync"
)

const (
	AssetTypeMrc20 = "mrc20"
)

type MarketMrc20OrderModel struct {
	Id                int64             `gorm:"column:id" json:"id"`
	OrderId           string            `gorm:"column:orderId" json:"orderId"`
	UtxoId            string            `gorm:"column:utxoId" json:"utxoId"`
	AssetType         AssetType         `gorm:"column:assetType" json:"assetType"`
	OutValue          int64             `gorm:"column:outValue" json:"outValue"`
	TickId            string            `gorm:"column:tickId" json:"tickId"`
	Tick              string            `gorm:"column:tick" json:"tick"`
	TokenName         string            `gorm:"column:tokenName" json:"tokenName"`
	Decimals          int64             `gorm:"column:decimals" json:"decimals"`
	Chain             string            `gorm:"column:chain" json:"chain"`
	Amount            int64             `gorm:"column:amount" json:"amount"`
	AmountStr         string            `gorm:"column:amountStr" json:"amountStr"`
	TokenPriceRate    float64           `gorm:"column:tokenPriceRate" json:"tokenPriceRate"`
	TokenPriceRateStr string            `gorm:"column:tokenPriceRateStr" json:"tokenPriceRateStr"`
	PriceAmount       int64             `gorm:"column:priceAmount" json:"priceAmount"`
	PriceDecimal      int64             `gorm:"column:priceDecimal" json:"priceDecimal"`
	PriceCoin         string            `gorm:"column:priceCoin" json:"priceCoin"`
	OrderState        OrderState        `gorm:"column:orderState" json:"orderState"`
	SellerAddress     string            `gorm:"column:sellerAddress" json:"sellerAddress"`
	SellerIp          string            `gorm:"column:sellerIp" json:"sellerIp"`
	BuyerAddress      string            `gorm:"column:buyerAddress" json:"buyerAddress"`
	BuyerIp           string            `gorm:"column:buyerIp" json:"buyerIp"`
	FeeAmount         int64             `gorm:"column:feeAmount" json:"feeAmount"`
	FeeRate           int64             `gorm:"column:feeRate" json:"feeRate"`
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

func (MarketMrc20OrderModel) TableName() string {
	return "tb_market_mrc20_order"
}

var _marketMrc20OrderModelOnce sync.Once
var _marketMrc20OrderModelManager *marketMrc20OrderModelDao

type marketMrc20OrderModelDao struct {
}

func MarketMrc20OrderModelDao() *marketMrc20OrderModelDao {
	_marketMrc20OrderModelOnce.Do(func() {
		_marketMrc20OrderModelManager = &marketMrc20OrderModelDao{}
	})
	return _marketMrc20OrderModelManager
}

func (_ *marketMrc20OrderModelDao) Set(model *MarketMrc20OrderModel) error {
	if model == nil {
		return errors.New("model is nil")
	}
	tx := major.GetSqlDB().Create(model)
	if tx.Error != nil {
		return tx.Error
	}
	return nil
}

func (_ *marketMrc20OrderModelDao) GetOne(qo *MarketMrc20OrderModel) (*MarketMrc20OrderModel, error) {
	model := &MarketMrc20OrderModel{}
	tx := major.GetSqlDB().Where(qo).First(model)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return model, nil
}

func (_ *marketMrc20OrderModelDao) GetLastOne(qo *MarketMrc20OrderModel) (*MarketMrc20OrderModel, error) {
	model := &MarketMrc20OrderModel{}
	tx := major.GetSqlDB().Where(qo).Order("blockHeight desc").First(model)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return model, nil
}

func (_ *marketMrc20OrderModelDao) GetList(qo *MarketMrc20OrderModel, offset, limit int64) ([]*MarketMrc20OrderModel, error) {
	var models []*MarketMrc20OrderModel
	tx := major.GetSqlDB().Where(qo).Limit(int(limit)).Offset(int(offset)).Order("timestamp asc").Find(&models)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return models, nil
}

func (_ *marketMrc20OrderModelDao) CountByState(qo *MarketMrc20OrderModel, address string) (int64, error) {
	var count int64
	filter := ""
	if qo.OrderState == OrderStateFinish {
		qo.SellerAddress = ""
		if address != "" {
			filter = fmt.Sprintf("sellerAddress = '%s' or buyerAddress = '%s'", address, address)
		}
	}
	tx := major.GetSqlDB().Model(&MarketMrc20OrderModel{}).Where(qo).Where(filter).Count(&count)
	if tx.Error != nil {
		return 0, tx.Error
	}
	return count, nil
}

func (_ *marketMrc20OrderModelDao) GetListByState(qo *MarketMrc20OrderModel, address string, offset, limit int64, sortKey, sortType string) ([]*MarketMrc20OrderModel, error) {
	var models []*MarketMrc20OrderModel
	filter := ""
	if qo.OrderState == OrderStateFinish {
		qo.SellerAddress = ""
		if address != "" {
			filter = fmt.Sprintf("sellerAddress = '%s' or buyerAddress = '%s'", address, address)
		}
	}
	tx := major.GetSqlDB().Where(qo).Where(filter).Limit(int(limit)).Offset(int(offset)).Order(fmt.Sprintf("%s %s", sortKey, sortType)).Find(&models)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return models, nil
}

func (_ *marketMrc20OrderModelDao) Count(qo *MarketMrc20OrderModel) (int64, error) {
	var count int64
	filter := ""
	tx := major.GetSqlDB().Model(&MarketMrc20OrderModel{}).Where(qo).Where(filter).Count(&count)
	if tx.Error != nil {
		return 0, tx.Error
	}
	return count, nil
}

func (_ *marketMrc20OrderModelDao) GetAll(qo *MarketMrc20OrderModel) ([]MarketMrc20OrderModel, error) {
	var models []MarketMrc20OrderModel
	tx := major.GetSqlDB().Where(qo).Find(&models)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return models, nil
}

func (_ *marketMrc20OrderModelDao) Update(q *MarketMrc20OrderModel) error {
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

func (_ *marketMrc20OrderModelDao) BatchSaveList(list []*MarketMrc20OrderModel) error {
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

func (_ *marketMrc20OrderModelDao) UpdateOrderEntityListForJobFunc(q *MarketMrc20OrderModel, usedUtxoList, newUtxoList []*MarketUtxoModel, txRaw string, jobFunc func(txRaw string) (string, error)) error {
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

func (_ *marketMrc20OrderModelDao) BatchUpdateOrderEntityListForJobFunc(qList []*MarketMrc20OrderModel, usedUtxoList, newUtxoList []*MarketUtxoModel, txRaw string, jobFunc func(txRaw string) (string, error)) error {
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
