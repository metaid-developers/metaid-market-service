package task

import (
	"fmt"
	"metaid-market-service/conf"
	"metaid-market-service/models"
	"metaid-market-service/service/common_service"
	"metaid-market-service/service/own_service"
	"strconv"
	"strings"
)

func JobForCheckRecordConfirm() {
	jobForCheckRecordConfirm()
	jobForCheckMrc20RecordConfirm()
	jobForCheckMrc20DeployRecordConfirm()
	jobForCheckMrc20MintRecordConfirm()
	jobForCheckMrc20TransferRecordConfirm()
	jobForCheckMrc20PreTransferOrderAskConfirm()
}

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

func jobForCheckMrc20RecordConfirm() {
	var (
		jobName    = "CheckMrc20OrderConfirm"
		entityList []*models.MarketMrc20OrderModel
		offset     int64 = 0
		limit      int64 = 100
	)
	entityList, _ = models.MarketMrc20OrderModelDao().GetList(
		&models.MarketMrc20OrderModel{OrderState: models.OrderStateFinish, ConfirmationState: models.ConfirmationStateUnconfirmed},
		offset, limit)
	fmt.Printf("[JOB][%s] check finsih and confirmed Mrc20 Order start. entityList: %d\n", jobName, len(entityList))
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
			err := models.MarketMrc20OrderModelDao().Update(entity)
			if err != nil {
				fmt.Printf("[JOB][%s][%s] UpdateEntityForConfirm error: %v\n", jobName, entity.OrderId, err)
				continue
			}
			fmt.Printf("[JOB][%s][%s] UpdateEntityForConfirm success\n", jobName, entity.OrderId)
		}
	}
	fmt.Printf("[JOB][%s] check finsih and confirmed Mrc20 Order end. entityList: %d\n", jobName, len(entityList))
}

func jobForCheckMrc20DeployRecordConfirm() {
	var (
		jobName    = "CheckMrc20DeployOrderConfirm"
		entityList []*models.Mrc20DeployOrderModel
		offset     int64 = 0
		limit      int64 = 100
	)
	entityList, _ = models.Mrc20DeployOrderModelDao().GetListAsc(
		&models.Mrc20DeployOrderModel{InscribeState: models.InscribeStateFinish, ConfirmationState: models.ConfirmationStateUnconfirmed},
		offset, limit)
	fmt.Printf("[JOB][%s] check finsih and confirmed Mrc20 Deploy start. entityList: %d\n", jobName, len(entityList))
	for _, entity := range entityList {
		if entity.InscribeState != models.InscribeStateFinish || entity.ConfirmationState != models.ConfirmationStateUnconfirmed {
			fmt.Printf("[JOB][%s][%s] InscribeState or ConfirmationState not match. InscribeState: %v, confirmState: %v\n", jobName, entity.OrderId, entity.InscribeState, entity.ConfirmationState)
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
			err := models.Mrc20DeployOrderModelDao().Update(entity)
			if err != nil {
				fmt.Printf("[JOB][%s][%s] UpdateEntityForConfirm error: %v\n", jobName, entity.OrderId, err)
				continue
			}
			fmt.Printf("[JOB][%s][%s] UpdateEntityForConfirm success\n", jobName, entity.OrderId)
		}
	}
	fmt.Printf("[JOB][%s] check finsih and confirmed Mrc20 Deploy end. entityList: %d\n", jobName, len(entityList))
}

func jobForCheckMrc20MintRecordConfirm() {
	var (
		jobName    = "CheckMrc20MintOrderConfirm"
		entityList []*models.Mrc20MintOrderModel
		offset     int64 = 0
		limit      int64 = 1000
	)
	entityList, _ = models.Mrc20MintOrderModelDao().GetListAsc(
		&models.Mrc20MintOrderModel{InscribeState: models.InscribeStateFinish, ConfirmationState: models.ConfirmationStateUnconfirmed},
		offset, limit)
	fmt.Printf("[JOB][%s] check finsih and confirmed Mrc20 Mint start. entityList: %d\n", jobName, len(entityList))
	for _, entity := range entityList {
		if entity.InscribeState != models.InscribeStateFinish || entity.ConfirmationState != models.ConfirmationStateUnconfirmed {
			fmt.Printf("[JOB][%s][%s] InscribeState or ConfirmationState not match. InscribeState: %v, confirmState: %v\n", jobName, entity.OrderId, entity.InscribeState, entity.ConfirmationState)
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
			err := models.Mrc20MintOrderModelDao().Update(entity)
			if err != nil {
				fmt.Printf("[JOB][%s][%s] UpdateEntityForConfirm error: %v\n", jobName, entity.OrderId, err)
				continue
			}
			fmt.Printf("[JOB][%s][%s] UpdateEntityForConfirm success\n", jobName, entity.OrderId)
		}
	}
	fmt.Printf("[JOB][%s] check finsih and confirmed Mrc20 Mint end. entityList: %d\n", jobName, len(entityList))
}

func jobForCheckMrc20TransferRecordConfirm() {
	var (
		jobName    = "CheckMrc20TransferOrderConfirm"
		entityList []*models.Mrc20TransferOrderModel
		offset     int64 = 0
		limit      int64 = 100
	)
	entityList, _ = models.Mrc20TransferOrderModelDao().GetListAsc(
		&models.Mrc20TransferOrderModel{InscribeState: models.InscribeStateFinish, ConfirmationState: models.ConfirmationStateUnconfirmed},
		offset, limit)
	fmt.Printf("[JOB][%s] check finsih and confirmed Mrc20 Transfer start. entityList: %d\n", jobName, len(entityList))
	for _, entity := range entityList {
		if entity.InscribeState != models.InscribeStateFinish || entity.ConfirmationState != models.ConfirmationStateUnconfirmed {
			fmt.Printf("[JOB][%s][%s] InscribeState or ConfirmationState not match. InscribeState: %v, confirmState: %v\n", jobName, entity.OrderId, entity.InscribeState, entity.ConfirmationState)
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
			err := models.Mrc20TransferOrderModelDao().Update(entity)
			if err != nil {
				fmt.Printf("[JOB][%s][%s] UpdateEntityForConfirm error: %v\n", jobName, entity.OrderId, err)
				continue
			}
			fmt.Printf("[JOB][%s][%s] UpdateEntityForConfirm success\n", jobName, entity.OrderId)
		}
	}
	fmt.Printf("[JOB][%s] check finsih and confirmed Mrc20 Transfer end. entityList: %d\n", jobName, len(entityList))
}

func jobForCheckMrc20PreTransferOrderAskConfirm() {
	var (
		jobName    = "CheckMrc20PreTransferOrderAskConfirm"
		entityList []*models.MarketMrc20OrderModel
		offset     int64 = 0
		limit      int64 = 100
	)
	entityList, _ = models.MarketMrc20OrderModelDao().GetList(
		&models.MarketMrc20OrderModel{
			OrderState: models.OrderStateCreate,
			AskType:    models.AskTypePreTransfer,
		},
		offset, limit)
	fmt.Printf("[JOB][%s] check finsih and confirmed Mrc20 Transfer start. entityList: %d\n", jobName, len(entityList))
	for _, entity := range entityList {
		if entity.OrderState != models.OrderStateCreate || entity.AskType != models.AskTypePreTransfer {
			fmt.Printf("[JOB][%s][%s] OrderState or AskType not match. OrderState: %v, AskType: %v\n", jobName, entity.OrderId, entity.OrderState, entity.AskType)
			continue
		}
		if entity.UtxoId == "" {
			fmt.Printf("[JOB][%s][%s] UtxoId is empty\n", jobName, entity.OrderId)
			continue
		}

		// check confirm
		utxoIdStrs := strings.Split(entity.UtxoId, "_")
		if len(utxoIdStrs) != 2 {
			fmt.Printf("[JOB][%s][%s] UtxoId format error\n", jobName, entity.OrderId)
			continue
		}
		txId := utxoIdStrs[0]
		vout, _ := strconv.ParseInt(utxoIdStrs[1], 10, 64)

		txInfo, err := own_service.GetTxInfo(conf.Net, txId)
		if err != nil {
			fmt.Printf("[JOB][%s][%s] GetTxInfo error: %v\n", jobName, entity.OrderId, err)
			continue
		}
		if txInfo.Confirmed && txInfo.Height > 0 {
			mrc20InfoList, mrc20InfoTotal, err := common_service.FetchTxPointInfo(txId, int64(vout), 0, 100)
			if err != nil {
				fmt.Printf("[JOB][%s][%s] FetchTxPointInfo error: %v\n", jobName, entity.OrderId, err)
				continue
			}
			if mrc20InfoTotal > 1 {
				fmt.Printf("[JOB][%s][%s] FetchTxPointInfo total > 1\n", jobName, entity.OrderId)
				entity.OrderState = models.OrderStateCancel
				entity.Reason = "FetchTxPointInfo total > 1"
				entity.AskType = models.AskTypePreTransferCheck
				err = models.MarketMrc20OrderModelDao().Update(entity)
				if err != nil {
					fmt.Printf("[JOB][%s][%s] Update error: %v\n", jobName, entity.OrderId, err)
					continue
				}
				continue
			}
			if mrc20InfoList == nil || len(mrc20InfoList) == 0 {
				fmt.Printf("[JOB][%s][%s] FetchTxPointInfo list is empty\n", jobName, entity.OrderId)
				entity.OrderState = models.OrderStateCancel
				entity.Reason = "FetchTxPointInfo list is empty"
				entity.AskType = models.AskTypePreTransferCheck
				err = models.MarketMrc20OrderModelDao().Update(entity)
				if err != nil {
					fmt.Printf("[JOB][%s][%s] Update error: %v\n", jobName, entity.OrderId, err)
					continue
				}
				continue
			}

			mrc20Info := mrc20InfoList[0]
			tick := mrc20Info.Tick

			coinAmountStr := mrc20Info.AmtChange
			coinAmount, _ := strconv.ParseInt(coinAmountStr, 10, 64)
			//outValue := mrc20Info.PointValue

			entity.Tick = tick
			entity.TickId = mrc20Info.Mrc20Id

			entity.AmountStr = coinAmountStr
			entity.Amount = int64(coinAmount)
			//entity.OutValue = outValue

			entity.AskType = models.AskTypePreTransferCheck
			err = models.MarketMrc20OrderModelDao().Update(entity)
			if err != nil {
				fmt.Printf("[JOB][%s][%s] Update error: %v\n", jobName, entity.OrderId, err)
				continue
			}
			fmt.Printf("[JOB][%s][%s] Update success\n", jobName, entity.OrderId)
		}
	}
	fmt.Printf("[JOB][%s] check asset-utxo confirmed Mrc20 pre Transfer end. entityList: %d\n", jobName, len(entityList))
}
