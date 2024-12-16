package models

import (
	"errors"
	"gorm.io/gorm"
	"metaid-market-service/major"
	"metaid-market-service/tool"
	"sync"
)

type MetanameRegisterOrderModel struct {
	Id                  int64             `gorm:"column:id" json:"id"`
	OrderId             string            `gorm:"column:orderId" json:"orderId"`
	InscribeState       InscribeState     `gorm:"column:inscribeState" json:"inscribeState"`
	RegisterAddress     string            `gorm:"column:registerAddress" json:"registerAddress"`
	ReceiveAddress      string            `gorm:"column:receiveAddress" json:"receiveAddress"`
	PinId               string            `gorm:"column:pinId" json:"pinId"`
	Metaname            string            `gorm:"column:metaname" json:"metaname"`
	Name                string            `gorm:"column:name" json:"name"`
	Namespace           string            `gorm:"column:namespace" json:"namespace"`
	Payload             string            `gorm:"column:payload" json:"payload"`
	Chain               string            `gorm:"column:chain" json:"chain"`
	NetworkFeeRate      int64             `gorm:"column:networkFeeRate" json:"networkFeeRate"`
	TotalFee            int64             `gorm:"column:totalFee" json:"totalFee"`
	MinerFee            int64             `gorm:"column:minerFee" json:"minerFee"`
	ServiceFee          int64             `gorm:"column:serviceFee" json:"serviceFee"`
	RedeemScript        string            `gorm:"column:redeemScript" json:"redeemScript"`
	ControlBlockWitness string            `gorm:"column:controlBlockWitness" json:"controlBlockWitness"`
	RevealTxPrivateKey  string            `gorm:"column:revealTxPrivateKey" json:"revealTxPrivateKey"`
	RevealTxAddress     string            `gorm:"column:revealTxAddress" json:"revealTxAddress"`
	RevealInputIndex    int64             `gorm:"column:revealInputIndex" json:"revealInputIndex"`
	RevealPrePsbtRaw    string            `gorm:"column:revealPrePsbtRaw" json:"revealPrePsbtRaw"`
	RevealFinalPsbtRaw  string            `gorm:"column:revealFinalPsbtRaw" json:"revealFinalPsbtRaw"`
	CommitTxRaw         string            `gorm:"column:commitTxRaw" json:"commitTxRaw"`
	RevealTxRaw         string            `gorm:"column:revealTxRaw" json:"revealTxRaw"`
	CommitTxId          string            `gorm:"column:commitTxId" json:"commitTxId"`
	RevealTxId          string            `gorm:"column:revealTxId" json:"revealTxId"`
	TxId                string            `gorm:"column:txId" json:"txId"`
	BlockHeight         int64             `gorm:"column:blockHeight" json:"blockHeight"`
	ConfirmationState   ConfirmationState `gorm:"column:confirmationState" json:"confirmationState"`
	Timestamp           int64             `gorm:"column:timestamp" json:"timestamp"`
	Version             int64             `gorm:"column:version" json:"version"`
	CreateTime          int64             `gorm:"column:createTime" json:"createTime"`
	UpdateTime          int64             `gorm:"column:updateTime" json:"updateTime"`
	State               int64             `gorm:"column:state" json:"state"`
}

func (MetanameRegisterOrderModel) TableName() string {
	return "tb_metaname_register_order"
}

var _metanameRegisterOrderModelOnce sync.Once
var _metanameRegisterOrderModelManager *metanameRegisterOrderModelDao

type metanameRegisterOrderModelDao struct {
}

func MetanameRegisterOrderModelDao() *metanameRegisterOrderModelDao {
	_metanameRegisterOrderModelOnce.Do(func() {
		_metanameRegisterOrderModelManager = &metanameRegisterOrderModelDao{}
	})
	return _metanameRegisterOrderModelManager
}

func (_ *metanameRegisterOrderModelDao) Set(model *MetanameRegisterOrderModel) error {
	if model == nil {
		return errors.New("model is nil")
	}
	tx := major.GetSqlDB().Create(model)
	if tx.Error != nil {
		return tx.Error
	}
	return nil
}

func (_ *metanameRegisterOrderModelDao) GetOne(qo *MetanameRegisterOrderModel) (*MetanameRegisterOrderModel, error) {
	model := &MetanameRegisterOrderModel{}
	tx := major.GetSqlDB().Where(qo).First(model)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return model, nil
}

func (_ *metanameRegisterOrderModelDao) GetLastOne(qo *MetanameRegisterOrderModel) (*MetanameRegisterOrderModel, error) {
	model := &MetanameRegisterOrderModel{}
	tx := major.GetSqlDB().Where(qo).Order("blockHeight desc").First(model)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return model, nil
}

func (_ *metanameRegisterOrderModelDao) GetList(qo *MetanameRegisterOrderModel, offset, limit int64) ([]*MetanameRegisterOrderModel, error) {
	var models []*MetanameRegisterOrderModel
	tx := major.GetSqlDB().Where(qo).Limit(int(limit)).Offset(int(offset)).Order("timestamp desc").Find(&models)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return models, nil
}

func (_ *metanameRegisterOrderModelDao) GetListAsc(qo *MetanameRegisterOrderModel, offset, limit int64) ([]*MetanameRegisterOrderModel, error) {
	var models []*MetanameRegisterOrderModel
	tx := major.GetSqlDB().Where(qo).Limit(int(limit)).Offset(int(offset)).Order("timestamp asc").Find(&models)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return models, nil
}

func (_ *metanameRegisterOrderModelDao) Count(qo *MetanameRegisterOrderModel) (int64, error) {
	var count int64
	filter := ""
	tx := major.GetSqlDB().Model(&MetanameRegisterOrderModel{}).Where(qo).Where(filter).Count(&count)
	if tx.Error != nil {
		return 0, tx.Error
	}
	return count, nil
}

func (_ *metanameRegisterOrderModelDao) GetAll(qo *MetanameRegisterOrderModel) ([]MetanameRegisterOrderModel, error) {
	var models []MetanameRegisterOrderModel
	tx := major.GetSqlDB().Where(qo).Find(&models)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return models, nil
}

func (_ *metanameRegisterOrderModelDao) Update(q *MetanameRegisterOrderModel) error {
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

func (_ *metanameRegisterOrderModelDao) SaveEntityForInscribing(model *MetanameRegisterOrderModel, txRawList []string, jobFunc func(txRaw string) (string, error)) error {
	err := major.GetSqlDB().Transaction(func(tx *gorm.DB) error {
		if model == nil {
			return errors.New("model is nil")
		}

		model.RedeemScript = ""
		model.ControlBlockWitness = ""
		//model.RevealTxPrivateKey = ""
		//model.RevealTxAddress = ""
		model.RevealPrePsbtRaw = ""
		model.RevealFinalPsbtRaw = ""
		model.CommitTxRaw = ""
		model.RevealTxRaw = ""
		model.InscribeState = InscribeStateFinish
		model.ConfirmationState = ConfirmationStateUnconfirmed

		if err := tx.Save(model).Error; err != nil {
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
