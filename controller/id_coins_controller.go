package controller

import (
	"errors"
	"github.com/gin-gonic/gin"
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
	"metaid-market-service/models"
	"metaid-market-service/service/mrc20_op_service"
	"metaid-market-service/tool"
	"net/http"
	"strconv"
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

// @Summary Refund invalid id coins pre
// @Description Refund invalid id coins pre
// @Produce  json
// @Param Request body request.RefundIdCoinsValidPreRequest true "Request"
// @Tags IdCoins
// @Success 200 {object} respond.RefundIdCoinsValidPreResp ""
// @Router /api/v1/id-coins/invalid/refund/pre [post]
func RefundIdCoinsInvalidLpPre(c *gin.Context) {
	var (
		t            int64 = tool.MakeTimestamp()
		requestModel *request.RefundIdCoinsValidPreRequest
		publicKey    string = ""
	)
	if c.ShouldBindJSON(&requestModel) == nil {
		publicKey = getAuthParams(c)
		responseModel, err := mrc20_op_service.RefundIdCoinsInvalidLpPre(requestModel, publicKey, c.ClientIP())
		if err != nil {
			c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
			return
		}
		c.JSONP(http.StatusOK, respond.RespSuccess(responseModel, tool.MakeTimestamp()-t))
		return
	}
	c.JSONP(http.StatusInternalServerError, respond.RespErr(errors.New("error parameter"), tool.MakeTimestamp()-t, respond.HttpsCodeError))
}

// @Summary Refund invalid id coins commit
// @Description Refund invalid id coins commit
// @Produce  json
// @Param Request body request.RefundIdCoinsValidCommitRequest true "Request"
// @Tags IdCoins
// @Success 200 {object} respond.RefundIdCoinsValidCommitResp ""
// @Router /api/v1/id-coins/invalid/refund/commit [post]
func RefundIdCoinsInvalidLpCommit(c *gin.Context) {
	var (
		t            int64 = tool.MakeTimestamp()
		requestModel *request.RefundIdCoinsValidCommitRequest
		publicKey    string = ""
	)
	if c.ShouldBindJSON(&requestModel) == nil {
		publicKey = getAuthParams(c)
		responseModel, err := mrc20_op_service.RefundIdCoinsInvalidLpCommit(requestModel, publicKey, c.ClientIP())
		if err != nil {
			c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
			return
		}
		c.JSONP(http.StatusOK, respond.RespSuccess(responseModel, tool.MakeTimestamp()-t))
		return
	}
	c.JSONP(http.StatusInternalServerError, respond.RespErr(errors.New("error parameter"), tool.MakeTimestamp()-t, respond.HttpsCodeError))
}

// @Summary Fetch inscribe id-coins orders
// @Description Fetch inscribe id-coins orders
// @Produce  json
// @Tags IdCoins
// @Param opOrderType query string true "default:deploy, mint"
// @Param address query string true "address"
// @Param tickId query string false "tickId"
// @Param cursor query int false "default:0"
// @Param size query int false "default:10, max:50"
// @Success 200 {object} respond.FetchIdCoinsOpOrdersResp ""
// @Router /api/v1/id-coins/inscribe/orders [get]
func FetchIdCoinsOpOrders(c *gin.Context) {
	var (
		t         int64                                = tool.MakeTimestamp()
		publicKey                                      = getAuthParams(c)
		cursorStr string                               = c.DefaultQuery("cursor", "0")
		sizeStr   string                               = c.DefaultQuery("size", "10")
		req       *request.FetchIdCoinsOpOrdersRequest = &request.FetchIdCoinsOpOrdersRequest{
			OpOrderType:  c.DefaultQuery("opOrderType", "deploy"),
			TickId:       c.DefaultQuery("tickId", ""),
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
	resp, err := mrc20_op_service.FetchIdCoinsOpOrders(req, publicKey, c.ClientIP())
	if err != nil {
		c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
		return
	}
	c.JSONP(http.StatusOK, respond.RespSuccess(resp, tool.MakeTimestamp()-t))
	return
}

// @Summary Fetch address id-coins mint orders
// @Description Fetch address id-coins mint orders
// @Produce  json
// @Tags IdCoins
// @Param address query string true "address"
// @Param tickId query string true "tickId"
// @Success 200 {object} respond.FetchOneIdCoinsMintOrderResp ""
// @Router /api/v1/id-coins/address/mint/order [get]
func FetchIdCoinsAddressMintOrder(c *gin.Context) {
	var (
		t         int64                                 = tool.MakeTimestamp()
		publicKey                                       = getAuthParams(c)
		req       *request.FetchIdCoinsMintOrderRequest = &request.FetchIdCoinsMintOrderRequest{
			TickId:  c.DefaultQuery("tickId", ""),
			Address: c.DefaultQuery("address", ""),
		}
	)
	resp, err := mrc20_op_service.FetchIdCoinsAddressMintOrder(req, publicKey, c.ClientIP())
	if err != nil {
		c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
		return
	}
	c.JSONP(http.StatusOK, respond.RespSuccess(resp, tool.MakeTimestamp()-t))
	return
}

// @Summary Fetch id-coins info list
// @Description Fetch id-coins info list
// @Produce  json
// @Tags IdCoins
// @Param address query string false "address"
// @Param searchTick query string false "searchTick"
// @Param orderBy query string false "supply/pool/timestamp, default:timestamp  "
// @Param sortType query int false "default:-1"
// @Param followerAddress query string false "followerAddress"
// @Param cursor query int false "default:0"
// @Param size query int false "default:10, max:50"
// @Success 200 {object} respond.FetchIdCoinsListResp ""
// @Router /api/v1/id-coins/coins-list [get]
func FetchIdCoinsList(c *gin.Context) {
	var (
		t         int64                            = tool.MakeTimestamp()
		publicKey                                  = getAuthParams(c)
		cursorStr string                           = c.DefaultQuery("cursor", "0")
		sizeStr   string                           = c.DefaultQuery("size", "10")
		req       *request.FetchIdCoinsListRequest = &request.FetchIdCoinsListRequest{
			Cursor:          0,
			Size:            0,
			Address:         c.DefaultQuery("address", ""),
			OrderBy:         c.DefaultQuery("orderBy", ""),
			FollowerAddress: c.DefaultQuery("followerAddress", ""),
			SearchTick:      c.DefaultQuery("searchTick", ""),
			Completed:       c.DefaultQuery("completed", ""),
		}
	)
	req.SortType, _ = strconv.Atoi(c.DefaultQuery("sortType", "-1"))
	req.Cursor, _ = strconv.ParseInt(cursorStr, 10, 64)
	req.Size, _ = strconv.ParseInt(sizeStr, 10, 64)
	resp, err := mrc20_op_service.FetchIdCoinsList(req, publicKey, c.ClientIP())
	if err != nil {
		c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
		return
	}
	c.JSONP(http.StatusOK, respond.RespSuccess(resp, tool.MakeTimestamp()-t))
	return
}

// @Summary Fetch id-coins info
// @Description Fetch id-coins info
// @Produce  json
// @Tags IdCoins
// @Param tickId query string true "tickId"
// @Param tick query string false "tick"
// @Param issuerAddress query string false "issuerAddress"
// @Param address query string false "address"
// @Success 200 {object} respond.IdCoinsInfoResp ""
// @Router /api/v1/id-coins/coins-info [get]
func FetchOneIdCoinsInfo(c *gin.Context) {
	var (
		t         int64                           = tool.MakeTimestamp()
		publicKey                                 = getAuthParams(c)
		req       *request.FetchOneIdCoinsRequest = &request.FetchOneIdCoinsRequest{
			TickId:        c.DefaultQuery("tickId", ""),
			Tick:          c.DefaultQuery("tick", ""),
			IssuerAddress: c.DefaultQuery("issuerAddress", ""),
			Address:       c.DefaultQuery("address", ""),
		}
	)

	resp, err := mrc20_op_service.FetchOneIdCoinsInfo(req, publicKey, c.ClientIP())
	if err != nil {
		c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
		return
	}
	c.JSONP(http.StatusOK, respond.RespSuccess(resp, tool.MakeTimestamp()-t))
	return
}

// @Summary Check id-coins deploy check info
// @Description Fetch id-coins info
// @Produce  json
// @Tags IdCoins
// @Param address query string false "address"
// @Success 200 {object} respond.FetchIdCoinsDeployCheckResp ""
// @Router /api/v1/id-coins/deploy/check/info [get]
func FetchIdCoinsDeployCheckInfo(c *gin.Context) {
	var (
		t         int64                                   = tool.MakeTimestamp()
		publicKey                                         = getAuthParams(c)
		req       *request.FetchIdCoinsDeployCheckRequest = &request.FetchIdCoinsDeployCheckRequest{
			Address: c.DefaultQuery("address", ""),
		}
	)

	resp, err := mrc20_op_service.FetchIdCoinsDeployCheckInfo(req, publicKey, c.ClientIP())
	if err != nil {
		c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
		return
	}
	c.JSONP(http.StatusOK, respond.RespSuccess(resp, tool.MakeTimestamp()-t))
	return
}

// @Summary Redeem mint bid order preview
// @Description Redeem mint bid order preview
// @Produce  json
// @Param Request body request.BookTakeMintBidPreviewReq true "Request"
// @Tags IdCoins
// @Success 200 {object} respond.BookMintOrderTakePreviewResp ""
// @Router /api/v1/id-coins/redeem/preview [post]
func BookOrderTakeMintBidPreview(c *gin.Context) {
	var (
		t            int64 = tool.MakeTimestamp()
		requestModel *request.BookTakeMintBidPreviewReq
		publicKey    string = ""
	)
	if c.ShouldBindJSON(&requestModel) == nil {
		publicKey = getAuthParams(c)
		responseModel, err := mrc20_op_service.BookOrderTakeMintBidPreview(requestModel, publicKey, c.ClientIP())
		if err != nil {
			c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
			return
		}
		c.JSONP(http.StatusOK, respond.RespSuccess(responseModel, tool.MakeTimestamp()-t))
		return
	}
	c.JSONP(http.StatusInternalServerError, respond.RespErr(errors.New("error parameter"), tool.MakeTimestamp()-t, respond.HttpsCodeError))
}

// @Summary Redeem mint bid order pre
// @Description Redeem mint bid order pre
// @Produce  json
// @Param Request body request.BookTakeMintBidPreReq true "Request"
// @Tags IdCoins
// @Success 200 {object} respond.BookMintOrderTakePreResp ""
// @Router /api/v1/id-coins/redeem/pre [post]
func BookOrderTakeMintBidPre(c *gin.Context) {
	var (
		t            int64 = tool.MakeTimestamp()
		requestModel *request.BookTakeMintBidPreReq
		publicKey    string = ""
	)
	if c.ShouldBindJSON(&requestModel) == nil {
		publicKey = getAuthParams(c)
		responseModel, err := mrc20_op_service.BookOrderTakeMintBidPre(requestModel, publicKey, c.ClientIP())
		if err != nil {
			c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
			return
		}
		c.JSONP(http.StatusOK, respond.RespSuccess(responseModel, tool.MakeTimestamp()-t))
		return
	}
	c.JSONP(http.StatusInternalServerError, respond.RespErr(errors.New("error parameter"), tool.MakeTimestamp()-t, respond.HttpsCodeError))
}

// @Summary Redeem id coins commit
// @Description Redeem id coins commit
// @Produce  json
// @Param Request body request.BookTakeMintBidCommitReq true "Request"
// @Tags IdCoins
// @Success 200 {object} respond.BookMintOrderTakeCommitResp ""
// @Router /api/v1/id-coins/redeem/commit [post]
func BookOrderTakeMintBidCommit(c *gin.Context) {
	var (
		t            int64 = tool.MakeTimestamp()
		requestModel *request.BookTakeMintBidCommitReq
		publicKey    string = ""
	)
	if c.ShouldBindJSON(&requestModel) == nil {
		publicKey = getAuthParams(c)
		responseModel, err := mrc20_op_service.BookOrderTakeMintBidCommit(requestModel, publicKey, c.ClientIP())
		if err != nil {
			c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
			return
		}
		c.JSONP(http.StatusOK, respond.RespSuccess(responseModel, tool.MakeTimestamp()-t))
		return
	}
	c.JSONP(http.StatusInternalServerError, respond.RespErr(errors.New("error parameter"), tool.MakeTimestamp()-t, respond.HttpsCodeError))
}
