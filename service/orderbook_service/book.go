package orderbook_service

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

const (
	maxInMemoryTradesPerPair = 500
	// DefaultDepthPrecision preserves up to 12 fractional digits (matching PriceToDecimal)
	// so sub-1 prices (e.g. 0.003 SPACE/MC or 3e-9 WBTC/MC) are never floored to "0" and merged.
	DefaultDepthPrecision int32 = 12
)

// LiveOrder represents one currently unspent OrderAskV2, OrderBidV2, or OrderTokenV2 UTXO in the orderbook.
type LiveOrder struct {
	Outpoint           string          `json:"outpoint"`
	Txid               string          `json:"txid"`
	Vout               int             `json:"vout"`
	TokenTxid          string          `json:"tokenTxid,omitempty"`
	TokenVout          int             `json:"tokenVout"`
	CodeHash           string          `json:"codeHash"`
	IsTokenOrder       bool            `json:"isTokenOrder,omitempty"`
	Side               uint8           `json:"side"` // 1=ask, 2=bid (canonical in PairKey)
	SideName           string          `json:"sideName"`
	PairKey            string          `json:"pairKey"`
	TokenCodeHash      string          `json:"tokenCodeHash"`
	TokenID            string          `json:"tokenID"`
	QuoteTokenCodeHash string          `json:"quoteTokenCodeHash,omitempty"`
	QuoteTokenID       string          `json:"quoteTokenID,omitempty"`
	SellTokenCodeHash  string          `json:"sellTokenCodeHash,omitempty"`
	SellTokenID        string          `json:"sellTokenID,omitempty"`
	BuyTokenCodeHash   string          `json:"buyTokenCodeHash,omitempty"`
	BuyTokenID         string          `json:"buyTokenID,omitempty"`
	MakerAddr          string          `json:"makerAddr"`
	MakerHash20        string          `json:"makerHash20"`
	PriceNum           uint64          `json:"priceNum"`
	PriceDen           uint64          `json:"priceDen"`
	RawPriceNum        uint64          `json:"rawPriceNum,omitempty"`
	RawPriceDen        uint64          `json:"rawPriceDen,omitempty"`
	Price              string          `json:"price"`
	PriceDecimal       decimal.Decimal `json:"-"`
	RemainAmount       uint64          `json:"remainAmount"` // SPACE ask=base, SPACE bid=sats, FT/FT=sellToken
	RemainToken        uint64          `json:"remainToken"`  // normalized Base token capacity
	RemainSats         uint64          `json:"remainSats"`   // normalized Quote capacity (sats or Quote FT raw units)
	MinFillAmount      uint64          `json:"minFillAmount"`
	FeeBps             uint16          `json:"feeBps"`
	FeeAddr            string          `json:"feeAddr"`
	SaltHex            string          `json:"saltHex"`
	Satoshis           uint64          `json:"satoshis"`
	ScriptHex          string          `json:"scriptHex,omitempty"`
	Height             int64           `json:"height"`
	IsMempool          bool            `json:"isMempool"`
	CreatedAt          int64           `json:"createdAt"` // unix ms
	SoftLocked         bool            `json:"softLocked,omitempty"`
}

// DepthLevel is a single aggregated price bucket in the L2 orderbook.
type DepthLevel struct {
	Price         string `json:"price"`
	TokenAmount   uint64 `json:"tokenAmount"`
	SatsTotal     uint64 `json:"satsTotal"`
	OrderCount    int    `json:"orderCount"`
	LockedCount   int    `json:"lockedCount,omitempty"`
	CumulativeTok uint64 `json:"cumulativeToken"`
}

// DepthSnapshot is returned by GET /api/v1/orderbook/depth.
type DepthSnapshot struct {
	PairKey   string       `json:"pairKey"`
	Precision int32        `json:"precision"`
	BestBid   string       `json:"bestBid"`
	BestAsk   string       `json:"bestAsk"`
	Spread    string       `json:"spread"`
	Asks      []DepthLevel `json:"asks"`
	Bids      []DepthLevel `json:"bids"`
	UpdatedAt int64        `json:"updatedAt"`
}

// TradeRecord represents one executed fill event (partial or full).
type TradeRecord struct {
	TradeKey      string `json:"tradeKey"`
	Txid          string `json:"txid"`
	OrderOutpoint string `json:"orderOutpoint"`
	PairKey       string `json:"pairKey"`
	Side          string `json:"side"` // "buy" (taker took Ask) | "sell" (taker took Bid)
	MakerAddr     string `json:"makerAddr"`
	TakerAddr     string `json:"takerAddr,omitempty"`
	PriceNum      uint64 `json:"priceNum"`
	PriceDen      uint64 `json:"priceDen"`
	Price         string `json:"price"`
	FillToken     uint64 `json:"fillToken"`
	SatsAmount    uint64 `json:"satsAmount"`
	FeeSats       uint64 `json:"feeSats"`
	IsPartial     bool   `json:"isPartial"`
	Height        int64  `json:"height"`
	IsMempool     bool   `json:"isMempool"`
	TradeTime     int64  `json:"tradeTime"`
}

// KlineBar represents one OHLCV candle.
type KlineBar struct {
	Time        int64  `json:"time"` // bucket start unix ms
	Open        string `json:"open"`
	High        string `json:"high"`
	Low         string `json:"low"`
	Close       string `json:"close"`
	VolumeToken uint64 `json:"volumeToken"`
	VolumeSats  uint64 `json:"volumeSats"`
	TradeCount  int    `json:"tradeCount"`
}

// PairSummary is returned by GET /api/v1/orderbook/pairs.
type PairSummary struct {
	PairKey            string `json:"pairKey"`
	IsTokenPair        bool   `json:"isTokenPair,omitempty"`
	TokenCodeHash      string `json:"tokenCodeHash"`
	TokenID            string `json:"tokenID"`
	Symbol             string `json:"symbol"`
	Decimals           int    `json:"decimals"`
	QuoteTokenCodeHash string `json:"quoteTokenCodeHash,omitempty"`
	QuoteTokenID       string `json:"quoteTokenID,omitempty"`
	QuoteSymbol        string `json:"quoteSymbol"`
	QuoteDecimals      int    `json:"quoteDecimals"`
	BestBid            string `json:"bestBid"`
	BestAsk            string `json:"bestAsk"`
	LastPrice          string `json:"lastPrice"`
	Change24hPct       string `json:"change24hPct"`
	Volume24hTok       uint64 `json:"volume24hToken"`
	Volume24hSats      uint64 `json:"volume24hSats"`
	AskCount           int    `json:"askCount"`
	BidCount           int    `json:"bidCount"`
}

// PairMetadata stores optional display info for a token pair.
type PairMetadata struct {
	Symbol        string
	Decimals      int
	QuoteSymbol   string
	QuoteDecimals int
	OrderIndex    int
}

// OrderBook maintains the live in-memory L3 orderbook, L2 aggregator, and recent trades.
type OrderBook struct {
	mu           sync.RWMutex
	net          string
	locker       SoftLocker
	orders       map[string]*LiveOrder            // outpoint -> LiveOrder
	byPair       map[string]map[string]*LiveOrder // pairKey -> outpoint -> LiveOrder
	bySalt       map[string]*LiveOrder            // saltHex -> latest active LiveOrder
	trades       map[string][]TradeRecord         // pairKey -> newest-last ring buffer
	pairMeta     map[string]PairMetadata
	nextMetaSeq  int
}

func NewOrderBook(net string, locker SoftLocker) *OrderBook {
	if locker == nil {
		locker = NewMemorySoftLocker()
	}
	return &OrderBook{
		net:      net,
		locker:   locker,
		orders:   make(map[string]*LiveOrder),
		byPair:   make(map[string]map[string]*LiveOrder),
		bySalt:   make(map[string]*LiveOrder),
		trades:   make(map[string][]TradeRecord),
		pairMeta: make(map[string]PairMetadata),
	}
}

func (b *OrderBook) Locker() SoftLocker {
	return b.locker
}

func (b *OrderBook) SetPairMetadata(pairKey, symbol string, decimals int) {
	b.SetPairMetadataFull(pairKey, symbol, decimals, "", 0)
}

func (b *OrderBook) SetPairMetadataFull(pairKey, symbol string, decimals int, quoteSymbol string, quoteDecimals int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	k := strings.ToLower(pairKey)
	if quoteSymbol == "" && !IsTokenPairKey(k) {
		quoteSymbol = "SPACE"
	}
	if quoteDecimals <= 0 {
		quoteDecimals = 8
	}
	existing, ok := b.pairMeta[k]
	orderIdx := existing.OrderIndex
	if !ok {
		b.nextMetaSeq++
		orderIdx = b.nextMetaSeq
	}
	if symbol == "" {
		symbol = existing.Symbol
	}
	if decimals <= 0 && existing.Decimals > 0 {
		decimals = existing.Decimals
	}
	if quoteSymbol == "" {
		quoteSymbol = existing.QuoteSymbol
	}
	b.pairMeta[k] = PairMetadata{
		Symbol:        symbol,
		Decimals:      decimals,
		QuoteSymbol:   quoteSymbol,
		QuoteDecimals: quoteDecimals,
		OrderIndex:    orderIdx,
	}
}

// ResetSnapshot replaces the entire live order table atomically (used on bootstrap or Higun resetRequired).
func (b *OrderBook) ResetSnapshot(orders []*LiveOrder) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.orders = make(map[string]*LiveOrder, len(orders))
	b.byPair = make(map[string]map[string]*LiveOrder)
	b.bySalt = make(map[string]*LiveOrder, len(orders))
	for _, o := range orders {
		if o == nil {
			continue
		}
		b.upsertLocked(o)
	}
}

// UpsertOrder inserts or updates one live order UTXO.
func (b *OrderBook) UpsertOrder(o *LiveOrder) {
	if o == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.upsertLocked(o)
}

func (b *OrderBook) upsertLocked(o *LiveOrder) {
	// Preserve exact LockTokenTx outpoint when an optimistic entry already knows it
	// and a subsequent Higun created event arrives with only the default vout+1 heuristic.
	if prev, ok := b.orders[o.Outpoint]; ok && prev != nil {
		if prev.TokenTxid != "" && prev.TokenTxid != prev.Txid && (o.TokenTxid == "" || o.TokenTxid == o.Txid) {
			o.TokenTxid = prev.TokenTxid
			o.TokenVout = prev.TokenVout
		}
	}
	b.orders[o.Outpoint] = o
	pm, ok := b.byPair[o.PairKey]
	if !ok {
		pm = make(map[string]*LiveOrder)
		b.byPair[o.PairKey] = pm
	}
	pm[o.Outpoint] = o
	if o.SaltHex != "" {
		b.bySalt[o.SaltHex] = o
	}
}

// RemoveOrder deletes a spent or evicted order outpoint and returns the removed order (if present).
func (b *OrderBook) RemoveOrder(outpoint string) *LiveOrder {
	b.mu.Lock()
	defer b.mu.Unlock()
	o, ok := b.orders[outpoint]
	if !ok {
		return nil
	}
	delete(b.orders, outpoint)
	if pm, ok := b.byPair[o.PairKey]; ok {
		delete(pm, outpoint)
	}
	if cur, ok := b.bySalt[o.SaltHex]; ok && cur.Outpoint == outpoint {
		delete(b.bySalt, o.SaltHex)
	}
	return o
}

// GetOrder returns a copy of a live order by outpoint.
func (b *OrderBook) GetOrder(outpoint string) (*LiveOrder, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	o, ok := b.orders[outpoint]
	if !ok {
		return nil, false
	}
	cp := *o
	cp.SoftLocked = b.locker.IsLocked(o.Outpoint)
	return &cp, true
}

// RecordTrade appends a trade (deduplicated by TradeKey) to the in-memory trade history.
func (b *OrderBook) RecordTrade(tr TradeRecord) {
	b.mu.Lock()
	defer b.mu.Unlock()
	list := b.trades[tr.PairKey]
	for i := len(list) - 1; i >= 0 && i >= len(list)-32; i-- {
		if list[i].TradeKey == tr.TradeKey {
			list[i].Height = tr.Height
			list[i].IsMempool = tr.IsMempool
			b.trades[tr.PairKey] = list
			return
		}
	}
	list = append(list, tr)
	if len(list) > maxInMemoryTradesPerPair {
		list = list[len(list)-maxInMemoryTradesPerPair:]
	}
	b.trades[tr.PairKey] = list
}

// ParseLiveOrder builds a LiveOrder from an on-chain/mempool OrderbookUTXO.
func ParseLiveOrder(txid string, vout int, satoshis uint64, scriptHex string, height int64, isMempool bool, createdAt int64, net string) (*LiveOrder, error) {
	parsed, err := ParseOrderScriptHex(scriptHex, net)
	if err != nil {
		return nil, err
	}
	if createdAt <= 0 {
		createdAt = time.Now().UnixMilli()
	}
	outpoint := fmt.Sprintf("%s:%d", strings.ToLower(txid), vout)
	lo := &LiveOrder{
		Outpoint:           outpoint,
		Txid:               strings.ToLower(txid),
		Vout:               vout,
		TokenVout:          -1,
		CodeHash:           parsed.ScriptCodeHash,
		IsTokenOrder:       parsed.IsTokenOrder,
		Side:               parsed.OrderSide,
		PairKey:            parsed.PairKey,
		TokenCodeHash:      parsed.TokenCodeHash,
		TokenID:            parsed.TokenID,
		QuoteTokenCodeHash: parsed.QuoteTokenCodeHash,
		QuoteTokenID:       parsed.QuoteTokenID,
		SellTokenCodeHash:  parsed.SellTokenCodeHash,
		SellTokenID:        parsed.SellTokenID,
		BuyTokenCodeHash:   parsed.BuyTokenCodeHash,
		BuyTokenID:         parsed.BuyTokenID,
		MakerAddr:          parsed.MakerAddr,
		MakerHash20:        parsed.MakerHash20,
		PriceNum:           parsed.PriceNum,
		PriceDen:           parsed.PriceDen,
		RawPriceNum:        parsed.RawPriceNum,
		RawPriceDen:        parsed.RawPriceDen,
		Price:              parsed.PriceDecimal.String(),
		PriceDecimal:       parsed.PriceDecimal,
		RemainAmount:       parsed.RemainAmount,
		MinFillAmount:      parsed.MinFillAmount,
		FeeBps:             parsed.FeeBps,
		FeeAddr:            parsed.FeeAddr,
		SaltHex:            parsed.SaltHex,
		Satoshis:           satoshis,
		ScriptHex:          scriptHex,
		Height:             height,
		IsMempool:          isMempool,
		CreatedAt:          createdAt,
	}
	if parsed.IsTokenOrder {
		lo.TokenTxid = lo.Txid
		lo.TokenVout = vout + 1
		if parsed.OrderSide == SideAsk {
			lo.SideName = "ask"
			lo.RemainToken = parsed.RemainAmount
			lo.RemainSats = CalcAskGrossSats(parsed.RemainAmount, parsed.PriceNum, parsed.PriceDen)
		} else {
			lo.SideName = "bid"
			lo.RemainSats = parsed.RemainAmount
			lo.RemainToken = CalcAskGrossSats(parsed.RemainAmount, parsed.RawPriceNum, parsed.RawPriceDen)
		}
	} else if parsed.OrderSide == SideAsk {
		lo.SideName = "ask"
		lo.TokenTxid = lo.Txid
		// Spec convention: Ask Order output is at vout, locked FT token output is at vout+1
		lo.TokenVout = vout + 1
		lo.RemainToken = parsed.RemainAmount
		lo.RemainSats = CalcAskGrossSats(parsed.RemainAmount, parsed.PriceNum, parsed.PriceDen)
	} else {
		lo.SideName = "bid"
		// For Bid orders, satoshis must equal tail remainAmount
		if satoshis > 0 && satoshis != parsed.RemainAmount {
			return nil, fmt.Errorf("bid utxo satoshis (%d) != tail remainAmount (%d)", satoshis, parsed.RemainAmount)
		}
		lo.RemainSats = parsed.RemainAmount
		lo.RemainToken = CalcBidMaxToken(parsed.RemainAmount, parsed.PriceNum, parsed.PriceDen)
	}
	return lo, nil
}

// GetDepth aggregates live orders into L2 price buckets:
//   - Asks: BucketPrice(..., SideAsk) [Ceil], sorted ascending
//   - Bids: BucketPrice(..., SideBid) [Floor], sorted descending
func (b *OrderBook) GetDepth(pairKey string, precision int32, limit int) DepthSnapshot {
	pairKey = strings.ToLower(strings.TrimSpace(pairKey))
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	b.mu.RLock()
	pm := b.byPair[pairKey]
	orders := make([]*LiveOrder, 0, len(pm))
	for _, o := range pm {
		orders = append(orders, o)
	}
	b.mu.RUnlock()

	type aggBucket struct {
		price       decimal.Decimal
		tokenAmount uint64
		satsTotal   uint64
		orderCount  int
		lockedCount int
	}

	askMap := make(map[string]*aggBucket)
	bidMap := make(map[string]*aggBucket)

	for _, o := range orders {
		if o.RemainToken == 0 {
			continue
		}
		locked := b.locker.IsLocked(o.Outpoint)
		bucketPrice := BucketPrice(o.PriceNum, o.PriceDen, precision, o.Side)
		key := bucketPrice.String()

		var target map[string]*aggBucket
		if o.Side == SideAsk {
			target = askMap
		} else {
			target = bidMap
		}
		ab, ok := target[key]
		if !ok {
			ab = &aggBucket{price: bucketPrice}
			target[key] = ab
		}
		ab.tokenAmount += o.RemainToken
		ab.satsTotal += o.RemainSats
		ab.orderCount++
		if locked {
			ab.lockedCount++
		}
	}

	askBuckets := make([]*aggBucket, 0, len(askMap))
	for _, v := range askMap {
		askBuckets = append(askBuckets, v)
	}
	sort.Slice(askBuckets, func(i, j int) bool {
		return askBuckets[i].price.LessThan(askBuckets[j].price)
	})
	if len(askBuckets) > limit {
		askBuckets = askBuckets[:limit]
	}

	bidBuckets := make([]*aggBucket, 0, len(bidMap))
	for _, v := range bidMap {
		bidBuckets = append(bidBuckets, v)
	}
	sort.Slice(bidBuckets, func(i, j int) bool {
		return bidBuckets[i].price.GreaterThan(bidBuckets[j].price)
	})
	if len(bidBuckets) > limit {
		bidBuckets = bidBuckets[:limit]
	}

	asks := make([]DepthLevel, 0, len(askBuckets))
	var cumAsk uint64
	for _, ab := range askBuckets {
		cumAsk += ab.tokenAmount
		asks = append(asks, DepthLevel{
			Price:         ab.price.String(),
			TokenAmount:   ab.tokenAmount,
			SatsTotal:     ab.satsTotal,
			OrderCount:    ab.orderCount,
			LockedCount:   ab.lockedCount,
			CumulativeTok: cumAsk,
		})
	}

	bids := make([]DepthLevel, 0, len(bidBuckets))
	var cumBid uint64
	for _, ab := range bidBuckets {
		cumBid += ab.tokenAmount
		bids = append(bids, DepthLevel{
			Price:         ab.price.String(),
			TokenAmount:   ab.tokenAmount,
			SatsTotal:     ab.satsTotal,
			OrderCount:    ab.orderCount,
			LockedCount:   ab.lockedCount,
			CumulativeTok: cumBid,
		})
	}

	snap := DepthSnapshot{
		PairKey:   pairKey,
		Precision: precision,
		Asks:      asks,
		Bids:      bids,
		UpdatedAt: time.Now().UnixMilli(),
	}
	if len(asks) > 0 {
		snap.BestAsk = asks[0].Price
	}
	if len(bids) > 0 {
		snap.BestBid = bids[0].Price
	}
	if len(asks) > 0 && len(bids) > 0 {
		snap.Spread = askBuckets[0].price.Sub(bidBuckets[0].price).String()
	}
	return snap
}

// ListLiveOrders filters currently active orders (optional makerAddr, pairKey, side).
func (b *OrderBook) ListLiveOrders(makerAddr, pairKey string, side uint8) []LiveOrder {
	makerAddr = strings.TrimSpace(makerAddr)
	pairKey = strings.ToLower(strings.TrimSpace(pairKey))

	b.mu.RLock()
	out := make([]LiveOrder, 0)
	for _, o := range b.orders {
		if makerAddr != "" && !strings.EqualFold(o.MakerAddr, makerAddr) && !strings.EqualFold(o.MakerHash20, makerAddr) {
			continue
		}
		if pairKey != "" && o.PairKey != pairKey {
			continue
		}
		if side != 0 && o.Side != side {
			continue
		}
		cp := *o
		cp.ScriptHex = "" // keep list response compact
		cp.SoftLocked = b.locker.IsLocked(o.Outpoint)
		out = append(out, cp)
	}
	b.mu.RUnlock()

	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt != out[j].CreatedAt {
			return out[i].CreatedAt > out[j].CreatedAt
		}
		return out[i].Outpoint < out[j].Outpoint
	})
	return out
}

// GetRecentTrades returns newest-first trades for a pair from memory.
func (b *OrderBook) GetRecentTrades(pairKey string, limit int) []TradeRecord {
	pairKey = strings.ToLower(strings.TrimSpace(pairKey))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	b.mu.RLock()
	list := b.trades[pairKey]
	n := len(list)
	if n == 0 {
		b.mu.RUnlock()
		return []TradeRecord{}
	}
	if limit > n {
		limit = n
	}
	out := make([]TradeRecord, 0, limit)
	for i := n - 1; i >= n-limit; i-- {
		out = append(out, list[i])
	}
	b.mu.RUnlock()
	return out
}

// BuildKlines aggregates trades into OHLCV bars for intervals: 1m, 5m, 15m, 1h, 4h, 1d.
func BuildKlines(trades []TradeRecord, interval string, limit int) []KlineBar {
	stepMs := intervalToMs(interval)
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	if len(trades) == 0 {
		return []KlineBar{}
	}
	// Ensure chronological order
	sorted := make([]TradeRecord, len(trades))
	copy(sorted, trades)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].TradeTime < sorted[j].TradeTime
	})

	type barState struct {
		ts    int64
		open  decimal.Decimal
		high  decimal.Decimal
		low   decimal.Decimal
		close decimal.Decimal
		volT  uint64
		volS  uint64
		cnt   int
	}

	order := make([]int64, 0)
	buckets := make(map[int64]*barState)
	for _, tr := range sorted {
		ts := (tr.TradeTime / stepMs) * stepMs
		px := PriceToDecimal(tr.PriceNum, tr.PriceDen)
		bs, ok := buckets[ts]
		if !ok {
			bs = &barState{ts: ts, open: px, high: px, low: px, close: px}
			buckets[ts] = bs
			order = append(order, ts)
		} else {
			if px.GreaterThan(bs.high) {
				bs.high = px
			}
			if px.LessThan(bs.low) {
				bs.low = px
			}
			bs.close = px
		}
		bs.volT += tr.FillToken
		bs.volS += tr.SatsAmount
		bs.cnt++
	}
	if len(order) > limit {
		order = order[len(order)-limit:]
	}
	out := make([]KlineBar, 0, len(order))
	for _, ts := range order {
		bs := buckets[ts]
		out = append(out, KlineBar{
			Time:        bs.ts,
			Open:        bs.open.String(),
			High:        bs.high.String(),
			Low:         bs.low.String(),
			Close:       bs.close.String(),
			VolumeToken: bs.volT,
			VolumeSats:  bs.volS,
			TradeCount:  bs.cnt,
		})
	}
	return out
}

func intervalToMs(interval string) int64 {
	switch strings.ToLower(strings.TrimSpace(interval)) {
	case "1m":
		return 60_000
	case "5m":
		return 5 * 60_000
	case "15m":
		return 15 * 60_000
	case "1h":
		return 60 * 60_000
	case "4h":
		return 4 * 60 * 60_000
	case "1d":
		return 24 * 60 * 60_000
	default:
		return 15 * 60_000
	}
}

// ListPairs returns summaries of all known active pairs (from live book, trades, or registered metadata).
func (b *OrderBook) ListPairs() []PairSummary {
	b.mu.RLock()
	keys := make(map[string]struct{})
	for k := range b.byPair {
		keys[k] = struct{}{}
	}
	for k := range b.trades {
		keys[k] = struct{}{}
	}
	for k := range b.pairMeta {
		keys[k] = struct{}{}
	}

	cutoff24h := time.Now().Add(-24 * time.Hour).UnixMilli()
	out := make([]PairSummary, 0, len(keys))
	for pk := range keys {
		baseCH, baseID, quoteCH, quoteID, isTokenPair := SplitPairKey(pk)
		meta := b.pairMeta[pk]
		quoteSym := meta.QuoteSymbol
		if quoteSym == "" && !isTokenPair {
			quoteSym = "SPACE"
		}
		quoteDec := meta.QuoteDecimals
		if quoteDec <= 0 {
			quoteDec = 8
		}
		sum := PairSummary{
			PairKey:            pk,
			IsTokenPair:        isTokenPair,
			TokenCodeHash:      baseCH,
			TokenID:            baseID,
			Symbol:             meta.Symbol,
			Decimals:           meta.Decimals,
			QuoteTokenCodeHash: quoteCH,
			QuoteTokenID:       quoteID,
			QuoteSymbol:        quoteSym,
			QuoteDecimals:      quoteDec,
		}
		var bestAsk, bestBid *LiveOrder
		for _, o := range b.byPair[pk] {
			if o.RemainToken == 0 {
				continue
			}
			if o.Side == SideAsk {
				sum.AskCount++
				if bestAsk == nil || ComparePrice(o.PriceNum, o.PriceDen, bestAsk.PriceNum, bestAsk.PriceDen) < 0 {
					bestAsk = o
				}
			} else {
				sum.BidCount++
				if bestBid == nil || ComparePrice(o.PriceNum, o.PriceDen, bestBid.PriceNum, bestBid.PriceDen) > 0 {
					bestBid = o
				}
			}
		}
		if bestAsk != nil {
			sum.BestAsk = bestAsk.Price
		}
		if bestBid != nil {
			sum.BestBid = bestBid.Price
		}
		trList := b.trades[pk]
		if len(trList) > 0 {
			last := trList[len(trList)-1]
			sum.LastPrice = last.Price
			var first24h *TradeRecord
			for i := range trList {
				if trList[i].TradeTime >= cutoff24h {
					if first24h == nil {
						first24h = &trList[i]
					}
					sum.Volume24hTok += trList[i].FillToken
					sum.Volume24hSats += trList[i].SatsAmount
				}
			}
			if first24h != nil {
				openPx := PriceToDecimal(first24h.PriceNum, first24h.PriceDen)
				lastPx := PriceToDecimal(last.PriceNum, last.PriceDen)
				if !openPx.IsZero() {
					chg := lastPx.Sub(openPx).Div(openPx).Mul(decimal.NewFromInt(100)).Round(2)
					sum.Change24hPct = chg.String()
				}
			}
		}
		out = append(out, sum)
	}
	b.mu.RUnlock()

	b.mu.RLock()
	orderIdxMap := make(map[string]int, len(b.pairMeta))
	for k, m := range b.pairMeta {
		orderIdxMap[k] = m.OrderIndex
	}
	b.mu.RUnlock()

	sort.Slice(out, func(i, j int) bool {
		if out[i].Volume24hSats != out[j].Volume24hSats {
			return out[i].Volume24hSats > out[j].Volume24hSats
		}
		oi := out[i].AskCount + out[i].BidCount
		oj := out[j].AskCount + out[j].BidCount
		if oi != oj {
			return oi > oj
		}
		mi, oki := orderIdxMap[out[i].PairKey]
		mj, okj := orderIdxMap[out[j].PairKey]
		if oki && okj && mi != mj {
			return mi < mj
		}
		if oki != okj {
			return oki
		}
		return out[i].PairKey < out[j].PairKey
	})
	return out
}
