package controller

import (
	"errors"
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
	"metaid-market-service/models"
	"metaid-market-service/service/order_service"
	"metaid-market-service/tool"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// @Summary Push Market mrc20 order
// @Description Push Market mrc20 order
// @Produce  json
// @Param Request body request.PushMrc20OrderReq true "Request"
// @Tags Market-Order
// @Success 200 {object} respond.PushMrc20OrderResp ""
// @Router /api/v1/market/mrc20/order/push [post]
func PushMarketMrc20Order(c *gin.Context) {
	var (
		t            int64 = tool.MakeTimestamp()
		requestModel *request.PushMrc20OrderReq
		publicKey    string = ""
	)
	if c.ShouldBindJSON(&requestModel) == nil {
		publicKey = getAuthParams(c)
		responseModel, err := order_service.PushMarketMrc20Order(requestModel, publicKey, c.ClientIP())
		if err != nil {
			c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
			return
		}
		c.JSONP(http.StatusOK, respond.RespSuccess(responseModel, tool.MakeTimestamp()-t))
		return
	}
	c.JSONP(http.StatusInternalServerError, respond.RespErr(errors.New("error parameter"), tool.MakeTimestamp()-t, respond.HttpsCodeError))
}

// @Summary Fetch market mrc20 order psbt
// @Description Fetch market mrc20 order psbt
// @Produce  json
// @Tags Market-Order
// @Param orderId query string true "orderId"
// @Param buyerAddress query string true "buyerAddress"
// @Param buyerChangeAmount query int false "buyerChangeAmount"
// @Success 200 {object} respond.Mrc20OrderInfo ""
// @Router /api/v1/market/mrc20/order/psbt [get]
func FetchMrc20OrderPsbt(c *gin.Context) {
	var (
		t                    int64                           = tool.MakeTimestamp()
		publicKey                                            = getAuthParams(c)
		buyerChangeAmountStr string                          = c.DefaultQuery("buyerChangeAmount", "0")
		req                  *request.FetchMrc20OrderPsbtReq = &request.FetchMrc20OrderPsbtReq{
			OrderId:           c.DefaultQuery("orderId", ""),
			BuyerAddress:      c.DefaultQuery("buyerAddress", ""),
			BuyerChangeAmount: 0,
		}
	)
	req.BuyerChangeAmount, _ = strconv.ParseUint(buyerChangeAmountStr, 10, 64)
	resp, err := order_service.FetchMrc20OrderPsbt(req, publicKey, c.ClientIP())
	if err != nil {
		c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
		return
	}
	c.JSONP(http.StatusOK, respond.RespSuccess(resp, tool.MakeTimestamp()-t))
	return
}

// @Summary Take Market mrc20 order
// @Description Take Market mrc20 order
// @Produce  json
// @Param Request body request.TakeMrc20OrderReq true "Request"
// @Tags Market-Order
// @Success 200 {object} respond.TakeMrc20OrderResp ""
// @Router /api/v1/market/mrc20/order/take [post]
func TakeMarketMrc20Order(c *gin.Context) {
	var (
		t            int64 = tool.MakeTimestamp()
		requestModel *request.TakeMrc20OrderReq
		publicKey    string = ""
	)
	if c.ShouldBindJSON(&requestModel) == nil {
		publicKey = getAuthParams(c)
		responseModel, err := order_service.TakeMarketMrc20Order(requestModel, publicKey, c.ClientIP())
		if err != nil {
			c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
			return
		}
		c.JSONP(http.StatusOK, respond.RespSuccess(responseModel, tool.MakeTimestamp()-t))
		return
	}
	c.JSONP(http.StatusInternalServerError, respond.RespErr(errors.New("error parameter"), tool.MakeTimestamp()-t, respond.HttpsCodeError))
}

// @Summary Cancel Market mrc20 order
// @Description Cancel Market mrc20 order
// @Produce  json
// @Param Request body request.CancelMrc20OrderReq true "Request"
// @Tags Market-Order
// @Success 200 {object} respond.CancelMrc20OrderResp ""
// @Router /api/v1/market/mrc20/order/cancel [post]
func CancelMarketMrc20Order(c *gin.Context) {
	var (
		t            int64 = tool.MakeTimestamp()
		requestModel *request.CancelMrc20OrderReq
		publicKey    string = ""
	)
	if c.ShouldBindJSON(&requestModel) == nil {
		publicKey = getAuthParams(c)
		responseModel, err := order_service.CancelMarketMrc20Order(requestModel, publicKey, c.ClientIP())
		if err != nil {
			c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
			return
		}
		c.JSONP(http.StatusOK, respond.RespSuccess(responseModel, tool.MakeTimestamp()-t))
		return
	}
	c.JSONP(http.StatusInternalServerError, respond.RespErr(errors.New("error parameter"), tool.MakeTimestamp()-t, respond.HttpsCodeError))
}

// @Summary Fetch market mrc20 orders
// @Description Fetch market mrc20 orders
// @Produce  json
// @Tags Market-Order
// @Param assetType query string true "mrc20"
// @Param orderState query int true "1-create, 2-cancel, 3-finish"
// @Param tickId query string false "tickId"
// @Param address query string false "address"
// @Param sortKey query string false "priceAmount/timestamp/tokenPriceRate, default:timestamp"
// @Param sortType query int false "-1/1, default:-1"
// @Param cursor query int false "default:0"
// @Param size query int false "default:10, max:50"
// @Success 200 {object} respond.Mrc20OrderListResp ""
// @Router /api/v1/market/mrc20/orders [get]
func FetchMarketMrc20Orders(c *gin.Context) {
	var (
		t             int64                              = tool.MakeTimestamp()
		publicKey                                        = getAuthParams(c)
		orderStateStr string                             = c.DefaultQuery("orderState", "1")
		cursorStr     string                             = c.DefaultQuery("cursor", "0")
		sizeStr       string                             = c.DefaultQuery("size", "10")
		sortTypeStr   string                             = c.DefaultQuery("sortType", "-1")
		req           *request.FetchMarketMrc20OrdersReq = &request.FetchMarketMrc20OrdersReq{
			OrderState: 0,
			TickId:     c.DefaultQuery("tickId", ""),
			AssetType:  models.AssetType(c.DefaultQuery("assetType", "")),
			Cursor:     0,
			Size:       0,
			Address:    c.DefaultQuery("address", ""),
			SortKey:    c.DefaultQuery("sortKey", "timestamp"),
			SortType:   0,
		}
	)
	orderStateInt, _ := strconv.ParseInt(orderStateStr, 10, 64)
	req.OrderState = models.OrderState(orderStateInt)
	req.Cursor, _ = strconv.ParseInt(cursorStr, 10, 64)
	req.Size, _ = strconv.ParseInt(sizeStr, 10, 64)
	req.SortType, _ = strconv.ParseInt(sortTypeStr, 10, 64)
	resp, err := order_service.FetchMarketMrc20Orders(req, publicKey, c.ClientIP())
	if err != nil {
		c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
		return
	}
	c.JSONP(http.StatusOK, respond.RespSuccess(resp, tool.MakeTimestamp()-t))
	return
}

// @Summary Fetch market mrc20 order detail
// @Description Fetch market mrc20 order detail
// @Produce  json
// @Tags Market-Order
// @Param orderId query string true "orderId"
// @Success 200 {object} respond.Mrc20OrderInfo ""
// @Router /api/v1/market/mrc20/order/detail [get]
func FetchMarketMrc20OneOrder(c *gin.Context) {
	var (
		t         int64                                = tool.MakeTimestamp()
		publicKey                                      = getAuthParams(c)
		req       *request.FetchMarketMrc20OneOrderReq = &request.FetchMarketMrc20OneOrderReq{
			OrderId: c.DefaultQuery("orderId", ""),
		}
	)
	resp, err := order_service.FetchMarketMrc20OneOrder(req, publicKey, c.ClientIP())
	if err != nil {
		c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
		return
	}
	c.JSONP(http.StatusOK, respond.RespSuccess(resp, tool.MakeTimestamp()-t))
	return
}

// @Summary Fetch market mrc20 hot list
// @Description Fetch market mrc20 hot list
// @Produce  json
// @Tags Market-Order
// @Param timeRange query int false "timeRange, default:24*60*60*1000"
// @Param cursor query int false "cursor, default:0"
// @Param size query int false "size, default:20, max:20"
// @Success 200 {object} respond.Mrc20HotListResp ""
// @Router /api/v1/market/mrc20/hot-list [get]
func FetchMarketMrc20HotList(c *gin.Context) {
	var (
		t         int64                               = tool.MakeTimestamp()
		publicKey                                     = getAuthParams(c)
		req       *request.FetchMarketMrc20HotListReq = &request.FetchMarketMrc20HotListReq{
			TimeRange: 0,
			Cursor:    0,
			Size:      0,
		}
	)
	req.TimeRange, _ = strconv.ParseInt(c.DefaultQuery("timeRange", "0"), 10, 64)
	req.Cursor, _ = strconv.ParseInt(c.DefaultQuery("cursor", "0"), 10, 64)
	req.Size, _ = strconv.ParseInt(c.DefaultQuery("size", "20"), 10, 64)
	if req.Size > 20 {
		req.Size = 20
	}
	resp, err := order_service.FetchMarketMrc20HotList(req, publicKey, c.ClientIP())
	if err != nil {
		c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
		return
	}
	c.JSONP(http.StatusOK, respond.RespSuccess(resp, tool.MakeTimestamp()-t))
	return
}

// @Summary Fetch market mrc20 newest list
// @Description Fetch market mrc20 newest list
// @Produce  json
// @Tags Market-Order
// @Param cursor query int false "cursor, default:0"
// @Param size query int false "size, default:20, max:20"
// @Success 200 {object} respond.Mrc20NewestListResp ""
// @Router /api/v1/market/mrc20/newest-list [get]
func FetchMarketMrc20NewestList(c *gin.Context) {
	var (
		t         int64                                  = tool.MakeTimestamp()
		publicKey                                        = getAuthParams(c)
		req       *request.FetchMarketMrc20NewestListReq = &request.FetchMarketMrc20NewestListReq{
			Cursor: 0,
			Size:   0,
		}
	)
	req.Cursor, _ = strconv.ParseInt(c.DefaultQuery("cursor", "0"), 10, 64)
	req.Size, _ = strconv.ParseInt(c.DefaultQuery("size", "20"), 10, 64)
	if req.Size > 20 {
		req.Size = 20
	}
	resp, err := order_service.FetchMarketMrc20NewestList(req, publicKey, c.ClientIP())
	if err != nil {
		c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
		return
	}
	c.JSONP(http.StatusOK, respond.RespSuccess(resp, tool.MakeTimestamp()-t))
	return
}
