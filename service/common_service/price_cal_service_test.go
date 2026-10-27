package common_service

import (
	"math"
	"testing"
	"time"
)

func recentTx(price, amount float64, hoursAgo float64) Transaction {
	return Transaction{
		Price:     price,
		Amount:    amount,
		Timestamp: time.Now().Add(-time.Duration(hoursAgo * float64(time.Hour))),
	}
}

// 无交易数据时应直接返回地板价
func TestCalculateCurrentPriceNoTransactions(t *testing.T) {
	price, err := CalculateCurrentPrice(PriceParams{
		LatestPrice: 0,
		FloorPrice:  5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if price != 5 {
		t.Fatalf("want floor price 5, got %f", price)
	}
}

// 地板价缺失（无活跃挂单）时应回退为成交价中位数，而不是以 0 为锚
func TestCalculateCurrentPriceMissingFloorUsesMedian(t *testing.T) {
	price, err := CalculateCurrentPrice(PriceParams{
		LatestPrice: 10,
		FloorPrice:  0,
		Transactions: []Transaction{
			recentTx(2, 100, 1),
			recentTx(3, 100, 2),
			recentTx(10, 100, 3),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(price-3) > 1e-9 {
		t.Fatalf("want median 3, got %f", price)
	}
}

// 单笔高价小成交不应明显拉高价格：地板价应占绝对主导
func TestCalculateCurrentPriceSingleTradeFloorDominated(t *testing.T) {
	price, err := CalculateCurrentPrice(PriceParams{
		LatestPrice: 5,
		FloorPrice:  1,
		Transactions: []Transaction{
			recentTx(5, 50, 1),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if price >= 1.5 {
		t.Fatalf("single trade should not inflate price much, want < 1.5, got %f", price)
	}
}

// 极端高价成交即使权重很小，也会被上限项圈限制在地板价 3 倍以内
func TestCalculateCurrentPriceExtremeOutlierCollared(t *testing.T) {
	price, err := CalculateCurrentPrice(PriceParams{
		LatestPrice: 100,
		FloorPrice:  1,
		Transactions: []Transaction{
			recentTx(100, 50, 1),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(price-3) > 1e-9 {
		t.Fatalf("want collar cap 3, got %f", price)
	}
}

// 即使最新成交价远超地板价，上限项圈也应把价格限制在地板价 3 倍以内
func TestCalculateCurrentPriceCollarCapsAtFloorTimes3(t *testing.T) {
	txs := make([]Transaction, 0, 5)
	for i := 0; i < 5; i++ {
		txs = append(txs, recentTx(100, 500, 0.1))
	}
	price, err := CalculateCurrentPrice(PriceParams{
		LatestPrice:  100,
		FloorPrice:   1,
		Transactions: txs,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(price-3) > 1e-9 {
		t.Fatalf("want collar cap 3, got %f", price)
	}
}

// 交易活跃且价格正常时，价格应处于地板价与加权最新价之间
func TestCalculateCurrentPriceActiveMarketBlend(t *testing.T) {
	txs := make([]Transaction, 0, 5)
	for i := 0; i < 5; i++ {
		txs = append(txs, recentTx(2, 100, float64(i+1)))
	}
	price, err := CalculateCurrentPrice(PriceParams{
		LatestPrice:  2,
		FloorPrice:   1,
		Transactions: txs,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if price <= 1 || price >= 2 {
		t.Fatalf("want price between floor 1 and latest 2, got %f", price)
	}
}

// 成交量应按人类可读数量计算：换算后的量级才符合 volumeWeight 的基准假设
func TestCalculate24hVolumeHumanUnits(t *testing.T) {
	volume := calculate24hVolume([]Transaction{
		recentTx(1, 10, 1),
		recentTx(1, 20, 2),
		recentTx(1, 5, 48), // 超过 24h，不计入
	})
	if volume != 30 {
		t.Fatalf("want volume 30, got %f", volume)
	}
}
