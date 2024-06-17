package task

import (
	"fmt"
	"metaid-market-service/conf"
	"metaid-market-service/models"
	"metaid-market-service/service/own_service"
)

func jobForCheckRecordConfirm() {
	var (
		jobName    = "CheckOrderConfirm"
		entityList []*models.MarketOrderModel
		offset     int64 = 0
		limit      int64 = 100
	)
	entityList, _ = models.MarketOrderModelDao().GetList(
		&models.MarketOrderModel{OrderState: models.OrderStateFinish, ConfirmationState: models.ConfirmationStateUnconfirmed},
		offset, limit)
	fmt.Printf("[JOB][%s] check finsih and confirmed Order start. entityList: %d\n", jobName, len(entityList))
	for _, entity := range entityList {
		if entity.OrderState != models.OrderStateFinish || entity.ConfirmationState != models.ConfirmationStateUnconfirmed {
			fmt.Printf("[JOB][%s][%s] OrderState or ConfirmationState not match. opState: %v, confirmState: %v\n", jobName, entity.OrderId, entity.OrderState, entity.ConfirmationState)
			continue
		}
		if entity.TxId == "" {
			fmt.Printf("[JOB][%s][%s] TxId is empty\n", jobName, entity.OrderId)
			continue
		}

		// check confirm
		txInfo, err := own_service.GetTxInfo(conf.Net, entity.TxId)
		if err != nil {
			fmt.Printf("[JOB][%s][%s] GetTxInfo error: %v\n", jobName, entity.OrderId, err)
			continue
		}
		if txInfo.Confirmed && txInfo.Height > 0 {
			entity.ConfirmationState = models.ConfirmationStateConfirmed
			entity.BlockHeight = txInfo.Height
			err := models.MarketOrderModelDao().Update(entity)
			if err != nil {
				fmt.Printf("[JOB][%s][%s] UpdateEntityForConfirm error: %v\n", jobName, entity.OrderId, err)
				continue
			}
			fmt.Printf("[JOB][%s][%s] UpdateEntityForConfirm success\n", jobName, entity.OrderId)
		}
	}
	fmt.Printf("[JOB][%s] check finsih and confirmed Order end. entityList: %d\n", jobName, len(entityList))
}
