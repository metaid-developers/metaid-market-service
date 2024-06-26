package controller

import (
	"errors"
	"github.com/gin-gonic/gin"
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
	"metaid-market-service/models"
	"metaid-market-service/service/order_service"
	"metaid-market-service/tool"
	"net/http"
	"strconv"
)

// @Summary Push Market order
// @Description Push Market order
// @Produce  json
// @Param Request body request.PushOrderReq true "Request"
// @Tags Market-Order
// @Success 200 {object} respond.PushOrderResp ""
// @Router /api/v1/market/order/push [post]
func PushMarketOrder(c *gin.Context) {
	var (
		t            int64 = tool.MakeTimestamp()
		requestModel *request.PushOrderReq
		publicKey    string = ""
	)
	if c.ShouldBindJSON(&requestModel) == nil {
		publicKey = getAuthParams(c)
		responseModel, err := order_service.PushMarketOrder(requestModel, publicKey, c.ClientIP())
		if err != nil {
			c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
			return
		}
		c.JSONP(http.StatusOK, respond.RespSuccess(responseModel, tool.MakeTimestamp()-t))
		return
	}
	c.JSONP(http.StatusInternalServerError, respond.RespErr(errors.New("error parameter"), tool.MakeTimestamp()-t, respond.HttpsCodeError))
}

// @Summary Fetch market order psbt
// @Description Fetch market order psbt
// @Produce  json
// @Tags Market-Order
// @Param orderId query string true "orderId"
// @Param buyerAddress query string true "buyerAddress"
// @Param buyerChangeAmount query int false "buyerChangeAmount"
// @Success 200 {object} respond.OrderInfo ""
// @Router /api/v1/market/order/psbt [get]
func FetchOrderPsbt(c *gin.Context) {
	var (
		t                    int64                      = tool.MakeTimestamp()
		publicKey                                       = getAuthParams(c)
		buyerChangeAmountStr string                     = c.DefaultQuery("buyerChangeAmount", "0")
		req                  *request.FetchOrderPsbtReq = &request.FetchOrderPsbtReq{
			OrderId:           c.DefaultQuery("orderId", ""),
			BuyerAddress:      c.DefaultQuery("buyerAddress", ""),
			BuyerChangeAmount: 0,
		}
	)
	req.BuyerChangeAmount, _ = strconv.ParseUint(buyerChangeAmountStr, 10, 64)
	resp, err := order_service.FetchOrderPsbt(req, publicKey, c.ClientIP())
	if err != nil {
		c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
		return
	}
	c.JSONP(http.StatusOK, respond.RespSuccess(resp, tool.MakeTimestamp()-t))
	return
}

// @Summary Take Market order
// @Description Take Market order
// @Produce  json
// @Param Request body request.TakeOrderReq true "Request"
// @Tags Market-Order
// @Success 200 {object} respond.TakeOrderResp ""
// @Router /api/v1/market/order/take [post]
func TakeMarketOrder(c *gin.Context) {
	var (
		t            int64 = tool.MakeTimestamp()
		requestModel *request.TakeOrderReq
		publicKey    string = ""
	)
	if c.ShouldBindJSON(&requestModel) == nil {
		publicKey = getAuthParams(c)
		responseModel, err := order_service.TakeMarketOrder(requestModel, publicKey, c.ClientIP())
		if err != nil {
			c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
			return
		}
		c.JSONP(http.StatusOK, respond.RespSuccess(responseModel, tool.MakeTimestamp()-t))
		return
	}
	c.JSONP(http.StatusInternalServerError, respond.RespErr(errors.New("error parameter"), tool.MakeTimestamp()-t, respond.HttpsCodeError))
}

// @Summary Cancel Market order
// @Description Cancel Market order
// @Produce  json
// @Param Request body request.CancelOrderReq true "Request"
// @Tags Market-Order
// @Success 200 {object} respond.CancelOrderResp ""
// @Router /api/v1/market/order/cancel [post]
func CancelMarketOrder(c *gin.Context) {
	var (
		t            int64 = tool.MakeTimestamp()
		requestModel *request.CancelOrderReq
		publicKey    string = ""
	)
	if c.ShouldBindJSON(&requestModel) == nil {
		publicKey = getAuthParams(c)
		responseModel, err := order_service.CancelMarketOrder(requestModel, publicKey, c.ClientIP())
		if err != nil {
			c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
			return
		}
		c.JSONP(http.StatusOK, respond.RespSuccess(responseModel, tool.MakeTimestamp()-t))
		return
	}
	c.JSONP(http.StatusInternalServerError, respond.RespErr(errors.New("error parameter"), tool.MakeTimestamp()-t, respond.HttpsCodeError))
}

// @Summary Fetch market orders
// @Description Fetch market orders
// @Produce  json
// @Tags Market-Order
// @Param assetType query string true "pins/ordinals"
// @Param orderState query int true "1-create, 2-cancel, 3-finish"
// @Param address query string false "address"
// @Param sortKey query string false "sellPriceAmount/timestamp/assetLevel, default:timestamp"
// @Param filter-path query string false "default:”"
// @Param filter-level query int false "default:0"
// @Param filter-uncastTickId query string false "default:”"
// @Param sortType query int false "-1/1, default:-1"
// @Param cursor query int false "default:0"
// @Param size query int false "default:10, max:50"
// @Success 200 {object} respond.OrderListResp ""
// @Router /api/v1/market/orders [get]
func FetchMarketOrders(c *gin.Context) {
	var (
		t             int64                         = tool.MakeTimestamp()
		publicKey                                   = getAuthParams(c)
		orderStateStr string                        = c.DefaultQuery("orderState", "1")
		cursorStr     string                        = c.DefaultQuery("cursor", "0")
		sizeStr       string                        = c.DefaultQuery("size", "10")
		sortTypeStr   string                        = c.DefaultQuery("sortType", "-1")
		req           *request.FetchMarketOrdersReq = &request.FetchMarketOrdersReq{
			OrderState: 0,
			AssetType:  models.AssetType(c.DefaultQuery("assetType", "")),
			Cursor:     0,
			Size:       0,
			Address:    c.DefaultQuery("address", ""),
			SortKey:    c.DefaultQuery("sortKey", "timestamp"),
			SortType:   0,
		}
	)
	req.Filters = &request.Filter{
		Path:         c.DefaultQuery("filter-path", ""),
		Level:        0,
		UncastTickId: c.DefaultQuery("filter-uncastTickId", ""),
	}
	req.Filters.Level, _ = strconv.ParseInt(c.DefaultQuery("filter-level", "0"), 10, 64)
	orderStateInt, _ := strconv.ParseInt(orderStateStr, 10, 64)
	req.OrderState = models.OrderState(orderStateInt)
	req.Cursor, _ = strconv.ParseInt(cursorStr, 10, 64)
	req.Size, _ = strconv.ParseInt(sizeStr, 10, 64)
	req.SortType, _ = strconv.ParseInt(sortTypeStr, 10, 64)
	resp, err := order_service.FetchMarketOrders(req, publicKey, c.ClientIP())
	if err != nil {
		c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
		return
	}
	c.JSONP(http.StatusOK, respond.RespSuccess(resp, tool.MakeTimestamp()-t))
	return
}

// @Summary Fetch market order detail
// @Description Fetch market order detail
// @Produce  json
// @Tags Market-Order
// @Param orderId query string true "orderId"
// @Success 200 {object} respond.OrderInfo ""
// @Router /api/v1/market/order/detail [get]
func FetchMarketOneOrder(c *gin.Context) {
	var (
		t         int64                           = tool.MakeTimestamp()
		publicKey                                 = getAuthParams(c)
		req       *request.FetchMarketOneOrderReq = &request.FetchMarketOneOrderReq{
			OrderId: c.DefaultQuery("orderId", ""),
		}
	)
	resp, err := order_service.FetchMarketOneOrder(req, publicKey, c.ClientIP())
	if err != nil {
		c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
		return
	}
	c.JSONP(http.StatusOK, respond.RespSuccess(resp, tool.MakeTimestamp()-t))
	return
}

// @Summary Fetch market address assets
// @Description Fetch market address assets
// @Produce  json
// @Tags Market-Asset
// @Param assetType query string true "pins/ordinals"
// @Param address query string true "address"
// @Param cursor query int false "default:0"
// @Param size query int false "default:10, max:50"
// @Success 200 {object} respond.AddressAssetListResp ""
// @Router /api/v1/market/address/assets [get]
func FetchAddressAssetList(c *gin.Context) {
	var (
		t         int64                             = tool.MakeTimestamp()
		publicKey                                   = getAuthParams(c)
		cursorStr string                            = c.DefaultQuery("cursor", "0")
		sizeStr   string                            = c.DefaultQuery("size", "10")
		req       *request.FetchAddressAssetListReq = &request.FetchAddressAssetListReq{
			Address:   c.DefaultQuery("address", ""),
			AssetType: models.AssetType(c.DefaultQuery("assetType", "")),
			Cursor:    0,
			Size:      0,
		}
	)
	req.Cursor, _ = strconv.ParseInt(cursorStr, 10, 64)
	req.Size, _ = strconv.ParseInt(sizeStr, 10, 64)
	resp, err := order_service.FetchAddressAssetList(req, publicKey, c.ClientIP())
	if err != nil {
		c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
		return
	}
	c.JSONP(http.StatusOK, respond.RespSuccess(resp, tool.MakeTimestamp()-t))
	return
}

// @Summary Fetch market asset detail
// @Description Fetch market asset detail
// @Produce  json
// @Tags Market-Asset
// @Param assetId query string true "assetId"
// @Param assetType query string true "pins/ordinals"
// @Success 200 {object} respond.OrderInfo ""
// @Router /api/v1/market/asset/detail [get]
func FetchAssetDetail(c *gin.Context) {
	var (
		t         int64                        = tool.MakeTimestamp()
		publicKey                              = getAuthParams(c)
		req       *request.FetchAssetDetailReq = &request.FetchAssetDetailReq{
			AssetId:   c.DefaultQuery("assetId", ""),
			AssetType: models.AssetType(c.DefaultQuery("assetType", "")),
		}
	)
	resp, err := order_service.FetchAssetDetail(req, publicKey, c.ClientIP())
	if err != nil {
		c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
		return
	}
	c.JSONP(http.StatusOK, respond.RespSuccess(resp, tool.MakeTimestamp()-t))
	return
}

// @Summary Push Market order
// @Description Push Market order
// @Produce  json
// @Param Request body request.BatchPushOrderReq true "Request"
// @Tags Market-Order
// @Success 200 {object} respond.BatchPushOrderResp ""
// @Router /api/v1/market/order/push/batch [post]
func BatchPushMarketOrder(c *gin.Context) {
	var (
		t            int64 = tool.MakeTimestamp()
		requestModel *request.BatchPushOrderReq
		publicKey    string = ""
	)
	if c.ShouldBindJSON(&requestModel) == nil {
		publicKey = getAuthParams(c)
		responseModel, err := order_service.BatchPushMarketOrder(requestModel, publicKey, c.ClientIP())
		if err != nil {
			c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
			return
		}
		c.JSONP(http.StatusOK, respond.RespSuccess(responseModel, tool.MakeTimestamp()-t))
		return
	}
	c.JSONP(http.StatusInternalServerError, respond.RespErr(errors.New("error parameter"), tool.MakeTimestamp()-t, respond.HttpsCodeError))
}

// @Summary Fetch market order psbt
// @Description Fetch market order psbt
// @Produce  json
// @Tags Market-Order
// @Param Request body request.BatchFetchOrderPsbtReq true "Request"
// @Success 200 {object} respond.OrderInfo ""
// @Router /api/v1/market/order/psbt/batch [post]
func BatchFetchOrderPsbt(c *gin.Context) {
	var (
		t            int64 = tool.MakeTimestamp()
		publicKey          = getAuthParams(c)
		requestModel *request.BatchFetchOrderPsbtReq
	)
	if c.ShouldBindJSON(&requestModel) == nil {
		publicKey = getAuthParams(c)
		responseModel, err := order_service.BatchFetchOrderPsbt(requestModel, publicKey, c.ClientIP())
		if err != nil {
			c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
			return
		}
		c.JSONP(http.StatusOK, respond.RespSuccess(responseModel, tool.MakeTimestamp()-t))
		return
	}
	c.JSONP(http.StatusInternalServerError, respond.RespErr(errors.New("error parameter"), tool.MakeTimestamp()-t, respond.HttpsCodeError))
}

// @Summary Take Market order
// @Description Take Market order
// @Produce  json
// @Param Request body request.BatchTakeOrderReq true "Request"
// @Tags Market-Order
// @Success 200 {object} respond.TakeOrderResp ""
// @Router /api/v1/market/order/take/batch [post]
func BatchTakeMarketOrder(c *gin.Context) {
	var (
		t            int64 = tool.MakeTimestamp()
		requestModel *request.BatchTakeOrderReq
		publicKey    string = ""
	)
	if c.ShouldBindJSON(&requestModel) == nil {
		publicKey = getAuthParams(c)
		responseModel, err := order_service.BatchTakeMarketOrder(requestModel, publicKey, c.ClientIP())
		if err != nil {
			c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
			return
		}
		c.JSONP(http.StatusOK, respond.RespSuccess(responseModel, tool.MakeTimestamp()-t))
		return
	}
	c.JSONP(http.StatusInternalServerError, respond.RespErr(errors.New("error parameter"), tool.MakeTimestamp()-t, respond.HttpsCodeError))
}

// @Summary Take Market order
// @Description Take Market order
// @Produce  json
// @Param Request body request.TestAuthReq true "Request"
// @Tags Market-Order
// @Success 200 {object} respond.TakeOrderResp ""
// @Router /api/v1/auth/test [post]
func TestAuth(c *gin.Context) {
	var (
		t            int64 = tool.MakeTimestamp()
		requestModel *request.TestAuthReq
		publicKey    string = ""
	)
	if c.ShouldBindJSON(&requestModel) == nil {
		publicKey = getAuthParams(c)
		responseModel, err := order_service.TestAuth(requestModel, publicKey)
		if err != nil {
			c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
			return
		}
		c.JSONP(http.StatusOK, respond.RespSuccess(responseModel, tool.MakeTimestamp()-t))
		return
	}
	c.JSONP(http.StatusInternalServerError, respond.RespErr(errors.New("error parameter"), tool.MakeTimestamp()-t, respond.HttpsCodeError))
}
