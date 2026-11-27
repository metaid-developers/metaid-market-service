package controller

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"metaid-market-service/conf"
	ob "metaid-market-service/service/orderbook_service"
)

func TestOrderbookHTTPEndpointsAndSoftLockLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tokenID := bytes.Repeat([]byte{0xaa}, 20)
	tokenCodeHash := bytes.Repeat([]byte{0xbb}, 20)
	pairKey := ob.MakePairKey(hex.EncodeToString(tokenCodeHash), hex.EncodeToString(tokenID))
	makerHash := bytes.Repeat([]byte{0x11}, 20)
	feeHash := bytes.Repeat([]byte{0x22}, 20)
	salt := bytes.Repeat([]byte{0x77}, 8)

	askPrefix := []byte{0x01, 0x02, 0x03}
	buildScript := func(remain uint64) string {
		s := append(append([]byte{}, askPrefix...), 0x6a)
		s = append(s, ob.BuildOrderTailHex(ob.SideAsk, makerHash, tokenID, tokenCodeHash, 100, 1, remain, 1000, feeHash, 25, salt)...)
		return hex.EncodeToString(s)
	}

	askTx0 := strings.Repeat("1", 64)
	contTx1 := strings.Repeat("2", 64)
	contScriptHex := buildScript(6000)

	// Mock TS orderbook-service sidecar
	var stepCount int
	sidecarSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/ob/take/ask/prepare":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 0,
				"data": map[string]interface{}{
					"draftId":    "draft-take-1",
					"action":     "take_ask",
					"step":       0,
					"totalSteps": 2,
					"signRequests": []map[string]interface{}{
						{"txIndex": 0, "inputIndex": 0, "txHex": "0a000000", "kind": "p2pkh"},
					},
				},
			})
		case "/internal/ob/finalize":
			stepCount++
			if stepCount == 1 {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"code": 0,
					"data": map[string]interface{}{
						"draftId":    "draft-take-1",
						"action":     "take_ask",
						"status":     "awaiting_signature",
						"step":       1,
						"totalSteps": 2,
					},
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 0,
				"data": map[string]interface{}{
					"draftId":     "draft-take-1",
					"action":      "take_ask",
					"status":      "completed",
					"txids":       []string{strings.Repeat("9", 64), contTx1},
					"verified":    true,
					"broadcasted": true,
					"newOrders": []map[string]interface{}{
						{
							"side":         "ask",
							"txid":         contTx1,
							"vout":         0,
							"satoshis":     546,
							"scriptHex":    contScriptHex,
							"remainAmount": "6000",
							"tokenTxid":    contTx1,
							"tokenVout":    1,
							"saltHex":      hex.EncodeToString(salt),
						},
					},
				},
			})
		case "/internal/ob/place/ask/prepare":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 0,
				"data": map[string]interface{}{"draftId": "draft-place-1", "action": "place_ask"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer sidecarSrv.Close()

	cfg := conf.DefaultOrderbookConfig()
	cfg.ServiceUrl = sidecarSrv.URL
	cfg.FeeAddress = ob.Hash20ToAddress(feeHash, "livenet")
	book := ob.NewOrderBook("livenet", ob.NewMemorySoftLocker())
	lo, err := ob.ParseLiveOrder(askTx0, 0, ob.DustLimit, buildScript(10000), 100, false, 1000, "livenet")
	if err != nil {
		t.Fatalf("ParseLiveOrder: %v", err)
	}
	book.UpsertOrder(lo)
	syncer := ob.NewSyncer(cfg, "livenet", book, ob.NewWSHub(), false)
	ob.SetOrderbookSyncerForTest(syncer)
	defer ob.SetOrderbookSyncerForTest(nil)

	r := gin.New()
	RegisterOrderbookRoutes(r.Group("/api/v1"))

	// 1. GET /depth
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/orderbook/depth?pair="+pairKey+"&precision=0", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"bestAsk":"100"`) {
		t.Fatalf("GET /depth failed: code=%d body=%s", w.Code, w.Body.String())
	}

	// 2. POST /take/quote
	quoteBody, _ := json.Marshal(map[string]interface{}{
		"pairKey":     pairKey,
		"side":        "buy",
		"targetToken": 4000,
	})
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/orderbook/take/quote", bytes.NewReader(quoteBody)))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"filledToken":4000`) {
		t.Fatalf("POST /take/quote failed: code=%d body=%s", w.Code, w.Body.String())
	}

	// 3. POST /take/prepare -> acquires Soft-Lock on askTx0:0
	prepBody, _ := json.Marshal(map[string]interface{}{
		"takerAddress": ob.Hash20ToAddress(bytes.Repeat([]byte{0x33}, 20), "livenet"),
		"pairKey":      pairKey,
		"side":         "buy",
		"targetToken":  4000,
	})
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/orderbook/take/prepare", bytes.NewReader(prepBody)))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"draftId":"draft-take-1"`) {
		t.Fatalf("POST /take/prepare failed: code=%d body=%s", w.Code, w.Body.String())
	}
	if !book.Locker().IsLocked(askTx0 + ":0") {
		t.Fatalf("expected askTx0:0 to be soft-locked after /take/prepare")
	}

	// 4. Concurrent second Taker sees no unlocked liquidity
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/orderbook/take/prepare", bytes.NewReader(prepBody)))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 no available liquidity while soft-locked, got code=%d body=%s", w.Code, w.Body.String())
	}

	// 5. POST /take/submit Step 0 (awaiting_signature -> lock remains held!)
	subBody, _ := json.Marshal(map[string]interface{}{"draftId": "draft-take-1", "signatures": []interface{}{}})
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/orderbook/take/submit", bytes.NewReader(subBody)))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"awaiting_signature"`) {
		t.Fatalf("step 0 submit failed: code=%d body=%s", w.Code, w.Body.String())
	}
	if !book.Locker().IsLocked(askTx0 + ":0") {
		t.Fatalf("lock must remain held during 2-step awaiting_signature")
	}

	// 6. POST /take/submit Step 1 (completed -> optimistic 0-conf update replaces askTx0:0 with contTx1:0 & records trade!)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/orderbook/take/submit", bytes.NewReader(subBody)))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"completed"`) {
		t.Fatalf("step 1 submit failed: code=%d body=%s", w.Code, w.Body.String())
	}
	if book.Locker().IsLocked(askTx0 + ":0") {
		t.Fatalf("lock should be released after completed submit")
	}
	if _, ok := book.GetOrder(askTx0 + ":0"); ok {
		t.Fatalf("old askTx0:0 should be removed optimistically")
	}
	if cont, ok := book.GetOrder(contTx1 + ":0"); !ok || cont.RemainAmount != 6000 {
		t.Fatalf("expected optimistic continuation order contTx1:0 remain=6000, got %+v", cont)
	}

	// 7. GET /trades & /klines & /pairs reflect the optimistic 0-conf trade immediately
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/orderbook/trades?pair="+pairKey, nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"fillToken":4000`) {
		t.Fatalf("GET /trades failed: code=%d body=%s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/orderbook/klines?pair="+pairKey+"&interval=1m", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"volumeToken":4000`) {
		t.Fatalf("GET /klines failed: code=%d body=%s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/orderbook/pairs", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), pairKey) {
		t.Fatalf("GET /pairs failed: code=%d body=%s", w.Code, w.Body.String())
	}
}
