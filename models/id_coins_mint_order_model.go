package models

import (
	"errors"
	"metaid-market-service/major"
	"metaid-market-service/tool"
	"sync"

	"gorm.io/gorm"
)

// IdCoinsMintOrderModel is a self-hosted ID-Coin mint order (two-reveal flow:
// reveal-inscribe creates the /follow pin, reveal-mint inscribes the MRC20
// mint). The reveal private keys are stored because the service must sign both
// reveals at commit time — they must NEVER be exposed through any API.
type IdCoinsMintOrderModel struct {
	Id                          int64             `gorm:"column:id" json:"id"`
	OrderId                     string            `gorm:"column:orderId" json:"orderId"`
	InscribeState               InscribeState     `gorm:"column:inscribeState" json:"inscribeState"`
	Address                     string            `gorm:"column:address" json:"address"`
	TickId                      string            `gorm:"column:tickId" json:"tickId"`
	PayToAddress                string            `gorm:"column:payToAddress" json:"payToAddress"`
	PayToAmount                 int64             `gorm:"column:payToAmount" json:"payToAmount"`
	PayUtxoId                   string            `gorm:"column:payUtxoId" json:"payUtxoId"`
	MintPins                    string            `gorm:"column:mintPins" json:"mintPins"`
	NetworkFeeRate              int64             `gorm:"column:networkFeeRate" json:"networkFeeRate"`
	TotalFee                    int64             `gorm:"column:totalFee" json:"totalFee"`
	InscribeMinerFee            int64             `gorm:"column:inscribeMinerFee" json:"inscribeMinerFee"`
	MintMinerFee                int64             `gorm:"column:mintMinerFee" json:"mintMinerFee"`
	ServiceFee                  int64             `gorm:"column:serviceFee" json:"serviceFee"`
	CommitTxRaw                 string            `gorm:"column:commitTxRaw" json:"commitTxRaw"`
	PinPrivateKey               string            `gorm:"column:pinPrivateKey" json:"-"`
	PinAddress                  string            `gorm:"column:pinAddress" json:"pinAddress"`
	RevealOutValue              int64             `gorm:"column:revealOutValue" json:"revealOutValue"`
	RevealInscribeTxPrivateKey  string            `gorm:"column:revealInscribeTxPrivateKey" json:"-"`
	RevealMintTxPrivateKey      string            `gorm:"column:revealMintTxPrivateKey" json:"-"`
	RevealInscribeTxAddress     string            `gorm:"column:revealInscribeTxAddress" json:"revealInscribeTxAddress"`
	RevealMintTxAddress         string            `gorm:"column:revealMintTxAddress" json:"revealMintTxAddress"`
	RedeemScriptInscribe        string            `gorm:"column:redeemScriptInscribe" json:"redeemScriptInscribe"`
	ControlBlockWitnessInscribe string            `gorm:"column:controlBlockWitnessInscribe" json:"controlBlockWitnessInscribe"`
	RevealInscribeInputIndex    int64             `gorm:"column:revealInscribeInputIndex" json:"revealInscribeInputIndex"`
	RevealInscribePrePsbtRaw    string            `gorm:"column:revealInscribePrePsbtRaw" json:"-"`
	RevealInscribeFinalPsbtRaw  string            `gorm:"column:revealInscribeFinalPsbtRaw" json:"-"`
	RedeemScriptMint            string            `gorm:"column:redeemScriptMint" json:"redeemScriptMint"`
	ControlBlockWitnessMint     string            `gorm:"column:controlBlockWitnessMint" json:"controlBlockWitnessMint"`
	RevealMintInputIndex        int64             `gorm:"column:revealMintInputIndex" json:"revealMintInputIndex"`
	RevealMintPrePsbtRaw        string            `gorm:"column:revealMintPrePsbtRaw" json:"-"`
	RevealMintFinalPsbtRaw      string            `gorm:"column:revealMintFinalPsbtRaw" json:"-"`
	CommitTxId                  string            `gorm:"column:commitTxId" json:"commitTxId"`
	RevealInscribeTxId          string            `gorm:"column:revealInscribeTxId" json:"revealInscribeTxId"`
	RevealMintTxId              string            `gorm:"column:revealMintTxId" json:"revealMintTxId"`
	BlockHeight                 int64             `gorm:"column:blockHeight" json:"blockHeight"`
	ConfirmationState           ConfirmationState `gorm:"column:confirmationState" json:"confirmationState"`
	Timestamp                   int64             `gorm:"column:timestamp" json:"timestamp"`
	Version                     int64             `gorm:"column:version" json:"version"`
	CreateTime                  int64             `gorm:"column:createTime" json:"createTime"`
	UpdateTime                  int64             `gorm:"column:updateTime" json:"updateTime"`
	State                       int64             `gorm:"column:state" json:"state"`
}

func (IdCoinsMintOrderModel) TableName() string {
	return "tb_id_coins_self_mint_order"
}

var _idCoinsMintOrderModelOnce sync.Once
var _idCoinsMintOrderModelManager *idCoinsMintOrderModelDao

type idCoinsMintOrderModelDao struct {
}

func IdCoinsMintOrderModelDao() *idCoinsMintOrderModelDao {
	_idCoinsMintOrderModelOnce.Do(func() {
		_idCoinsMintOrderModelManager = &idCoinsMintOrderModelDao{}
	})
	return _idCoinsMintOrderModelManager
}

func (_ *idCoinsMintOrderModelDao) Set(model *IdCoinsMintOrderModel) error {
	if model == nil {
		return errors.New("model is nil")
	}
	return major.GetSqlDB().Create(model).Error
}

func (_ *idCoinsMintOrderModelDao) GetOne(qo *IdCoinsMintOrderModel) (*IdCoinsMintOrderModel, error) {
	model := &IdCoinsMintOrderModel{}
	tx := major.GetSqlDB().Where(qo).First(model)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return model, nil
}

func (_ *idCoinsMintOrderModelDao) Update(model *IdCoinsMintOrderModel) error {
	if model == nil {
		return errors.New("model is nil")
	}
	model.Version++
	model.UpdateTime = tool.MakeTimestamp()
	result := major.GetSqlDB().Where("id = ? AND version = ?", model.Id, model.Version-1).Save(model)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("mint order state conflict")
	}
	return nil
}

func (_ *idCoinsMintOrderModelDao) GetList(qo *IdCoinsMintOrderModel, offset, limit int64) ([]*IdCoinsMintOrderModel, error) {
	var list []*IdCoinsMintOrderModel
	tx := major.GetSqlDB().Where(qo).Limit(int(limit)).Offset(int(offset)).Order("timestamp desc").Find(&list)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return list, nil
}

func (_ *idCoinsMintOrderModelDao) GetListAsc(qo *IdCoinsMintOrderModel, offset, limit int64) ([]*IdCoinsMintOrderModel, error) {
	var list []*IdCoinsMintOrderModel
	tx := major.GetSqlDB().Where(qo).Limit(int(limit)).Offset(int(offset)).Order("timestamp asc").Find(&list)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return list, nil
}

func (_ *idCoinsMintOrderModelDao) Count(qo *IdCoinsMintOrderModel) (int64, error) {
	var count int64
	tx := major.GetSqlDB().Model(&IdCoinsMintOrderModel{}).Where(qo).Count(&count)
	if tx.Error != nil {
		return 0, tx.Error
	}
	return count, nil
}

// UpdateEntityListForInscribing marks the order finish inside one optimistic-
// locked transaction. Broadcast must happen outside it (see the mrc20 mint
// order conventions): pass the raw txs on the first call so they stay stored
// for crash recovery, then call again with nil after broadcast to scrub them
// together with the reveal secrets.
func (_ *idCoinsMintOrderModelDao) UpdateEntityListForInscribing(q *IdCoinsMintOrderModel, txRawList []string, jobFunc func(chain, txRaw string) (string, error)) error {
	if jobFunc != nil {
		return errors.New("broadcast must run outside the database transaction")
	}
	if q == nil {
		return errors.New("mint order is nil")
	}
	return major.GetSqlDB().Transaction(func(tx *gorm.DB) error {
		q.Version++
		q.UpdateTime = tool.MakeTimestamp()
		q.RevealInscribePrePsbtRaw = ""
		q.RevealMintPrePsbtRaw = ""
		q.RevealInscribeTxPrivateKey = ""
		q.RevealMintTxPrivateKey = ""
		q.PinPrivateKey = ""
		if len(txRawList) == 0 {
			q.CommitTxRaw = ""
			q.RevealInscribeFinalPsbtRaw = ""
			q.RevealMintFinalPsbtRaw = ""
		}
		q.InscribeState = InscribeStateFinish
		q.ConfirmationState = ConfirmationStateUnconfirmed
		result := tx.Where("id = ? AND version = ?", q.Id, q.Version-1).Save(q)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("mint order state conflict")
		}
		return nil
	})
}
