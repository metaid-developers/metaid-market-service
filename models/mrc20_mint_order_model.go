package models

import (
	"errors"
	"gorm.io/gorm"
	"metaid-market-service/major"
	"metaid-market-service/tool"
	"sync"
)

type InscribeState int64

const (
	InscribeStateNone     InscribeState = 0
	InscribeStatePending  InscribeState = 1
	InscribeStatePaid     InscribeState = 2
	InscribeStateFinish   InscribeState = 3
	InscribeStateErr      InscribeState = 100
	InscribeStateOverTime InscribeState = 500
)

type Mrc20MintOrderModel struct {
	Id                  int64             `gorm:"column:id" json:"id"`
	OrderId             string            `gorm:"column:orderId" json:"orderId"`
	InscribeState       InscribeState     `gorm:"column:inscribeState" json:"inscribeState"`
	TicketId            string            `gorm:"column:ticketId" json:"ticketId"`
	TotalFee            int64             `gorm:"column:totalFee" json:"totalFee"`
	MinerFee            int64             `gorm:"column:minerFee" json:"minerFee"`
	ServiceFee          int64             `gorm:"column:serviceFee" json:"serviceFee"`
	RevealOutValue      int64             `gorm:"column:revealOutValue" json:"revealOutValue"`
	RedeemScript        string            `gorm:"column:redeemScript" json:"redeemScript"`
	ControlBlockWitness string            `gorm:"column:controlBlockWitness" json:"controlBlockWitness"`
	RevealTxPrivateKey  string            `gorm:"column:revealTxPrivateKey" json:"revealTxPrivateKey"`
	RevealTxAddress     string            `gorm:"column:revealTxAddress" json:"revealTxAddress"`
	CommitTxRaw         string            `gorm:"column:commitTxRaw" json:"commitTxRaw"`
	RevealInputIndex    int64             `gorm:"column:revealInputIndex" json:"revealInputIndex"`
	RevealPrePsbtRaw    string            `gorm:"column:revealPrePsbtRaw" json:"revealPrePsbtRaw"`
	RevealMidPsbtRaw    string            `gorm:"column:revealMidPsbtRaw" json:"revealMidPsbtRaw"`
	RevealFinalPsbtRaw  string            `gorm:"column:revealFinalPsbtRaw" json:"revealFinalPsbtRaw"`
	CommitTxId          string            `gorm:"column:commitTxId" json:"commitTxId"`
	TxId                string            `gorm:"column:txId" json:"txId"`
	BlockHeight         int64             `gorm:"column:blockHeight" json:"blockHeight"`
	ConfirmationState   ConfirmationState `gorm:"column:confirmationState" json:"confirmationState"`
	Timestamp           int64             `gorm:"column:timestamp" json:"timestamp"`
	Version             int64             `gorm:"column:version" json:"version"`
	CreateTime          int64             `gorm:"column:createTime" json:"createTime"`
	UpdateTime          int64             `gorm:"column:updateTime" json:"updateTime"`
	State               int64             `gorm:"column:state" json:"state"`
}

func (Mrc20MintOrderModel) TableName() string {
	return "tb_mrc20_mint_order"
}

var _mrc20MintOrderModelOnce sync.Once
var _mrc20MintOrderModelManager *mrc20MintOrderModelDao

type mrc20MintOrderModelDao struct {
}

func Mrc20MintOrderModelDao() *mrc20MintOrderModelDao {
	_mrc20MintOrderModelOnce.Do(func() {
		_mrc20MintOrderModelManager = &mrc20MintOrderModelDao{}
	})
	return _mrc20MintOrderModelManager
}

func (_ *mrc20MintOrderModelDao) Set(model *Mrc20MintOrderModel) error {
	if model == nil {
		return errors.New("model is nil")
	}
	tx := major.GetSqlDB().Create(model)
	if tx.Error != nil {
		return tx.Error
	}
	return nil
}

func (_ *mrc20MintOrderModelDao) GetOne(qo *Mrc20MintOrderModel) (*Mrc20MintOrderModel, error) {
	model := &Mrc20MintOrderModel{}
	tx := major.GetSqlDB().Where(qo).First(model)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return model, nil
}

func (_ *mrc20MintOrderModelDao) GetLastOne(qo *Mrc20MintOrderModel) (*Mrc20MintOrderModel, error) {
	model := &Mrc20MintOrderModel{}
	tx := major.GetSqlDB().Where(qo).Order("blockHeight desc").First(model)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return model, nil
}

func (_ *mrc20MintOrderModelDao) GetList(qo *Mrc20MintOrderModel, offset, limit int64) ([]*Mrc20MintOrderModel, error) {
	var models []*Mrc20MintOrderModel
	tx := major.GetSqlDB().Where(qo).Limit(int(limit)).Offset(int(offset)).Order("timestamp asc").Find(&models)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return models, nil
}

func (_ *mrc20MintOrderModelDao) Count(qo *Mrc20MintOrderModel) (int64, error) {
	var count int64
	filter := ""
	tx := major.GetSqlDB().Model(&Mrc20MintOrderModel{}).Where(qo).Where(filter).Count(&count)
	if tx.Error != nil {
		return 0, tx.Error
	}
	return count, nil
}

func (_ *mrc20MintOrderModelDao) GetAll(qo *Mrc20MintOrderModel) ([]Mrc20MintOrderModel, error) {
	var models []Mrc20MintOrderModel
	tx := major.GetSqlDB().Where(qo).Find(&models)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return models, nil
}

func (_ *mrc20MintOrderModelDao) Update(q *Mrc20MintOrderModel) error {
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

func (_ *mrc20MintOrderModelDao) UpdateEntityListForInscribing(q *Mrc20MintOrderModel, txRawList []string, jobFunc func(txRaw string) (string, error)) error {
	err := major.GetSqlDB().Transaction(func(tx *gorm.DB) error {
		nowTime := tool.MakeTimestamp()

		sv := q.Version
		q.Version += 1
		q.UpdateTime = nowTime
		q.CommitTxRaw = ""
		q.RevealPrePsbtRaw = ""
		q.RevealMidPsbtRaw = ""
		q.RevealFinalPsbtRaw = ""
		q.InscribeState = InscribeStateFinish
		q.ConfirmationState = ConfirmationStateUnconfirmed
		if err := tx.Save(q).Where(map[string]interface{}{"version": sv, "id": q.Id}).Error; err != nil {
			return err
		}

		for _, txRaw := range txRawList {
			_, err := jobFunc(txRaw)
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return err
	}
	return nil
}
