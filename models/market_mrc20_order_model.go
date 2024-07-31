package models

import (
	"errors"
	"fmt"
	"github.com/godaddy-x/freego/utils/decimal"
	"gorm.io/gorm"
	"metaid-market-service/major"
	"metaid-market-service/tool"
	"sync"
)

const (
	AssetTypeMrc20 = "mrc20"
)

type MarketMrc20OrderModel struct {
	Id                int64             `gorm:"column:id" json:"id"`
	OrderId           string            `gorm:"column:orderId" json:"orderId"`
	UtxoId            string            `gorm:"column:utxoId" json:"utxoId"`
	AssetType         AssetType         `gorm:"column:assetType" json:"assetType"`
	OutValue          int64             `gorm:"column:outValue" json:"outValue"`
	TickId            string            `gorm:"column:tickId" json:"tickId"`
	Tick              string            `gorm:"column:tick" json:"tick"`
	TokenName         string            `gorm:"column:tokenName" json:"tokenName"`
	Decimals          int64             `gorm:"column:decimals" json:"decimals"`
	Chain             string            `gorm:"column:chain" json:"chain"`
	Amount            int64             `gorm:"column:amount" json:"amount"`
	AmountStr         string            `gorm:"column:amountStr" json:"amountStr"`
	TokenPriceRate    float64           `gorm:"column:tokenPriceRate" json:"tokenPriceRate"`
	TokenPriceRateStr string            `gorm:"column:tokenPriceRateStr" json:"tokenPriceRateStr"`
	PriceAmount       int64             `gorm:"column:priceAmount" json:"priceAmount"`
	PriceDecimal      int64             `gorm:"column:priceDecimal" json:"priceDecimal"`
	PriceCoin         string            `gorm:"column:priceCoin" json:"priceCoin"`
	OrderState        OrderState        `gorm:"column:orderState" json:"orderState"`
	SellerAddress     string            `gorm:"column:sellerAddress" json:"sellerAddress"`
	SellerIp          string            `gorm:"column:sellerIp" json:"sellerIp"`
	BuyerAddress      string            `gorm:"column:buyerAddress" json:"buyerAddress"`
	BuyerIp           string            `gorm:"column:buyerIp" json:"buyerIp"`
	FeeAmount         int64             `gorm:"column:feeAmount" json:"feeAmount"`
	FeeRate           int64             `gorm:"column:feeRate" json:"feeRate"`
	MakerPsbt         string            `gorm:"column:makerPsbt" json:"makerPsbt"`
	TakerPsbt         string            `gorm:"column:takerPsbt" json:"takerPsbt"`
	FinalPsbt         string            `gorm:"column:finalPsbt" json:"finalPsbt"`
	TxId              string            `gorm:"column:txId" json:"txId"`
	DealTime          int64             `gorm:"column:dealTime" json:"dealTime"`
	BlockHeight       int64             `gorm:"column:blockHeight" json:"blockHeight"`
	ConfirmationState ConfirmationState `gorm:"column:confirmationState" json:"confirmationState"`
	Timestamp         int64             `gorm:"column:timestamp" json:"timestamp"`
	Version           int64             `gorm:"column:version" json:"version"`
	CreateTime        int64             `gorm:"column:createTime" json:"createTime"`
	UpdateTime        int64             `gorm:"column:updateTime" json:"updateTime"`
	State             int64             `gorm:"column:state" json:"state"`
}

func (MarketMrc20OrderModel) TableName() string {
	return "tb_market_mrc20_order"
}

var _marketMrc20OrderModelOnce sync.Once
var _marketMrc20OrderModelManager *marketMrc20OrderModelDao

type marketMrc20OrderModelDao struct {
}

func MarketMrc20OrderModelDao() *marketMrc20OrderModelDao {
	_marketMrc20OrderModelOnce.Do(func() {
		_marketMrc20OrderModelManager = &marketMrc20OrderModelDao{}
	})
	return _marketMrc20OrderModelManager
}

func (_ *marketMrc20OrderModelDao) Set(model *MarketMrc20OrderModel) error {
	if model == nil {
		return errors.New("model is nil")
	}
	tx := major.GetSqlDB().Create(model)
	if tx.Error != nil {
		return tx.Error
	}
	return nil
}
func (_ *marketMrc20OrderModelDao) SetForPushOrder(model *MarketMrc20OrderModel, supply string) error {
	err := major.GetSqlDB().Transaction(func(tx *gorm.DB) error {
		updateTime := tool.MakeTimestamp()
		if model == nil {
			return errors.New("model is nil")
		}
		if err := tx.Create(model).Error; err != nil {
			return err
		}

		floorPrice := float64(0)
		floorEntity, err := MarketMrc20OrderModelDao().GetMinTokenPriceRateByTickIdAndOrderId(&MarketMrc20OrderModel{TickId: model.TickId, OrderState: OrderStateCreate}, model.OrderId)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if floorEntity != nil {
			floorPrice = floorEntity.TokenPriceRate
		}
		if floorPrice == 0 || floorPrice > model.TokenPriceRate {
			floorPrice = model.TokenPriceRate
		}

		marketTickInfo, err := MarketMrc20InfoModelDao().GetOne(&MarketMrc20InfoModel{TickId: model.TickId})
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		if marketTickInfo != nil {
			marketCap := int64(0)
			if marketTickInfo.LastPrice != 0 {
				currentPriceDe := decimal.NewFromFloat(marketTickInfo.LastPrice)
				supplyDe, _ := decimal.NewFromString(supply)
				if supplyDe.GreaterThan(decimal.Zero) {
					marketCap = supplyDe.Mul(currentPriceDe).IntPart()
				}
				marketTickInfo.MarketCap = marketCap
			}

			marketTickInfo.FloorPrice = floorPrice
			sv := marketTickInfo.Version
			marketTickInfo.Version += 1
			marketTickInfo.UpdateTime = updateTime
			if err := tx.Save(marketTickInfo).Where(map[string]interface{}{"version": sv, "id": marketTickInfo.Id}).Error; err != nil {
				return err
			}
		} else {
			marketTickInfo = &MarketMrc20InfoModel{
				TickId:      model.TickId,
				Tick:        model.Tick,
				TokenName:   model.TokenName,
				Decimals:    model.Decimals,
				Chain:       model.Chain,
				Supply:      "0",
				TotalVolume: 0,
				MarketCap:   0,
				LastPrice:   0,
				FloorPrice:  floorPrice,
				Change24H:   0,
				Timestamp:   updateTime,
				//Version:     0,
				CreateTime: updateTime,
				//UpdateTime:  0,
				State: STATE_EXIST,
			}
			if err := tx.Create(marketTickInfo).Error; err != nil {
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

func (_ *marketMrc20OrderModelDao) GetOne(qo *MarketMrc20OrderModel) (*MarketMrc20OrderModel, error) {
	model := &MarketMrc20OrderModel{}
	tx := major.GetSqlDB().Where(qo).First(model)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return model, nil
}

func (_ *marketMrc20OrderModelDao) GetLastOne(qo *MarketMrc20OrderModel) (*MarketMrc20OrderModel, error) {
	model := &MarketMrc20OrderModel{}
	tx := major.GetSqlDB().Where(qo).Order("blockHeight desc").First(model)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return model, nil
}

func (_ *marketMrc20OrderModelDao) GetList(qo *MarketMrc20OrderModel, offset, limit int64) ([]*MarketMrc20OrderModel, error) {
	var models []*MarketMrc20OrderModel
	tx := major.GetSqlDB().Where(qo).Limit(int(limit)).Offset(int(offset)).Order("timestamp asc").Find(&models)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return models, nil
}

func (_ *marketMrc20OrderModelDao) CountByState(qo *MarketMrc20OrderModel, address string) (int64, error) {
	var count int64
	filter := ""
	if qo.OrderState == OrderStateFinish {
		qo.SellerAddress = ""
		if address != "" {
			filter = fmt.Sprintf("sellerAddress = '%s' or buyerAddress = '%s'", address, address)
		}
	}
	tx := major.GetSqlDB().Model(&MarketMrc20OrderModel{}).Where(qo).Where(filter).Count(&count)
	if tx.Error != nil {
		return 0, tx.Error
	}
	return count, nil
}

func (_ *marketMrc20OrderModelDao) GetListByState(qo *MarketMrc20OrderModel, address string, offset, limit int64, sortKey, sortType string) ([]*MarketMrc20OrderModel, error) {
	var models []*MarketMrc20OrderModel
	filter := ""
	if qo.OrderState == OrderStateFinish {
		qo.SellerAddress = ""
		if address != "" {
			filter = fmt.Sprintf("sellerAddress = '%s' or buyerAddress = '%s'", address, address)
		}
	}
	tx := major.GetSqlDB().Where(qo).Where(filter).Limit(int(limit)).Offset(int(offset)).Order(fmt.Sprintf("%s %s", sortKey, sortType)).Find(&models)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return models, nil
}

func (_ *marketMrc20OrderModelDao) GetMinTokenPriceRateByTickId(qo *MarketMrc20OrderModel) (*MarketMrc20OrderModel, error) {
	model := &MarketMrc20OrderModel{}
	tx := major.GetSqlDB().Where(qo).Order("tokenPriceRate asc").First(model)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return model, nil
}

func (_ *marketMrc20OrderModelDao) GetMinTokenPriceRateByTickIdAndOrderId(qo *MarketMrc20OrderModel, orderId string) (*MarketMrc20OrderModel, error) {
	model := &MarketMrc20OrderModel{}
	tx := major.GetSqlDB().Where(qo).Not("orderId", orderId).Order("tokenPriceRate asc").First(model)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return model, nil
}

func (_ *marketMrc20OrderModelDao) SumPriceAmountByTickId(qo *MarketMrc20OrderModel) (int64, error) {
	var total int64
	tx := major.GetSqlDB().Model(&MarketMrc20OrderModel{}).Where(qo).Select("SUM(priceAmount) as total").Scan(&total)
	if tx.Error != nil {
		return 0, nil
	}
	return total, nil
}

func (_ *marketMrc20OrderModelDao) Count(qo *MarketMrc20OrderModel) (int64, error) {
	var count int64
	filter := ""
	tx := major.GetSqlDB().Model(&MarketMrc20OrderModel{}).Where(qo).Where(filter).Count(&count)
	if tx.Error != nil {
		return 0, tx.Error
	}
	return count, nil
}

func (_ *marketMrc20OrderModelDao) GetAll(qo *MarketMrc20OrderModel) ([]MarketMrc20OrderModel, error) {
	var models []MarketMrc20OrderModel
	tx := major.GetSqlDB().Where(qo).Find(&models)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return models, nil
}

func (_ *marketMrc20OrderModelDao) Update(q *MarketMrc20OrderModel) error {
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

func (_ *marketMrc20OrderModelDao) UpdateForPushAndCancel(q *MarketMrc20OrderModel, supply string) error {
	err := major.GetSqlDB().Transaction(func(tx *gorm.DB) error {
		if q == nil {
			return errors.New("model is nil")
		}
		updateTime := tool.MakeTimestamp()
		sv := q.Version
		q.Version += 1
		q.UpdateTime = updateTime
		if err := tx.Save(q).Where(map[string]interface{}{"version": sv, "id": q.Id}).Error; err != nil {
			return err
		}

		floorPrice := float64(0)
		floorEntity, err := MarketMrc20OrderModelDao().GetMinTokenPriceRateByTickIdAndOrderId(&MarketMrc20OrderModel{TickId: q.TickId, OrderState: OrderStateCreate}, q.OrderId)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if floorEntity != nil {
			floorPrice = floorEntity.TokenPriceRate
		}
		if q.OrderState == OrderStateCreate && (floorPrice == 0 || floorPrice > q.TokenPriceRate) {
			floorPrice = q.TokenPriceRate
		}

		marketTickInfo, err := MarketMrc20InfoModelDao().GetOne(&MarketMrc20InfoModel{TickId: q.TickId})
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		if marketTickInfo != nil {
			marketCap := int64(0)
			if marketTickInfo.LastPrice != 0 {
				currentPriceDe := decimal.NewFromFloat(marketTickInfo.LastPrice)
				supplyDe, _ := decimal.NewFromString(supply)
				if supplyDe.GreaterThan(decimal.Zero) {
					marketCap = supplyDe.Mul(currentPriceDe).IntPart()
				}
				marketTickInfo.MarketCap = marketCap
			}

			marketTickInfo.Supply = supply

			marketTickInfo.FloorPrice = floorPrice
			sv := marketTickInfo.Version
			marketTickInfo.Version += 1
			marketTickInfo.UpdateTime = updateTime
			if err := tx.Save(marketTickInfo).Where(map[string]interface{}{"version": sv, "id": marketTickInfo.Id}).Error; err != nil {
				return err
			}
		} else {
			marketTickInfo = &MarketMrc20InfoModel{
				TickId:      q.TickId,
				Tick:        q.Tick,
				TokenName:   q.TokenName,
				Decimals:    q.Decimals,
				Chain:       q.Chain,
				Supply:      "0",
				TotalVolume: 0,
				MarketCap:   0,
				LastPrice:   0,
				FloorPrice:  floorPrice,
				Change24H:   0,
				Timestamp:   updateTime,
				//Version:     0,
				CreateTime: updateTime,
				//UpdateTime:  0,
				State: STATE_EXIST,
			}
			if err := tx.Create(marketTickInfo).Error; err != nil {
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

func (_ *marketMrc20OrderModelDao) BatchSaveList(list []*MarketMrc20OrderModel) error {
	err := major.GetSqlDB().Transaction(func(tx *gorm.DB) error {
		for _, model := range list {
			if model.Id == 0 {
				if err := tx.Create(model).Error; err != nil {
					return err
				}
			} else {
				sv := model.Version
				model.Version += 1
				model.UpdateTime = tool.MakeTimestamp()
				if err := tx.Save(model).Where(map[string]interface{}{"version": sv, "id": model.Id}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (_ *marketMrc20OrderModelDao) UpdateOrderEntityListForJobFunc(q *MarketMrc20OrderModel, usedUtxoList, newUtxoList []*MarketUtxoModel, supply string, txRaw string, jobFunc func(txRaw string) (string, error)) error {
	err := major.GetSqlDB().Transaction(func(tx *gorm.DB) error {
		nowTime := tool.MakeTimestamp()

		updateTime := nowTime
		q.DealTime = nowTime
		sv := q.Version
		q.Version += 1
		q.UpdateTime = updateTime
		q.TakerPsbt = ""
		q.MakerPsbt = ""
		q.FinalPsbt = ""
		if err := tx.Save(q).Where(map[string]interface{}{"version": sv, "id": q.Id}).Error; err != nil {
			return err
		}
		for _, utxo := range usedUtxoList {
			av := utxo.Version
			utxo.Version += 1
			utxo.UpdateTime = updateTime
			if err := tx.Save(utxo).Where(map[string]interface{}{"version": av, "id": utxo.Id}).Error; err != nil {
				return err
			}
		}
		for _, utxo := range newUtxoList {
			utxo.Timestamp = nowTime
			utxo.CreateTime = nowTime
			if err := tx.Create(utxo).Error; err != nil {
				return err
			}
		}

		currentPrice := q.TokenPriceRate
		change24H := int64(0)
		marketCap := int64(0)
		floorPrice := float64(0)
		floorEntity, err := MarketMrc20OrderModelDao().GetMinTokenPriceRateByTickIdAndOrderId(&MarketMrc20OrderModel{TickId: q.TickId, OrderState: OrderStateCreate}, q.OrderId)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if floorEntity != nil {
			floorPrice = floorEntity.TokenPriceRate
		}

		marketTickInfo, err := MarketMrc20InfoModelDao().GetOne(&MarketMrc20InfoModel{TickId: q.TickId})
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		currentPriceDe := decimal.NewFromFloat(currentPrice)
		supplyDe, _ := decimal.NewFromString(supply)
		if supplyDe.GreaterThan(decimal.Zero) {
			marketCap = supplyDe.Mul(currentPriceDe).IntPart()
		}

		totalVolume, _ := MarketMrc20OrderModelDao().SumPriceAmountByTickId(&MarketMrc20OrderModel{TickId: q.TickId, OrderState: OrderStateFinish})

		totalVolume += q.PriceAmount
		if marketTickInfo != nil {
			lastPrice := marketTickInfo.LastPrice
			if lastPrice != 0 {
				lastPriceDe := decimal.NewFromFloat(lastPrice)
				change24HDe := currentPriceDe.Sub(lastPriceDe).Div(lastPriceDe).Mul(decimal.New(100, 0))
				if currentPriceDe.Sub(lastPriceDe).GreaterThan(decimal.Zero) {
					change24HDe = change24HDe.Neg()
				}
				change24H = change24HDe.IntPart()
			} else {
				change24H = 0
			}
			if totalVolume != 0 {
				marketTickInfo.TotalVolume = totalVolume
			}
			marketTickInfo.LastPrice = currentPrice
			marketTickInfo.FloorPrice = floorPrice
			marketTickInfo.Supply = supply
			marketTickInfo.MarketCap = marketCap
			marketTickInfo.Change24H = change24H
			sv := marketTickInfo.Version
			marketTickInfo.Version += 1
			marketTickInfo.UpdateTime = updateTime
			if err := tx.Save(marketTickInfo).Where(map[string]interface{}{"version": sv, "id": marketTickInfo.Id}).Error; err != nil {
				return err
			}
		} else {
			marketTickInfo = &MarketMrc20InfoModel{
				TickId:      q.TickId,
				Tick:        q.Tick,
				TokenName:   q.TokenName,
				Decimals:    q.Decimals,
				Chain:       q.Chain,
				Supply:      supply,
				TotalVolume: totalVolume,
				MarketCap:   marketCap,
				LastPrice:   currentPrice,
				FloorPrice:  floorPrice,
				Change24H:   10000,
				Timestamp:   updateTime,
				//Version:     0,
				CreateTime: updateTime,
				//UpdateTime:  0,
				State: STATE_EXIST,
			}
			if err := tx.Create(marketTickInfo).Error; err != nil {
				return err
			}
		}

		_, err = jobFunc(txRaw)
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

func (_ *marketMrc20OrderModelDao) BatchUpdateOrderEntityListForJobFunc(qList []*MarketMrc20OrderModel, usedUtxoList, newUtxoList []*MarketUtxoModel, txRaw string, jobFunc func(txRaw string) (string, error)) error {
	err := major.GetSqlDB().Transaction(func(tx *gorm.DB) error {
		nowTime := tool.MakeTimestamp()

		updateTime := nowTime
		for _, q := range qList {
			q.DealTime = nowTime
			sv := q.Version
			q.Version += 1
			q.UpdateTime = updateTime
			q.TakerPsbt = ""
			q.MakerPsbt = ""
			q.FinalPsbt = ""
			if err := tx.Save(q).Where(map[string]interface{}{"version": sv, "id": q.Id}).Error; err != nil {
				return err
			}
		}
		for _, utxo := range usedUtxoList {
			av := utxo.Version
			utxo.Version += 1
			utxo.UpdateTime = updateTime
			if err := tx.Save(utxo).Where(map[string]interface{}{"version": av, "id": utxo.Id}).Error; err != nil {
				return err
			}
		}
		for _, utxo := range newUtxoList {
			utxo.Timestamp = nowTime
			utxo.CreateTime = nowTime
			if err := tx.Create(utxo).Error; err != nil {
				return err
			}
		}

		_, err := jobFunc(txRaw)
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
