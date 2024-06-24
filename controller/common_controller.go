package controller

import (
	"github.com/gin-gonic/gin"
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
	"metaid-market-service/service/common_service"
	"metaid-market-service/tool"
	"net/http"
	"strconv"
)

// @Summary Fetch mrc20 tick info
// @Description Fetch mrc20 tick info
// @Produce  json
// @Tags Common
// @Param tickId query string true "tickId"
// @Success 200 {object} respond.Mrc20TickInfo ""
// @Router /api/v1/common/mrc20/tick/info [get]
func FetchMrc20TickInfo(c *gin.Context) {
	var (
		t   int64                          = tool.MakeTimestamp()
		req *request.FetchMrc20TickInfoReq = &request.FetchMrc20TickInfoReq{
			TickId: c.DefaultQuery("tickId", ""),
		}
	)
	responseModel, err := common_service.FetchMrc20TickInfo(req)
	if err != nil {
		c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
		return
	}
	c.JSONP(http.StatusOK, respond.RespSuccess(responseModel, tool.MakeTimestamp()-t))
	return
}

// @Summary Fetch address mrc20 balances
// @Description Fetch address mrc20 balances
// @Produce  json
// @Tags Common
// @Param address query string true "address"
// @Param cursor query int false "cursor"
// @Param size query int false "size：max=50"
// @Success 200 {object} respond.Mrc20BalanceInfoResp ""
// @Router /api/v1/common/mrc20/address/balance-list [get]
func FetchMrc20TickAddressBalances(c *gin.Context) {
	var (
		t   int64                            = tool.MakeTimestamp()
		req *request.Mrc20AddressBalancesReq = &request.Mrc20AddressBalancesReq{
			Address: c.DefaultQuery("address", ""),
			Cursor:  0,
			Size:    0,
		}
	)
	req.Cursor, _ = strconv.ParseInt(c.DefaultQuery("cursor", "0"), 10, 64)
	req.Size, _ = strconv.ParseInt(c.DefaultQuery("size", "10"), 10, 64)
	responseModel, err := common_service.FetchMrc20TickAddressBalances(req)
	if err != nil {
		c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
		return
	}
	c.JSONP(http.StatusOK, respond.RespSuccess(responseModel, tool.MakeTimestamp()-t))
	return
}

// @Summary Fetch address mrc20 utxo
// @Description Fetch address mrc20 utxo
// @Produce  json
// @Tags Common
// @Param address query string true "address"
// @Param tickId query string true "tickId"
// @Param cursor query int false "cursor"
// @Param size query int false "size：max=50"
// @Success 200 {object} respond.Mrc20UtxoResp ""
// @Router /api/v1/common/mrc20/address/utxo [get]
func FetchMrc20AddressUtxoList(c *gin.Context) {
	var (
		t   int64                         = tool.MakeTimestamp()
		req *request.Mrc20AddressUtxosReq = &request.Mrc20AddressUtxosReq{
			TickId:  c.DefaultQuery("tickId", ""),
			Address: c.DefaultQuery("address", ""),
			Cursor:  0,
			Size:    0,
		}
	)
	req.Cursor, _ = strconv.ParseInt(c.DefaultQuery("cursor", "0"), 10, 64)
	req.Size, _ = strconv.ParseInt(c.DefaultQuery("size", "10"), 10, 64)
	responseModel, err := common_service.FetchMrc20TickAddressUtxos(req)
	if err != nil {
		c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
		return
	}
	c.JSONP(http.StatusOK, respond.RespSuccess(responseModel, tool.MakeTimestamp()-t))
	return
}

// @Summary Fetch address mrc20 shovels
// @Description Fetch address mrc20 shovels
// @Produce  json
// @Tags Common
// @Param address query string true "address"
// @Param tickId query string true "tickId"
// @Param cursor query int false "cursor"
// @Param size query int false "size：max=50"
// @Success 200 {object} respond.Mrc20ShovelResp ""
// @Router /api/v1/common/mrc20/address/shovel [get]
func FetchMrc20TickAddressShovels(c *gin.Context) {
	var (
		t   int64                           = tool.MakeTimestamp()
		req *request.Mrc20AddressShovelsReq = &request.Mrc20AddressShovelsReq{
			TickId:  c.DefaultQuery("tickId", ""),
			Address: c.DefaultQuery("address", ""),
			Cursor:  0,
			Size:    0,
		}
	)
	req.Cursor, _ = strconv.ParseInt(c.DefaultQuery("cursor", "0"), 10, 64)
	req.Size, _ = strconv.ParseInt(c.DefaultQuery("size", "10"), 10, 64)
	responseModel, err := common_service.FetchMrc20TickAddressShovels(req)
	if err != nil {
		c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
		return
	}
	c.JSONP(http.StatusOK, respond.RespSuccess(responseModel, tool.MakeTimestamp()-t))
	return
}

// @Summary Fetch mrc20 tick info list
// @Description Fetch mrc20 tick info list
// @Produce  json
// @Tags Common
// @Param orderBy query string false "pinnumber/totalminted/holders/txcount"
// @Param completed query bool false "true/false/null, default null"
// @Param cursor query int false "cursor"
// @Param size query int false "size：max=50"
// @Success 200 {object} respond.Mrc20TickListResp ""
// @Router /api/v1/common/mrc20/tick/info-list [get]
func FetchMrc20TickList(c *gin.Context) {
	var (
		t   int64                          = tool.MakeTimestamp()
		req *request.FetchMrc20TickListReq = &request.FetchMrc20TickListReq{
			Cursor:    0,
			Size:      0,
			Completed: false,
			OrderBy:   c.DefaultQuery("orderBy", "pinnumber"),
		}
	)
	req.Completed, _ = strconv.ParseBool(c.DefaultQuery("completed", ""))
	req.Cursor, _ = strconv.ParseInt(c.DefaultQuery("cursor", "0"), 10, 64)
	req.Size, _ = strconv.ParseInt(c.DefaultQuery("size", "10"), 10, 64)
	responseModel, err := common_service.FetchMrc20TickList(req)
	if err != nil {
		c.JSONP(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
		return
	}
	c.JSONP(http.StatusOK, respond.RespSuccess(responseModel, tool.MakeTimestamp()-t))
	return
}
