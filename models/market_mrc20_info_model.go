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

// GetFixedTopThreeMrc20CoreInfo 获取固定的前三个币种核心信息
func (_ *marketMrc20InfoModelDao) GetFixedTopThreeMrc20CoreInfo() ([]*HotMrc20CoreInfo, error) {
	var results []*HotMrc20CoreInfo

	// 固定的前三个tickId
	fixedTickIds := []string{
		"5fad846577a9a9645162c8c5e9dc7db65bccaee38b8e641a579a10dc2448f333i0",
		"8ce590918a3d493631ab9d9e3bbc89e322f3dd08354af87f97c07218818b78f4i0",
		"644dba0433aced0ec4cecef9baa951eccabb1751f222d48d33e7a309738ff0d2i0",
	}

	// 构建查询条件
	tickIdList := ""
	for i, tickId := range fixedTickIds {
		if i > 0 {
			tickIdList += ","
		}
		tickIdList += fmt.Sprintf("'%s'", tickId)
	}

	query := fmt.Sprintf(`
		SELECT i.tickId, i.tick, i.tokenName, i.marketCap, i.lastPrice, i.change24H,
		       COUNT(o.id) as tradeCount
		FROM tb_market_mrc20_info i
		LEFT JOIN tb_market_mrc20_order o ON i.tickId = o.tickId 
			AND o.orderState = %d
		WHERE i.state = %d AND i.tickId IN (%s)
		GROUP BY i.tickId, i.tick, i.tokenName, i.marketCap, i.lastPrice, i.change24H
		ORDER BY FIELD(i.tickId, %s)
	`, OrderStateFinish, STATE_EXIST, tickIdList, tickIdList)

	tx := major.GetSqlDB().Raw(query).Scan(&results)
	if tx.Error != nil {
		return nil, tx.Error
	}
	fmt.Printf("GetFixedTopThreeMrc20CoreInfo: results: %+v\n", results)

	return results, nil
}

// GetHotMrc20CoreInfo 获取最近热门币种的核心信息列表
// timeRange: 时间范围（毫秒），例如 24小时 = 24*60*60*1000
// 如果指定时间范围内没有交易，则按 orderCount 排序返回活跃币种
// 确保返回至少5个结果
func (_ *marketMrc20InfoModelDao) GetHotMrc20CoreInfo(timeRange int64, offset, limit int64) ([]*HotMrc20CoreInfo, error) {
	var results []*HotMrc20CoreInfo

	// 首先获取固定的前三个币种
	fixedResults, err := _marketMrc20InfoModelManager.GetFixedTopThreeMrc20CoreInfo()
	if err != nil {
		return nil, err
	}
	results = append(results, fixedResults...)

	// 计算时间范围
	startTime := tool.MakeTimestamp() - timeRange

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
	needMore := limit - int64(len(results))
	if needMore > 0 {
		// 首先尝试按交易数排序查询
		queryWithTrade := fmt.Sprintf(`
			SELECT i.tickId, i.tick, i.tokenName, i.marketCap, i.lastPrice, i.change24H,
			       COUNT(o.id) as tradeCount
			FROM tb_market_mrc20_info i
			LEFT JOIN tb_market_mrc20_order o ON i.tickId = o.tickId 
				AND o.orderState = %d 
				AND o.dealTime >= %d
			WHERE i.state = %d AND i.tickId != '49d049501fad03efdd0ace2fe862c3cbe7e99fd82c7d0bad7b4f3f3c22d157e3i0' %s
			GROUP BY i.tickId, i.tick, i.tokenName, i.marketCap, i.lastPrice, i.change24H
			HAVING tradeCount > 0
			ORDER BY tradeCount DESC, i.lastPrice DESC
			LIMIT %d
		`, OrderStateFinish, startTime, STATE_EXIST, excludeCondition, needMore)

		var tradeResults []*HotMrc20CoreInfo
		tx := major.GetSqlDB().Raw(queryWithTrade).Scan(&tradeResults)
		if tx.Error != nil {
			return nil, tx.Error
		}

		results = append(results, tradeResults...)

		// 如果结果仍然不够，按orderCount排序查询补充数据
		if len(results) < int(limit) {
			// 更新已存在的tickId列表
			for _, result := range tradeResults {
				existingTickIds[result.TickId] = true
			}

			// 重新构建排除条件
			excludeCondition = ""
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

			needMore = limit - int64(len(results))
			if needMore > 0 {
				queryByOrderCount := fmt.Sprintf(`
					SELECT i.tickId, i.tick, i.tokenName, i.marketCap, i.lastPrice, i.change24H,
					       0 as tradeCount
					FROM tb_market_mrc20_info i
					WHERE i.state = %d AND i.orderCount > 0 AND i.tickId != '49d049501fad03efdd0ace2fe862c3cbe7e99fd82c7d0bad7b4f3f3c22d157e3i0' %s
					ORDER BY i.orderCount DESC, i.lastPrice DESC
					LIMIT %d
				`, STATE_EXIST, excludeCondition, needMore)

				var additionalResults []*HotMrc20CoreInfo
				tx = major.GetSqlDB().Raw(queryByOrderCount).Scan(&additionalResults)
				if tx.Error != nil {
					return nil, tx.Error
				}

				results = append(results, additionalResults...)
			}
		}
	}

	return results, nil
}

// GetLatestTradeMrc20CoreInfo 获取根据最新交易时间排序的币种核心信息列表
// 返回按最新交易时间倒序排列的币种列表，包含交易时间信息
func (_ *marketMrc20InfoModelDao) GetLatestTradeMrc20CoreInfo(offset, limit int64) ([]*HotMrc20CoreInfo, error) {
	var results []*HotMrc20CoreInfo

	// 首先获取固定的前三个币种
	fixedResults, err := _marketMrc20InfoModelManager.GetFixedTopThreeMrc20CoreInfo()
	if err != nil {
		return nil, err
	}
	results = append(results, fixedResults...)

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
	needMore := limit - int64(len(results))
	if needMore > 0 {
		// 查询按最新交易时间排序的币种信息
		query := fmt.Sprintf(`
			SELECT i.tickId, i.tick, i.tokenName, i.marketCap, i.lastPrice, i.change24H,
			       COUNT(o.id) as tradeCount
			FROM tb_market_mrc20_info i
			LEFT JOIN tb_market_mrc20_order o ON i.tickId = o.tickId 
				AND o.orderState = %d
			WHERE i.state = %d AND i.tickId != '49d049501fad03efdd0ace2fe862c3cbe7e99fd82c7d0bad7b4f3f3c22d157e3i0' %s
			GROUP BY i.tickId, i.tick, i.tokenName, i.marketCap, i.lastPrice, i.change24H
			HAVING tradeCount > 0
			ORDER BY MAX(o.dealTime) DESC, i.lastPrice DESC
			LIMIT %d
		`, OrderStateFinish, STATE_EXIST, excludeCondition, needMore)

		var additionalResults []*HotMrc20CoreInfo
		tx := major.GetSqlDB().Raw(query).Scan(&additionalResults)
		if tx.Error != nil {
			return nil, tx.Error
		}

		results = append(results, additionalResults...)
	}

	return results, nil
}
