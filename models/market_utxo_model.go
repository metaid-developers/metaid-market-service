package models

import (
	"errors"
	"gorm.io/gorm"
	"metaid-market-service/major"
	"metaid-market-service/tool"
	"sync"
)

type UtxoType int

const (
	UtxoTypeDummy600  UtxoType = 1
	UtxoTypeDummy1200 UtxoType = 2
)

type UsedState int

const (
	UsedNo  UsedState = 1
	UsedYes UsedState = 2
	UsedErr UsedState = 3
	UsedDel UsedState = 4
)

type ConfirmStatus int

const (
	Unconfirmed ConfirmStatus = 0
	Confirmed   ConfirmStatus = 1
	RoadBack    ConfirmStatus = 1000
)

type MarketUtxoModel struct {
	Id             int64         `gorm:"column:id" json:"id"`
	UtxoId         string        `gorm:"column:utxoId" json:"utxoId"` //txId_index
	UtxoType       UtxoType      `gorm:"column:utxoType" json:"utxoType"`
	Amount         uint64        `gorm:"column:amount" json:"amount"`
	Address        string        `gorm:"column:address" json:"address"`
	PrivateKeyHex  string        `gorm:"column:privateKeyHex" json:"privateKeyHex"`
	TxId           string        `gorm:"column:txId" json:"txId"`
	Index          int64         `gorm:"column:index" json:"index"`
	PkScript       string        `gorm:"column:pkScript" json:"pkScript"`
	UsedState      UsedState     `gorm:"column:usedState" json:"usedState"`
	UsedTxId       string        `gorm:"column:usedTxId" json:"usedTxId"`
	OrderId        string        `gorm:"column:orderId" json:"orderId"`
	SortIndex      int64         `gorm:"column:sortIndex" json:"sortIndex"`
	ConfirmStatus  ConfirmStatus `gorm:"column:confirmStatus" json:"confirmStatus"`
	FromOrderId    string        `gorm:"column:fromOrderId" json:"fromOrderId"`
	NetworkFeeRate int64         `gorm:"column:networkFeeRate" json:"networkFeeRate"`
	Timestamp      int64         `gorm:"column:timestamp" json:"timestamp"`
	Version        int64         `gorm:"column:version" json:"version"`
	CreateTime     int64         `gorm:"column:createTime" json:"createTime"`
	UpdateTime     int64         `gorm:"column:updateTime" json:"updateTime"`
	State          int64         `gorm:"column:state" json:"state"`
}

func (MarketUtxoModel) TableName() string {
	return "tb_market_utxo"
}

var _marketUtxoModelOnce sync.Once
var _marketUtxoModelManager *marketUtxoModelDao

type marketUtxoModelDao struct {
}

func MarketUtxoModelDao() *marketUtxoModelDao {
	_marketUtxoModelOnce.Do(func() {
		_marketUtxoModelManager = &marketUtxoModelDao{}
	})
	return _marketUtxoModelManager
}

func (_ *marketUtxoModelDao) Set(model *MarketUtxoModel) error {
	if model == nil {
		return errors.New("model is nil")
	}
	tx := major.GetSqlDB().Create(model)
	if tx.Error != nil {
		return tx.Error
	}
	return nil
}

func (_ *marketUtxoModelDao) GetOne(qo *MarketUtxoModel) (*MarketUtxoModel, error) {
	model := &MarketUtxoModel{}
	tx := major.GetSqlDB().Where(qo).First(model)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return model, nil
}

func (_ *marketUtxoModelDao) GetList(qo *MarketUtxoModel, offset, limit int64) ([]*MarketUtxoModel, error) {
	var models []*MarketUtxoModel
	tx := major.GetSqlDB().Where(qo).Limit(int(limit)).Offset(int(offset)).Order("timestamp asc").Find(&models)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return models, nil
}

func (_ *marketUtxoModelDao) GetListUnconfirmed(qo *MarketUtxoModel, offset, limit int64) ([]*MarketUtxoModel, error) {
	var models []*MarketUtxoModel
	filter := "confirmStatus = 0"
	tx := major.GetSqlDB().Where(qo).Where(filter).Limit(int(limit)).Offset(int(offset)).Order("timestamp asc").Find(&models)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return models, nil
}

func (_ *marketUtxoModelDao) GetDummyList(qo *MarketUtxoModel, offset, limit int64) ([]*MarketUtxoModel, error) {
	var models []*MarketUtxoModel
	tx := major.GetSqlDB().Where(qo).Limit(int(limit)).Offset(int(offset)).Order("timestamp asc").Find(&models)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return models, nil
}

func (_ *marketUtxoModelDao) Count(qo *MarketUtxoModel) (int64, error) {
	var count int64
	filter := ""
	tx := major.GetSqlDB().Model(&MarketUtxoModel{}).Where(qo).Where(filter).Count(&count)
	if tx.Error != nil {
		return 0, tx.Error
	}
	return count, nil
}

func (_ *marketUtxoModelDao) GetAll(qo *MarketUtxoModel) ([]MarketUtxoModel, error) {
	var models []MarketUtxoModel
	tx := major.GetSqlDB().Where(qo).Find(&models)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return models, nil
}

func (_ *marketUtxoModelDao) Update(q *MarketUtxoModel) error {
	if q == nil {
		return errors.New("model is nil")
	}
	v := q.Version
	q.Version += 1
	q.UpdateTime = tool.MakeTimestamp()
	tx := major.GetSqlDB().Where(map[string]interface{}{"version": v, "id": q.Id}).Updates(q)
	if tx.Error != nil {
		return tx.Error
	}
	return nil
}
func (_ *marketUtxoModelDao) ColdDownUtxoEntityListForJobFunc(utxoList []*MarketUtxoModel, txRaw string, jobFunc func(rawTx string) (string, error)) (string, error) {
	realTxId := ""
	err := major.GetSqlDB().Transaction(func(tx *gorm.DB) error {
		for _, utxo := range utxoList {
			if err := tx.Save(utxo).Error; err != nil {
				return err
			}
		}

		txId, err := jobFunc(txRaw)
		if err != nil {
			return err
		}
		realTxId = txId

		return nil
	})
	if err != nil {
		return "", err
	}
	return realTxId, nil
}
