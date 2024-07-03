package task

import (
	"fmt"
	"metaid-market-service/common"
	"metaid-market-service/conf"
	"metaid-market-service/models"
	"strconv"
	"strings"
)

func JobForCheckValidOrders() {
	jobForCheckValidMrc20OrdersValid()
	jobForCheckValidOrdersValid()
}

func jobForCheckValidMrc20OrdersValid() {
	var (
		jobName    = "CheckValidMrc20Order"
		entityList []*models.MarketMrc20OrderModel
		offset     int64 = 0
		limit      int64 = 100
	)
	entityList, _ = models.MarketMrc20OrderModelDao().GetList(
		&models.MarketMrc20OrderModel{OrderState: models.OrderStateCreate},
		offset, limit)
	fmt.Printf("[JOB][%s] check finsih and confirmed Order start. entityList: %d\n", jobName, len(entityList))
	for _, entity := range entityList {
		if entity.OrderState != models.OrderStateCreate {
			fmt.Printf("[JOB][%s][%s] OrderState or ConfirmationState not match. opState: %v\n", jobName, entity.OrderId, entity.OrderState)
			continue
		}
		if entity.UtxoId == "" {
			fmt.Printf("[JOB][%s][%s] UtxoId is empty\n", jobName, entity.OrderId)
			continue
		}
		utxoIdStrs := strings.Split(entity.UtxoId, "_")
		txId := utxoIdStrs[0]
		vout, _ := strconv.ParseInt(utxoIdStrs[1], 10, 64)
		utxoInfo := common.GetUtxoInfo(conf.Net, txId, vout)
		if utxoInfo == nil {
			fmt.Printf("[JOB][%s][%s] GetUtxoInfo error\n", jobName, entity.OrderId)
			continue
		}
		if !utxoInfo.IsExist || utxoInfo.SpendStatus == "spend" {
			entity.OrderState = models.OrderStateCancel
			err := models.MarketMrc20OrderModelDao().Update(entity)
			if err != nil {
				fmt.Printf("[JOB][%s][%s] UpdateEntityForConfirm error: %v\n", jobName, entity.OrderId, err)
				continue
			}
			fmt.Printf("[JOB][%s][%s] UpdateEntityForConfirm success\n", jobName, entity.OrderId)
		}
	}
	fmt.Printf("[JOB][%s] check Cancel and confirmed Order end. entityList: %d\n", jobName, len(entityList))
}

func jobForCheckValidOrdersValid() {
	var (
		jobName    = "CheckValidPinOrder"
		entityList []*models.MarketOrderModel
		offset     int64 = 0
		limit      int64 = 100
	)
	entityList, _ = models.MarketOrderModelDao().GetList(
		&models.MarketOrderModel{OrderState: models.OrderStateCreate},
		offset, limit)
	fmt.Printf("[JOB][%s] check finsih and confirmed Order start. entityList: %d\n", jobName, len(entityList))
	for _, entity := range entityList {
		if entity.OrderState != models.OrderStateCreate {
			fmt.Printf("[JOB][%s][%s] OrderState or ConfirmationState not match. opState: %v\n", jobName, entity.OrderId, entity.OrderState)
			continue
		}
		if entity.UtxoId == "" {
			fmt.Printf("[JOB][%s][%s] UtxoId is empty\n", jobName, entity.OrderId)
			continue
		}
		utxoIdStrs := strings.Split(entity.UtxoId, "_")
		txId := utxoIdStrs[0]
		vout, _ := strconv.ParseInt(utxoIdStrs[1], 10, 64)
		utxoInfo := common.GetUtxoInfo(conf.Net, txId, vout)
		if utxoInfo == nil {
			fmt.Printf("[JOB][%s][%s] GetUtxoInfo error\n", jobName, entity.OrderId)
			continue
		}
		if !utxoInfo.IsExist || utxoInfo.SpendStatus == "spend" {
			entity.OrderState = models.OrderStateCancel
			err := models.MarketOrderModelDao().Update(entity)
			if err != nil {
				fmt.Printf("[JOB][%s][%s] UpdateEntityForConfirm error: %v\n", jobName, entity.OrderId, err)
				continue
			}
			fmt.Printf("[JOB][%s][%s] UpdateEntityForConfirm success\n", jobName, entity.OrderId)
		}
	}
	fmt.Printf("[JOB][%s] check Cancel and confirmed Order end. entityList: %d\n", jobName, len(entityList))
}
