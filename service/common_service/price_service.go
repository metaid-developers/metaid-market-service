package common_service

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"metaid-market-service/models"
	"metaid-market-service/service/grpc_service"
	"metaid-market-service/service/mempool_space_service"
	"metaid-market-service/tool"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
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
	price, _ = FetchCoinPrice("btc")
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
	if marketPrice != nil && marketPrice.Usd > 0 {
		return fmt.Sprintf("%d", marketPrice.Usd)
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
	mexcDomain        = "https://api.mexc.com"
	mexcHTTPClient    = &http.Client{Timeout: 3 * time.Second}
	priceFreshTTL     = 60 * time.Second
	priceRetryBackoff = 30 * time.Second
	priceStaleTTL     = 30 * time.Minute
	priceNowFunc      = time.Now

	btcPriceMu       sync.Mutex
	btcLastGood      *BitcoinPrice
	btcFetchedAt     time.Time
	btcLastFailureAt time.Time
	btcRefreshing    bool
)

// FetchCoinPrice 统一获取币价，返回 BitcoinPrice 格式（USD 为重点字段）
// coin=btc 使用带缓存和降级的 BTC 价格源，其他币种通过 gRPC FetchCommonCoinPrice 获取
func FetchCoinPrice(coin string) (*BitcoinPrice, error) {
	if coin == "btc" {
		return fetchBtcCoinPrice()
	}
	conn, err := grpc_service.GetAssetBaseConn()
	if err != nil {
		return nil, err
	}
	resp, err := conn.FetchCommonCoinPrice(coin)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errors.New("grpc coin price response is empty")
	}
	nowTime := tool.MakeTimestamp()
	// gRPC 返回 PriceUsd 为 float64（美元），转为 int64 与 mempool 返回格式一致
	usd := int64(resp.GetPriceUsd())
	return &BitcoinPrice{
		Time: nowTime,
		USD:  usd,
	}, nil
}

func copyBitcoinPrice(price *BitcoinPrice) *BitcoinPrice {
	if price == nil {
		return nil
	}
	copy := *price
	return &copy
}

func staleBtcPriceLocked(now time.Time) *BitcoinPrice {
	if btcLastGood == nil || now.Sub(btcFetchedAt) > priceStaleTTL {
		return nil
	}
	return copyBitcoinPrice(btcLastGood)
}

func fetchBtcCoinPrice() (*BitcoinPrice, error) {
	now := priceNowFunc()
	btcPriceMu.Lock()
	if btcLastGood != nil && now.Sub(btcFetchedAt) < priceFreshTTL {
		price := copyBitcoinPrice(btcLastGood)
		btcPriceMu.Unlock()
		return price, nil
	}
	if btcRefreshing || (!btcLastFailureAt.IsZero() && now.Sub(btcLastFailureAt) < priceRetryBackoff) {
		price := staleBtcPriceLocked(now)
		btcPriceMu.Unlock()
		if price != nil {
			return price, nil
		}
		return nil, errors.New("btc price temporarily unavailable")
	}
	btcRefreshing = true
	btcPriceMu.Unlock()

	price, fetchErr := fetchBtcPriceUpstream()

	btcPriceMu.Lock()
	btcRefreshing = false
	if fetchErr == nil && price != nil {
		btcLastGood = copyBitcoinPrice(price)
		btcFetchedAt = priceNowFunc()
		btcLastFailureAt = time.Time{}
	} else {
		btcLastFailureAt = priceNowFunc()
	}
	stale := staleBtcPriceLocked(priceNowFunc())
	btcPriceMu.Unlock()

	if fetchErr == nil && price != nil {
		return price, nil
	}
	if stale != nil {
		return stale, nil
	}
	return nil, errors.New("btc price temporarily unavailable")
}

func fetchBtcPriceUpstream() (*BitcoinPrice, error) {
	price, err := fetchBtcPriceFromMexc()
	if err == nil && price != nil && price.USD > 0 {
		return price, nil
	}
	return GetPriceFormMempoolSpace()
}

func fetchBtcPriceFromMexc() (*BitcoinPrice, error) {
	url := strings.TrimRight(mexcDomain, "/") + "/api/v3/ticker/price?symbol=BTCUSDT"
	resp, err := mexcHTTPClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mexc price status %d", resp.StatusCode)
	}
	var data struct {
		Price string `json:"price"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	usdFloat, err := strconv.ParseFloat(data.Price, 64)
	if err != nil || usdFloat <= 0 {
		return nil, errors.New("mexc price invalid")
	}
	return &BitcoinPrice{
		Time: priceNowFunc().Unix(),
		USD:  int64(usdFloat),
	}, nil
}

func GetPriceFormMempoolSpace() (*BitcoinPrice, error) {
	result, status, err := mempool_space_service.Get("/api/v1/prices", mempool_space_service.Foreground)
	if err != nil {
		return nil, err
	}
	if status != 200 {
		return nil, fmt.Errorf("mempool.space prices status %d", status)
	}
	var data *BitcoinPrice
	if err = tool.JsonToObject(result, &data); err != nil {
		return nil, errors.New(fmt.Sprintf("Get request err:%s", err))
	}
	if data == nil || data.USD <= 0 {
		return nil, errors.New("mempool.space price invalid")
	}
	return data, nil
}

func CalMarketPrice(marketInfo *models.MarketMrc20InfoModel) (string, error) {
	var (
		orderList []*models.MarketMrc20OrderModel

		transactionList []Transaction = make([]Transaction, 0)
	)
	orderList, _ = models.MarketMrc20OrderModelDao().GetLastTenOrderListByTickId(&models.MarketMrc20OrderModel{
		TickId: marketInfo.TickId,
	})
	if len(orderList) > 0 {
		for _, v := range orderList {
			// 成交量用人类可读数量（AmountStr），旧数据缺失时按 decimals 从最小单位换算
			amountF, err := strconv.ParseFloat(v.AmountStr, 64)
			if err != nil || amountF <= 0 {
				amountF = float64(v.Amount) / math.Pow10(int(marketInfo.Decimals))
			}

			// DealTime 是毫秒时间戳（与 24h 查询窗口同一时钟），不能用 time.Unix 按秒解析
			timestamp := time.UnixMilli(v.DealTime)
			transactionList = append(transactionList, Transaction{
				Price:     v.TokenPriceRate,
				Amount:    amountF,
				Timestamp: timestamp,
			})
		}
	}

	priceParams := PriceParams{
		LatestPrice:  marketInfo.LastPrice,
		FloorPrice:   marketInfo.FloorPrice,
		Transactions: transactionList,
	}
	calPrice, err := CalculateCurrentPrice(priceParams)
	if err != nil {
		return "", err
	}

	fmt.Printf("[%s]24hTxCount:%d, LastPrice:%f, FloorPrice:%f, CalPrice:%f\n", marketInfo.Tick, len(orderList), marketInfo.LastPrice, marketInfo.FloorPrice, calPrice)

	if calPrice > 0 && calPrice < 1e-8 {
		return strconv.FormatFloat(calPrice, 'f', -1, 64), nil
	}
	return fmt.Sprintf("%.8f", calPrice), nil
}
