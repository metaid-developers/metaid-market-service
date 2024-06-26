package models

import (
	"errors"
	"metaid-market-service/major"
	"metaid-market-service/tool"
	"sync"
)

type MarketPriceModel struct {
	Id         int64  `gorm:"column:id" json:"id"`
	Coin       string `gorm:"column:coin" json:"coin"`
	Usd        int64  `gorm:"column:usd" json:"usd"`
	Timestamp  int64  `gorm:"column:timestamp" json:"timestamp"`
	Version    int64  `gorm:"column:version" json:"version"`
	CreateTime int64  `gorm:"column:createTime" json:"createTime"`
	UpdateTime int64  `gorm:"column:updateTime" json:"updateTime"`
	State      int64  `gorm:"column:state" json:"state"`
}

func (MarketPriceModel) TableName() string {
	return "tb_market_price"
}

var _marketPriceModelOnce sync.Once
var _marketPriceModelManager *marketPriceModelDao

type marketPriceModelDao struct {
}

func MarketPriceModelDao() *marketPriceModelDao {
	_marketPriceModelOnce.Do(func() {
		_marketPriceModelManager = &marketPriceModelDao{}
	})
	return _marketPriceModelManager
}

func (_ *marketPriceModelDao) Set(model *MarketPriceModel) error {
	if model == nil {
		return errors.New("model is nil")
	}
	tx := major.GetSqlDB().Create(model)
	if tx.Error != nil {
		return tx.Error
	}
	return nil
}

func (_ *marketPriceModelDao) GetOne(qo *MarketPriceModel) (*MarketPriceModel, error) {
	model := &MarketPriceModel{}
	tx := major.GetSqlDB().Where(qo).First(model)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return model, nil
}

func (_ *marketPriceModelDao) GetLastOne(qo *MarketPriceModel) (*MarketPriceModel, error) {
	model := &MarketPriceModel{}
	tx := major.GetSqlDB().Where(qo).Order("blockHeight desc").First(model)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return model, nil
}

func (_ *marketPriceModelDao) GetList(qo *MarketPriceModel, offset, limit int64) ([]*MarketPriceModel, error) {
	var models []*MarketPriceModel
	tx := major.GetSqlDB().Where(qo).Limit(int(limit)).Offset(int(offset)).Order("timestamp asc").Find(&models)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return models, nil
}

func (_ *marketPriceModelDao) Count(qo *MarketPriceModel) (int64, error) {
	var count int64
	filter := ""
	tx := major.GetSqlDB().Model(&MarketPriceModel{}).Where(qo).Where(filter).Count(&count)
	if tx.Error != nil {
		return 0, tx.Error
	}
	return count, nil
}

func (_ *marketPriceModelDao) GetAll(qo *MarketPriceModel) ([]MarketPriceModel, error) {
	var models []MarketPriceModel
	tx := major.GetSqlDB().Where(qo).Find(&models)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return models, nil
}

func (_ *marketPriceModelDao) Update(q *MarketPriceModel) error {
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
