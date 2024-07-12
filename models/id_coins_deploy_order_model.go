package models

import (
	"errors"
	"gorm.io/gorm"
	"metaid-market-service/major"
	"metaid-market-service/tool"
	"sync"
)

type DeployType string

const (
	DeployTypeDefault DeployType = ""
	DeployTypeNormal  DeployType = "normal"
	DeployTypeIdCoins DeployType = "idCoins"
)

type IdCoinsDeployOrderModel struct {
	Id                  int64             `gorm:"column:id" json:"id"`
	OrderId             string            `gorm:"column:orderId" json:"orderId"`
	DeployType          DeployType        `gorm:"column:deployType" json:"deployType"`
	InscribeState       InscribeState     `gorm:"column:inscribeState" json:"inscribeState"`
	IssuerMetaId        string            `gorm:"column:issuerMetaId" json:"issuerMetaId"`
	IssuerAddress       string            `gorm:"column:issuerAddress" json:"issuerAddress"`
	IssuerPublicKey     string            `gorm:"column:issuerPublicKey" json:"issuerPublicKey"`
	IssuerSign          string            `gorm:"column:issuerSign" json:"issuerSign"`
	TickId              string            `gorm:"column:tickId" json:"tickId"`
	Tick                string            `gorm:"column:tick" json:"tick"`
	TokenName           string            `gorm:"column:tokenName" json:"tokenName"`
	Decimals            string            `gorm:"column:decimals" json:"decimals"`
	AmtPerMint          string            `gorm:"column:amtPerMint" json:"amtPerMint"`
	MintCount           string            `gorm:"column:mintCount" json:"mintCount"`
	PremineCount        string            `gorm:"column:premineCount" json:"premineCount"`
	StartBlockHeight    string            `gorm:"column:startBlockHeight" json:"startBlockHeight"`
	Metadata            string            `gorm:"column:metadata" json:"metadata"`
	TickSign            string            `gorm:"column:tickSign" json:"tickSign"`
	PinCheck            string            `gorm:"column:pinCheck" json:"pinCheck"`
	PayCheckPublicKey   string            `gorm:"column:payCheckPublicKey" json:"payCheckPublicKey"`
	PayCheckAddress     string            `gorm:"column:payCheckAddress" json:"payCheckAddress"`
	PayCheckAmount      int64             `gorm:"column:payCheckAmount" json:"payCheckAmount"`
	Payload             string            `gorm:"column:payload" json:"payload"`
	Chain               string            `gorm:"column:chain" json:"chain"`
	TotalFee            int64             `gorm:"column:totalFee" json:"totalFee"`
	MinerFee            int64             `gorm:"column:minerFee" json:"minerFee"`
	ServiceFee          int64             `gorm:"column:serviceFee" json:"serviceFee"`
	RedeemScript        string            `gorm:"column:redeemScript" json:"redeemScript"`
	ControlBlockWitness string            `gorm:"column:controlBlockWitness" json:"controlBlockWitness"`
	RevealTxPrivateKey  string            `gorm:"column:revealTxPrivateKey" json:"revealTxPrivateKey"`
	RevealTxAddress     string            `gorm:"column:revealTxAddress" json:"revealTxAddress"`
	RevealInputIndex    int64             `gorm:"column:revealInputIndex" json:"revealInputIndex"`
	CommitTxRaw         string            `gorm:"column:commitTxRaw" json:"commitTxRaw"`
	RevealPrePsbtRaw    string            `gorm:"column:revealPrePsbtRaw" json:"revealPrePsbtRaw"`
	RevealFinalPsbtRaw  string            `gorm:"column:revealFinalPsbtRaw" json:"revealFinalPsbtRaw"`
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

func (IdCoinsDeployOrderModel) TableName() string {
	return "tb_id_coins_deploy_order"
}

var _idCoinsDeployOrderModelOnce sync.Once
var _idCoinsDeployOrderModelManager *idCoinsDeployOrderModelDao

type idCoinsDeployOrderModelDao struct {
}

func IdCoinsDeployOrderModelDao() *idCoinsDeployOrderModelDao {
	_idCoinsDeployOrderModelOnce.Do(func() {
		_idCoinsDeployOrderModelManager = &idCoinsDeployOrderModelDao{}
	})
	return _idCoinsDeployOrderModelManager
}

func (_ *idCoinsDeployOrderModelDao) Set(model *IdCoinsDeployOrderModel) error {
	if model == nil {
		return errors.New("model is nil")
	}
	tx := major.GetSqlDB().Create(model)
	if tx.Error != nil {
		return tx.Error
	}
	return nil
}

func (_ *idCoinsDeployOrderModelDao) GetOne(qo *IdCoinsDeployOrderModel) (*IdCoinsDeployOrderModel, error) {
	model := &IdCoinsDeployOrderModel{}
	tx := major.GetSqlDB().Where(qo).First(model)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return model, nil
}

func (_ *idCoinsDeployOrderModelDao) GetLastOne(qo *IdCoinsDeployOrderModel) (*IdCoinsDeployOrderModel, error) {
	model := &IdCoinsDeployOrderModel{}
	tx := major.GetSqlDB().Where(qo).Order("blockHeight desc").First(model)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return model, nil
}

func (_ *idCoinsDeployOrderModelDao) GetList(qo *IdCoinsDeployOrderModel, offset, limit int64) ([]*IdCoinsDeployOrderModel, error) {
	var models []*IdCoinsDeployOrderModel
	tx := major.GetSqlDB().Where(qo).Limit(int(limit)).Offset(int(offset)).Order("timestamp desc").Find(&models)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return models, nil
}

func (_ *idCoinsDeployOrderModelDao) GetListAsc(qo *IdCoinsDeployOrderModel, offset, limit int64) ([]*IdCoinsDeployOrderModel, error) {
	var models []*IdCoinsDeployOrderModel
	tx := major.GetSqlDB().Where(qo).Limit(int(limit)).Offset(int(offset)).Order("timestamp asc").Find(&models)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return models, nil
}

func (_ *idCoinsDeployOrderModelDao) Count(qo *IdCoinsDeployOrderModel) (int64, error) {
	var count int64
	filter := ""
	tx := major.GetSqlDB().Model(&IdCoinsDeployOrderModel{}).Where(qo).Where(filter).Count(&count)
	if tx.Error != nil {
		return 0, tx.Error
	}
	return count, nil
}

func (_ *idCoinsDeployOrderModelDao) GetAll(qo *IdCoinsDeployOrderModel) ([]IdCoinsDeployOrderModel, error) {
	var models []IdCoinsDeployOrderModel
	tx := major.GetSqlDB().Where(qo).Find(&models)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return models, nil
}

func (_ *idCoinsDeployOrderModelDao) Update(q *IdCoinsDeployOrderModel) error {
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

func (_ *idCoinsDeployOrderModelDao) UpdateEntityForInscribing(model *IdCoinsDeployOrderModel, txRawList []string, jobFunc func(txRaw string) (string, error)) error {
	err := major.GetSqlDB().Transaction(func(tx *gorm.DB) error {
		if model == nil {
			return errors.New("model is nil")
		}

		model.CommitTxRaw = ""
		model.RevealPrePsbtRaw = ""
		model.RevealFinalPsbtRaw = ""
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
