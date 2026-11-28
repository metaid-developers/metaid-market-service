package mempool_space_service

import (
	"errors"
	"metaid-market-service/conf"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

type guardTestConfig struct {
	t *testing.T
}

func setupGuardTest(t *testing.T, handler http.HandlerFunc) (*httptest.Server, func()) {
	t.Helper()
	server := httptest.NewServer(handler)
	oldDomain := conf.MempoolSpace
	oldHTTPTimeout := httpTimeout
	oldFGInterval := fgInterval
	oldFGMaxWait := fgMaxWait
	oldBGInterval := bgInterval
	oldFailureThreshold := failureThreshold
	oldOpenDuration := openDuration
	oldNowFunc := nowFunc
	oldClientTimeout := httpClient.Timeout

	conf.MempoolSpace = server.URL
	fgInterval = 2 * time.Millisecond
	fgMaxWait = 20 * time.Millisecond
	bgInterval = 30 * time.Millisecond
	failureThreshold = 3
	openDuration = 30 * time.Millisecond
	nowFunc = time.Now
	ResetGuardForTest()

	cleanup := func() {
		conf.MempoolSpace = oldDomain
		httpTimeout = oldHTTPTimeout
		fgInterval = oldFGInterval
		fgMaxWait = oldFGMaxWait
		bgInterval = oldBGInterval
		failureThreshold = oldFailureThreshold
		openDuration = oldOpenDuration
		nowFunc = oldNowFunc
		httpClient.Timeout = oldClientTimeout
		ResetGuardForTest()
		server.Close()
	}
	return server, cleanup
}

func TestGuardPassThroughBodyAndStatus(t *testing.T) {
	_, cleanup := setupGuardTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created-body"))
	})
	defer cleanup()

	body, status, err := Get("/api/test", Foreground)
	if err != nil || status != http.StatusCreated || body != "created-body" {
		t.Fatalf("Get got (%q, %d, %v), want pass-through", body, status, err)
	}
}

func TestGuardOpensAfterThree500sAndSkipsServer(t *testing.T) {
	var hits int32
	_, cleanup := setupGuardTest(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer cleanup()

	for i := 0; i < 3; i++ {
		_, _, err := Get("/api/fail", Foreground)
		if err == nil {
			t.Fatalf("call %d unexpectedly succeeded", i+1)
		}
	}
	if got := atomic.LoadInt32(&hits); got != 3 {
		t.Fatalf("server hits=%d, want 3", got)
	}
	_, _, err := Get("/api/fail", Foreground)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("open breaker err=%v, want ErrUnavailable", err)
	}
	if got := atomic.LoadInt32(&hits); got != 3 {
		t.Fatalf("open breaker hit server, hits=%d", got)
	}
}

func TestGuardOpensAfterTimeoutsAndSkipsServer(t *testing.T) {
	var hits int32
	_, cleanup := setupGuardTest(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		time.Sleep(50 * time.Millisecond)
	})
	defer cleanup()
	httpTimeout = 5 * time.Millisecond
	httpClient.Timeout = 5 * time.Millisecond

	for i := 0; i < 3; i++ {
		_, _, err := Get("/api/slow", Foreground)
		if !errors.Is(err, ErrUnavailable) {
			t.Fatalf("timeout call %d err=%v, want ErrUnavailable", i+1, err)
		}
	}
	_, _, err := Get("/api/slow", Foreground)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("open breaker err=%v, want ErrUnavailable", err)
	}
	if got := atomic.LoadInt32(&hits); got != 3 {
		t.Fatalf("server hits=%d, want 3", got)
	}
}

func TestGuard404DoesNotCountAsFailure(t *testing.T) {
	var hits int32
	_, cleanup := setupGuardTest(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("not found"))
	})
	defer cleanup()

	for i := 0; i < failureThreshold+1; i++ {
		body, status, err := Get("/api/missing", Foreground)
		if err != nil || status != http.StatusNotFound || body != "not found" {
			t.Fatalf("404 call got (%q, %d, %v)", body, status, err)
		}
	}
	if got := atomic.LoadInt32(&hits); got != int32(failureThreshold+1) {
		t.Fatalf("server hits=%d, want %d", got, failureThreshold+1)
	}
}

func TestGuard429OpensImmediately(t *testing.T) {
	var hits int32
	_, cleanup := setupGuardTest(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusTooManyRequests)
	})
	defer cleanup()

	_, _, err := Get("/api/limited", Foreground)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("429 err=%v, want ErrUnavailable", err)
	}
	_, _, err = Get("/api/limited", Foreground)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("open breaker err=%v, want ErrUnavailable", err)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("server hits=%d, want 1", got)
	}
}

func TestGuardHalfOpenSingleProbeSuccessCloses(t *testing.T) {
	var hits int32
	failures := int32(3)
	blockProbe := make(chan struct{})
	releaseProbe := make(chan struct{})
	_, cleanup := setupGuardTest(t, func(w http.ResponseWriter, r *http.Request) {
		hit := atomic.AddInt32(&hits, 1)
		if hit <= failures {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if hit == failures+1 {
			close(blockProbe)
			<-releaseProbe
		}
		_, _ = w.Write([]byte("ok"))
	})
	defer cleanup()

	for i := 0; i < 3; i++ {
		_, _, _ = Get("/api/probe", Foreground)
	}
	time.Sleep(openDuration + 5*time.Millisecond)

	probeResult := make(chan error, 1)
	go func() {
		_, _, err := Get("/api/probe", Foreground)
		probeResult <- err
	}()
	<-blockProbe

	_, _, err := Get("/api/probe", Foreground)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("concurrent half-open call err=%v, want ErrUnavailable", err)
	}
	if got := atomic.LoadInt32(&hits); got != 4 {
		t.Fatalf("hits during probe=%d, want exactly 4", got)
	}
	close(releaseProbe)
	if err := <-probeResult; err != nil {
		t.Fatalf("probe err=%v, want success", err)
	}

	body, status, err := Get("/api/probe", Foreground)
	if err != nil || status != http.StatusOK || body != "ok" {
		t.Fatalf("post-close call got (%q, %d, %v)", body, status, err)
	}
}

func TestGuardHalfOpenProbeFailureReopens(t *testing.T) {
	var hits int32
	_, cleanup := setupGuardTest(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer cleanup()

	for i := 0; i < 3; i++ {
		_, _, _ = Get("/api/probe-fail", Foreground)
	}
	time.Sleep(openDuration + 5*time.Millisecond)
	_, _, err := Get("/api/probe-fail", Foreground)
	if err == nil {
		t.Fatalf("failing probe unexpectedly succeeded")
	}
	if got := atomic.LoadInt32(&hits); got != 4 {
		t.Fatalf("hits after probe=%d, want 4", got)
	}
	_, _, err = Get("/api/probe-fail", Foreground)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("re-opened breaker err=%v, want ErrUnavailable", err)
	}
	if got := atomic.LoadInt32(&hits); got != 4 {
		t.Fatalf("re-opened breaker hit server, hits=%d", got)
	}
}

func TestGuardBackgroundNeverWaits(t *testing.T) {
	var hits int32
	_, cleanup := setupGuardTest(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		_, _ = w.Write([]byte("ok"))
	})
	defer cleanup()

	_, _, err := Get("/api/bg", Background)
	if err != nil {
		t.Fatalf("first background err=%v", err)
	}
	_, _, err = Get("/api/bg", Background)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("second background err=%v, want ErrUnavailable", err)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("server hits=%d, want 1", got)
	}
}

func TestGuardForegroundWaitsWithinMaxAndRejectsBeyond(t *testing.T) {
	var hits int32
	_, cleanup := setupGuardTest(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		_, _ = w.Write([]byte("ok"))
	})
	defer cleanup()
	fgInterval = 10 * time.Millisecond
	fgMaxWait = 25 * time.Millisecond

	_, _, err := Get("/api/fg", Foreground)
	if err != nil {
		t.Fatalf("first foreground err=%v", err)
	}
	start := time.Now()
	_, _, err = Get("/api/fg", Foreground)
	if err != nil {
		t.Fatalf("second foreground err=%v, want slot wait", err)
	}
	if elapsed := time.Since(start); elapsed < 5*time.Millisecond {
		t.Fatalf("second foreground did not appear to wait for slot, elapsed=%s", elapsed)
	}
	if got := atomic.LoadInt32(&hits); got != 2 {
		t.Fatalf("server hits=%d, want 2", got)
	}

	guard.mu.Lock()
	guard.nextForeground = time.Now().Add(fgMaxWait + time.Millisecond)
	guard.mu.Unlock()
	before := atomic.LoadInt32(&hits)
	_, _, err = Get("/api/fg", Foreground)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("over-wait foreground err=%v, want ErrUnavailable", err)
	}
	if got := atomic.LoadInt32(&hits); got != before {
		t.Fatalf("rejected foreground hit server, hits=%d want %d", got, before)
	}
}
