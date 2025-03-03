package common_service

import (
	"sort"
	"time"
)

// Transaction
type Transaction struct {
	Price     float64   //
	Amount    float64   //
	Timestamp time.Time //
}

// PriceParams
type PriceParams struct {
	LatestPrice  float64       //
	FloorPrice   float64       //
	Transactions []Transaction //
}

// CalculateCurrentPrice
func CalculateCurrentPrice(p PriceParams) (float64, error) {
	// 异常情况处理：无交易数据时返回地板价
	if len(p.Transactions) == 0 {
		return p.FloorPrice, nil
	}

	// 步骤1：计算24小时交易量权重
	volume24h := calculate24hVolume(p.Transactions)
	volumeWeight := volume24h / 10 // 假设10为基准交易量
	if volumeWeight > 1 {
		volumeWeight = 1
	}

	// 步骤2：时间衰减因子（最新交易时间权重）
	timeDecay := calculateTimeDecay(p.Transactions)

	// 步骤3：动态权重计算
	latestWeight := volumeWeight * timeDecay
	floorWeight := 1 - latestWeight

	// 步骤4：异常值过滤（最新价偏离中位数20%以上时使用中位数）
	filteredLatest := filterOutliers(p.LatestPrice, p.Transactions)

	// 步骤5：加权计算
	currentPrice := filteredLatest*latestWeight + p.FloorPrice*floorWeight

	// 地板价保护：不低于地板价的90%
	if currentPrice < p.FloorPrice*0.9 {
		currentPrice = p.FloorPrice * 0.9
	}

	return currentPrice, nil
}

// calculate24hVolume 计算24小时交易量
func calculate24hVolume(txs []Transaction) float64 {
	var volume float64
	now := time.Now()

	for _, tx := range txs {
		if now.Sub(tx.Timestamp) <= 24*time.Hour {
			volume += tx.Amount
		}
	}
	return volume
}

// calculateTimeDecay 计算时间衰减因子
// 最新交易时间越近，衰减越小（权重越高）
func calculateTimeDecay(txs []Transaction) float64 {
	if len(txs) == 0 {
		return 0
	}

	// 获取最新交易时间
	latestTime := txs[0].Timestamp
	for _, tx := range txs {
		if tx.Timestamp.After(latestTime) {
			latestTime = tx.Timestamp
		}
	}

	// 计算时间衰减（每小时衰减3%）
	hoursPassed := time.Since(latestTime).Hours()
	decay := 0.7 * (1 - 0.03*hoursPassed) // 基础衰减系数0.7
	if decay < 0.1 {
		decay = 0.1 // 最小衰减系数
	}
	return decay
}

// filterOutliers 异常值过滤（使用中位数法）
func filterOutliers(latest float64, txs []Transaction) float64 {
	// 提取最近10笔交易价格
	var prices []float64
	for i := len(txs) - 1; i >= 0 && len(prices) < 10; i-- {
		prices = append(prices, txs[i].Price)
	}

	// 计算中位数
	sort.Float64s(prices)
	median := median(prices)

	// 偏离超过20%时使用中位数
	if abs(latest-median)/median > 0.2 {
		return median
	}
	return latest
}

// median 计算中位数
func median(prices []float64) float64 {
	n := len(prices)
	if n == 0 {
		return 0
	}
	if n%2 == 0 {
		return (prices[n/2-1] + prices[n/2]) / 2
	}
	return prices[n/2]
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
