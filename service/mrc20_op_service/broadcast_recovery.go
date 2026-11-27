package mrc20_op_service

import (
	"metaid-market-service/common"
	"metaid-market-service/conf"
	"metaid-market-service/models"
	"metaid-market-service/service/common_service"
)

const orderChainForIdCoins = "btc"

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

// RetryIdCoinsMintInscription rebroadcasts the commit -> reveal-inscribe ->
// reveal-mint chain when a crash happened between the first persistence and
// the broadcast, using the stored commit raw and the signed final reveal
// PSBTs. Once the mint tx is visible on-chain it only finalizes storage.
func RetryIdCoinsMintInscription(order *models.IdCoinsMintOrderModel) error {
	if order == nil || order.RevealMintTxId == "" {
		return nil
	}
	if order.CommitTxRaw == "" && order.RevealMintFinalPsbtRaw == "" {
		return nil
	}
	finalize := func() error {
		return models.IdCoinsMintOrderModelDao().UpdateEntityListForInscribing(order, nil, nil)
	}
	if _, err := common_service.GetTxInfo(orderChainForIdCoins, order.RevealMintTxId); err == nil {
		return finalize()
	}
	revealInscribeRaw := ""
	if order.RevealInscribeFinalPsbtRaw != "" {
		builder, err := common.NewPsbtBuilder(common.GetNetParams(conf.Net, orderChainForIdCoins), order.RevealInscribeFinalPsbtRaw)
		if err != nil {
			return err
		}
		revealInscribeRaw, err = builder.ExtractPsbtTransaction()
		if err != nil {
			return err
		}
	}
	revealMintRaw := ""
	if order.RevealMintFinalPsbtRaw != "" {
		builder, err := common.NewPsbtBuilder(common.GetNetParams(conf.Net, orderChainForIdCoins), order.RevealMintFinalPsbtRaw)
		if err != nil {
			return err
		}
		revealMintRaw, err = builder.ExtractPsbtTransaction()
		if err != nil {
			return err
		}
	}
	if err := BroadcastInscribeTxs(orderChainForIdCoins, []string{order.CommitTxRaw, revealInscribeRaw, revealMintRaw}); err != nil {
		return err
	}
	return finalize()
}
