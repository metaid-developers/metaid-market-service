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

// @Summary Inscribe Mrc20 Mint Pre
// @Description Inscribe Mrc20 Mint Pre
// @Produce  json
// @Param Request body request.Mrc20MintPreRequest true "Request"
// @Tags Inscribe-Mrc20
// @Success 200 {object} respond.Mrc20MintPreResp ""
// @Router /api/v1/inscribe/mrc20/mint/pre [post]
func Mrc20MintPre(c *gin.Context) {
	var (
		t            int64  = tool.MakeTimestamp()
		publicKey    string = ""
		requestModel *request.Mrc20MintPreRequest
	)
	if c.ShouldBindJSON(&requestModel) == nil {
		publicKey = getAuthParams(c)
		responseModel, err := mrc20_op_service.Mrc20MintPre(requestModel, publicKey, c.ClientIP())
		if err != nil {
			c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
			return
		}
		c.JSONP(http.StatusOK, respond.RespSuccess(responseModel, tool.MakeTimestamp()-t))
		return
	}
	c.JSONP(http.StatusInternalServerError, respond.RespErr(errors.New("error parameter"), tool.MakeTimestamp()-t, respond.HttpsCodeError))
}

// @Summary Inscribe Mrc20 Mint Commit
// @Description Inscribe Mrc20 Mint Commit
// @Produce  json
// @Param Request body request.Mrc20MintCommitRequest true "Request"
// @Tags Inscribe-Mrc20
// @Success 200 {object} respond.Mrc20MintCommitResp ""
// @Router /api/v1/inscribe/mrc20/mint/commit [post]
func Mrc20MintCommit(c *gin.Context) {
	var (
		t            int64  = tool.MakeTimestamp()
		publicKey    string = ""
		requestModel *request.Mrc20MintCommitRequest
	)
	if c.ShouldBindJSON(&requestModel) == nil {
		publicKey = getAuthParams(c)
		responseModel, err := mrc20_op_service.Mrc20MintCommit(requestModel, publicKey, c.ClientIP())
		if err != nil {
			c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
			return
		}
		c.JSONP(http.StatusOK, respond.RespSuccess(responseModel, tool.MakeTimestamp()-t))
		return
	}
	c.JSONP(http.StatusInternalServerError, respond.RespErr(errors.New("error parameter"), tool.MakeTimestamp()-t, respond.HttpsCodeError))
}
