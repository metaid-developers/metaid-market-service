package models

import (
	"errors"
	"gorm.io/gorm"
	"metaid-market-service/major"
	"metaid-market-service/tool"
	"sync"
)

type Mrc20DeployOrderModel struct {
	Id                int64             `gorm:"column:id" json:"id"`
	OrderId           string            `gorm:"column:orderId" json:"orderId"`
	InscribeState     InscribeState     `gorm:"column:inscribeState" json:"inscribeState"`
	Address           string            `gorm:"column:address" json:"address"`
	TickId            string            `gorm:"column:tickId" json:"tickId"`
	Tick              string            `gorm:"column:tick" json:"tick"`
	TokenName         string            `gorm:"column:tokenName" json:"tokenName"`
	Decimals          string            `gorm:"column:decimals" json:"decimals"`
	AmtPerMint        string            `gorm:"column:amtPerMint" json:"amtPerMint"`
	MintCount         string            `gorm:"column:mintCount" json:"mintCount"`
	PremineCount      string            `gorm:"column:premineCount" json:"premineCount"`
	StartBlockHeight  string            `gorm:"column:startBlockHeight" json:"startBlockHeight"`
	Qual              string            `gorm:"column:qual" json:"qual"`
	Payload           string            `gorm:"column:payload" json:"payload"`
	Chain             string            `gorm:"column:chain" json:"chain"`
	CommitTxRaw       string            `gorm:"column:commitTxRaw" json:"commitTxRaw"`
	RevealTxRaw       string            `gorm:"column:revealTxRaw" json:"revealTxRaw"`
	CommitTxId        string            `gorm:"column:commitTxId" json:"commitTxId"`
	RevealTxId        string            `gorm:"column:revealTxId" json:"revealTxId"`
	TxId              string            `gorm:"column:txId" json:"txId"`
	BlockHeight       int64             `gorm:"column:blockHeight" json:"blockHeight"`
	ConfirmationState ConfirmationState `gorm:"column:confirmationState" json:"confirmationState"`
	Timestamp         int64             `gorm:"column:timestamp" json:"timestamp"`
	Version           int64             `gorm:"column:version" json:"version"`
	CreateTime        int64             `gorm:"column:createTime" json:"createTime"`
	UpdateTime        int64             `gorm:"column:updateTime" json:"updateTime"`
	State             int64             `gorm:"column:state" json:"state"`
}

func (Mrc20DeployOrderModel) TableName() string {
	return "tb_mrc20_deploy_order"
}

var _mrc20DeployOrderModelOnce sync.Once
var _mrc20DeployOrderModelManager *mrc20DeployOrderModelDao

type mrc20DeployOrderModelDao struct {
}

func Mrc20DeployOrderModelDao() *mrc20DeployOrderModelDao {
	_mrc20DeployOrderModelOnce.Do(func() {
		_mrc20DeployOrderModelManager = &mrc20DeployOrderModelDao{}
	})
	return _mrc20DeployOrderModelManager
}

func (_ *mrc20DeployOrderModelDao) Set(model *Mrc20DeployOrderModel) error {
	if model == nil {
		return errors.New("model is nil")
	}
	tx := major.GetSqlDB().Create(model)
	if tx.Error != nil {
		return tx.Error
	}
	return nil
}

func (_ *mrc20DeployOrderModelDao) GetOne(qo *Mrc20DeployOrderModel) (*Mrc20DeployOrderModel, error) {
	model := &Mrc20DeployOrderModel{}
	tx := major.GetSqlDB().Where(qo).First(model)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return model, nil
}

func (_ *mrc20DeployOrderModelDao) GetLastOne(qo *Mrc20DeployOrderModel) (*Mrc20DeployOrderModel, error) {
	model := &Mrc20DeployOrderModel{}
	tx := major.GetSqlDB().Where(qo).Order("blockHeight desc").First(model)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return model, nil
}

func (_ *mrc20DeployOrderModelDao) GetList(qo *Mrc20DeployOrderModel, offset, limit int64) ([]*Mrc20DeployOrderModel, error) {
	var models []*Mrc20DeployOrderModel
	tx := major.GetSqlDB().Where(qo).Limit(int(limit)).Offset(int(offset)).Order("timestamp asc").Find(&models)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return models, nil
}

func (_ *mrc20DeployOrderModelDao) Count(qo *Mrc20DeployOrderModel) (int64, error) {
	var count int64
	filter := ""
	tx := major.GetSqlDB().Model(&Mrc20DeployOrderModel{}).Where(qo).Where(filter).Count(&count)
	if tx.Error != nil {
		return 0, tx.Error
	}
	return count, nil
}

func (_ *mrc20DeployOrderModelDao) GetAll(qo *Mrc20DeployOrderModel) ([]Mrc20DeployOrderModel, error) {
	var models []Mrc20DeployOrderModel
	tx := major.GetSqlDB().Where(qo).Find(&models)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return models, nil
}

func (_ *mrc20DeployOrderModelDao) Update(q *Mrc20DeployOrderModel) error {
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

func (_ *mrc20DeployOrderModelDao) SaveEntityForInscribing(model *Mrc20DeployOrderModel, txRawList []string, jobFunc func(txRaw string) (string, error)) error {
	err := major.GetSqlDB().Transaction(func(tx *gorm.DB) error {
		if model == nil {
			return errors.New("model is nil")
		}

		model.CommitTxRaw = ""
		model.RevealTxRaw = ""
		model.InscribeState = InscribeStateFinish
		model.ConfirmationState = ConfirmationStateUnconfirmed

		if err := tx.Create(model).Error; err != nil {
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
