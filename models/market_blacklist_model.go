package models

import (
	"errors"
	"metaid-market-service/major"
	"metaid-market-service/tool"
	"sync"
)

const (
	BlacklistTypeAddress          = "address"
	BlacklistTypeTxId             = "txid"
	BlacklistTypeUtxo             = "utxo"
	BlacklistTypeWhitelistAddress = "whitelist_address"
	BlacklistTypeWhitelistTxId    = "whitelist_txid"
	BlacklistTypeWhitelistUtxo    = "whitelist_utxo"
)

type MarketBlacklistModel struct {
	Id          int64  `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	EntryType   string `gorm:"column:entryType" json:"entryType"`
	EntryValue  string `gorm:"column:entryValue" json:"entryValue"`
	Source      string `gorm:"column:source" json:"source"`
	ParentValue string `gorm:"column:parentValue" json:"parentValue"`
	Reason      string `gorm:"column:reason" json:"reason"`
	CreateTime  int64  `gorm:"column:createTime" json:"createTime"`
	UpdateTime  int64  `gorm:"column:updateTime" json:"updateTime"`
	State       int64  `gorm:"column:state" json:"state"`
}

func (MarketBlacklistModel) TableName() string {
	return "tb_market_blacklist"
}

var _marketBlacklistModelOnce sync.Once
var _marketBlacklistModelManager *marketBlacklistModelDao

type marketBlacklistModelDao struct{}

func MarketBlacklistModelDao() *marketBlacklistModelDao {
	_marketBlacklistModelOnce.Do(func() {
		_marketBlacklistModelManager = &marketBlacklistModelDao{}
	})
	return _marketBlacklistModelManager
}

func (_ *marketBlacklistModelDao) EnsureTable() error {
	db := major.GetSqlDB()
	if db == nil {
		return errors.New("sql db is not initialized")
	}
	ddl := `CREATE TABLE IF NOT EXISTS tb_market_blacklist (
		id BIGINT NOT NULL AUTO_INCREMENT,
		entryType VARCHAR(32) NOT NULL,
		entryValue VARCHAR(160) NOT NULL,
		source VARCHAR(64) NOT NULL DEFAULT '',
		parentValue VARCHAR(160) NOT NULL DEFAULT '',
		reason VARCHAR(512) NOT NULL DEFAULT '',
		createTime BIGINT NOT NULL DEFAULT 0,
		updateTime BIGINT NOT NULL DEFAULT 0,
		state BIGINT NOT NULL DEFAULT 1,
		PRIMARY KEY (id),
		UNIQUE KEY uk_blacklist_type_value (entryType, entryValue),
		KEY idx_blacklist_state (state)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`
	return db.Exec(ddl).Error
}

// SeedIgnore inserts an entry only if (entryType, entryValue) does not already exist,
// preserving any manual state changes made by administrators.
func (_ *marketBlacklistModelDao) SeedIgnore(model *MarketBlacklistModel) error {
	db := major.GetSqlDB()
	if db == nil || model == nil {
		return errors.New("invalid db or model")
	}
	now := tool.MakeTimestamp()
	if model.CreateTime == 0 {
		model.CreateTime = now
	}
	if model.UpdateTime == 0 {
		model.UpdateTime = now
	}
	if model.State == 0 {
		model.State = STATE_EXIST
	}
	sql := `INSERT IGNORE INTO tb_market_blacklist
		(entryType, entryValue, source, parentValue, reason, createTime, updateTime, state)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	return db.Exec(sql,
		model.EntryType,
		model.EntryValue,
		model.Source,
		model.ParentValue,
		model.Reason,
		model.CreateTime,
		model.UpdateTime,
		model.State,
	).Error
}

func (_ *marketBlacklistModelDao) Upsert(model *MarketBlacklistModel) error {
	db := major.GetSqlDB()
	if db == nil || model == nil {
		return errors.New("invalid db or model")
	}
	now := tool.MakeTimestamp()
	if model.CreateTime == 0 {
		model.CreateTime = now
	}
	model.UpdateTime = now
	if model.State == 0 {
		model.State = STATE_EXIST
	}
	sql := `INSERT INTO tb_market_blacklist
		(entryType, entryValue, source, parentValue, reason, createTime, updateTime, state)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			source = VALUES(source),
			parentValue = VALUES(parentValue),
			reason = VALUES(reason),
			updateTime = VALUES(updateTime),
			state = VALUES(state)`
	return db.Exec(sql,
		model.EntryType,
		model.EntryValue,
		model.Source,
		model.ParentValue,
		model.Reason,
		model.CreateTime,
		model.UpdateTime,
		model.State,
	).Error
}

func (_ *marketBlacklistModelDao) GetAllActive() ([]*MarketBlacklistModel, error) {
	db := major.GetSqlDB()
	if db == nil {
		return nil, errors.New("sql db is not initialized")
	}
	var list []*MarketBlacklistModel
	if err := db.Where("state = ?", STATE_EXIST).Order("id asc").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (_ *marketBlacklistModelDao) GetList(entryType string, state int64, offset, limit int64) ([]*MarketBlacklistModel, int64, error) {
	db := major.GetSqlDB()
	if db == nil {
		return nil, 0, errors.New("sql db is not initialized")
	}
	q := db.Model(&MarketBlacklistModel{})
	if entryType != "" {
		q = q.Where("entryType = ?", entryType)
	}
	if state > 0 {
		q = q.Where("state = ?", state)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	var list []*MarketBlacklistModel
	if err := q.Order("id desc").Offset(int(offset)).Limit(int(limit)).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
