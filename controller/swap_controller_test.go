package controller

import (
	"metaid-market-service/conf"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSwapProxyForwardsReadAndWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var gotPath string
	var gotQuery string
	var gotMethod string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"pools":[]}}`))
	}))
	defer upstream.Close()

	prev := conf.SwapServiceUrl
	conf.SwapServiceUrl = upstream.URL
	defer func() { conf.SwapServiceUrl = prev }()

	r := gin.New()
	r.GET("/api/v1/swap/pools", SwapListPools)
	r.POST("/api/v1/swap/quote", SwapQuote)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/swap/pools?refresh=1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if gotPath != "/internal/swap/pools" || gotQuery != "refresh=1" || gotMethod != http.MethodGet {
		t.Fatalf("unexpected upstream request: %s %s?%s", gotMethod, gotPath, gotQuery)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/swap/quote", strings.NewReader(`{"poolId":"space-mvst-test"}`))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK || gotPath != "/internal/swap/quote" || gotMethod != http.MethodPost {
		t.Fatalf("unexpected quote proxy result: code=%d %s %s", w2.Code, gotMethod, gotPath)
	}
}
