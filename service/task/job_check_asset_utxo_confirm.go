package task

import (
	"fmt"
	"metaid-market-service/models"
	"metaid-market-service/service/common_service"
)

func JobForCheckAssetConfirm() {
	jobForCheckAssetConfirm()
	//jobForCheckAssetRuneConfirm()
}

func jobForCheckAssetConfirm() {
	var (
		jobName    = "CheckAssetConfirm"
		entityList []*models.MarketUtxoModel
		offset     int64 = 0
		limit      int64 = 100
	)
	entityList, _ = models.MarketUtxoModelDao().GetListUnconfirmed(&models.MarketUtxoModel{
		UsedState:     models.UsedNo,
		ConfirmStatus: models.Unconfirmed,
	}, offset, limit)
	fmt.Printf("[JOB][%s] check unused asset start. entityList: %d\n", jobName, len(entityList))
	for _, entity := range entityList {
		if entity.UsedState != models.UsedNo {
			fmt.Printf("[JOB][%s][%s] UsedState not match. UsedState: %v\n", jobName, entity.UtxoId, entity.UsedState)
			continue
		}
		if entity.ConfirmStatus != models.Unconfirmed {
			fmt.Printf("[JOB][%s][%s] ConfirmStatus not match. ConfirmStatus: %v\n", jobName, entity.UtxoId, entity.ConfirmStatus)
			continue
		}
		if entity.TxId == "" {
			fmt.Printf("[JOB][%s][%s] UtxoTxId is empty\n", jobName, entity.TxId)
			continue
		}

		// check confirm
		txInfo, err := common_service.GetTxInfo(entity.Chain, entity.TxId)
		if err != nil {
			fmt.Printf("[JOB][%s][%s] GetTxInfo error: %v\n", jobName, entity.UtxoId, err)
			continue
		}
		fmt.Printf("[JOB][%s][%s][%s] txInfo: %+v\n", jobName, entity.Chain, entity.UtxoId, txInfo)
		if txInfo.Confirmed {
			entity.ConfirmStatus = models.Confirmed
			err := models.MarketUtxoModelDao().Update(entity)
			if err != nil {
				fmt.Printf("[JOB][%s][%s] Update asset confirm error: %v\n", jobName, entity.UtxoId, err)
				continue
			}
			fmt.Printf("[JOB][%s][%s] Update asset confirm success\n", jobName, entity.UtxoId)
		}
	}
	fmt.Printf("[JOB][%s] check finsih and confirmed asset end. entityList: %d\n", jobName, len(entityList))
}
