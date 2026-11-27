package orderbook_service

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/shopspring/decimal"
)

const (
	DefaultMaxAskLegs = 4
	DefaultMaxBidLegs = 2
)

// RouteRequest defines a Taker quote or sweep routing request.
type RouteRequest struct {
	PairKey          string `json:"pairKey"`
	TokenCodeHash    string `json:"tokenCodeHash"`
	TokenID          string `json:"tokenID"`
	Side             string `json:"side"` // "buy" (take Ask) or "sell" (take Bid)
	TargetToken      uint64 `json:"targetToken"`
	MaxPriceNum      uint64 `json:"maxPriceNum,omitempty"` // optional limit for buy
	MaxPriceDen      uint64 `json:"maxPriceDen,omitempty"`
	MinPriceNum      uint64 `json:"minPriceNum,omitempty"` // optional limit for sell
	MinPriceDen      uint64 `json:"minPriceDen,omitempty"`
	MaxLegs          int    `json:"maxLegs,omitempty"`
	ExcludeOutpoints []string `json:"excludeOutpoints,omitempty"`
}

// PlannedLeg is a single order UTXO fill selected by the Smart Order Router.
type PlannedLeg struct {
	Outpoint      string `json:"outpoint"`
	OrderTxid     string `json:"orderTxid"`
	OrderVout     int    `json:"orderVout"`
	TokenTxid     string `json:"tokenTxid,omitempty"`
	TokenVout     int    `json:"tokenVout,omitempty"`
	MakerAddr     string `json:"makerAddr"`
	PriceNum      uint64 `json:"priceNum"`
	PriceDen      uint64 `json:"priceDen"`
	Price         string `json:"price"`
	FillToken     uint64 `json:"fillToken"`
	SatsAmount    uint64 `json:"satsAmount"` // Ask: grossSats paid to maker; Bid: spendSats spent from bid
	FeeSats       uint64 `json:"feeSats"`
	NetTakerSats  uint64 `json:"netTakerSats"` // Ask: grossSats + feeSats paid; Bid: spendSats - feeSats received (before miner fee)
	IsPartial     bool   `json:"isPartial"`
	RemainAfter   uint64 `json:"remainAfter"` // Ask: remain token; Bid: remain sats
}

// RoutePlan is the Smart Order Router output passed directly to /take/quote and sidecar prepare.
type RoutePlan struct {
	PairKey           string       `json:"pairKey"`
	Side              string       `json:"side"` // "buy" or "sell"
	RequestedToken    uint64       `json:"requestedToken"`
	FilledToken       uint64       `json:"filledToken"`
	UnfilledToken     uint64       `json:"unfilledToken"`
	TotalSats         uint64       `json:"totalSats"`
	TotalFeeSats      uint64       `json:"totalFeeSats"`
	TotalTakerSats    uint64       `json:"totalTakerSats"`
	AvgPrice          string       `json:"avgPrice"`
	EstimatedMinerFee uint64       `json:"estimatedMinerFee"`
	Legs              []PlannedLeg `json:"legs"`
}

// RouteTake computes the optimal multi-UTXO sweep plan following the spec priority:
//   1. Best price (Ask lowest first; Bid highest first)
//   2. Fewest legs / largest single-order capacity at the same price
//   3. Time priority (older CreatedAt first)
// Respects Redis/memory soft-locks, minFillAmount, and Bid zombie-dust guard.
func (b *OrderBook) RouteTake(req RouteRequest) (*RoutePlan, error) {
	side := strings.ToLower(strings.TrimSpace(req.Side))
	if side != "buy" && side != "sell" {
		return nil, errors.New("side must be 'buy' or 'sell'")
	}
	if req.TargetToken == 0 {
		return nil, errors.New("targetToken must be > 0")
	}
	pairKey := strings.ToLower(strings.TrimSpace(req.PairKey))
	if pairKey == "" && req.TokenCodeHash != "" && req.TokenID != "" {
		pairKey = MakePairKey(req.TokenCodeHash, req.TokenID)
	}
	if pairKey == "" {
		return nil, errors.New("pairKey (or tokenCodeHash + tokenID) is required")
	}

	targetOrderSide := SideAsk
	maxLegs := req.MaxLegs
	if side == "buy" {
		targetOrderSide = SideAsk
		if maxLegs <= 0 {
			maxLegs = DefaultMaxAskLegs
		}
	} else {
		targetOrderSide = SideBid
		if maxLegs <= 0 {
			maxLegs = DefaultMaxBidLegs
		}
	}

	excludeSet := make(map[string]struct{}, len(req.ExcludeOutpoints))
	for _, op := range req.ExcludeOutpoints {
		excludeSet[strings.ToLower(strings.TrimSpace(op))] = struct{}{}
	}

	// Snapshot candidate orders
	b.mu.RLock()
	pm := b.byPair[pairKey]
	cands := make([]*LiveOrder, 0, len(pm))
	for _, o := range pm {
		if o.Side != targetOrderSide || o.RemainToken == 0 {
			continue
		}
		if _, skip := excludeSet[o.Outpoint]; skip {
			continue
		}
		if b.locker.IsLocked(o.Outpoint) {
			continue
		}
		// Optional price limit filter
		if side == "buy" && req.MaxPriceNum > 0 && req.MaxPriceDen > 0 {
			if ComparePrice(o.PriceNum, o.PriceDen, req.MaxPriceNum, req.MaxPriceDen) > 0 {
				continue
			}
		}
		if side == "sell" && req.MinPriceNum > 0 && req.MinPriceDen > 0 {
			if ComparePrice(o.PriceNum, o.PriceDen, req.MinPriceNum, req.MinPriceDen) < 0 {
				continue
			}
		}
		cp := *o
		cands = append(cands, &cp)
	}
	b.mu.RUnlock()

	if len(cands) == 0 {
		return nil, fmt.Errorf("no available %s liquidity for pair %s", side, pairKey)
	}

	// Sort: 1) Price priority -> 2) Larger capacity (fewest legs) -> 3) Time priority (older first)
	sort.Slice(cands, func(i, j int) bool {
		cmp := ComparePrice(cands[i].PriceNum, cands[i].PriceDen, cands[j].PriceNum, cands[j].PriceDen)
		if cmp != 0 {
			if targetOrderSide == SideAsk {
				return cmp < 0
			}
			return cmp > 0
		}
		if cands[i].RemainToken != cands[j].RemainToken {
			return cands[i].RemainToken > cands[j].RemainToken
		}
		if cands[i].CreatedAt != cands[j].CreatedAt {
			return cands[i].CreatedAt < cands[j].CreatedAt
		}
		return cands[i].Outpoint < cands[j].Outpoint
	})

	remaining := req.TargetToken
	legs := make([]PlannedLeg, 0, maxLegs)
	var totalFilled, totalSats, totalFee, totalTaker uint64

	for _, o := range cands {
		if remaining == 0 || len(legs) >= maxLegs {
			break
		}
		leg, ok := planOrderLeg(o, remaining)
		if !ok {
			continue
		}
		legs = append(legs, leg)
		remaining -= leg.FillToken
		totalFilled += leg.FillToken
		totalSats += leg.SatsAmount
		totalFee += leg.FeeSats
		totalTaker += leg.NetTakerSats
	}

	if len(legs) == 0 {
		return nil, fmt.Errorf("no fillable orders satisfy minFill/dust rules for targetToken=%d", req.TargetToken)
	}

	avgPrice := decimal.Zero
	if totalFilled > 0 {
		avgPrice = decimal.NewFromInt(int64(totalSats)).DivRound(decimal.NewFromInt(int64(totalFilled)), 12)
	}

	var estMinerFee uint64
	if side == "buy" {
		estMinerFee = uint64(3000 + len(legs)*12000)
	} else {
		estMinerFee = uint64(11000 + len(legs)*11000)
	}

	return &RoutePlan{
		PairKey:           pairKey,
		Side:              side,
		RequestedToken:    req.TargetToken,
		FilledToken:       totalFilled,
		UnfilledToken:     remaining,
		TotalSats:         totalSats,
		TotalFeeSats:      totalFee,
		TotalTakerSats:    totalTaker,
		AvgPrice:          avgPrice.String(),
		EstimatedMinerFee: estMinerFee,
		Legs:              legs,
	}, nil
}

// planOrderLeg determines the maximum contract-valid fillToken <= wantToken for one order UTXO.
func planOrderLeg(o *LiveOrder, wantToken uint64) (PlannedLeg, bool) {
	if o.Side == SideAsk {
		fill := wantToken
		if fill > o.RemainToken {
			fill = o.RemainToken
		}
		// Partial fill rules in OrderAskV2:
		//   1) fill >= minFill
		//   2) remainNext == 0 || remainNext >= minFill
		if fill < o.RemainToken {
			if fill < o.MinFillAmount {
				return PlannedLeg{}, false
			}
			remainNext := o.RemainToken - fill
			if remainNext < o.MinFillAmount {
				// Reduce fill so remainNext == minFill if still >= minFill
				if o.RemainToken >= 2*o.MinFillAmount {
					fill = o.RemainToken - o.MinFillAmount
				} else {
					// Cannot partially fill without leaving zombie dust; skip unless caller wants full order
					return PlannedLeg{}, false
				}
			}
		}
		gross := CalcAskGrossSats(fill, o.PriceNum, o.PriceDen)
		if gross < DustLimit {
			return PlannedLeg{}, false
		}
		fee := CalcFeeSats(gross, o.FeeBps)
		if o.FeeAddr == "" || fee < DustLimit {
			fee = 0
		}
		remainAfter := o.RemainToken - fill
		return PlannedLeg{
			Outpoint:     o.Outpoint,
			OrderTxid:    o.Txid,
			OrderVout:    o.Vout,
			TokenTxid:    o.TokenTxid,
			TokenVout:    o.TokenVout,
			MakerAddr:    o.MakerAddr,
			PriceNum:     o.PriceNum,
			PriceDen:     o.PriceDen,
			Price:        o.Price,
			FillToken:    fill,
			SatsAmount:   gross,
			FeeSats:      fee,
			NetTakerSats: gross + fee,
			IsPartial:    remainAfter > 0,
			RemainAfter:  remainAfter,
		}, true
	}

	// SideBid:
	// spendSats = floor(fill * priceNum / priceDen) <= o.RemainSats
	// If remainSatsNext > 0:
	//   fill >= minFill && remainSatsNext >= 546 && floor(remainSatsNext * priceDen / priceNum) >= minFill
	maxTok := CalcBidMaxToken(o.RemainSats, o.PriceNum, o.PriceDen)
	if maxTok == 0 {
		return PlannedLeg{}, false
	}
	fill := wantToken
	if fill > maxTok {
		fill = maxTok
	}
	spend := CalcBidSpendSats(fill, o.PriceNum, o.PriceDen)
	if spend == 0 || spend > o.RemainSats {
		return PlannedLeg{}, false
	}
	remainSatsNext := o.RemainSats - spend
	if remainSatsNext > 0 {
		// Check partial fill validity
		if fill < o.MinFillAmount || remainSatsNext < DustLimit || CalcBidMaxToken(remainSatsNext, o.PriceNum, o.PriceDen) < o.MinFillAmount {
			// Try clamping fill down so remaining SPACE can still afford minFill tokens
			minRemainSats := CalcAskGrossSats(o.MinFillAmount, o.PriceNum, o.PriceDen)
			if minRemainSats < DustLimit {
				minRemainSats = DustLimit
			}
			if o.RemainSats <= minRemainSats {
				return PlannedLeg{}, false
			}
			maxPartialSpend := o.RemainSats - minRemainSats
			clampedFill := CalcBidMaxToken(maxPartialSpend, o.PriceNum, o.PriceDen)
			if clampedFill < o.MinFillAmount || clampedFill == 0 {
				return PlannedLeg{}, false
			}
			fill = clampedFill
			spend = CalcBidSpendSats(fill, o.PriceNum, o.PriceDen)
			remainSatsNext = o.RemainSats - spend
			if remainSatsNext < DustLimit || CalcBidMaxToken(remainSatsNext, o.PriceNum, o.PriceDen) < o.MinFillAmount {
				return PlannedLeg{}, false
			}
		}
	}

	fee := CalcFeeSats(spend, o.FeeBps)
	if o.FeeAddr == "" || fee < DustLimit {
		fee = 0
	}
	// Taker also pays ~11000 sat SettleTx miner fee out of spendSats (offset by 2*546 dust)
	if spend <= fee+DustLimit {
		return PlannedLeg{}, false
	}
	return PlannedLeg{
		Outpoint:     o.Outpoint,
		OrderTxid:    o.Txid,
		OrderVout:    o.Vout,
		MakerAddr:    o.MakerAddr,
		PriceNum:     o.PriceNum,
		PriceDen:     o.PriceDen,
		Price:        o.Price,
		FillToken:    fill,
		SatsAmount:   spend,
		FeeSats:      fee,
		NetTakerSats: spend - fee,
		IsPartial:    remainSatsNext > 0,
		RemainAfter:  remainSatsNext,
	}, true
}
