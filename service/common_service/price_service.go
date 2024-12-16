package common_service

import (
	"errors"
	"fmt"
	"metaid-market-service/models"
	"metaid-market-service/tool"
)

func GetPriceForUsd(coin string) string {
	var (
		price       *BitcoinPrice
		marketPrice *models.MarketPriceModel
		nowTime     int64 = tool.MakeTimestamp()
	)
	marketPrice, _ = models.MarketPriceModelDao().GetOne(&models.MarketPriceModel{
		Coin: coin,
	})
	if marketPrice != nil && (nowTime-marketPrice.UpdateTime) <= 5*60*1000 {
		return fmt.Sprintf("%d", marketPrice.Usd)
	}
	price, _ = GetPriceFormMempoolSpace()
	if price != nil {
		if marketPrice != nil {
			marketPrice.Usd = price.USD
			models.MarketPriceModelDao().Update(marketPrice)
		} else {
			models.MarketPriceModelDao().Set(&models.MarketPriceModel{
				Coin:       coin,
				Usd:        price.USD,
				Timestamp:  price.Time,
				CreateTime: nowTime,
				State:      1,
			})
		}
		return fmt.Sprintf("%d", price.USD)
	}
	return "0"
}

type BitcoinPrice struct {
	Time int64 `json:"time"`
	USD  int64 `json:"USD"`
	EUR  int64 `json:"EUR"`
	GBP  int64 `json:"GBP"`
	CAD  int64 `json:"CAD"`
	CHF  int64 `json:"CHF"`
	AUD  int64 `json:"AUD"`
	JPY  int64 `json:"JPY"`
}

var (
	cachedPrice *BitcoinPrice
	cachedTime  int64 = 0
)

func GetPriceFormMempoolSpace() (*BitcoinPrice, error) {
	var (
		url    string = "https://mempool.space/api/v1/prices"
		result string
		data   *BitcoinPrice
		err    error
	)
	if cachedPrice != nil && (tool.MakeTimestamp()-cachedTime) <= 5*60*1000 {
		return cachedPrice, nil
	}

	result, err = tool.GetUrl(url, nil, nil)
	if err != nil {
		return nil, errors.New("request err")
	}

	//fmt.Println(result)
	if err = tool.JsonToObject(result, &data); err != nil {
		return nil, errors.New(fmt.Sprintf("Get request err:%s", err))
	}
	cachedPrice = data
	cachedTime = tool.MakeTimestamp()
	return data, nil
}
