package controller

import (
	"errors"
	"metaid-market-service/controller/auth"
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
	"metaid-market-service/tool"
	"net/http"

	"github.com/gin-gonic/gin"
)

func AuthLogin(c *gin.Context) {
	t := tool.MakeTimestamp()
	var req *request.AuthLoginReq
	if err := c.ShouldBindJSON(&req); err != nil || req == nil || req.PublicKey == "" || req.Signature == "" || req.Nonce == "" {
		c.JSON(http.StatusBadRequest, respond.RespErr(errors.New("publicKey, signature and nonce are required"), tool.MakeTimestamp()-t, respond.HttpsCodeError))
		return
	}
	session, err := auth.Login(req.PublicKey, req.Signature, req.Nonce)
	if err != nil {
		c.JSON(http.StatusUnauthorized, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeErrorAuth))
		return
	}
	c.JSON(http.StatusOK, respond.RespSuccess(session, tool.MakeTimestamp()-t))
}

func AuthLogout(c *gin.Context) {
	t := tool.MakeTimestamp()
	if err := auth.Logout(c.GetHeader("X-Session")); err != nil {
		c.JSON(http.StatusInternalServerError, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
		return
	}
	c.JSON(http.StatusOK, respond.RespSuccess("ok", tool.MakeTimestamp()-t))
}

func AuthChallenge(c *gin.Context) {
	t := tool.MakeTimestamp()
	var req *request.AuthChallengeReq
	if err := c.ShouldBindJSON(&req); err != nil || req == nil || req.PublicKey == "" {
		c.JSON(http.StatusBadRequest, respond.RespErr(errors.New("valid publicKey is required"), tool.MakeTimestamp()-t, respond.HttpsCodeError))
		return
	}
	challenge, err := auth.CreateChallenge(req.PublicKey)
	if err != nil {
		c.JSON(http.StatusBadRequest, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
		return
	}
	c.JSON(http.StatusOK, respond.RespSuccess(challenge, tool.MakeTimestamp()-t))
}
