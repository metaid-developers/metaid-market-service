package models

import (
	"errors"
	"fmt"
	"gorm.io/gorm"
	"metaid-market-service/major"
	"metaid-market-service/tool"
	"sync"
)

type AutoStatus int64

const (
	AutoStatusCreate AutoStatus = 1
	AutoStatusFinish AutoStatus = 2
)

type MarketAutoBridgeCreateModel struct {
	Id            int64      `gorm:"column:id" json:"id"`
	TickId        string     `gorm:"column:tickId" json:"tickId"`
	Tick          string     `gorm:"column:tick" json:"tick"`
	TokenName     string     `gorm:"column:tokenName" json:"tokenName"`
	Decimals      int64      `gorm:"column:decimals" json:"decimals"`
	OrderCount    int64      `gorm:"column:orderCount" json:"orderCount"`
	MarketCapSat  int64      `gorm:"column:marketCapSat" json:"marketCapSat"`
	MarketCapUsdt int64      `gorm:"column:marketCapUsdt" json:"marketCapUsdt"`
	AutoStatus    AutoStatus `gorm:"column:autoStatus" json:"autoStatus"`
	Timestamp     int64      `gorm:"column:timestamp" json:"timestamp"`
	Version       int64      `gorm:"column:version" json:"version"`
	CreateTime    int64      `gorm:"column:createTime" json:"createTime"`
	UpdateTime    int64      `gorm:"column:updateTime" json:"updateTime"`
	State         int64      `gorm:"column:state" json:"state"`
}

func (MarketAutoBridgeCreateModel) TableName() string {
	return "tb_market_auto_bridge_create"
}

var _marketAutoBridgeCreateModelOnce sync.Once
var _marketAutoBridgeCreateModelManager *marketAutoBridgeCreateModelDao

type marketAutoBridgeCreateModelDao struct {
}

func MarketAutoBridgeCreateModelDao() *marketAutoBridgeCreateModelDao {
	_marketAutoBridgeCreateModelOnce.Do(func() {
		_marketAutoBridgeCreateModelManager = &marketAutoBridgeCreateModelDao{}
	})
	return _marketAutoBridgeCreateModelManager
}

func (_ *marketAutoBridgeCreateModelDao) Set(model *MarketAutoBridgeCreateModel) error {
	if model == nil {
		return errors.New("model is nil")
	}
	tx := major.GetSqlDB().Create(model)
	if tx.Error != nil {
		return tx.Error
	}
	return nil
}

func (_ *marketAutoBridgeCreateModelDao) SetAndFunc(model *MarketAutoBridgeCreateModel, marketTickInfo *MarketMrc20InfoModel, jobFunc func() error) error {
	err := major.GetSqlDB().Transaction(func(tx *gorm.DB) error {

		if marketTickInfo != nil {
			marketTickInfo.AutoStatus = AutoStatusCreate
			mv := marketTickInfo.Version
			marketTickInfo.Version += 1
			marketTickInfo.UpdateTime = tool.MakeTimestamp()
			if err := tx.Where(map[string]interface{}{"version": mv, "id": marketTickInfo.Id}).Updates(marketTickInfo).Error; err != nil {
				return err
			}
		} else {
			return errors.New("marketTickInfo is nil")
		}

		if err := tx.Create(model).Error; err != nil {
			return err
		}

		err := jobFunc()
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

func (_ *marketAutoBridgeCreateModelDao) GetOne(qo *MarketAutoBridgeCreateModel) (*MarketAutoBridgeCreateModel, error) {
	model := &MarketAutoBridgeCreateModel{}
	tx := major.GetSqlDB().Where(qo).First(model)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return model, nil
}

func (_ *marketAutoBridgeCreateModelDao) GetLastOne(qo *MarketAutoBridgeCreateModel) (*MarketAutoBridgeCreateModel, error) {
	model := &MarketAutoBridgeCreateModel{}
	tx := major.GetSqlDB().Where(qo).Order("blockHeight desc").First(model)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return model, nil
}

func (_ *marketAutoBridgeCreateModelDao) GetList(qo *MarketAutoBridgeCreateModel, offset, limit int64) ([]*MarketAutoBridgeCreateModel, error) {
	var models []*MarketAutoBridgeCreateModel
	tx := major.GetSqlDB().Where(qo).Limit(int(limit)).Offset(int(offset)).Order("timestamp asc").Find(&models)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return models, nil
}

func (_ *marketAutoBridgeCreateModelDao) GetListByOrder(qo *MarketAutoBridgeCreateModel, offset, limit int64, orderBy, sort string) ([]*MarketAutoBridgeCreateModel, error) {
	var models []*MarketAutoBridgeCreateModel
	tx := major.GetSqlDB().Where(qo).Limit(int(limit)).Offset(int(offset)).Order(fmt.Sprintf("%s %s, id asc", orderBy, sort)).Find(&models)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return models, nil
}

func (_ *marketAutoBridgeCreateModelDao) Count(qo *MarketAutoBridgeCreateModel) (int64, error) {
	var count int64
	filter := ""
	tx := major.GetSqlDB().Model(&MarketAutoBridgeCreateModel{}).Where(qo).Where(filter).Count(&count)
	if tx.Error != nil {
		return 0, tx.Error
	}
	return count, nil
}

func (_ *marketAutoBridgeCreateModelDao) GetAll(qo *MarketAutoBridgeCreateModel) ([]*MarketAutoBridgeCreateModel, error) {
	var models []*MarketAutoBridgeCreateModel
	tx := major.GetSqlDB().Where(qo).Find(&models)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return models, nil
}

func (_ *marketAutoBridgeCreateModelDao) Update(q *MarketAutoBridgeCreateModel) error {
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

func (_ *marketAutoBridgeCreateModelDao) UpdateAndMarketInfo(q *MarketAutoBridgeCreateModel, marketTickInfo *MarketMrc20InfoModel) error {
	err := major.GetSqlDB().Transaction(func(tx *gorm.DB) error {

		if marketTickInfo != nil {
			marketTickInfo.AutoStatus = AutoStatusFinish
			mv := marketTickInfo.Version
			marketTickInfo.Version += 1
			marketTickInfo.UpdateTime = tool.MakeTimestamp()
			if err := tx.Where(map[string]interface{}{"version": mv, "id": marketTickInfo.Id}).Updates(marketTickInfo).Error; err != nil {
				return err
			}
		} else {
			return errors.New("marketTickInfo is nil")
		}

		sv := q.Version
		q.Version += 1
		q.UpdateTime = tool.MakeTimestamp()
		if err := tx.Where(map[string]interface{}{"version": sv, "id": q.Id}).Updates(q).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}
