package controller

import (
	"bytes"
	"io"
	"metaid-market-service/conf"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type swapRateBucket struct {
	mu       sync.Mutex
	requests map[string][]time.Time
}

var swapWriteLimiter = &swapRateBucket{
	requests: make(map[string][]time.Time),
}

func (b *swapRateBucket) allow(ip string, limit int, window time.Duration) bool {
	now := time.Now()
	cutoff := now.Add(-window)
	b.mu.Lock()
	defer b.mu.Unlock()
	list := b.requests[ip]
	kept := list[:0]
	for _, ts := range list {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}
	if len(kept) >= limit {
		b.requests[ip] = kept
		return false
	}
	b.requests[ip] = append(kept, now)
	return true
}

var swapHTTPClient = &http.Client{
	Timeout: 60 * time.Second,
}

func proxySwapService(c *gin.Context, internalPath string, isWrite bool) {
	if isWrite && !swapWriteLimiter.allow(c.ClientIP(), 30, time.Minute) {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"code":    429,
			"message": "swap rate limit exceeded, please retry shortly",
		})
		return
	}

	base := strings.TrimRight(conf.SwapServiceUrl, "/")
	if base == "" {
		base = "http://127.0.0.1:18080"
	}
	targetURL := base + internalPath
	if rawQuery := c.Request.URL.RawQuery; rawQuery != "" {
		targetURL += "?" + rawQuery
	}

	var bodyReader io.Reader
	if c.Request.Body != nil && c.Request.Method != http.MethodGet {
		bodyBytes, err := io.ReadAll(io.LimitReader(c.Request.Body, 2<<20))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "failed to read request body"})
			return
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, targetURL, bodyReader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "failed to build upstream swap request"})
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := swapHTTPClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"code":    502,
			"message": "swap-service unavailable: " + err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"code": 502, "message": "failed to read swap-service response"})
		return
	}
	c.Data(resp.StatusCode, "application/json; charset=utf-8", respBytes)
}

func SwapListPools(c *gin.Context) {
	proxySwapService(c, "/internal/swap/pools", false)
}

func SwapGetPool(c *gin.Context) {
	proxySwapService(c, "/internal/swap/pools/"+c.Param("poolId"), false)
}

func SwapQuote(c *gin.Context) {
	proxySwapService(c, "/internal/swap/quote", false)
}

func SwapPositions(c *gin.Context) {
	proxySwapService(c, "/internal/swap/positions", false)
}

func SwapHistory(c *gin.Context) {
	proxySwapService(c, "/internal/swap/history", false)
}

func SwapTxStatus(c *gin.Context) {
	proxySwapService(c, "/internal/swap/tx/"+c.Param("txid"), false)
}

func SwapSessionStart(c *gin.Context) {
	proxySwapService(c, "/internal/swap/session/start", true)
}

func SwapSessionSign(c *gin.Context) {
	proxySwapService(c, "/internal/swap/session/"+c.Param("id")+"/sign", true)
}

func SwapSessionCancel(c *gin.Context) {
	proxySwapService(c, "/internal/swap/session/"+c.Param("id")+"/cancel", true)
}
