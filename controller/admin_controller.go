package controller

import (
	"errors"
	"github.com/gin-gonic/gin"
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
	"metaid-market-service/service/admin_service"
	"metaid-market-service/tool"
	"net/http"
)

// @Summary Colddown Market utxo
// @Description Colddown Market utxo
// @Produce  json
// @Param Request body request.ColdDownDummyUtxoRequest true "Request"
// @Tags Market-Admin
// @Success 200 {object} respond.Message ""
// @Router /api/v1/admin/utxo/colddown [post]
func ColdDownDummyUtxo(c *gin.Context) {
	var (
		t            int64 = tool.MakeTimestamp()
		requestModel *request.ColdDownDummyUtxoRequest
	)
	if c.ShouldBindJSON(&requestModel) == nil {
		responseModel, err := admin_service.ColdDownDummyUtxo(requestModel)
		if err != nil {
			c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
			return
		}
		c.JSONP(http.StatusOK, respond.RespSuccess(responseModel, tool.MakeTimestamp()-t))
		return
	}
	c.JSONP(http.StatusInternalServerError, respond.RespErr(errors.New("error parameter"), tool.MakeTimestamp()-t, respond.HttpsCodeError))
}

func AddAutoCreateBridge(c *gin.Context) {
	var (
		t            int64 = tool.MakeTimestamp()
		requestModel *request.AddAutoCreateBridgeRequest
	)
	if c.ShouldBindJSON(&requestModel) == nil {
		responseModel, err := admin_service.AddAutoCreateBridge(requestModel)
		if err != nil {
			c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
			return
		}
		c.JSONP(http.StatusOK, respond.RespSuccess(responseModel, tool.MakeTimestamp()-t))
		return
	}
	c.JSONP(http.StatusInternalServerError, respond.RespErr(errors.New("error parameter"), tool.MakeTimestamp()-t, respond.HttpsCodeError))
}
