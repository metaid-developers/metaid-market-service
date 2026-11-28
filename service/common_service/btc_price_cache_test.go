package common_service

import (
	"metaid-market-service/conf"
	"metaid-market-service/service/mempool_space_service"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func resetBtcPriceCacheForTest() {
	btcPriceMu.Lock()
	defer btcPriceMu.Unlock()
	btcLastGood = nil
	btcFetchedAt = time.Time{}
	btcLastFailureAt = time.Time{}
	btcRefreshing = false
}

func setupBtcPriceTest(t *testing.T, mexcHandler, mempoolHandler http.HandlerFunc) (mexcHits, mempoolHits *int32, advance func(time.Duration), cleanup func()) {
	t.Helper()
	mexcHits = new(int32)
	mempoolHits = new(int32)
	mexcServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(mexcHits, 1)
		mexcHandler(w, r)
	}))
	mempoolServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(mempoolHits, 1)
		mempoolHandler(w, r)
	}))

	oldMexcDomain := mexcDomain
	oldMempoolDomain := conf.MempoolSpace
	oldNowFunc := priceNowFunc
	oldFreshTTL := priceFreshTTL
	oldBackoff := priceRetryBackoff
	oldStaleTTL := priceStaleTTL

	mexcDomain = mexcServer.URL
	conf.MempoolSpace = mempoolServer.URL
	now := time.Unix(1700000000, 0)
	priceNowFunc = func() time.Time { return now }
	priceFreshTTL = 60 * time.Second
	priceRetryBackoff = 30 * time.Second
	priceStaleTTL = 30 * time.Minute
	resetBtcPriceCacheForTest()
	mempool_space_service.ResetGuardForTest()

	advance = func(d time.Duration) { now = now.Add(d) }
	cleanup = func() {
		mexcDomain = oldMexcDomain
		conf.MempoolSpace = oldMempoolDomain
		priceNowFunc = oldNowFunc
		priceFreshTTL = oldFreshTTL
		priceRetryBackoff = oldBackoff
		priceStaleTTL = oldStaleTTL
		resetBtcPriceCacheForTest()
		mempool_space_service.ResetGuardForTest()
		mexcServer.Close()
		mempoolServer.Close()
	}
	return mexcHits, mempoolHits, advance, cleanup
}

func writeMexcPrice(w http.ResponseWriter, price string) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"symbol":"BTCUSDT","price":"` + price + `"}`))
}

func TestBtcPriceMexcPrimaryUsed(t *testing.T) {
	mexcHits, mempoolHits, _, cleanup := setupBtcPriceTest(t,
		func(w http.ResponseWriter, r *http.Request) { writeMexcPrice(w, "59772.94") },
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
	defer cleanup()

	price, err := FetchCoinPrice("btc")
	if err != nil || price == nil || price.USD != 59772 {
		t.Fatalf("FetchCoinPrice got (%+v, %v), want USD 59772", price, err)
	}
	if price.Time != 1700000000 {
		t.Fatalf("Time=%d, want fetch unix time", price.Time)
	}
	if got := atomic.LoadInt32(mexcHits); got != 1 {
		t.Fatalf("mexc hits=%d, want 1", got)
	}
	if got := atomic.LoadInt32(mempoolHits); got != 0 {
		t.Fatalf("mempool hits=%d, want 0", got)
	}
}

func TestBtcPriceMexcFailureFallsBackToMempool(t *testing.T) {
	_, mempoolHits, _, cleanup := setupBtcPriceTest(t,
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) },
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"time":1700000001,"USD":60000}`))
		})
	defer cleanup()

	price, err := FetchCoinPrice("btc")
	if err != nil || price == nil || price.USD != 60000 {
		t.Fatalf("FetchCoinPrice got (%+v, %v), want mempool USD 60000", price, err)
	}
	if got := atomic.LoadInt32(mempoolHits); got != 1 {
		t.Fatalf("mempool hits=%d, want 1", got)
	}
}

func TestBtcPriceBothFailServesStale(t *testing.T) {
	fail := int32(0)
	_, _, advance, cleanup := setupBtcPriceTest(t,
		func(w http.ResponseWriter, r *http.Request) {
			if atomic.LoadInt32(&fail) == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			writeMexcPrice(w, "60000.9")
		},
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
	defer cleanup()

	price, err := FetchCoinPrice("btc")
	if err != nil || price.USD != 60000 {
		t.Fatalf("initial price got (%+v, %v)", price, err)
	}
	atomic.StoreInt32(&fail, 1)
	advance(priceFreshTTL + time.Second)
	mempool_space_service.ResetGuardForTest()
	price, err = FetchCoinPrice("btc")
	if err != nil || price == nil || price.USD != 60000 {
		t.Fatalf("stale price got (%+v, %v), want last good", price, err)
	}
}

func TestBtcPriceBothFailColdReturnsError(t *testing.T) {
	_, _, _, cleanup := setupBtcPriceTest(t,
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) },
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
	defer cleanup()

	price, err := FetchCoinPrice("btc")
	if err == nil || price != nil {
		t.Fatalf("cold failure got (%+v, %v), want error", price, err)
	}
}

func TestBtcPriceFailureBackoffSkipsUpstream(t *testing.T) {
	mexcHits, mempoolHits, advance, cleanup := setupBtcPriceTest(t,
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) },
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
	defer cleanup()

	if _, err := FetchCoinPrice("btc"); err == nil {
		t.Fatalf("first cold failure unexpectedly succeeded")
	}
	advance(priceRetryBackoff - time.Second)
	if _, err := FetchCoinPrice("btc"); err == nil {
		t.Fatalf("backoff cold failure unexpectedly succeeded")
	}
	if got := atomic.LoadInt32(mexcHits); got != 1 {
		t.Fatalf("mexc hits=%d, want 1 during backoff", got)
	}
	if got := atomic.LoadInt32(mempoolHits); got != 1 {
		t.Fatalf("mempool hits=%d, want 1 during backoff", got)
	}
}

func TestBtcPriceStaleBeyondThirtyMinutesReturnsError(t *testing.T) {
	fail := int32(0)
	_, _, advance, cleanup := setupBtcPriceTest(t,
		func(w http.ResponseWriter, r *http.Request) {
			if atomic.LoadInt32(&fail) == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			writeMexcPrice(w, "60000")
		},
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
	defer cleanup()

	if _, err := FetchCoinPrice("btc"); err != nil {
		t.Fatalf("initial fetch err=%v", err)
	}
	atomic.StoreInt32(&fail, 1)
	advance(priceStaleTTL + time.Second)
	mempool_space_service.ResetGuardForTest()
	price, err := FetchCoinPrice("btc")
	if err == nil || price != nil {
		t.Fatalf("expired stale got (%+v, %v), want error", price, err)
	}
}
