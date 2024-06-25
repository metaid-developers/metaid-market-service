package common_service

import (
	"metaid-market-service/common"
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
)

func BroadcastTx(req *request.BroadcastTxReq) (*respond.BroadcastTxResp, error) {
	var (
		txId string
		err  error
	)
	txId, err = common.BroadcastTx(req.TxHex)
	if err != nil {
		return nil, err
	}
	return &respond.BroadcastTxResp{
		TxId: txId,
	}, nil
}
