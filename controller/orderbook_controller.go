package controller

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"metaid-market-service/models"
	ob "metaid-market-service/service/orderbook_service"
)

func newReservationID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

var obHTTPClient = &http.Client{
	Timeout: 60 * time.Second,
}

var obWriteLimiter = &swapRateBucket{
	requests: make(map[string][]time.Time),
}

// RegisterOrderbookRoutes registers all /api/v1/orderbook/* endpoints on a Gin router group.
func RegisterOrderbookRoutes(v1 *gin.RouterGroup) {
	g := v1.Group("/orderbook")
	{
		g.GET("/pairs", OrderbookListPairs)
		g.GET("/depth", OrderbookGetDepth)
		g.GET("/trades", OrderbookGetTrades)
		g.GET("/klines", OrderbookGetKlines)
		g.GET("/orders", OrderbookGetOrders)
		g.GET("/ws", OrderbookWebSocket)

		g.POST("/place/prepare", OrderbookPlacePrepare)
		g.POST("/place/submit", OrderbookFinalizeSubmit)

		g.POST("/take/quote", OrderbookTakeQuote)
		g.POST("/take/prepare", OrderbookTakePrepare)
		g.POST("/take/submit", OrderbookFinalizeSubmit)

		g.POST("/cancel/prepare", OrderbookCancelPrepare)
		g.POST("/cancel/submit", OrderbookFinalizeSubmit)
	}
}

func obOK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    data,
	})
}

func obErr(c *gin.Context, httpStatus int, msg string) {
	c.JSON(httpStatus, gin.H{
		"code":    httpStatus,
		"message": msg,
	})
}

func resolvePairKeyFromQuery(c *gin.Context) string {
	pair := strings.ToLower(strings.TrimSpace(c.Query("pair")))
	if pair != "" {
		return pair
	}
	ch := strings.TrimSpace(c.Query("tokenCodeHash"))
	tid := strings.TrimSpace(c.Query("tokenID"))
	if ch != "" && tid != "" {
		return ob.MakePairKey(ch, tid)
	}
	return ""
}

// GET /api/v1/orderbook/pairs
func OrderbookListPairs(c *gin.Context) {
	s := ob.GetOrderbookSyncer()
	obOK(c, gin.H{
		"pairs":   s.Book().ListPairs(),
		"lastSeq": s.LastSeq(),
	})
}

// GET /api/v1/orderbook/depth?pair=<codehash:tokenId>&precision=12&limit=50
func OrderbookGetDepth(c *gin.Context) {
	pairKey := resolvePairKeyFromQuery(c)
	if pairKey == "" {
		obErr(c, http.StatusBadRequest, "pair (or tokenCodeHash + tokenID) is required")
		return
	}
	precision := ob.DefaultDepthPrecision
	if raw := strings.TrimSpace(c.Query("precision")); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 32)
		if err != nil || v < -8 || v > 12 {
			obErr(c, http.StatusBadRequest, "precision must be an integer between -8 and 12")
			return
		}
		precision = int32(v)
	}
	limit := 50
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			limit = v
		}
	}
	s := ob.GetOrderbookSyncer()
	obOK(c, s.Book().GetDepth(pairKey, precision, limit))
}

// GET /api/v1/orderbook/trades?pair=<codehash:tokenId>&limit=50
func OrderbookGetTrades(c *gin.Context) {
	pairKey := resolvePairKeyFromQuery(c)
	if pairKey == "" {
		obErr(c, http.StatusBadRequest, "pair (or tokenCodeHash + tokenID) is required")
		return
	}
	limit := 50
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			limit = v
		}
	}
	s := ob.GetOrderbookSyncer()
	trades := s.Book().GetRecentTrades(pairKey, limit)
	if len(trades) == 0 {
		// Fall back to MySQL historical trades if available
		if rows, err := models.OrderbookDao().ListRecentTrades(pairKey, limit); err == nil && len(rows) > 0 {
			trades = make([]ob.TradeRecord, 0, len(rows))
			for _, r := range rows {
				trades = append(trades, ob.TradeRecord{
					TradeKey:      r.TradeKey,
					Txid:          r.Txid,
					OrderOutpoint: r.OrderOutpoint,
					PairKey:       r.PairKey,
					Side:          r.Side,
					MakerAddr:     r.MakerAddr,
					TakerAddr:     r.TakerAddr,
					PriceNum:      r.PriceNum,
					PriceDen:      r.PriceDen,
					Price:         r.PriceDec,
					FillToken:     r.FillToken,
					SatsAmount:    r.SatsAmount,
					FeeSats:       r.FeeSats,
					IsPartial:     r.IsPartial != 0,
					Height:        r.Height,
					IsMempool:     r.IsMempool != 0,
					TradeTime:     r.TradeTime,
				})
			}
		}
	}
	obOK(c, gin.H{"pairKey": pairKey, "trades": trades})
}

// GET /api/v1/orderbook/klines?pair=<codehash:tokenId>&interval=15m&limit=200
func OrderbookGetKlines(c *gin.Context) {
	pairKey := resolvePairKeyFromQuery(c)
	if pairKey == "" {
		obErr(c, http.StatusBadRequest, "pair (or tokenCodeHash + tokenID) is required")
		return
	}
	interval := strings.TrimSpace(c.DefaultQuery("interval", "15m"))
	limit := 200
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			limit = v
		}
	}
	s := ob.GetOrderbookSyncer()
	trades := s.Book().GetRecentTrades(pairKey, 500)
	if rows, err := models.OrderbookDao().ListRecentTrades(pairKey, 500); err == nil && len(rows) > len(trades) {
		trades = make([]ob.TradeRecord, 0, len(rows))
		for _, r := range rows {
			trades = append(trades, ob.TradeRecord{
				TradeKey:      r.TradeKey,
				Txid:          r.Txid,
				OrderOutpoint: r.OrderOutpoint,
				PairKey:       r.PairKey,
				Side:          r.Side,
				MakerAddr:     r.MakerAddr,
				TakerAddr:     r.TakerAddr,
				PriceNum:      r.PriceNum,
				PriceDen:      r.PriceDen,
				Price:         r.PriceDec,
				FillToken:     r.FillToken,
				SatsAmount:    r.SatsAmount,
				FeeSats:       r.FeeSats,
				IsPartial:     r.IsPartial != 0,
				Height:        r.Height,
				IsMempool:     r.IsMempool != 0,
				TradeTime:     r.TradeTime,
			})
		}
	}
	bars := ob.BuildKlines(trades, interval, limit)
	obOK(c, gin.H{
		"pairKey":  pairKey,
		"interval": interval,
		"klines":   bars,
	})
}

// GET /api/v1/orderbook/orders?address=<addr>&pair=<pair>&side=ask|bid&status=open|all
func OrderbookGetOrders(c *gin.Context) {
	addr := strings.TrimSpace(c.Query("address"))
	pairKey := resolvePairKeyFromQuery(c)
	status := strings.ToLower(strings.TrimSpace(c.DefaultQuery("status", "open")))
	var sideFilter uint8
	switch strings.ToLower(strings.TrimSpace(c.Query("side"))) {
	case "ask", "1":
		sideFilter = ob.SideAsk
	case "bid", "2":
		sideFilter = ob.SideBid
	}

	s := ob.GetOrderbookSyncer()
	if status == "open" || status == "" {
		orders := s.Book().ListLiveOrders(addr, pairKey, sideFilter)
		obOK(c, gin.H{"orders": orders})
		return
	}

	// Historical orders query from MySQL (falling back to live orders if DB unconfigured)
	rows, err := models.OrderbookDao().ListOrdersByMaker(addr, pairKey, status, 100)
	if err != nil {
		obErr(c, http.StatusInternalServerError, err.Error())
		return
	}
	if len(rows) == 0 && status == "all" {
		obOK(c, gin.H{"orders": s.Book().ListLiveOrders(addr, pairKey, sideFilter)})
		return
	}
	obOK(c, gin.H{"orders": rows})
}

// GET /api/v1/orderbook/ws
func OrderbookWebSocket(c *gin.Context) {
	ob.GetOrderbookSyncer().Hub().ServeHTTP(c.Writer, c.Request)
}

// POST /api/v1/orderbook/take/quote
func OrderbookTakeQuote(c *gin.Context) {
	var req ob.RouteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		obErr(c, http.StatusBadRequest, "invalid quote request: "+err.Error())
		return
	}
	s := ob.GetOrderbookSyncer()
	plan, err := s.Book().RouteTake(req)
	if err != nil {
		obErr(c, http.StatusBadRequest, err.Error())
		return
	}
	obOK(c, plan)
}

// POST /api/v1/orderbook/place/prepare
func OrderbookPlacePrepare(c *gin.Context) {
	if !obWriteLimiter.allow(c.ClientIP(), 60, time.Minute) {
		obErr(c, http.StatusTooManyRequests, "orderbook rate limit exceeded")
		return
	}
	var payload map[string]interface{}
	if err := c.ShouldBindJSON(&payload); err != nil {
		obErr(c, http.StatusBadRequest, "invalid place prepare body: "+err.Error())
		return
	}
	side, _ := payload["side"].(string)
	side = strings.ToLower(strings.TrimSpace(side))
	if sellCH, _ := payload["sellTokenCodeHash"].(string); strings.TrimSpace(sellCH) != "" {
		side = "token"
	}
	if side != "ask" && side != "bid" && side != "token" {
		obErr(c, http.StatusBadRequest, "side must be 'ask', 'bid', or 'token'")
		return
	}

	s := ob.GetOrderbookSyncer()
	if _, ok := payload["feeAddress"]; !ok && s.Config().FeeAddress != "" {
		payload["feeAddress"] = s.Config().FeeAddress
	}
	if _, ok := payload["feeBps"]; !ok {
		if feeAddr, _ := payload["feeAddress"].(string); strings.TrimSpace(feeAddr) != "" {
			payload["feeBps"] = s.Config().FeeBps
		} else {
			payload["feeBps"] = 0
		}
	}

	path := "/internal/ob/place/ask/prepare"
	if side == "bid" {
		path = "/internal/ob/place/bid/prepare"
	} else if side == "token" {
		path = "/internal/ob/place/token/prepare"
	}
	status, raw, err := callOrderbookSidecar(c, s.Config().ServiceUrl, path, payload)
	if err != nil {
		obErr(c, http.StatusBadGateway, "orderbook-service unavailable: "+err.Error())
		return
	}
	c.Data(status, "application/json; charset=utf-8", raw)
}

type takePrepareRequest struct {
	TakerAddress     string   `json:"takerAddress"`
	PairKey          string   `json:"pairKey"`
	TokenCodeHash    string   `json:"tokenCodeHash"`
	TokenID          string   `json:"tokenID"`
	Side             string   `json:"side"` // "buy" (take Ask) | "sell" (take Bid)
	TargetToken      uint64   `json:"targetToken"`
	MaxPriceNum      uint64   `json:"maxPriceNum,omitempty"`
	MaxPriceDen      uint64   `json:"maxPriceDen,omitempty"`
	MinPriceNum      uint64   `json:"minPriceNum,omitempty"`
	MinPriceDen      uint64   `json:"minPriceDen,omitempty"`
	MaxLegs          int      `json:"maxLegs,omitempty"`
	ExcludeOutpoints []string `json:"excludeOutpoints,omitempty"`
	FeeRate          float64  `json:"feeRate,omitempty"`
}

// POST /api/v1/orderbook/take/prepare
func OrderbookTakePrepare(c *gin.Context) {
	if !obWriteLimiter.allow(c.ClientIP(), 60, time.Minute) {
		obErr(c, http.StatusTooManyRequests, "orderbook rate limit exceeded")
		return
	}
	var req takePrepareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		obErr(c, http.StatusBadRequest, "invalid take prepare body: "+err.Error())
		return
	}
	if strings.TrimSpace(req.TakerAddress) == "" {
		obErr(c, http.StatusBadRequest, "takerAddress is required")
		return
	}

	s := ob.GetOrderbookSyncer()
	plan, err := s.Book().RouteTake(ob.RouteRequest{
		PairKey:          req.PairKey,
		TokenCodeHash:    req.TokenCodeHash,
		TokenID:          req.TokenID,
		Side:             req.Side,
		TargetToken:      req.TargetToken,
		MaxPriceNum:      req.MaxPriceNum,
		MaxPriceDen:      req.MaxPriceDen,
		MinPriceNum:      req.MinPriceNum,
		MinPriceDen:      req.MinPriceDen,
		MaxLegs:          req.MaxLegs,
		ExcludeOutpoints: req.ExcludeOutpoints,
	})
	if err != nil {
		obErr(c, http.StatusBadRequest, err.Error())
		return
	}

	outpoints := make([]string, 0, len(plan.Legs))
	for _, leg := range plan.Legs {
		outpoints = append(outpoints, leg.Outpoint)
	}
	reservationID := newReservationID()
	lockTTL := time.Duration(s.Config().SoftLockTTLSec) * time.Second
	if lockTTL <= 0 {
		lockTTL = 25 * time.Second
	}
	if err := s.Book().Locker().Acquire(outpoints, reservationID, lockTTL); err != nil {
		if errors.Is(err, ob.ErrOutpointLocked) {
			obErr(c, http.StatusConflict, err.Error())
			return
		}
		obErr(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Build sidecar request
	var sidecarPath string
	var sidecarBody map[string]interface{}
	if plan.IsTokenPair {
		sidecarPath = "/internal/ob/take/token/prepare"
		legs := make([]map[string]interface{}, 0, len(plan.Legs))
		for _, l := range plan.Legs {
			legs = append(legs, map[string]interface{}{
				"orderTxid":      l.OrderTxid,
				"orderVout":      l.OrderVout,
				"tokenTxid":      l.TokenTxid,
				"tokenVout":      l.TokenVout,
				"fillSellAmount": strconv.FormatUint(l.FillSellAmount, 10),
			})
		}
		sidecarBody = map[string]interface{}{
			"takerAddress":     req.TakerAddress,
			"buyTokenCodeHash": plan.BuyTokenCodeHash,
			"buyTokenID":       plan.BuyTokenID,
			"legs":             legs,
		}
	} else if plan.Side == "buy" {
		sidecarPath = "/internal/ob/take/ask/prepare"
		legs := make([]map[string]interface{}, 0, len(plan.Legs))
		for _, l := range plan.Legs {
			legs = append(legs, map[string]interface{}{
				"orderTxid": l.OrderTxid,
				"orderVout": l.OrderVout,
				"tokenTxid": l.TokenTxid,
				"tokenVout": l.TokenVout,
				"fillToken": strconv.FormatUint(l.FillToken, 10),
			})
		}
		sidecarBody = map[string]interface{}{
			"takerAddress": req.TakerAddress,
			"legs":         legs,
		}
	} else {
		sidecarPath = "/internal/ob/take/bid/prepare"
		parts := strings.SplitN(plan.PairKey, ":", 2)
		tokenCodeHash, tokenID := req.TokenCodeHash, req.TokenID
		if len(parts) == 2 {
			tokenCodeHash, tokenID = parts[0], parts[1]
		}
		legs := make([]map[string]interface{}, 0, len(plan.Legs))
		for _, l := range plan.Legs {
			legs = append(legs, map[string]interface{}{
				"orderTxid": l.OrderTxid,
				"orderVout": l.OrderVout,
				"fillToken": strconv.FormatUint(l.FillToken, 10),
			})
		}
		sidecarBody = map[string]interface{}{
			"takerAddress":  req.TakerAddress,
			"tokenCodeHash": tokenCodeHash,
			"tokenID":       tokenID,
			"legs":          legs,
		}
	}
	if req.FeeRate > 0 {
		sidecarBody["feeRate"] = req.FeeRate
	}

	status, raw, err := callOrderbookSidecar(c, s.Config().ServiceUrl, sidecarPath, sidecarBody)
	if err != nil {
		_ = s.Book().Locker().Release(outpoints, reservationID)
		obErr(c, http.StatusBadGateway, "orderbook-service unavailable: "+err.Error())
		return
	}
	if status != http.StatusOK {
		_ = s.Book().Locker().Release(outpoints, reservationID)
		c.Data(status, "application/json; charset=utf-8", raw)
		return
	}

	var sidecarResp struct {
		Code int                    `json:"code"`
		Data map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(raw, &sidecarResp); err == nil && sidecarResp.Code == 0 && sidecarResp.Data != nil {
		if draftID, ok := sidecarResp.Data["draftId"].(string); ok && draftID != "" {
			s.RegisterDraftReservation(draftID, ob.DraftReservation{
				ReservationID: reservationID,
				Outpoints:     outpoints,
				Side:          plan.Side,
				PairKey:       plan.PairKey,
				TakerAddr:     req.TakerAddress,
				Legs:          plan.Legs,
				CreatedAt:     time.Now(),
			})
		}
		sidecarResp.Data["routePlan"] = plan
		sidecarResp.Data["reservationId"] = reservationID
		obOK(c, sidecarResp.Data)
		return
	}
	c.Data(status, "application/json; charset=utf-8", raw)
}

type cancelPrepareRequest struct {
	MakerAddress string  `json:"makerAddress"`
	Side         string  `json:"side"` // "ask" | "bid"
	OrderTxid    string  `json:"orderTxid"`
	OrderVout    int     `json:"orderVout"`
	TokenTxid    string  `json:"tokenTxid,omitempty"`
	TokenVout    *int    `json:"tokenVout,omitempty"`
	FeeRate      float64 `json:"feeRate,omitempty"`
}

// POST /api/v1/orderbook/cancel/prepare
func OrderbookCancelPrepare(c *gin.Context) {
	if !obWriteLimiter.allow(c.ClientIP(), 60, time.Minute) {
		obErr(c, http.StatusTooManyRequests, "orderbook rate limit exceeded")
		return
	}
	var req cancelPrepareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		obErr(c, http.StatusBadRequest, "invalid cancel prepare body: "+err.Error())
		return
	}
	req.Side = strings.ToLower(strings.TrimSpace(req.Side))
	if req.MakerAddress == "" || req.OrderTxid == "" || (req.Side != "ask" && req.Side != "bid" && req.Side != "token") {
		obErr(c, http.StatusBadRequest, "makerAddress, side ('ask'|'bid'|'token'), and orderTxid are required")
		return
	}

	s := ob.GetOrderbookSyncer()
	outpoint := fmt.Sprintf("%s:%d", strings.ToLower(req.OrderTxid), req.OrderVout)

	// Auto-detect OrderTokenV2 orders and fill tokenTxid/tokenVout from live book if omitted
	tokenTxid := req.TokenTxid
	tokenVout := -1
	if req.TokenVout != nil {
		tokenVout = *req.TokenVout
	}
	if lo, ok := s.Book().GetOrder(outpoint); ok {
		if lo.IsTokenOrder {
			req.Side = "token"
		}
		if (req.Side == "ask" || req.Side == "token") && (tokenTxid == "" || tokenVout < 0) {
			tokenTxid = lo.TokenTxid
			tokenVout = lo.TokenVout
		}
	} else if (req.Side == "ask" || req.Side == "token") && (tokenTxid == "" || tokenVout < 0) {
		tokenTxid = req.OrderTxid
		tokenVout = req.OrderVout + 1
	}

	reservationID := newReservationID()
	lockTTL := time.Duration(s.Config().SoftLockTTLSec) * time.Second
	if err := s.Book().Locker().Acquire([]string{outpoint}, reservationID, lockTTL); err != nil {
		if errors.Is(err, ob.ErrOutpointLocked) {
			obErr(c, http.StatusConflict, err.Error())
			return
		}
		obErr(c, http.StatusInternalServerError, err.Error())
		return
	}

	body := map[string]interface{}{
		"makerAddress": req.MakerAddress,
		"side":         req.Side,
		"orderTxid":    req.OrderTxid,
		"orderVout":    req.OrderVout,
	}
	if req.Side == "ask" || req.Side == "token" {
		body["tokenTxid"] = tokenTxid
		body["tokenVout"] = tokenVout
	}
	if req.FeeRate > 0 {
		body["feeRate"] = req.FeeRate
	}

	status, raw, err := callOrderbookSidecar(c, s.Config().ServiceUrl, "/internal/ob/cancel/prepare", body)
	if err != nil {
		_ = s.Book().Locker().Release([]string{outpoint}, reservationID)
		obErr(c, http.StatusBadGateway, "orderbook-service unavailable: "+err.Error())
		return
	}
	if status != http.StatusOK {
		_ = s.Book().Locker().Release([]string{outpoint}, reservationID)
		c.Data(status, "application/json; charset=utf-8", raw)
		return
	}

	var sidecarResp struct {
		Code int                    `json:"code"`
		Data map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(raw, &sidecarResp); err == nil && sidecarResp.Code == 0 && sidecarResp.Data != nil {
		if draftID, ok := sidecarResp.Data["draftId"].(string); ok && draftID != "" {
			s.RegisterDraftReservation(draftID, ob.DraftReservation{
				ReservationID: reservationID,
				Outpoints:     []string{outpoint},
				Side:          req.Side,
				CreatedAt:     time.Now(),
			})
		}
	}
	c.Data(status, "application/json; charset=utf-8", raw)
}

// POST /api/v1/orderbook/{place,take,cancel}/submit
// Proxies user signatures to sidecar /internal/ob/finalize and applies optimistic 0-conf book updates.
func OrderbookFinalizeSubmit(c *gin.Context) {
	if !obWriteLimiter.allow(c.ClientIP(), 90, time.Minute) {
		obErr(c, http.StatusTooManyRequests, "orderbook rate limit exceeded")
		return
	}
	var payload map[string]interface{}
	if err := c.ShouldBindJSON(&payload); err != nil {
		obErr(c, http.StatusBadRequest, "invalid submit body: "+err.Error())
		return
	}
	draftID, _ := payload["draftId"].(string)
	if strings.TrimSpace(draftID) == "" {
		obErr(c, http.StatusBadRequest, "draftId is required")
		return
	}

	s := ob.GetOrderbookSyncer()
	status, raw, err := callOrderbookSidecar(c, s.Config().ServiceUrl, "/internal/ob/finalize", payload)
	if err != nil {
		obErr(c, http.StatusBadGateway, "orderbook-service unavailable: "+err.Error())
		return
	}

	if status == http.StatusOK {
		var parsed struct {
			Code int `json:"code"`
			Data struct {
				DraftID   string                  `json:"draftId"`
				Action    string                  `json:"action"`
				Status    string                  `json:"status"`
				Txids     []string                `json:"txids"`
				NewOrders []ob.OptimisticNewOrder `json:"newOrders"`
			} `json:"data"`
		}
		if err := json.Unmarshal(raw, &parsed); err == nil && parsed.Code == 0 {
			s.HandleFinalizeResult(draftID, parsed.Data.Action, parsed.Data.Status, parsed.Data.Txids, parsed.Data.NewOrders)
		}
	} else {
		var errResp struct {
			ConflictingOutpoints []string `json:"conflictingOutpoints"`
			SettledOutpoints     []string `json:"settledOutpoints"`
		}
		if err := json.Unmarshal(raw, &errResp); err == nil {
			if len(errResp.SettledOutpoints) > 0 {
				s.EvictConflictingOutpoints(errResp.SettledOutpoints, "settled-partial")
			}
			if len(errResp.ConflictingOutpoints) > 0 {
				s.EvictConflictingOutpoints(errResp.ConflictingOutpoints, "mempool-conflict")
			}
		}
		s.HandleFinalizeResult(draftID, "", "failed", nil, nil)
	}

	c.Data(status, "application/json; charset=utf-8", raw)
}

func callOrderbookSidecar(c *gin.Context, baseURL, path string, body interface{}) (int, []byte, error) {
	base := strings.TrimRight(baseURL, "/")
	if base == "" {
		base = "http://127.0.0.1:18082"
	}
	b, err := json.Marshal(body)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, base+path, bytes.NewReader(b))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := obHTTPClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, respBytes, nil
}
