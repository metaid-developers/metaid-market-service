package controller

import (
	"errors"
	"github.com/gin-gonic/gin"
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
	"metaid-market-service/service/mrc20_op_service"
	"metaid-market-service/tool"
	"net/http"
	"strconv"
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

// @Summary Inscribe Mrc20 Transfer Pre
// @Description Inscribe Mrc20 Transfer Pre
// @Produce  json
// @Param Request body request.Mrc20TransferPreRequest true "Request"
// @Tags Inscribe-Mrc20
// @Success 200 {object} respond.Mrc20TransferPreResp ""
// @Router /api/v1/inscribe/mrc20/transfer/pre [post]
func Mrc20TransferPre(c *gin.Context) {
	var (
		t            int64  = tool.MakeTimestamp()
		publicKey    string = ""
		requestModel *request.Mrc20TransferPreRequest
	)
	if c.ShouldBindJSON(&requestModel) == nil {
		publicKey = getAuthParams(c)
		responseModel, err := mrc20_op_service.Mrc20TransferPre(requestModel, publicKey, c.ClientIP())
		if err != nil {
			c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
			return
		}
		c.JSONP(http.StatusOK, respond.RespSuccess(responseModel, tool.MakeTimestamp()-t))
		return
	}
	c.JSONP(http.StatusInternalServerError, respond.RespErr(errors.New("error parameter"), tool.MakeTimestamp()-t, respond.HttpsCodeError))
}

// @Summary Inscribe Mrc20 Transfer Commit
// @Description Inscribe Mrc20 Transfer Commit
// @Produce  json
// @Param Request body request.Mrc20TransferCommitRequest true "Request"
// @Tags Inscribe-Mrc20
// @Success 200 {object} respond.Mrc20TransferCommitResp ""
// @Router /api/v1/inscribe/mrc20/transfer/commit [post]
func Mrc20TransferCommit(c *gin.Context) {
	var (
		t            int64  = tool.MakeTimestamp()
		publicKey    string = ""
		requestModel *request.Mrc20TransferCommitRequest
	)
	if c.ShouldBindJSON(&requestModel) == nil {
		publicKey = getAuthParams(c)
		responseModel, err := mrc20_op_service.Mrc20TransferCommit(requestModel, publicKey, c.ClientIP())
		if err != nil {
			c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
			return
		}
		c.JSONP(http.StatusOK, respond.RespSuccess(responseModel, tool.MakeTimestamp()-t))
		return
	}
	c.JSONP(http.StatusInternalServerError, respond.RespErr(errors.New("error parameter"), tool.MakeTimestamp()-t, respond.HttpsCodeError))
}

// @Summary Inscribe Mrc20 deploy commit
// @Description Inscribe Mrc20 deploy commit
// @Produce  json
// @Param Request body request.Mrc20DeployRequest true "Request"
// @Tags Inscribe-Mrc20
// @Success 200 {object} respond.Mrc20DeployResp ""
// @Router /api/v1/inscribe/mrc20/deploy/commit [post]
func Mrc20Deploy(c *gin.Context) {
	var (
		t            int64  = tool.MakeTimestamp()
		publicKey    string = ""
		requestModel *request.Mrc20DeployRequest
	)
	if c.ShouldBindJSON(&requestModel) == nil {
		publicKey = getAuthParams(c)
		responseModel, err := mrc20_op_service.Mrc20Deploy(requestModel, publicKey, c.ClientIP())
		if err != nil {
			c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
			return
		}
		c.JSONP(http.StatusOK, respond.RespSuccess(responseModel, tool.MakeTimestamp()-t))
		return
	}
	c.JSONP(http.StatusInternalServerError, respond.RespErr(errors.New("error parameter"), tool.MakeTimestamp()-t, respond.HttpsCodeError))
}

// @Summary Fetch inscribe mrc20 orders
// @Description Fetch inscribe mrc20 orders
// @Produce  json
// @Tags Inscribe-Mrc20
// @Param opOrderType query string true "default:deploy, mint, transfer"
// @Param address query string true "address"
// @Param tickId query string false "tickId"
// @Param cursor query int false "default:0"
// @Param size query int false "default:10, max:50"
// @Success 200 {object} respond.FetchMrc20OpOrdersResp ""
// @Router /api/v1/inscribe/mrc20/orders [get]
func FetchMrc20OpOrders(c *gin.Context) {
	var (
		t         int64                              = tool.MakeTimestamp()
		publicKey                                    = getAuthParams(c)
		cursorStr string                             = c.DefaultQuery("cursor", "0")
		sizeStr   string                             = c.DefaultQuery("size", "10")
		req       *request.FetchMrc20OpOrdersRequest = &request.FetchMrc20OpOrdersRequest{
			OpOrderType: c.DefaultQuery("opOrderType", "deploy"),
			TickId:      c.DefaultQuery("tickId", ""),
			Cursor:      0,
			Size:        0,
			Address:     c.DefaultQuery("address", ""),
		}
	)

	req.Cursor, _ = strconv.ParseInt(cursorStr, 10, 64)
	req.Size, _ = strconv.ParseInt(sizeStr, 10, 64)
	resp, err := mrc20_op_service.FetchMrc20OpOrders(req, publicKey, c.ClientIP())
	if err != nil {
		c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
		return
	}
	c.JSONP(http.StatusOK, respond.RespSuccess(resp, tool.MakeTimestamp()-t))
	return
}
