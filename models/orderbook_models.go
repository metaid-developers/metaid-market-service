package models

import (
	"metaid-market-service/major"
	"sync"

	"gorm.io/gorm/clause"
)

type OrderbookPairModel struct {
	Id            int64  `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	PairKey       string `gorm:"column:pair_key;size:180;uniqueIndex" json:"pairKey"`
	TokenCodeHash string `gorm:"column:token_codehash" json:"tokenCodeHash"`
	TokenID       string `gorm:"column:token_id" json:"tokenID"`
	Symbol        string `gorm:"column:symbol" json:"symbol"`
	Decimals      int    `gorm:"column:decimals" json:"decimals"`
	QuoteCodeHash string `gorm:"column:quote_codehash;size:40;default:''" json:"quoteCodeHash"`
	QuoteTokenID  string `gorm:"column:quote_token_id;size:40;default:''" json:"quoteTokenID"`
	QuoteSymbol   string `gorm:"column:quote_symbol;size:32;default:'SPACE'" json:"quoteSymbol"`
	QuoteDecimals int    `gorm:"column:quote_decimals;default:8" json:"quoteDecimals"`
	TickSize      string `gorm:"column:tick_size" json:"tickSize"`
	Status        int    `gorm:"column:status" json:"status"`
	CreatedAt     int64  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt     int64  `gorm:"column:updated_at" json:"updatedAt"`
}

func (OrderbookPairModel) TableName() string {
	return "tb_ob_pairs"
}

type OrderbookOrderModel struct {
	Id                int64  `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Outpoint          string `gorm:"column:outpoint;uniqueIndex" json:"outpoint"`
	Txid              string `gorm:"column:txid" json:"txid"`
	Vout              int    `gorm:"column:vout" json:"vout"`
	TokenTxid         string `gorm:"column:token_txid" json:"tokenTxid"`
	TokenVout         int    `gorm:"column:token_vout" json:"tokenVout"`
	PairKey           string `gorm:"column:pair_key;size:180" json:"pairKey"`
	TokenCodeHash     string `gorm:"column:token_codehash" json:"tokenCodeHash"`
	TokenID           string `gorm:"column:token_id" json:"tokenID"`
	IsTokenOrder      int    `gorm:"column:is_token_order;default:0" json:"isTokenOrder"`
	SellTokenCodeHash string `gorm:"column:sell_token_codehash;size:40;default:''" json:"sellTokenCodeHash"`
	SellTokenID       string `gorm:"column:sell_token_id;size:40;default:''" json:"sellTokenID"`
	BuyTokenCodeHash  string `gorm:"column:buy_token_codehash;size:40;default:''" json:"buyTokenCodeHash"`
	BuyTokenID        string `gorm:"column:buy_token_id;size:40;default:''" json:"buyTokenID"`
	Side              uint8  `gorm:"column:side" json:"side"`
	MakerAddr         string `gorm:"column:maker_addr" json:"makerAddr"`
	PriceNum          uint64 `gorm:"column:price_num" json:"priceNum"`
	PriceDen          uint64 `gorm:"column:price_den" json:"priceDen"`
	PriceDec          string `gorm:"column:price_dec" json:"priceDec"`
	RemainAmount      uint64 `gorm:"column:remain_amount" json:"remainAmount"`
	MinFillAmount     uint64 `gorm:"column:min_fill_amount" json:"minFillAmount"`
	FeeBps            int    `gorm:"column:fee_bps" json:"feeBps"`
	FeeAddr           string `gorm:"column:fee_addr" json:"feeAddr"`
	SaltHex           string `gorm:"column:salt_hex" json:"saltHex"`
	Status            string `gorm:"column:status" json:"status"`
	SpentBy           string `gorm:"column:spent_by" json:"spentBy"`
	Height            int64  `gorm:"column:height" json:"height"`
	IsMempool         int    `gorm:"column:is_mempool" json:"isMempool"`
	CreatedAt         int64  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt         int64  `gorm:"column:updated_at" json:"updatedAt"`
}

func (OrderbookOrderModel) TableName() string {
	return "tb_ob_orders"
}

type OrderbookTradeModel struct {
	Id            int64  `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TradeKey      string `gorm:"column:trade_key;uniqueIndex" json:"tradeKey"`
	Txid          string `gorm:"column:txid" json:"txid"`
	OrderOutpoint string `gorm:"column:order_outpoint" json:"orderOutpoint"`
	PairKey       string `gorm:"column:pair_key;size:180" json:"pairKey"`
	Side          string `gorm:"column:side" json:"side"`
	MakerAddr     string `gorm:"column:maker_addr" json:"makerAddr"`
	TakerAddr     string `gorm:"column:taker_addr" json:"takerAddr"`
	PriceNum      uint64 `gorm:"column:price_num" json:"priceNum"`
	PriceDen      uint64 `gorm:"column:price_den" json:"priceDen"`
	PriceDec      string `gorm:"column:price_dec" json:"priceDec"`
	FillToken     uint64 `gorm:"column:fill_token" json:"fillToken"`
	SatsAmount    uint64 `gorm:"column:sats_amount" json:"satsAmount"`
	FeeSats       uint64 `gorm:"column:fee_sats" json:"feeSats"`
	IsPartial     int    `gorm:"column:is_partial" json:"isPartial"`
	Height        int64  `gorm:"column:height" json:"height"`
	IsMempool     int    `gorm:"column:is_mempool" json:"isMempool"`
	TradeTime     int64  `gorm:"column:trade_time" json:"tradeTime"`
}

func (OrderbookTradeModel) TableName() string {
	return "tb_ob_trades"
}

var (
	_obDaoOnce sync.Once
	_obDao     *orderbookDao
)

type orderbookDao struct{}

func OrderbookDao() *orderbookDao {
	_obDaoOnce.Do(func() {
		_obDao = &orderbookDao{}
	})
	return _obDao
}

func (d *orderbookDao) AutoMigrate() error {
	db := major.GetSqlDB()
	if db == nil {
		return nil
	}
	return db.AutoMigrate(&OrderbookPairModel{}, &OrderbookOrderModel{}, &OrderbookTradeModel{})
}

func (d *orderbookDao) UpsertPair(m *OrderbookPairModel) error {
	db := major.GetSqlDB()
	if db == nil || m == nil {
		return nil
	}
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "pair_key"}},
		DoUpdates: clause.AssignmentColumns([]string{"symbol", "decimals", "quote_codehash", "quote_token_id", "quote_symbol", "quote_decimals", "tick_size", "status", "updated_at"}),
	}).Create(m).Error
}

func (d *orderbookDao) ListOpenOrdersMap() (map[string]OrderbookOrderModel, error) {
	db := major.GetSqlDB()
	if db == nil {
		return nil, nil
	}
	var rows []OrderbookOrderModel
	if err := db.Where("status = ?", "open").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]OrderbookOrderModel, len(rows))
	for _, r := range rows {
		out[r.Outpoint] = r
	}
	return out, nil
}

// ListClosedOutpointsSet returns all outpoints already marked non-open in DB
// (filled, partial_filled, cancelled, spent) or already recorded in tb_orderbook_trade
// so BootstrapSnapshot does not resurrect mempool-spent orders when Higun still reports spentBy=null.
func (d *orderbookDao) ListClosedOutpointsSet() (map[string]struct{}, error) {
	db := major.GetSqlDB()
	if db == nil {
		return nil, nil
	}
	var outpoints []string
	if err := db.Model(&OrderbookOrderModel{}).Where("status <> ?", "open").Pluck("outpoint", &outpoints).Error; err != nil {
		return nil, err
	}
	out := make(map[string]struct{}, len(outpoints))
	for _, op := range outpoints {
		out[op] = struct{}{}
	}
	var tradedOutpoints []string
	if err := db.Model(&OrderbookTradeModel{}).Distinct("order_outpoint").Pluck("order_outpoint", &tradedOutpoints).Error; err == nil {
		for _, op := range tradedOutpoints {
			if op != "" {
				out[op] = struct{}{}
			}
		}
	}
	return out, nil
}

func (d *orderbookDao) ListPairs() ([]OrderbookPairModel, error) {
	db := major.GetSqlDB()
	if db == nil {
		return nil, nil
	}
	var out []OrderbookPairModel
	err := db.Where("status = 1").Order("id asc").Find(&out).Error
	return out, err
}

func (d *orderbookDao) UpsertOrder(m *OrderbookOrderModel) error {
	db := major.GetSqlDB()
	if db == nil || m == nil {
		return nil
	}
	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "outpoint"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"token_txid", "token_vout", "status", "spent_by", "height", "is_mempool", "updated_at",
		}),
	}).Create(m).Error
}

func (d *orderbookDao) UpdateOrderStatus(outpoint, status, spentBy string, height int64, isMempool int, updatedAt int64) error {
	db := major.GetSqlDB()
	if db == nil {
		return nil
	}
	return db.Model(&OrderbookOrderModel{}).
		Where("outpoint = ?", outpoint).
		Updates(map[string]interface{}{
			"status":     status,
			"spent_by":   spentBy,
			"height":     height,
			"is_mempool": isMempool,
			"updated_at": updatedAt,
		}).Error
}

func (d *orderbookDao) ListOrdersByMaker(makerAddr, pairKey, status string, limit int) ([]OrderbookOrderModel, error) {
	db := major.GetSqlDB()
	if db == nil {
		return nil, nil
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	q := db.Model(&OrderbookOrderModel{})
	if makerAddr != "" {
		q = q.Where("maker_addr = ?", makerAddr)
	}
	if pairKey != "" {
		q = q.Where("pair_key = ?", pairKey)
	}
	if status != "" && status != "all" {
		q = q.Where("status = ?", status)
	}
	var out []OrderbookOrderModel
	err := q.Order("updated_at desc").Limit(limit).Find(&out).Error
	return out, err
}

func (d *orderbookDao) UpsertTrade(m *OrderbookTradeModel) error {
	db := major.GetSqlDB()
	if db == nil || m == nil {
		return nil
	}
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "trade_key"}},
		DoUpdates: clause.AssignmentColumns([]string{"height", "is_mempool"}),
	}).Create(m).Error
}

func (d *orderbookDao) ListRecentTrades(pairKey string, limit int) ([]OrderbookTradeModel, error) {
	db := major.GetSqlDB()
	if db == nil {
		return nil, nil
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := db.Model(&OrderbookTradeModel{})
	if pairKey != "" {
		q = q.Where("pair_key = ?", pairKey)
	}
	var out []OrderbookTradeModel
	err := q.Order("trade_time desc").Limit(limit).Find(&out).Error
	return out, err
}

func (d *orderbookDao) ListTradesSince(pairKey string, sinceMs int64, limit int) ([]OrderbookTradeModel, error) {
	db := major.GetSqlDB()
	if db == nil {
		return nil, nil
	}
	if limit <= 0 || limit > 10000 {
		limit = 5000
	}
	var out []OrderbookTradeModel
	err := db.Model(&OrderbookTradeModel{}).
		Where("pair_key = ? AND trade_time >= ?", pairKey, sinceMs).
		Order("trade_time asc").
		Limit(limit).
		Find(&out).Error
	return out, err
}
