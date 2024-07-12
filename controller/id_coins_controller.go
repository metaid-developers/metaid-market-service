package controller

import (
	"errors"
	"github.com/gin-gonic/gin"
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
	"metaid-market-service/service/mrc20_op_service"
	"metaid-market-service/tool"
	"net/http"
)

// @Summary Deploy id coins pre
// @Description Deploy id coins pre
// @Produce  json
// @Param Request body request.BuildIdCoinsPreRequest true "Request"
// @Tags IdCoins
// @Success 200 {object} respond.BuildIdCoinsPreResp ""
// @Router /api/v1/id-coins/deploy/pre [post]
func BuildIdCoinsPre(c *gin.Context) {
	var (
		t            int64 = tool.MakeTimestamp()
		requestModel *request.BuildIdCoinsPreRequest
		publicKey    string = ""
	)
	if c.ShouldBindJSON(&requestModel) == nil {
		publicKey = getAuthParams(c)
		responseModel, err := mrc20_op_service.BuildIdCoinsPre(requestModel, publicKey, c.ClientIP())
		if err != nil {
			c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
			return
		}
		c.JSONP(http.StatusOK, respond.RespSuccess(responseModel, tool.MakeTimestamp()-t))
		return
	}
	c.JSONP(http.StatusInternalServerError, respond.RespErr(errors.New("error parameter"), tool.MakeTimestamp()-t, respond.HttpsCodeError))
}

// @Summary Deploy id coins commit
// @Description Deploy id coins commit
// @Produce  json
// @Param Request body request.BuildIdCoinsCommitRequest true "Request"
// @Tags IdCoins
// @Success 200 {object} respond.BuildIdCoinsCommitResp ""
// @Router /api/v1/id-coins/deploy/commit [post]
func BuildIdCoinsCommit(c *gin.Context) {
	var (
		t            int64 = tool.MakeTimestamp()
		requestModel *request.BuildIdCoinsCommitRequest
		publicKey    string = ""
	)
	if c.ShouldBindJSON(&requestModel) == nil {
		publicKey = getAuthParams(c)
		responseModel, err := mrc20_op_service.BuildIdCoinsCommit(requestModel, publicKey, c.ClientIP())
		if err != nil {
			c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
			return
		}
		c.JSONP(http.StatusOK, respond.RespSuccess(responseModel, tool.MakeTimestamp()-t))
		return
	}
	c.JSONP(http.StatusInternalServerError, respond.RespErr(errors.New("error parameter"), tool.MakeTimestamp()-t, respond.HttpsCodeError))
}

// @Summary Mint id coins pre
// @Description Mint id coins pre
// @Produce  json
// @Param Request body request.IdCoinsMintPreRequest true "Request"
// @Tags IdCoins
// @Success 200 {object} respond.IdCoinsMintPreResp ""
// @Router /api/v1/id-coins/mint/pre [post]
func IdCoinsMintPre(c *gin.Context) {
	var (
		t            int64 = tool.MakeTimestamp()
		requestModel *request.IdCoinsMintPreRequest
		publicKey    string = ""
	)
	if c.ShouldBindJSON(&requestModel) == nil {
		publicKey = getAuthParams(c)
		responseModel, err := mrc20_op_service.IdCoinsMintPre(requestModel, publicKey, c.ClientIP())
		if err != nil {
			c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
			return
		}
		c.JSONP(http.StatusOK, respond.RespSuccess(responseModel, tool.MakeTimestamp()-t))
		return
	}
	c.JSONP(http.StatusInternalServerError, respond.RespErr(errors.New("error parameter"), tool.MakeTimestamp()-t, respond.HttpsCodeError))
}

// @Summary Mint id coins commit
// @Description Mint id coins commit
// @Produce  json
// @Param Request body request.IdCoinsMintCommitRequest true "Request"
// @Tags IdCoins
// @Success 200 {object} respond.IdCoinsMintCommitResp ""
// @Router /api/v1/id-coins/mint/commit [post]
func IdCoinsMintCommit(c *gin.Context) {
	var (
		t            int64 = tool.MakeTimestamp()
		requestModel *request.IdCoinsMintCommitRequest
		publicKey    string = ""
	)
	if c.ShouldBindJSON(&requestModel) == nil {
		publicKey = getAuthParams(c)
		responseModel, err := mrc20_op_service.IdCoinsMintCommit(requestModel, publicKey, c.ClientIP())
		if err != nil {
			c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
			return
		}
		c.JSONP(http.StatusOK, respond.RespSuccess(responseModel, tool.MakeTimestamp()-t))
		return
	}
	c.JSONP(http.StatusInternalServerError, respond.RespErr(errors.New("error parameter"), tool.MakeTimestamp()-t, respond.HttpsCodeError))
}
