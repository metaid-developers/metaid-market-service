package metaname_service

import (
	"metaid-market-service/models"
	"metaid-market-service/service/common_service"
	"metaid-market-service/service/mrc20_op_service"
)

func RetryRegisterInscription(order *models.MetanameRegisterOrderModel) error {
	if order == nil || order.TxId == "" || (order.CommitTxRaw == "" && order.RevealTxRaw == "") {
		return nil
	}
	finalize := func() error {
		return models.MetanameRegisterOrderModelDao().SaveEntityForInscribing(order.Chain, order, nil, nil)
	}
	if _, err := common_service.GetTxInfo(order.Chain, order.TxId); err == nil {
		return finalize()
	}
	if err := mrc20_op_service.BroadcastInscribeTxs(order.Chain, []string{order.CommitTxRaw, order.RevealTxRaw}); err != nil {
		return err
	}
	return finalize()
}
