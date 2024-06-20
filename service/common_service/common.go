package common_service

import (
	"metaid-market-service/service/man_service"
)

func FetchTxPointInfo(txId string, index, cursor, size int64) ([]*man_service.Mrc20Utxo, int64, error) {
	var (
		txPointInfo *man_service.Mrc20UtxoResp
		err         error
	)
	txPointInfo, err = man_service.FetchMrc20txPointList(txId, index, cursor, size)
	if err != nil {
		return nil, 0, err
	}
	return txPointInfo.List, txPointInfo.Total, nil
}
