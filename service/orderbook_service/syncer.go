package orderbook_service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"metaid-market-service/conf"
	"metaid-market-service/models"
)

// Higun REST response models (matching higun/indexer/orderbook_index.go).
type HigunOrderbookUTXO struct {
	TxID      string  `json:"txid"`
	Vout      int     `json:"vout"`
	Codehash  string  `json:"codehash"`
	Satoshis  uint64  `json:"satoshis"`
	ScriptHex string  `json:"scriptHex"`
	Height    int64   `json:"height"`
	Mempool   bool    `json:"mempool"`
	SpentBy   *string `json:"spentBy"`
}

type HigunOrderbookUTXOsPage struct {
	Seq        uint64               `json:"seq"`
	NextCursor string               `json:"nextCursor"`
	Items      []HigunOrderbookUTXO `json:"items"`
}

type HigunOrderbookEvent struct {
	Seq       uint64 `json:"seq"`
	Type      string `json:"type"` // "created" | "spent" | "evicted"
	Codehash  string `json:"codehash"`
	TxID      string `json:"txid"`
	Vout      int    `json:"vout"`
	Satoshis  uint64 `json:"satoshis"`
	ScriptHex string `json:"scriptHex"`
	SpentBy   string `json:"spentBy"`
	Height    int64  `json:"height"`
	Mempool   bool   `json:"mempool"`
	Timestamp int64  `json:"timestamp"`
}

type HigunOrderbookEventsPage struct {
	LatestSeq     uint64                `json:"latestSeq"`
	ResetRequired bool                  `json:"resetRequired"`
	Events        []HigunOrderbookEvent `json:"events"`
}

// OptimisticNewOrder represents a newly created order returned immediately by sidecar finalize.
type OptimisticNewOrder struct {
	Side         string `json:"side"`
	Txid         string `json:"txid"`
	Vout         int    `json:"vout"`
	Satoshis     uint64 `json:"satoshis"`
	ScriptHex    string `json:"scriptHex"`
	RemainAmount string `json:"remainAmount"`
	TokenTxid    string `json:"tokenTxid,omitempty"`
	TokenVout    int    `json:"tokenVout,omitempty"`
	SaltHex      string `json:"saltHex"`
}

// Syncer consumes Higun's /orderbook/utxos and /orderbook/events endpoints,
// maintains the in-memory OrderBook, persists to MySQL, and pushes WS updates.
type Syncer struct {
	cfg        *conf.OrderbookConfig
	net        string
	book       *OrderBook
	hub        *WSHub
	httpClient *http.Client
	persistDB  bool

	mu           sync.Mutex
	lastSeq      uint64
	bootstrapped bool
	resetCount   uint64
	cancelFn     context.CancelFunc
	draftLocks   map[string]DraftReservation // draftId -> reserved outpoints
}

type DraftReservation struct {
	ReservationID string
	Outpoints     []string
	Side          string
	PairKey       string
	TakerAddr     string
	Legs          []PlannedLeg
	CreatedAt     time.Time
}

func NewSyncer(cfg *conf.OrderbookConfig, net string, book *OrderBook, hub *WSHub, persistDB bool) *Syncer {
	if cfg == nil {
		cfg = conf.DefaultOrderbookConfig()
	}
	if book == nil {
		book = NewOrderBook(net, NewMemorySoftLocker())
	}
	if hub == nil {
		hub = NewWSHub()
	}
	return &Syncer{
		cfg:        cfg,
		net:        net,
		book:       book,
		hub:        hub,
		httpClient: &http.Client{Timeout: 15 * time.Second},
		persistDB:  persistDB,
		draftLocks: make(map[string]DraftReservation),
	}
}

func (s *Syncer) Book() *OrderBook     { return s.book }
func (s *Syncer) Hub() *WSHub          { return s.hub }
func (s *Syncer) Config() *conf.OrderbookConfig { return s.cfg }
func (s *Syncer) LastSeq() uint64      { s.mu.Lock(); defer s.mu.Unlock(); return s.lastSeq }
func (s *Syncer) ResetCount() uint64   { s.mu.Lock(); defer s.mu.Unlock(); return s.resetCount }

// Start runs initial snapshot bootstrap and background polling until Stop is called.
func (s *Syncer) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancelFn = cancel
	s.mu.Unlock()

	if s.persistDB {
		if err := models.OrderbookDao().AutoMigrate(); err != nil {
			log.Printf("[Orderbook] AutoMigrate warning: %v", err)
		}
		if pairs, err := models.OrderbookDao().ListPairs(); err == nil {
			for _, p := range pairs {
				s.book.SetPairMetadata(p.PairKey, p.Symbol, p.Decimals)
			}
		}
	}

	if err := s.BootstrapSnapshot(ctx); err != nil {
		log.Printf("[Orderbook] initial Higun snapshot failed (will retry in poll loop): %v", err)
	}

	interval := time.Duration(s.cfg.PollIntervalMs) * time.Millisecond
	if interval < 50*time.Millisecond {
		interval = 300 * time.Millisecond
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.PollOnce(ctx); err != nil {
					log.Printf("[Orderbook] poll error: %v", err)
				}
			}
		}
	}()
}

func (s *Syncer) Stop() {
	s.mu.Lock()
	cancel := s.cancelFn
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// BootstrapSnapshot reloads all active Ask & Bid UTXOs from Higun and captures the snapshot seq.
func (s *Syncer) BootstrapSnapshot(ctx context.Context) error {
	codehashes := s.configuredCodehashes()
	if len(codehashes) == 0 {
		return fmt.Errorf("no orderbook codehashes configured")
	}

	var allOrders []*LiveOrder
	var snapSeq uint64

	for _, ch := range codehashes {
		cursor := ""
		for {
			page, err := s.fetchHigunUTXOs(ctx, ch, cursor, 500)
			if err != nil {
				return err
			}
			if snapSeq == 0 || page.Seq < snapSeq {
				snapSeq = page.Seq
			}
			for _, item := range page.Items {
				if item.SpentBy != nil && *item.SpentBy != "" {
					continue
				}
				lo, err := ParseLiveOrder(item.TxID, item.Vout, item.Satoshis, item.ScriptHex, item.Height, item.Mempool, 0, s.net)
				if err != nil {
					continue
				}
				allOrders = append(allOrders, lo)
			}
			if page.NextCursor == "" || len(page.Items) == 0 {
				break
			}
			cursor = page.NextCursor
		}
	}

	s.book.ResetSnapshot(allOrders)

	if s.persistDB {
		now := time.Now().UnixMilli()
		for _, lo := range allOrders {
			_ = models.OrderbookDao().UpsertOrder(liveOrderToModel(lo, "open", "", now))
		}
	}

	s.mu.Lock()
	s.lastSeq = snapSeq
	s.bootstrapped = true
	s.mu.Unlock()

	// Broadcast fresh depth for affected pairs
	seenPairs := make(map[string]struct{})
	for _, lo := range allOrders {
		if _, ok := seenPairs[lo.PairKey]; !ok {
			seenPairs[lo.PairKey] = struct{}{}
			s.broadcastDepth(lo.PairKey)
		}
	}
	return nil
}

// PollOnce pulls one batch of incremental Higun events and updates the OrderBook.
func (s *Syncer) PollOnce(ctx context.Context) error {
	s.mu.Lock()
	bootstrapped := s.bootstrapped
	afterSeq := s.lastSeq
	s.mu.Unlock()

	if !bootstrapped {
		return s.BootstrapSnapshot(ctx)
	}

	page, err := s.fetchHigunEvents(ctx, afterSeq, 500)
	if err != nil {
		return err
	}

	if page.ResetRequired {
		s.mu.Lock()
		s.resetCount++
		s.mu.Unlock()
		log.Printf("[Orderbook] Higun signaled resetRequired=true (afterSeq=%d, latestSeq=%d); reloading snapshot", afterSeq, page.LatestSeq)
		return s.BootstrapSnapshot(ctx)
	}

	if len(page.Events) == 0 {
		if page.LatestSeq > afterSeq {
			s.mu.Lock()
			s.lastSeq = page.LatestSeq
			s.mu.Unlock()
		}
		return nil
	}

	s.applyEventsBatch(page.Events)

	nextSeq := page.LatestSeq
	for _, ev := range page.Events {
		if ev.Seq > nextSeq {
			nextSeq = ev.Seq
		}
	}
	s.mu.Lock()
	s.lastSeq = nextSeq
	s.mu.Unlock()
	return nil
}

// applyEventsBatch processes a chronological batch of Higun OrderbookEvents.
// Grouping by txid lets us match spent orders with their continuation order output
// in the same transaction (partial fills share the exact same saltHex).
func (s *Syncer) applyEventsBatch(events []HigunOrderbookEvent) {
	// First pass: index newly created orders by txid+salt so spent events in the same tx
	// can immediately tell partial fill vs full fill / cancel.
	createdByTxSalt := make(map[string]*LiveOrder)
	parsedCreated := make(map[uint64]*LiveOrder)
	for _, ev := range events {
		if ev.Type != "created" || ev.ScriptHex == "" {
			continue
		}
		tsMs := ev.Timestamp
		if tsMs > 0 && tsMs < 1e12 {
			tsMs *= 1000
		}
		lo, err := ParseLiveOrder(ev.TxID, ev.Vout, ev.Satoshis, ev.ScriptHex, ev.Height, ev.Mempool, tsMs, s.net)
		if err != nil {
			continue
		}
		parsedCreated[ev.Seq] = lo
		createdByTxSalt[strings.ToLower(ev.TxID)+":"+lo.SaltHex] = lo
	}

	dirtyPairs := make(map[string]struct{})
	now := time.Now().UnixMilli()

	for _, ev := range events {
		outpoint := fmt.Sprintf("%s:%d", strings.ToLower(ev.TxID), ev.Vout)
		tsMs := ev.Timestamp
		if tsMs > 0 && tsMs < 1e12 {
			tsMs *= 1000
		}
		if tsMs <= 0 {
			tsMs = now
		}

		switch ev.Type {
		case "created":
			lo := parsedCreated[ev.Seq]
			if lo == nil {
				continue
			}
			s.book.UpsertOrder(lo)
			dirtyPairs[lo.PairKey] = struct{}{}
			if s.persistDB {
				_ = models.OrderbookDao().UpsertOrder(liveOrderToModel(lo, "open", "", tsMs))
			}
			s.hub.Broadcast("orders:"+strings.ToLower(lo.MakerAddr), WSMessage{
				Channel: "orders",
				Address: lo.MakerAddr,
				Pair:    lo.PairKey,
				Data:    map[string]interface{}{"event": "created", "order": lo},
				Ts:      tsMs,
			})

		case "spent":
			prev := s.book.RemoveOrder(outpoint)
			_ = s.book.Locker().Release([]string{outpoint}, "")
			if prev == nil {
				continue
			}
			dirtyPairs[prev.PairKey] = struct{}{}

			spentBy := strings.ToLower(ev.SpentBy)
			cont := createdByTxSalt[spentBy+":"+prev.SaltHex]
			if cont != nil && cont.RemainAmount < prev.RemainAmount {
				// Confirmed partial fill!
				var fillToken, satsAmount, feeSats uint64
				var tradeSide string
				if prev.Side == SideAsk {
					fillToken = prev.RemainAmount - cont.RemainAmount
					satsAmount = CalcAskGrossSats(fillToken, prev.PriceNum, prev.PriceDen)
					feeSats = CalcFeeSats(satsAmount, prev.FeeBps)
					tradeSide = "buy"
				} else {
					satsAmount = prev.RemainAmount - cont.RemainAmount
					fillToken = CalcBidMaxToken(satsAmount, prev.PriceNum, prev.PriceDen)
					feeSats = CalcFeeSats(satsAmount, prev.FeeBps)
					tradeSide = "sell"
				}
				if prev.FeeAddr == "" || feeSats < DustLimit {
					feeSats = 0
				}
				tr := TradeRecord{
					TradeKey:      spentBy + ":" + outpoint,
					Txid:          spentBy,
					OrderOutpoint: outpoint,
					PairKey:       prev.PairKey,
					Side:          tradeSide,
					MakerAddr:     prev.MakerAddr,
					PriceNum:      prev.PriceNum,
					PriceDen:      prev.PriceDen,
					Price:         prev.Price,
					FillToken:     fillToken,
					SatsAmount:    satsAmount,
					FeeSats:       feeSats,
					IsPartial:     true,
					Height:        ev.Height,
					IsMempool:     ev.Mempool,
					TradeTime:     tsMs,
				}
				s.recordAndBroadcastTrade(tr)
				if s.persistDB {
					_ = models.OrderbookDao().UpdateOrderStatus(outpoint, "partial_filled", spentBy, ev.Height, boolToInt(ev.Mempool), tsMs)
				}
			} else {
				// Full fill or Cancel (optimistic submit may already have recorded full fill trade;
				// mark order closed and notify maker).
				if s.persistDB {
					_ = models.OrderbookDao().UpdateOrderStatus(outpoint, "closed", spentBy, ev.Height, boolToInt(ev.Mempool), tsMs)
				}
			}
			s.hub.Broadcast("orders:"+strings.ToLower(prev.MakerAddr), WSMessage{
				Channel: "orders",
				Address: prev.MakerAddr,
				Pair:    prev.PairKey,
				Data:    map[string]interface{}{"event": "spent", "outpoint": outpoint, "spentBy": spentBy},
				Ts:      tsMs,
			})

		case "evicted":
			prev := s.book.RemoveOrder(outpoint)
			_ = s.book.Locker().Release([]string{outpoint}, "")
			if prev != nil {
				dirtyPairs[prev.PairKey] = struct{}{}
				if s.persistDB {
					_ = models.OrderbookDao().UpdateOrderStatus(outpoint, "evicted", "", 0, 0, tsMs)
				}
			}
		}
	}

	for pk := range dirtyPairs {
		s.broadcastDepth(pk)
	}
}

// RegisterDraftReservation tracks soft-locked outpoints for an in-flight sidecar draft.
func (s *Syncer) RegisterDraftReservation(draftID string, res DraftReservation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Prune stale entries older than 5 minutes
	cutoff := time.Now().Add(-5 * time.Minute)
	for k, v := range s.draftLocks {
		if v.CreatedAt.Before(cutoff) {
			delete(s.draftLocks, k)
		}
	}
	if res.CreatedAt.IsZero() {
		res.CreatedAt = time.Now()
	}
	s.draftLocks[draftID] = res
}

// HandleFinalizeResult updates the in-memory orderbook immediately in 0-conf once
// sidecar finalize completes, and manages Redis Soft-Lock lifetime for 1-step & 2-step flows.
func (s *Syncer) HandleFinalizeResult(draftID, action, status string, txids []string, newOrders []OptimisticNewOrder) {
	s.mu.Lock()
	res, hasRes := s.draftLocks[draftID]
	if status == "completed" {
		delete(s.draftLocks, draftID)
	}
	s.mu.Unlock()

	if status == "awaiting_signature" {
		// Extend lock while user signs step 1
		if hasRes {
			ttl := time.Duration(s.cfg.SoftLockTTLSec) * time.Second
			_ = s.book.Locker().Extend(res.Outpoints, res.ReservationID, ttl)
		}
		return
	}

	if status != "completed" {
		if hasRes {
			_ = s.book.Locker().Release(res.Outpoints, res.ReservationID)
		}
		return
	}

	now := time.Now().UnixMilli()
	dirtyPairs := make(map[string]struct{})

	// 1. Optimistically remove spent orders & record trades for completed take/cancel actions
	if hasRes {
		_ = s.book.Locker().Release(res.Outpoints, res.ReservationID)
		for idx, op := range res.Outpoints {
			prev := s.book.RemoveOrder(op)
			if prev != nil {
				dirtyPairs[prev.PairKey] = struct{}{}
			}
			if (action == "take_ask" || action == "take_bid") && idx < len(res.Legs) {
				leg := res.Legs[idx]
				settleTxid := ""
				if len(txids) > 0 {
					settleTxid = txids[len(txids)-1]
				}
				tr := TradeRecord{
					TradeKey:      strings.ToLower(settleTxid) + ":" + op,
					Txid:          strings.ToLower(settleTxid),
					OrderOutpoint: op,
					PairKey:       res.PairKey,
					Side:          res.Side,
					MakerAddr:     leg.MakerAddr,
					TakerAddr:     res.TakerAddr,
					PriceNum:      leg.PriceNum,
					PriceDen:      leg.PriceDen,
					Price:         leg.Price,
					FillToken:     leg.FillToken,
					SatsAmount:    leg.SatsAmount,
					FeeSats:       leg.FeeSats,
					IsPartial:     leg.IsPartial,
					Height:        0,
					IsMempool:     true,
					TradeTime:     now,
				}
				s.recordAndBroadcastTrade(tr)
				if s.persistDB {
					st := "filled"
					if leg.IsPartial {
						st = "partial_filled"
					}
					_ = models.OrderbookDao().UpdateOrderStatus(op, st, strings.ToLower(settleTxid), 0, 1, now)
				}
			} else if (action == "cancel_ask" || action == "cancel_bid") && s.persistDB {
				cancelTxid := ""
				if len(txids) > 0 {
					cancelTxid = strings.ToLower(txids[len(txids)-1])
				}
				_ = models.OrderbookDao().UpdateOrderStatus(op, "cancelled", cancelTxid, 0, 1, now)
			}
		}
	}

	// 2. Optimistically insert new / continuation order UTXOs into the live 0-conf book
	for _, no := range newOrders {
		lo, err := ParseLiveOrder(no.Txid, no.Vout, no.Satoshis, no.ScriptHex, 0, true, now, s.net)
		if err != nil {
			continue
		}
		if no.TokenTxid != "" {
			lo.TokenTxid = strings.ToLower(no.TokenTxid)
			lo.TokenVout = no.TokenVout
		}
		s.book.UpsertOrder(lo)
		dirtyPairs[lo.PairKey] = struct{}{}
		if s.persistDB {
			_ = models.OrderbookDao().UpsertOrder(liveOrderToModel(lo, "open", "", now))
		}
		s.hub.Broadcast("orders:"+strings.ToLower(lo.MakerAddr), WSMessage{
			Channel: "orders",
			Address: lo.MakerAddr,
			Pair:    lo.PairKey,
			Data:    map[string]interface{}{"event": "created", "order": lo},
			Ts:      now,
		})
	}

	for pk := range dirtyPairs {
		s.broadcastDepth(pk)
	}
}

func (s *Syncer) recordAndBroadcastTrade(tr TradeRecord) {
	s.book.RecordTrade(tr)
	if s.persistDB {
		_ = models.OrderbookDao().UpsertTrade(&models.OrderbookTradeModel{
			TradeKey:      tr.TradeKey,
			Txid:          tr.Txid,
			OrderOutpoint: tr.OrderOutpoint,
			PairKey:       tr.PairKey,
			Side:          tr.Side,
			MakerAddr:     tr.MakerAddr,
			TakerAddr:     tr.TakerAddr,
			PriceNum:      tr.PriceNum,
			PriceDen:      tr.PriceDen,
			PriceDec:      tr.Price,
			FillToken:     tr.FillToken,
			SatsAmount:    tr.SatsAmount,
			FeeSats:       tr.FeeSats,
			IsPartial:     boolToInt(tr.IsPartial),
			Height:        tr.Height,
			IsMempool:     boolToInt(tr.IsMempool),
			TradeTime:     tr.TradeTime,
		})
	}
	s.hub.Broadcast("trades:"+tr.PairKey, WSMessage{
		Channel: "trades",
		Pair:    tr.PairKey,
		Data:    tr,
		Ts:      tr.TradeTime,
	})
}

func (s *Syncer) broadcastDepth(pairKey string) {
	depth := s.book.GetDepth(pairKey, 0, 50)
	s.hub.Broadcast("depth:"+pairKey, WSMessage{
		Channel: "depth",
		Pair:    pairKey,
		Data:    depth,
		Ts:      depth.UpdatedAt,
	})
}

func (s *Syncer) configuredCodehashes() []string {
	out := make([]string, 0, 2)
	if h := strings.ToLower(strings.TrimSpace(s.cfg.AskCodeHash)); len(h) == 40 {
		out = append(out, h)
	}
	if h := strings.ToLower(strings.TrimSpace(s.cfg.BidCodeHash)); len(h) == 40 {
		out = append(out, h)
	}
	return out
}

func (s *Syncer) fetchHigunUTXOs(ctx context.Context, codehash, cursor string, limit int) (*HigunOrderbookUTXOsPage, error) {
	base := strings.TrimRight(s.cfg.HigunUrl, "/")
	q := url.Values{}
	q.Set("codehash", codehash)
	q.Set("spent", "false")
	q.Set("includeMempool", "1")
	q.Set("limit", fmt.Sprintf("%d", limit))
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/orderbook/utxos?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("higun /orderbook/utxos status=%d body=%s", resp.StatusCode, string(body))
	}
	var page HigunOrderbookUTXOsPage
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

func (s *Syncer) fetchHigunEvents(ctx context.Context, afterSeq uint64, limit int) (*HigunOrderbookEventsPage, error) {
	base := strings.TrimRight(s.cfg.HigunUrl, "/")
	q := url.Values{}
	q.Set("afterSeq", fmt.Sprintf("%d", afterSeq))
	q.Set("limit", fmt.Sprintf("%d", limit))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/orderbook/events?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("higun /orderbook/events status=%d body=%s", resp.StatusCode, string(body))
	}
	var page HigunOrderbookEventsPage
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

func liveOrderToModel(lo *LiveOrder, status, spentBy string, now int64) *models.OrderbookOrderModel {
	return &models.OrderbookOrderModel{
		Outpoint:      lo.Outpoint,
		Txid:          lo.Txid,
		Vout:          lo.Vout,
		TokenTxid:     lo.TokenTxid,
		TokenVout:     lo.TokenVout,
		PairKey:       lo.PairKey,
		TokenCodeHash: lo.TokenCodeHash,
		TokenID:       lo.TokenID,
		Side:          lo.Side,
		MakerAddr:     lo.MakerAddr,
		PriceNum:      lo.PriceNum,
		PriceDen:      lo.PriceDen,
		PriceDec:      lo.Price,
		RemainAmount:  lo.RemainAmount,
		MinFillAmount: lo.MinFillAmount,
		FeeBps:        int(lo.FeeBps),
		FeeAddr:       lo.FeeAddr,
		SaltHex:       lo.SaltHex,
		Status:        status,
		SpentBy:       spentBy,
		Height:        lo.Height,
		IsMempool:     boolToInt(lo.IsMempool),
		CreatedAt:     lo.CreatedAt,
		UpdatedAt:     now,
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// =========================================================================
// Global Orderbook Service Singleton
// =========================================================================

var (
	globalMu     sync.RWMutex
	globalSyncer *Syncer
)

func InitOrderbookService() *Syncer {
	globalMu.Lock()
	defer globalMu.Unlock()
	if globalSyncer != nil {
		return globalSyncer
	}
	cfg := conf.OrderbookConfigData
	if cfg == nil {
		cfg = conf.DefaultOrderbookConfig()
	}
	locker := NewRedisSoftLockerFromConf()
	book := NewOrderBook(conf.Net, locker)
	hub := NewWSHub()
	s := NewSyncer(cfg, conf.Net, book, hub, true)
	globalSyncer = s
	if cfg.Enabled {
		s.Start()
	}
	return s
}

func GetOrderbookSyncer() *Syncer {
	globalMu.RLock()
	s := globalSyncer
	globalMu.RUnlock()
	if s != nil {
		return s
	}
	return InitOrderbookService()
}

// SetOrderbookSyncerForTest swaps the global syncer (used by controller tests).
func SetOrderbookSyncerForTest(s *Syncer) {
	globalMu.Lock()
	globalSyncer = s
	globalMu.Unlock()
}
