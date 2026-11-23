package controller

import (
	"errors"
	"github.com/gin-gonic/gin"
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
	"metaid-market-service/models"
	"metaid-market-service/service/admin_service"
	"metaid-market-service/service/risk_service"
	"metaid-market-service/tool"
	"net/http"
	"strconv"
	"strings"
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

func ListMarketBlacklist(c *gin.Context) {
	t := tool.MakeTimestamp()
	entryType := strings.TrimSpace(c.Query("entryType"))
	state, _ := strconv.ParseInt(c.Query("state"), 10, 64)
	offset, _ := strconv.ParseInt(c.Query("cursor"), 10, 64)
	limit, _ := strconv.ParseInt(c.Query("size"), 10, 64)
	list, total, err := models.MarketBlacklistModelDao().GetList(entryType, state, offset, limit)
	if err != nil {
		c.JSON(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
		return
	}
	c.JSON(http.StatusOK, respond.RespSuccess(map[string]interface{}{
		"total": total,
		"list":  list,
	}, tool.MakeTimestamp()-t))
}

func UpsertMarketBlacklist(c *gin.Context) {
	t := tool.MakeTimestamp()
	var req request.BlacklistUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, respond.RespErr(errors.New("invalid request body"), tool.MakeTimestamp()-t, respond.HttpsCodeError))
		return
	}
	items := req.Entries
	if len(items) == 0 && req.EntryType != "" && req.EntryValue != "" {
		items = []request.BlacklistUpsertItem{{
			EntryType:   req.EntryType,
			EntryValue:  req.EntryValue,
			Reason:      req.Reason,
			ParentValue: req.ParentValue,
			State:       req.State,
		}}
	}
	if len(items) == 0 {
		c.JSON(http.StatusBadRequest, respond.RespErr(errors.New("entries cannot be empty"), tool.MakeTimestamp()-t, respond.HttpsCodeError))
		return
	}
	allowedTypes := map[string]bool{
		models.BlacklistTypeAddress:          true,
		models.BlacklistTypeTxId:             true,
		models.BlacklistTypeUtxo:             true,
		models.BlacklistTypeWhitelistAddress: true,
		models.BlacklistTypeWhitelistTxId:    true,
		models.BlacklistTypeWhitelistUtxo:    true,
	}
	upserted := 0
	for _, it := range items {
		et := strings.TrimSpace(it.EntryType)
		ev := strings.TrimSpace(it.EntryValue)
		if !allowedTypes[et] || ev == "" {
			c.JSON(http.StatusBadRequest, respond.RespErr(errors.New("unsupported entryType or empty entryValue"), tool.MakeTimestamp()-t, respond.HttpsCodeError))
			return
		}
		switch et {
		case models.BlacklistTypeAddress, models.BlacklistTypeWhitelistAddress:
			ev = risk_service.NormalizeAddress(ev)
		case models.BlacklistTypeTxId, models.BlacklistTypeWhitelistTxId:
			ev = risk_service.NormalizeTxId(ev)
		case models.BlacklistTypeUtxo, models.BlacklistTypeWhitelistUtxo:
			ev = risk_service.NormalizeUtxoId(ev)
		}
		st := it.State
		if st != models.STATE_DELETED {
			st = models.STATE_EXIST
		}
		if err := models.MarketBlacklistModelDao().Upsert(&models.MarketBlacklistModel{
			EntryType:   et,
			EntryValue:  ev,
			Source:      "admin",
			ParentValue: strings.TrimSpace(it.ParentValue),
			Reason:      strings.TrimSpace(it.Reason),
			State:       st,
		}); err != nil {
			c.JSON(http.StatusOK, respond.RespErr(err, tool.MakeTimestamp()-t, respond.HttpsCodeError))
			return
		}
		upserted++
	}
	_ = risk_service.ReloadBlacklistFromDB()
	canceledMrc20, canceledPin := risk_service.SweepAllActiveOrders()
	c.JSON(http.StatusOK, respond.RespSuccess(map[string]interface{}{
		"upserted":      upserted,
		"canceledMrc20": canceledMrc20,
		"canceledPin":   canceledPin,
	}, tool.MakeTimestamp()-t))
}

func SweepBlockedMarketOrders(c *gin.Context) {
	t := tool.MakeTimestamp()
	_ = risk_service.ReloadBlacklistFromDB()
	canceledMrc20, canceledPin := risk_service.SweepAllActiveOrders()
	c.JSON(http.StatusOK, respond.RespSuccess(map[string]interface{}{
		"canceledMrc20": canceledMrc20,
		"canceledPin":   canceledPin,
	}, tool.MakeTimestamp()-t))
}
