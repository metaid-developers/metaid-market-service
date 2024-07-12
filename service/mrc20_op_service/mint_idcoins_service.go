package mrc20_op_service

import (
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
	"metaid-market-service/service/orders_exchange_service"
)

func IdCoinsMintPre(req *request.IdCoinsMintPreRequest, publicKey, ip string) (*respond.IdCoinsMintPreResp, error) {
	return IdCoinsMintPreFromOrders(req, publicKey, ip)
}

func IdCoinsMintCommit(req *request.IdCoinsMintCommitRequest, publicKey, ip string) (*respond.IdCoinsMintCommitResp, error) {
	return IdCoinsMintCommitFromOrder(req, publicKey, ip)
}

func IdCoinsMintPreFromOrders(req *request.IdCoinsMintPreRequest, publicKey, ip string) (*respond.IdCoinsMintPreResp, error) {
	var (
		headers map[string]string = map[string]string{
			"X-Public-Key": publicKey,
		}

		reqOrders *orders_exchange_service.IdCoinsMintPreRequest = &orders_exchange_service.IdCoinsMintPreRequest{
			NetworkFeeRate: req.NetworkFeeRate,
			TickId:         req.TickId,
			OutAddress:     req.OutAddress,
			OutValue:       req.OutValue,
		}
		respOrders *orders_exchange_service.IdCoinsMintPreResp
		err        error
	)
	respOrders, err = orders_exchange_service.IdCoinsMintPre(reqOrders, headers)
	if err != nil {
		return nil, err
	}
	return &respond.IdCoinsMintPreResp{
		OrderId:               respOrders.OrderId,
		TotalFee:              respOrders.TotalFee,
		RevealInscribeFee:     respOrders.RevealInscribeFee,
		RevealMintFee:         respOrders.RevealMintFee,
		RevealInscribeAddress: respOrders.RevealInscribeAddress,
		RevealMintAddress:     respOrders.RevealMintAddress,
		ServiceFee:            respOrders.ServiceFee,
		ServiceAddress:        respOrders.ServiceAddress,
		Extra:                 respOrders.Extra,
	}, nil
}

func IdCoinsMintCommitFromOrder(req *request.IdCoinsMintCommitRequest, publicKey, ip string) (*respond.IdCoinsMintCommitResp, error) {
	var (
		headers map[string]string = map[string]string{
			"X-Public-Key": publicKey,
		}

		reqOrders *orders_exchange_service.IdCoinsMintCommitRequest = &orders_exchange_service.IdCoinsMintCommitRequest{
			OrderId:                  req.OrderId,
			CommitTxRaw:              req.CommitTxRaw,
			CommitTxOutInscribeIndex: req.CommitTxOutInscribeIndex,
			CommitTxOutMintIndex:     req.CommitTxOutMintIndex,
		}
		respOrders *orders_exchange_service.IdCoinsMintCommitResp
		err        error
	)
	respOrders, err = orders_exchange_service.IdCoinsMintCommit(reqOrders, headers)
	if err != nil {
		return nil, err
	}
	return &respond.IdCoinsMintCommitResp{
		OrderId:            respOrders.OrderId,
		CommitTxId:         respOrders.CommitTxId,
		RevealInscribeTxId: respOrders.RevealInscribeTxId,
		RevealMintTxId:     respOrders.RevealMintTxId,
	}, nil
}
