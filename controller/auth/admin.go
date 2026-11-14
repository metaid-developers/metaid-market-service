package auth

import (
	"crypto/subtle"
	"errors"
	"metaid-market-service/conf"
	"metaid-market-service/controller/respond"
	"metaid-market-service/tool"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

var ErrAdminDisabled = errors.New("admin API is not configured")
var ErrAdminUnauthorized = errors.New("admin authorization failed")

func AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		t := tool.MakeTimestamp()
		if conf.AdminAPIKey == "" {
			c.JSON(http.StatusServiceUnavailable, respond.RespErr(ErrAdminDisabled, tool.MakeTimestamp()-t, respond.HttpsCodeErrorAuth))
			c.Abort()
			return
		}
		const prefix = "Bearer "
		authorization := c.GetHeader("Authorization")
		if !strings.HasPrefix(authorization, prefix) || subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(authorization, prefix)), []byte(conf.AdminAPIKey)) != 1 {
			c.JSON(http.StatusUnauthorized, respond.RespErr(ErrAdminUnauthorized, tool.MakeTimestamp()-t, respond.HttpsCodeErrorAuth))
			c.Abort()
			return
		}
		c.Next()
	}
}
