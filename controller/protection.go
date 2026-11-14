package controller

import (
	"errors"
	"metaid-market-service/controller/respond"
	"metaid-market-service/tool"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const maxRequestBodyBytes = 4 << 20

func RequestProtection() gin.HandlerFunc {
	type window struct {
		second int64
		count  int
	}
	windows := make(map[string]window)
	var mu sync.Mutex
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBodyBytes)
		limit := 60
		path := c.Request.URL.Path
		if strings.Contains(path, "/take") || strings.Contains(path, "/commit") || strings.Contains(path, "/broadcast") || strings.Contains(path, "/admin") || strings.HasSuffix(path, "/psbt") {
			limit = 15
		}
		now := time.Now().Unix()
		key := c.ClientIP() + path
		mu.Lock()
		entry := windows[key]
		if entry.second != now {
			for k, v := range windows {
				if v.second < now-2 {
					delete(windows, k)
				}
			}
			entry = window{second: now}
		}
		entry.count++
		windows[key] = entry
		allowed := entry.count <= limit
		mu.Unlock()
		if !allowed {
			t := tool.MakeTimestamp()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, respond.RespErr(errors.New("rate limit exceeded"), tool.MakeTimestamp()-t, respond.HttpsCodeError))
			return
		}
		c.Next()
	}
}
