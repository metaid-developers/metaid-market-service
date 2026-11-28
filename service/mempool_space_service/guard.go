package mempool_space_service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"metaid-market-service/conf"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Priority int

const (
	Foreground Priority = iota // user request paths: may wait up to fgMaxWait for a limiter slot
	Background                 // cron / taint walk: never waits; no free slot -> ErrUnavailable
)

var ErrUnavailable = errors.New("mempool.space temporarily unavailable")

var (
	httpTimeout      = 5 * time.Second
	fgInterval       = 500 * time.Millisecond
	fgMaxWait        = 1 * time.Second
	bgInterval       = 1 * time.Second
	failureThreshold = 3
	openDuration     = 5 * time.Minute
	nowFunc          = time.Now

	httpClient = &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
)

type guardState struct {
	mu             sync.Mutex
	failures       int
	open           bool
	openedAt       time.Time
	probing        bool
	nextForeground time.Time
	nextBackground time.Time
}

var guard guardState

// Get issues GET <domain><path> through the shared limiter + circuit breaker. path starts with "/".
func Get(path string, prio Priority) (body string, status int, err error) {
	if !strings.HasPrefix(path, "/") {
		return "", 0, fmt.Errorf("mempool.space path must start with /")
	}
	probe, wait, err := reserveSlot(prio)
	if err != nil {
		return "", 0, err
	}
	if wait > 0 {
		time.Sleep(wait)
	}

	domain := strings.TrimRight(conf.MempoolSpace, "/")
	if domain == "" {
		domain = "https://mempool.space"
	}
	ctx, cancel := context.WithTimeout(context.Background(), httpTimeout)
	defer cancel()
	req, buildErr := http.NewRequestWithContext(ctx, http.MethodGet, domain+path, nil)
	if buildErr != nil {
		recordResult(probe, true, buildErr, false)
		return "", 0, ErrUnavailable
	}
	resp, reqErr := httpClient.Do(req)
	if reqErr != nil {
		recordResult(probe, true, reqErr, false)
		return "", 0, ErrUnavailable
	}
	defer resp.Body.Close()
	data, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if readErr != nil {
		recordResult(probe, true, readErr, false)
		return "", resp.StatusCode, ErrUnavailable
	}

	failure := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError
	if failure {
		recordResult(probe, true, fmt.Errorf("mempool.space status %d", resp.StatusCode), resp.StatusCode == http.StatusTooManyRequests)
		if resp.StatusCode == http.StatusTooManyRequests {
			return string(data), resp.StatusCode, ErrUnavailable
		}
		return string(data), resp.StatusCode, fmt.Errorf("mempool.space status %d", resp.StatusCode)
	}
	recordResult(probe, false, nil, false)
	return string(data), resp.StatusCode, nil
}

func reserveSlot(prio Priority) (probe bool, wait time.Duration, err error) {
	now := nowFunc()
	guard.mu.Lock()
	defer guard.mu.Unlock()

	breakerOpen := guard.open
	if breakerOpen {
		if now.Sub(guard.openedAt) < openDuration || guard.probing {
			return false, 0, ErrUnavailable
		}
	}

	interval := fgInterval
	maxWait := fgMaxWait
	next := &guard.nextForeground
	if prio == Background {
		interval = bgInterval
		maxWait = 0
		next = &guard.nextBackground
	}
	slot := *next
	if slot.Before(now) {
		slot = now
	}
	if slot.Sub(now) > maxWait {
		return false, 0, ErrUnavailable
	}
	*next = slot.Add(interval)

	if breakerOpen {
		guard.probing = true
		probe = true
	}
	return probe, slot.Sub(now), nil
}

func recordResult(probe bool, failure bool, cause error, openImmediately bool) {
	now := nowFunc()
	guard.mu.Lock()
	defer guard.mu.Unlock()

	if !failure {
		wasOpen := guard.open || probe
		guard.failures = 0
		guard.open = false
		guard.probing = false
		if wasOpen {
			fmt.Printf("mempool.space circuit breaker closed\n")
		}
		return
	}

	if probe {
		guard.open = true
		guard.openedAt = now
		guard.probing = false
		fmt.Printf("mempool.space circuit breaker opened: %v\n", cause)
		return
	}

	guard.failures++
	if openImmediately {
		guard.failures = failureThreshold
	}
	if guard.failures >= failureThreshold && !guard.open {
		guard.open = true
		guard.openedAt = now
		fmt.Printf("mempool.space circuit breaker opened: %v\n", cause)
	}
}

// ResetGuardForTest resets breaker + limiter state (used by tests in other packages).
func ResetGuardForTest() {
	guard.mu.Lock()
	defer guard.mu.Unlock()
	guard.failures = 0
	guard.open = false
	guard.openedAt = time.Time{}
	guard.probing = false
	guard.nextForeground = time.Time{}
	guard.nextBackground = time.Time{}
}
