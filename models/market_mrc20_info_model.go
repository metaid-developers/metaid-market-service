package models

import (
	"errors"
	"metaid-market-service/major"
	"metaid-market-service/tool"
	"sync"
)

type MarketMrc20InfoModel struct {
	Id          int64   `gorm:"column:id" json:"id"`
	TickId      string  `gorm:"column:tickId" json:"tickId"`
	Tick        string  `gorm:"column:tick" json:"tick"`
	TokenName   string  `gorm:"column:tokenName" json:"tokenName"`
	Decimals    int64   `gorm:"column:decimals" json:"decimals"`
	Chain       string  `gorm:"column:chain" json:"chain"`
	Supply      string  `gorm:"column:supply" json:"supply"`
	TotalVolume int64   `gorm:"column:totalVolume" json:"totalVolume"`
	MarketCap   int64   `gorm:"column:marketCap" json:"marketCap"`
	LastPrice   float64 `gorm:"column:lastPrice" json:"lastPrice"`
	FloorPrice  float64 `gorm:"column:floorPriceStr" json:"floorPriceStr"`
	Change24H   int64   `gorm:"column:change24H" json:"change24H"`
	Timestamp   int64   `gorm:"column:timestamp" json:"timestamp"`
	Version     int64   `gorm:"column:version" json:"version"`
	CreateTime  int64   `gorm:"column:createTime" json:"createTime"`
	UpdateTime  int64   `gorm:"column:updateTime" json:"updateTime"`
	State       int64   `gorm:"column:state" json:"state"`
}

func (MarketMrc20InfoModel) TableName() string {
	return "tb_market_mrc20_info"
}

var _marketMrc20InfoModelOnce sync.Once
var _marketMrc20InfoModelManager *marketMrc20InfoModelDao

type marketMrc20InfoModelDao struct {
}

func MarketMrc20InfoModelDao() *marketMrc20InfoModelDao {
	_marketMrc20InfoModelOnce.Do(func() {
		_marketMrc20InfoModelManager = &marketMrc20InfoModelDao{}
	})
	return _marketMrc20InfoModelManager
}

func (_ *marketMrc20InfoModelDao) Set(model *MarketMrc20InfoModel) error {
	if model == nil {
		return errors.New("model is nil")
	}
	tx := major.GetSqlDB().Create(model)
	if tx.Error != nil {
		return tx.Error
	}
	return nil
}

func (_ *marketMrc20InfoModelDao) GetOne(qo *MarketMrc20InfoModel) (*MarketMrc20InfoModel, error) {
	model := &MarketMrc20InfoModel{}
	tx := major.GetSqlDB().Where(qo).First(model)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return model, nil
}

func (_ *marketMrc20InfoModelDao) GetLastOne(qo *MarketMrc20InfoModel) (*MarketMrc20InfoModel, error) {
	model := &MarketMrc20InfoModel{}
	tx := major.GetSqlDB().Where(qo).Order("blockHeight desc").First(model)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return model, nil
}

func (_ *marketMrc20InfoModelDao) GetList(qo *MarketMrc20InfoModel, offset, limit int64) ([]*MarketMrc20InfoModel, error) {
	var models []*MarketMrc20InfoModel
	tx := major.GetSqlDB().Where(qo).Limit(int(limit)).Offset(int(offset)).Order("timestamp asc").Find(&models)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return models, nil
}

func (_ *marketMrc20InfoModelDao) Count(qo *MarketMrc20InfoModel) (int64, error) {
	var count int64
	filter := ""
	tx := major.GetSqlDB().Model(&MarketMrc20InfoModel{}).Where(qo).Where(filter).Count(&count)
	if tx.Error != nil {
		return 0, tx.Error
	}
	return count, nil
}

func (_ *marketMrc20InfoModelDao) GetAll(qo *MarketMrc20InfoModel) ([]MarketMrc20InfoModel, error) {
	var models []MarketMrc20InfoModel
	tx := major.GetSqlDB().Where(qo).Find(&models)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return models, nil
}

func (_ *marketMrc20InfoModelDao) Update(q *MarketMrc20InfoModel) error {
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
