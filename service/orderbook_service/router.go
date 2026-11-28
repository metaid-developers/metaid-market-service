package orderbook_service

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/shopspring/decimal"
)

const (
	DefaultMaxAskLegs   = 4
	DefaultMaxBidLegs   = 2
	DefaultMaxTokenLegs = 2
	// Mirrors buildTakeBid's SettleTx miner fee budget (29,000 B full / 38,000 B partial @ 1.25 sat/B)
	// so RouteTake never selects a tiny Bid leg that buildTakeBid would reject for insufficient proceeds.
	MinBidSettleMinerFee        uint64 = 36250
	MinBidSettleMinerFeePartial uint64 = 47500
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
	Outpoint          string `json:"outpoint"`
	OrderTxid         string `json:"orderTxid"`
	OrderVout         int    `json:"orderVout"`
	TokenTxid         string `json:"tokenTxid,omitempty"`
	TokenVout         int    `json:"tokenVout,omitempty"`
	IsTokenOrder      bool   `json:"isTokenOrder,omitempty"`
	SellTokenCodeHash string `json:"sellTokenCodeHash,omitempty"`
	SellTokenID       string `json:"sellTokenID,omitempty"`
	BuyTokenCodeHash  string `json:"buyTokenCodeHash,omitempty"`
	BuyTokenID        string `json:"buyTokenID,omitempty"`
	FillSellAmount    uint64 `json:"fillSellAmount,omitempty"`
	MakerAddr         string `json:"makerAddr"`
	PriceNum          uint64 `json:"priceNum"`
	PriceDen          uint64 `json:"priceDen"`
	Price             string `json:"price"`
	FillToken         uint64 `json:"fillToken"`
	SatsAmount        uint64 `json:"satsAmount"` // SPACE or Quote FT units
	FeeSats           uint64 `json:"feeSats"`
	NetTakerSats      uint64 `json:"netTakerSats"` // Ask: quote+fee paid; Bid: quote received
	IsPartial         bool   `json:"isPartial"`
	RemainAfter       uint64 `json:"remainAfter"` // SPACE Ask: remain base; SPACE Bid: remain sats; FT/FT: remain sellToken
}

// RoutePlan is the Smart Order Router output passed directly to /take/quote and sidecar prepare.
type RoutePlan struct {
	PairKey           string       `json:"pairKey"`
	IsTokenPair       bool         `json:"isTokenPair,omitempty"`
	BuyTokenCodeHash  string       `json:"buyTokenCodeHash,omitempty"`
	BuyTokenID        string       `json:"buyTokenID,omitempty"`
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
	isTokenPairReq := IsTokenPairKey(pairKey)
	if side == "buy" {
		targetOrderSide = SideAsk
		if maxLegs <= 0 {
			if isTokenPairReq {
				maxLegs = DefaultMaxTokenLegs
			} else {
				maxLegs = DefaultMaxAskLegs
			}
		} else if isTokenPairReq && maxLegs > DefaultMaxTokenLegs {
			maxLegs = DefaultMaxTokenLegs
		}
	} else {
		targetOrderSide = SideBid
		if maxLegs <= 0 {
			maxLegs = DefaultMaxBidLegs
		} else if maxLegs > DefaultMaxBidLegs {
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

	isTokenPair := IsTokenPairKey(pairKey) || legs[0].IsTokenOrder
	var estMinerFee uint64
	if isTokenPair {
		estMinerFee = uint64(12000 + len(legs)*24000)
	} else if side == "buy" {
		estMinerFee = uint64(3000 + len(legs)*12000)
	} else {
		estMinerFee = uint64(11000 + len(legs)*11000)
	}

	return &RoutePlan{
		PairKey:           pairKey,
		IsTokenPair:       isTokenPair,
		BuyTokenCodeHash:  legs[0].BuyTokenCodeHash,
		BuyTokenID:        legs[0].BuyTokenID,
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
	if o.IsTokenOrder {
		return planTokenOrderLeg(o, wantToken)
	}
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
	// When o.RemainSats is not evenly divisible by priceNum/priceDen, floor(RemainSats * priceDen / priceNum)
	// can leave a 1-sat rounding remainder that fails the partial-fill dust guard; maxTok+1 closes it cleanly.
	if CalcBidSpendSats(maxTok, o.PriceNum, o.PriceDen) < o.RemainSats &&
		CalcBidSpendSats(maxTok+1, o.PriceNum, o.PriceDen) == o.RemainSats {
		maxTok++
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
	// Taker pays SettleTx miner fee out of spendSats + 2*546 dust, and must receive >= 546 dust.
	minSettleFee := MinBidSettleMinerFee
	if remainSatsNext > 0 {
		minSettleFee = MinBidSettleMinerFeePartial
	}
	if spend+2*DustLimit < fee+minSettleFee+DustLimit {
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

// planTokenOrderLeg plans a contract-valid OrderTokenV2 fill for either canonical Ask (sell=Base, buy=Quote)
// or canonical Bid (sell=Quote, buy=Base).
func planTokenOrderLeg(o *LiveOrder, wantBaseToken uint64) (PlannedLeg, bool) {
	if o.RemainAmount == 0 || o.RawPriceNum == 0 || o.RawPriceDen == 0 {
		return PlannedLeg{}, false
	}

	if o.Side == SideAsk {
		// Maker sells Base (sellToken) for Quote (buyToken). Taker wants wantBaseToken Base.
		fillBase := wantBaseToken
		if fillBase > o.RemainAmount {
			fillBase = o.RemainAmount
		}
		if fillBase < o.RemainAmount {
			if fillBase < o.MinFillAmount {
				return PlannedLeg{}, false
			}
			remainNext := o.RemainAmount - fillBase
			if remainNext < o.MinFillAmount {
				if o.RemainAmount >= 2*o.MinFillAmount {
					fillBase = o.RemainAmount - o.MinFillAmount
				} else {
					return PlannedLeg{}, false
				}
			}
		}
		if fillBase == 0 {
			return PlannedLeg{}, false
		}
		buyQuote := CalcAskGrossSats(fillBase, o.RawPriceNum, o.RawPriceDen)
		if buyQuote == 0 {
			return PlannedLeg{}, false
		}
		feeQuote := CalcFeeSats(buyQuote, o.FeeBps)
		if o.FeeAddr == "" {
			feeQuote = 0
		}
		remainAfter := o.RemainAmount - fillBase
		return PlannedLeg{
			Outpoint:          o.Outpoint,
			OrderTxid:         o.Txid,
			OrderVout:         o.Vout,
			TokenTxid:         o.TokenTxid,
			TokenVout:         o.TokenVout,
			IsTokenOrder:      true,
			SellTokenCodeHash: o.SellTokenCodeHash,
			SellTokenID:       o.SellTokenID,
			BuyTokenCodeHash:  o.BuyTokenCodeHash,
			BuyTokenID:        o.BuyTokenID,
			FillSellAmount:    fillBase,
			MakerAddr:         o.MakerAddr,
			PriceNum:          o.PriceNum,
			PriceDen:          o.PriceDen,
			Price:             o.Price,
			FillToken:         fillBase,
			SatsAmount:        buyQuote,
			FeeSats:           feeQuote,
			NetTakerSats:      buyQuote + feeQuote,
			IsPartial:         remainAfter > 0,
			RemainAfter:       remainAfter,
		}, true
	}

	// Canonical Bid: Maker sells Quote (sellToken, remainAmount=Quote) for Base (buyToken).
	// Taker wants to sell <= wantBaseToken Base and receive fillQuote Quote.
	// Contract enforces: buyBase = ceil(fillQuote * rawPriceNum / rawPriceDen).
	fillQuote := CalcBidMaxToken(wantBaseToken, o.RawPriceNum, o.RawPriceDen)
	if fillQuote > o.RemainAmount {
		fillQuote = o.RemainAmount
	}
	for fillQuote > 0 && CalcAskGrossSats(fillQuote, o.RawPriceNum, o.RawPriceDen) > wantBaseToken {
		fillQuote--
	}
	if fillQuote == 0 {
		return PlannedLeg{}, false
	}
	if fillQuote < o.RemainAmount {
		if fillQuote < o.MinFillAmount {
			return PlannedLeg{}, false
		}
		remainQuoteNext := o.RemainAmount - fillQuote
		if remainQuoteNext < o.MinFillAmount {
			if o.RemainAmount >= 2*o.MinFillAmount {
				fillQuote = o.RemainAmount - o.MinFillAmount
			} else {
				return PlannedLeg{}, false
			}
		}
	}
	buyBase := CalcAskGrossSats(fillQuote, o.RawPriceNum, o.RawPriceDen)
	if buyBase == 0 || buyBase > wantBaseToken {
		return PlannedLeg{}, false
	}
	feeBase := CalcFeeSats(buyBase, o.FeeBps)
	if o.FeeAddr == "" {
		feeBase = 0
	}
	remainAfter := o.RemainAmount - fillQuote
	return PlannedLeg{
		Outpoint:          o.Outpoint,
		OrderTxid:         o.Txid,
		OrderVout:         o.Vout,
		TokenTxid:         o.TokenTxid,
		TokenVout:         o.TokenVout,
		IsTokenOrder:      true,
		SellTokenCodeHash: o.SellTokenCodeHash,
		SellTokenID:       o.SellTokenID,
		BuyTokenCodeHash:  o.BuyTokenCodeHash,
		BuyTokenID:        o.BuyTokenID,
		FillSellAmount:    fillQuote,
		MakerAddr:         o.MakerAddr,
		PriceNum:          o.PriceNum,
		PriceDen:          o.PriceDen,
		Price:             o.Price,
		FillToken:         buyBase,
		SatsAmount:        fillQuote,
		FeeSats:           feeBase,
		NetTakerSats:      fillQuote,
		IsPartial:         remainAfter > 0,
		RemainAfter:       remainAfter,
	}, true
}
