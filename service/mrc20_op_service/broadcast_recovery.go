package mrc20_op_service

import (
	"metaid-market-service/common"
	"metaid-market-service/conf"
	"metaid-market-service/models"
	"metaid-market-service/service/common_service"
)

func BroadcastInscribeTxs(chain string, rawTxs []string) error {
	for _, raw := range rawTxs {
		if raw == "" {
			continue
		}
		_, err := common.BroadcastTx(chain, raw)
		if err == nil {
			continue
		}
		tx, parseErr := common.TxRawToTx(raw)
		if parseErr != nil {
			return err
		}
		if _, checkErr := common_service.GetTxInfo(chain, tx.TxHash().String()); checkErr != nil {
			return err
		}
	}
	return nil
}

func retryMintOrTransfer(chain, commitRaw, revealPsbtRaw, revealTxId string, finalize func() error) error {
	if revealTxId == "" || (commitRaw == "" && revealPsbtRaw == "") {
		return nil
	}
	if _, err := common_service.GetTxInfo(chain, revealTxId); err == nil {
		return finalize()
	}
	revealRaw := ""
	if revealPsbtRaw != "" {
		builder, err := common.NewPsbtBuilder(common.GetNetParams(conf.Net, chain), revealPsbtRaw)
		if err != nil {
			return err
		}
		revealRaw, err = builder.ExtractPsbtTransaction()
		if err != nil {
			return err
		}
	}
	if err := BroadcastInscribeTxs(chain, []string{commitRaw, revealRaw}); err != nil {
		return err
	}
	return finalize()
}

func RetryMintInscription(order *models.Mrc20MintOrderModel) error {
	if order == nil {
		return nil
	}
	return retryMintOrTransfer(order.Chain, order.CommitTxRaw, order.RevealFinalPsbtRaw, order.TxId, func() error {
		return models.Mrc20MintOrderModelDao().UpdateEntityListForInscribing(order.Chain, order, nil, nil)
	})
}

func RetryTransferInscription(order *models.Mrc20TransferOrderModel) error {
	if order == nil {
		return nil
	}
	return retryMintOrTransfer(order.Chain, order.CommitTxRaw, order.RevealFinalPsbtRaw, order.TxId, func() error {
		return models.Mrc20TransferOrderModelDao().UpdateEntityListForInscribing(order.Chain, order, nil, nil)
	})
}

func RetryDeployInscription(order *models.Mrc20DeployOrderModel) error {
	if order == nil || order.TxId == "" || (order.CommitTxRaw == "" && order.RevealTxRaw == "") {
		return nil
	}
	finalize := func() error {
		return models.Mrc20DeployOrderModelDao().SaveEntityForInscribing(order.Chain, order, nil, nil)
	}
	if _, err := common_service.GetTxInfo(order.Chain, order.TxId); err == nil {
		return finalize()
	}
	if err := BroadcastInscribeTxs(order.Chain, []string{order.CommitTxRaw, order.RevealTxRaw}); err != nil {
		return err
	}
	return finalize()
}
