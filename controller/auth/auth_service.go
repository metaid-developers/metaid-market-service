package auth

import (
	"errors"
	"metaid-market-service/controller/respond"
	"metaid-market-service/tool"
	"net/http"

	"github.com/gin-gonic/gin"
)

var (
	AuthErrParams1               error = errors.New("Auth session is empty")
	AuthErrParamsVerifiedSignErr error = errors.New("Auth session is invalid or expired")
)

func AuthSignMiddleware() gin.HandlerFunc {
	return AuthSignMiddlewareWithRepository(redisSessionRepository{})
}

func AuthSignMiddlewareWithRepository(repository SessionRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		t := tool.MakeTimestamp()
		token := c.GetHeader("X-Session")
		if token == "" {
			c.JSON(http.StatusUnauthorized, respond.RespErr(AuthErrParams1, tool.MakeTimestamp()-t, respond.HttpsCodeErrorAuth))
			c.Abort()
			return
		}
		session, err := ValidateSession(token, repository)
		if err != nil {
			c.JSON(http.StatusUnauthorized, respond.RespErr(AuthErrParamsVerifiedSignErr, tool.MakeTimestamp()-t, respond.HttpsCodeErrorAuth))
			c.Abort()
			return
		}
		c.Set("publicKey", session.PublicKey)
		c.Next()
	}
}
