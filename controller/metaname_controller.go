package controller

import (
	"errors"
	"github.com/gin-gonic/gin"
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
	"metaid-market-service/models"
	"metaid-market-service/service/metaname_service"
	"metaid-market-service/tool"
	"net/http"
	"strconv"
)

// @Summary MetaName register pre
// @Description MetaName register pre
// @Produce  json
// @Param Request body request.RegisterMetaNamePreRequest true "Request"
// @Tags MetaName
// @Success 200 {object} respond.RegisterMetaNamePreResp ""
// @Router /api/v1/metaname/register/pre [post]
func RegisterMetaNamePre(c *gin.Context) {
	var (
		t            int64 = tool.MakeTimestamp()
		requestModel *request.RegisterMetaNamePreRequest
		publicKey    string = ""
	)
	if c.ShouldBindJSON(&requestModel) == nil {
		publicKey = getAuthParams(c)
		responseModel, err := metaname_service.RegisterMetaNamePre(requestModel, publicKey, c.ClientIP())
		if err != nil {
			c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
			return
		}
		c.JSONP(http.StatusOK, respond.RespSuccess(responseModel, tool.MakeTimestamp()-t))
		return
	}
	c.JSONP(http.StatusInternalServerError, respond.RespErr(errors.New("error parameter"), tool.MakeTimestamp()-t, respond.HttpsCodeError))
}

// @Summary MetaName register commit
// @Description MetaName register commit
// @Produce  json
// @Param Request body request.RegisterMetaNameCommitRequest true "Request"
// @Tags MetaName
// @Success 200 {object} respond.RegisterMetaNameCommitResp ""
// @Router /api/v1/metaname/register/commit [post]
func RegisterMetaNameCommit(c *gin.Context) {
	var (
		t            int64 = tool.MakeTimestamp()
		requestModel *request.RegisterMetaNameCommitRequest
		publicKey    string = ""
	)
	if c.ShouldBindJSON(&requestModel) == nil {
		publicKey = getAuthParams(c)
		responseModel, err := metaname_service.RegisterMetaNameCommit(requestModel, publicKey, c.ClientIP())
		if err != nil {
			c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
			return
		}
		c.JSONP(http.StatusOK, respond.RespSuccess(responseModel, tool.MakeTimestamp()-t))
		return
	}
	c.JSONP(http.StatusInternalServerError, respond.RespErr(errors.New("error parameter"), tool.MakeTimestamp()-t, respond.HttpsCodeError))
}

// @Summary Fetch metaname op orders
// @Description Fetch metaname op orders
// @Produce  json
// @Tags MetaName
// @Param address query string true "address"
// @Param cursor query int false "default:0"
// @Param size query int false "default:10, max:50"
// @Success 200 {object} respond.FetchMetaNameOpOrdersResp ""
// @Router /api/v1/metaname/op/orders [get]
func FetchAddressMetaNameOrder(c *gin.Context) {
	var (
		t         int64                                 = tool.MakeTimestamp()
		publicKey                                       = getAuthParams(c)
		cursorStr string                                = c.DefaultQuery("cursor", "0")
		sizeStr   string                                = c.DefaultQuery("size", "10")
		req       *request.FetchMetaNameOpOrdersRequest = &request.FetchMetaNameOpOrdersRequest{
			Cursor:       0,
			Size:         0,
			Address:      c.DefaultQuery("address", ""),
			Confirmation: 0,
		}
	)
	confirmation, _ := strconv.ParseInt(c.DefaultQuery("confirmation", "0"), 10, 64)
	req.Confirmation = models.ConfirmationState(confirmation)
	req.Cursor, _ = strconv.ParseInt(cursorStr, 10, 64)
	req.Size, _ = strconv.ParseInt(sizeStr, 10, 64)
	resp, err := metaname_service.FetchAddressMetaNameOrder(req, publicKey, c.ClientIP())
	if err != nil {
		c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
		return
	}
	c.JSONP(http.StatusOK, respond.RespSuccess(resp, tool.MakeTimestamp()-t))
	return
}
