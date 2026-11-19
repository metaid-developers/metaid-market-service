package common_service

import (
	"errors"
	"metaid-market-service/controller/respond"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestMrc20PriceListCacheTTL(t *testing.T) {
	now := time.Unix(1700000000, 0)
	var calls int32

	cache := newMrc20PriceListCache(10*time.Second, func() time.Time { return now })
	loader := func() (*respond.Mrc20TickMarketPriceListResp, error) {
		n := atomic.AddInt32(&calls, 1)
		return &respond.Mrc20TickMarketPriceListResp{Total: int64(n)}, nil
	}

	r1, err := cache.getOrLoad(loader)
	if err != nil || r1.Total != 1 {
		t.Fatalf("first call got (%+v, %v), want Total=1", r1, err)
	}

	now = now.Add(9 * time.Second)
	r2, err := cache.getOrLoad(loader)
	if err != nil || r2.Total != 1 {
		t.Fatalf("cached call got (%+v, %v), want Total=1", r2, err)
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("want 1 loader call within TTL, got %d", calls)
	}

	now = now.Add(2 * time.Second)
	r3, err := cache.getOrLoad(loader)
	if err != nil || r3.Total != 2 {
		t.Fatalf("expired call got (%+v, %v), want Total=2", r3, err)
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("want 2 loader calls after TTL, got %d", calls)
	}
}

func TestMrc20PriceListCacheSingleflight(t *testing.T) {
	now := time.Unix(1700000000, 0)
	var calls int32
	started := make(chan struct{})
	release := make(chan struct{})

	cache := newMrc20PriceListCache(10*time.Second, func() time.Time { return now })
	loader := func() (*respond.Mrc20TickMarketPriceListResp, error) {
		atomic.AddInt32(&calls, 1)
		close(started)
		<-release
		return &respond.Mrc20TickMarketPriceListResp{Total: 42}, nil
	}

	const workers = 20
	var wg sync.WaitGroup
	results := make([]int64, workers)
	errs := make([]error, workers)

	wg.Add(1)
	go func() {
		defer wg.Done()
		resp, err := cache.getOrLoad(loader)
		if resp != nil {
			results[0] = resp.Total
		}
		errs[0] = err
	}()
	<-started

	for i := 1; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			resp, err := cache.getOrLoad(loader)
			if resp != nil {
				results[idx] = resp.Total
			}
			errs[idx] = err
		}(i)
	}

	time.Sleep(20 * time.Millisecond)
	close(release)
	wg.Wait()

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("singleflight want 1 loader call, got %d", got)
	}
	for i := 0; i < workers; i++ {
		if errs[i] != nil || results[i] != 42 {
			t.Fatalf("worker %d got (%d, %v), want (42, nil)", i, results[i], errs[i])
		}
	}
}

func TestMrc20PriceListCacheDoesNotCacheError(t *testing.T) {
	now := time.Unix(1700000000, 0)
	var calls int32
	wantErr := errors.New("db down")

	cache := newMrc20PriceListCache(10*time.Second, func() time.Time { return now })
	loader := func() (*respond.Mrc20TickMarketPriceListResp, error) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			return nil, wantErr
		}
		return &respond.Mrc20TickMarketPriceListResp{Total: 7}, nil
	}

	if _, err := cache.getOrLoad(loader); !errors.Is(err, wantErr) {
		t.Fatalf("first call err=%v, want %v", err, wantErr)
	}
	resp, err := cache.getOrLoad(loader)
	if err != nil || resp == nil || resp.Total != 7 {
		t.Fatalf("second call got (%+v, %v), want Total=7", resp, err)
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("want 2 loader calls, got %d", calls)
	}
}
