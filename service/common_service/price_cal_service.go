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

	// 交易少于3笔时，降低权重系数，防止单笔交易操纵价格
	if len(p.Transactions) < 3 {
		volumeWeight = volumeWeight * 0.2
	}

	// 步骤2：时间衰减因子（最新交易时间权重）
	timeDecay := calculateTimeDecay(p.Transactions)

	// 步骤3：动态权重计算
	latestWeight := volumeWeight * timeDecay
	floorWeight := 1 - latestWeight

	// 步骤4：异常值过滤（使用增强版过滤逻辑）
	filteredLatest := enhancedFilterOutliers(p.LatestPrice, p.Transactions, p.FloorPrice)

	// 步骤5：加权计算
	currentPrice := filteredLatest*latestWeight + p.FloorPrice*floorWeight

	// 地板价保护：不低于地板价的90%
	if currentPrice < p.FloorPrice*0.9 {
		currentPrice = p.FloorPrice * 0.9
	}

	// 价格上限保护：增加最大涨幅限制，防止价格过度飙升
	// 如果计算的价格超过地板价的10倍，则将其限制在地板价的10倍以内
	if currentPrice > p.FloorPrice*10 {
		currentPrice = p.FloorPrice * 10
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

// 增强版异常值过滤，同时考虑地板价
func enhancedFilterOutliers(latest float64, txs []Transaction, floorPrice float64) float64 {
	// 如果最新价格是0，使用最新交易价格或地板价
	if latest == 0 {
		if len(txs) > 0 {
			return txs[len(txs)-1].Price
		}
		return floorPrice
	}

	// 提取交易价格
	var prices []float64
	for _, tx := range txs {
		// 过滤掉明显异常的价格（比如超过地板价100倍的价格）
		if tx.Price <= floorPrice*100 {
			prices = append(prices, tx.Price)
		}
	}

	// 计算中位数
	sort.Float64s(prices)
	medianPrice := median(prices)
	if medianPrice == 0 {
		medianPrice = floorPrice
	}

	// 如果最新价格偏离中位数或地板价过大，则使用中位数
	if abs(latest-medianPrice)/max(medianPrice, 1) > 0.5 || latest > floorPrice*20 {
		return medianPrice
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

// max 获取两个数的较大值
func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
