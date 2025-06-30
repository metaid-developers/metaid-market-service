package models

import (
	"errors"
	"fmt"
	"metaid-market-service/major"
	"metaid-market-service/tool"
	"sync"
)

type MarketMrc20InfoModel struct {
	Id          int64      `gorm:"column:id" json:"id"`
	TickId      string     `gorm:"column:tickId" json:"tickId"`
	Tick        string     `gorm:"column:tick" json:"tick"`
	TokenName   string     `gorm:"column:tokenName" json:"tokenName"`
	Decimals    int64      `gorm:"column:decimals" json:"decimals"`
	Chain       string     `gorm:"column:chain" json:"chain"`
	Supply      string     `gorm:"column:supply" json:"supply"`
	AutoStatus  AutoStatus `gorm:"column:autoStatus" json:"autoStatus"`
	OrderCount  int64      `gorm:"column:orderCount" json:"orderCount"`
	TotalVolume int64      `gorm:"column:totalVolume" json:"totalVolume"`
	MarketCap   int64      `gorm:"column:marketCap" json:"marketCap"`
	LastPrice   float64    `gorm:"column:lastPrice" json:"lastPrice"`
	FloorPrice  float64    `gorm:"column:floorPrice" json:"floorPrice"`
	Change24H   int64      `gorm:"column:change24H" json:"change24H"`
	Timestamp   int64      `gorm:"column:timestamp" json:"timestamp"`
	Version     int64      `gorm:"column:version" json:"version"`
	CreateTime  int64      `gorm:"column:createTime" json:"createTime"`
	UpdateTime  int64      `gorm:"column:updateTime" json:"updateTime"`
	State       int64      `gorm:"column:state" json:"state"`
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

func (_ *marketMrc20InfoModelDao) GetListByOrder(qo *MarketMrc20InfoModel, offset, limit int64, orderBy, sort string) ([]*MarketMrc20InfoModel, error) {
	var models []*MarketMrc20InfoModel
	tx := major.GetSqlDB().Where(qo).Limit(int(limit)).Offset(int(offset)).Order(fmt.Sprintf("%s %s, id asc", orderBy, sort)).Find(&models)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return models, nil
}

func (_ *marketMrc20InfoModelDao) GetCanAutoBridgeList(qo *MarketMrc20InfoModel, orderCount, offset, limit int64) ([]*MarketMrc20InfoModel, error) {
	var models []*MarketMrc20InfoModel
	filter := "autoStatus == 0"
	if orderCount > 0 {
		filter = fmt.Sprintf("and orderCount >= %d", orderCount)
	}
	tx := major.GetSqlDB().Where(qo).Where(filter).Limit(int(limit)).Offset(int(offset)).Order("timestamp asc").Find(&models)
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

func (_ *marketMrc20InfoModelDao) GetAll(qo *MarketMrc20InfoModel) ([]*MarketMrc20InfoModel, error) {
	var models []*MarketMrc20InfoModel
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

// HotMrc20CoreInfo 热门币种核心信息结构体
type HotMrc20CoreInfo struct {
	TickId     string  `json:"tickId" gorm:"column:tickId"`
	Tick       string  `json:"tick" gorm:"column:tick"`
	TokenName  string  `json:"tokenName" gorm:"column:tokenName"`
	MarketCap  int64   `json:"marketCap" gorm:"column:marketCap"`
	LastPrice  float64 `json:"lastPrice" gorm:"column:lastPrice"`
	Change24H  int64   `json:"change24H" gorm:"column:change24H"`
	TradeCount int64   `json:"tradeCount" gorm:"column:tradeCount"`
}

// GetHotMrc20CoreInfo 获取最近热门币种的核心信息列表
// timeRange: 时间范围（毫秒），例如 24小时 = 24*60*60*1000
// 如果指定时间范围内没有交易，则按 orderCount 排序返回活跃币种
// 确保返回至少5个结果
func (_ *marketMrc20InfoModelDao) GetHotMrc20CoreInfo(timeRange int64, offset, limit int64) ([]*HotMrc20CoreInfo, error) {
	var results []*HotMrc20CoreInfo

	// 计算时间范围
	startTime := tool.MakeTimestamp() - timeRange

	// 首先尝试按交易数排序查询
	queryWithTrade := fmt.Sprintf(`
		SELECT i.tickId, i.tick, i.tokenName, i.marketCap, i.lastPrice, i.change24H,
		       COUNT(o.id) as tradeCount
		FROM tb_market_mrc20_info i
		LEFT JOIN tb_market_mrc20_order o ON i.tickId = o.tickId 
			AND o.orderState = %d 
			AND o.dealTime >= %d
		WHERE i.state = %d
		GROUP BY i.tickId, i.tick, i.tokenName, i.marketCap, i.lastPrice, i.change24H
		HAVING tradeCount > 0
		ORDER BY tradeCount DESC, i.lastPrice DESC
		LIMIT %d OFFSET %d
	`, OrderStateFinish, startTime, STATE_EXIST, limit, offset)

	tx := major.GetSqlDB().Raw(queryWithTrade).Scan(&results)
	if tx.Error != nil {
		return nil, tx.Error
	}

	// 如果结果少于5个，需要补充数据
	if len(results) < 5 {
		// 获取已经查询到的tickId列表，用于排除
		existingTickIds := make(map[string]bool)
		for _, result := range results {
			existingTickIds[result.TickId] = true
		}

		// 构建排除条件
		excludeCondition := ""
		if len(existingTickIds) > 0 {
			tickIdList := ""
			for tickId := range existingTickIds {
				if tickIdList != "" {
					tickIdList += ","
				}
				tickIdList += fmt.Sprintf("'%s'", tickId)
			}
			excludeCondition = fmt.Sprintf("AND i.tickId NOT IN (%s)", tickIdList)
		}

		// 计算还需要补充的数量
		needMore := 5 - len(results)
		if needMore > 0 {
			// 按orderCount排序查询补充数据
			queryByOrderCount := fmt.Sprintf(`
				SELECT i.tickId, i.tick, i.tokenName, i.marketCap, i.lastPrice, i.change24H,
				       0 as tradeCount
				FROM tb_market_mrc20_info i
				WHERE i.state = %d AND i.orderCount > 0 %s
				ORDER BY i.orderCount DESC, i.lastPrice DESC
				LIMIT %d
			`, STATE_EXIST, excludeCondition, needMore)

			var additionalResults []*HotMrc20CoreInfo
			tx = major.GetSqlDB().Raw(queryByOrderCount).Scan(&additionalResults)
			if tx.Error != nil {
				return nil, tx.Error
			}

			// 将补充的结果添加到原有结果中
			results = append(results, additionalResults...)
		}
	}

	return results, nil
}

// GetLatestTradeMrc20CoreInfo 获取根据最新交易时间排序的币种核心信息列表
// 返回按最新交易时间倒序排列的币种列表，包含交易时间信息
func (_ *marketMrc20InfoModelDao) GetLatestTradeMrc20CoreInfo(offset, limit int64) ([]*HotMrc20CoreInfo, error) {
	var results []*HotMrc20CoreInfo

	// 查询按最新交易时间排序的币种信息
	query := fmt.Sprintf(`
		SELECT i.tickId, i.tick, i.tokenName, i.marketCap, i.lastPrice, i.change24H,
		       COUNT(o.id) as tradeCount
		FROM tb_market_mrc20_info i
		LEFT JOIN tb_market_mrc20_order o ON i.tickId = o.tickId 
			AND o.orderState = %d
		WHERE i.state = %d
		GROUP BY i.tickId, i.tick, i.tokenName, i.marketCap, i.lastPrice, i.change24H
		HAVING tradeCount > 0
		ORDER BY MAX(o.dealTime) DESC, i.lastPrice DESC
		LIMIT %d OFFSET %d
	`, OrderStateFinish, STATE_EXIST, limit, offset)

	tx := major.GetSqlDB().Raw(query).Scan(&results)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return results, nil
}
