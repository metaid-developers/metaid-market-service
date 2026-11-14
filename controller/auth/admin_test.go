package auth

import (
	"metaid-market-service/conf"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAdminMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previous := conf.AdminAPIKey
	defer func() { conf.AdminAPIKey = previous }()

	request := func(key string) *httptest.ResponseRecorder {
		router := gin.New()
		router.Use(AdminMiddleware())
		router.POST("/admin", func(c *gin.Context) { c.Status(http.StatusNoContent) })
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/admin", nil)
		if key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
		router.ServeHTTP(recorder, req)
		return recorder
	}

	conf.AdminAPIKey = ""
	if got := request("").Code; got != http.StatusServiceUnavailable {
		t.Fatalf("empty admin key must fail closed, got %d", got)
	}
	conf.AdminAPIKey = "test-admin-secret"
	if got := request("wrong").Code; got != http.StatusUnauthorized {
		t.Fatalf("wrong admin key must be rejected, got %d", got)
	}
	if got := request("test-admin-secret").Code; got != http.StatusNoContent {
		t.Fatalf("valid admin key must be accepted, got %d", got)
	}
}
