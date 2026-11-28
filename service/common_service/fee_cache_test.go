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

func resetFeeCacheForTest() {
	feeMu.Lock()
	defer feeMu.Unlock()
	feeLastGood = nil
	feeFetchedAt = time.Time{}
	feeLastFailureAt = time.Time{}
	feeRefreshing = false
}

func setupFeeTest(t *testing.T, handler http.HandlerFunc) (hits *int32, advance func(time.Duration), cleanup func()) {
	t.Helper()
	hits = new(int32)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(hits, 1)
		handler(w, r)
	}))

	oldMempoolDomain := conf.MempoolSpace
	oldNowFunc := feeNowFunc
	oldFreshTTL := feeFreshTTL
	oldBackoff := feeRetryBackoff
	oldStaleTTL := feeStaleTTL

	conf.MempoolSpace = server.URL
	now := time.Unix(1700000000, 0)
	feeNowFunc = func() time.Time { return now }
	feeFreshTTL = 5 * time.Minute
	feeRetryBackoff = 60 * time.Second
	feeStaleTTL = 30 * time.Minute
	resetFeeCacheForTest()
	mempool_space_service.ResetGuardForTest()

	advance = func(d time.Duration) { now = now.Add(d) }
	cleanup = func() {
		conf.MempoolSpace = oldMempoolDomain
		feeNowFunc = oldNowFunc
		feeFreshTTL = oldFreshTTL
		feeRetryBackoff = oldBackoff
		feeStaleTTL = oldStaleTTL
		resetFeeCacheForTest()
		mempool_space_service.ResetGuardForTest()
		server.Close()
	}
	return hits, advance, cleanup
}

func writeFees(w http.ResponseWriter) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"fastestFee":10,"halfHourFee":8,"hourFee":6,"economyFee":3,"minimumFee":2}`))
}

func TestFeeSummarySuccess(t *testing.T) {
	hits, _, cleanup := setupFeeTest(t, func(w http.ResponseWriter, r *http.Request) { writeFees(w) })
	defer cleanup()

	fees, err := GetFeeSummary()
	if err != nil || fees == nil {
		t.Fatalf("GetFeeSummary got (%+v, %v)", fees, err)
	}
	if fees.FastestFee != 10 || fees.HalfHourFee != 8 || fees.HourFee != 6 || fees.EconomyFee != 3 || fees.MinimumFee != 2 {
		t.Fatalf("unexpected fees: %+v", fees)
	}
	if got := atomic.LoadInt32(hits); got != 1 {
		t.Fatalf("upstream hits=%d, want 1", got)
	}
}

func TestFeeSummaryFailureServesLastGood(t *testing.T) {
	fail := int32(0)
	_, advance, cleanup := setupFeeTest(t, func(w http.ResponseWriter, r *http.Request) {
		if atomic.LoadInt32(&fail) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		writeFees(w)
	})
	defer cleanup()

	first, err := GetFeeSummary()
	if err != nil {
		t.Fatalf("initial GetFeeSummary err=%v", err)
	}
	atomic.StoreInt32(&fail, 1)
	advance(feeFreshTTL + time.Second)
	mempool_space_service.ResetGuardForTest()
	second, err := GetFeeSummary()
	if err != nil || second == nil || *second != *first {
		t.Fatalf("stale GetFeeSummary got (%+v, %v), want %+v", second, err, first)
	}
}

func TestFeeSummaryFailureColdReturnsError(t *testing.T) {
	_, _, cleanup := setupFeeTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer cleanup()

	fees, err := GetFeeSummary()
	if err == nil || fees != nil {
		t.Fatalf("cold failure got (%+v, %v), want error", fees, err)
	}
	if err.Error() != "fee rate temporarily unavailable" {
		t.Fatalf("err=%q, want fee rate temporarily unavailable", err)
	}
}

func TestFeeSummaryFailureBackoffSkipsUpstream(t *testing.T) {
	hits, advance, cleanup := setupFeeTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer cleanup()

	if _, err := GetFeeSummary(); err == nil {
		t.Fatalf("first cold failure unexpectedly succeeded")
	}
	advance(feeRetryBackoff - time.Second)
	if _, err := GetFeeSummary(); err == nil {
		t.Fatalf("backoff failure unexpectedly succeeded")
	}
	if got := atomic.LoadInt32(hits); got != 1 {
		t.Fatalf("upstream hits=%d, want 1 during backoff", got)
	}
}

func TestFeeSummaryStaleBeyondThirtyMinutesReturnsError(t *testing.T) {
	fail := int32(0)
	_, advance, cleanup := setupFeeTest(t, func(w http.ResponseWriter, r *http.Request) {
		if atomic.LoadInt32(&fail) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		writeFees(w)
	})
	defer cleanup()

	if _, err := GetFeeSummary(); err != nil {
		t.Fatalf("initial GetFeeSummary err=%v", err)
	}
	atomic.StoreInt32(&fail, 1)
	advance(feeStaleTTL + time.Second)
	mempool_space_service.ResetGuardForTest()
	fees, err := GetFeeSummary()
	if err == nil || fees != nil {
		t.Fatalf("expired stale got (%+v, %v), want error", fees, err)
	}
}

func TestFeeSummaryDoesNotMutateCachedStruct(t *testing.T) {
	_, _, cleanup := setupFeeTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"fastestFee":1,"halfHourFee":1,"hourFee":1,"economyFee":1,"minimumFee":1}`))
	})
	defer cleanup()

	first, err := GetFeeSummary()
	if err != nil {
		t.Fatalf("first GetFeeSummary err=%v", err)
	}
	second, err := GetFeeSummary()
	if err != nil {
		t.Fatalf("second GetFeeSummary err=%v", err)
	}
	if first.FastestFee != 2 || first.HalfHourFee != 2 || first.HourFee != 2 || first.MinimumFee != 1 {
		t.Fatalf("first adjusted fees wrong: %+v", first)
	}
	if second.FastestFee != 2 || second.HalfHourFee != 2 || second.HourFee != 2 || second.MinimumFee != 1 {
		t.Fatalf("second adjusted fees wrong: %+v", second)
	}
	feeMu.Lock()
	cached := *feeLastGood
	feeMu.Unlock()
	if cached.FastestFee != 1 || cached.HalfHourFee != 1 || cached.HourFee != 1 {
		t.Fatalf("cached fee was mutated: %+v", cached)
	}
}
