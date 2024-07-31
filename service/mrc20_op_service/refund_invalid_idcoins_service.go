package mrc20_op_service

import (
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
	"metaid-market-service/service/orders_exchange_service"
)

func RefundIdCoinsInvalidLpPre(req *request.RefundIdCoinsValidPreRequest, publicKey, ip string) (*respond.RefundIdCoinsValidPreResp, error) {
	return RefundIdCoinsInvalidLpPreFromOrders(req, publicKey, ip)
}

func RefundIdCoinsInvalidLpCommit(req *request.RefundIdCoinsValidCommitRequest, publicKey, ip string) (*respond.RefundIdCoinsValidCommitResp, error) {
	return RefundIdCoinsInvalidLpCommitFromOrder(req, publicKey, ip)
}

func RefundIdCoinsInvalidLpPreFromOrders(req *request.RefundIdCoinsValidPreRequest, publicKey, ip string) (*respond.RefundIdCoinsValidPreResp, error) {
	var (
		headers map[string]string = map[string]string{
			"X-Public-Key": publicKey,
		}

		reqOrders *orders_exchange_service.RefundIdCoinsValidPreRequest = &orders_exchange_service.RefundIdCoinsValidPreRequest{
			Address: req.Address,
			OrderId: req.OrderId,
		}
		respOrders *orders_exchange_service.RefundIdCoinsValidPreResp
		err        error
	)
	respOrders, err = orders_exchange_service.RefundIdCoinsInvalidLpPre(reqOrders, headers)
	if err != nil {
		return nil, err
	}
	return &respond.RefundIdCoinsValidPreResp{
		OrderId:       respOrders.OrderId,
		RefundAddress: respOrders.RefundAddress,
		RefundAmount:  respOrders.RefundAmount,
		PsbtRaw:       respOrders.PsbtRaw,
	}, nil
}

func RefundIdCoinsInvalidLpCommitFromOrder(req *request.RefundIdCoinsValidCommitRequest, publicKey, ip string) (*respond.RefundIdCoinsValidCommitResp, error) {
	var (
		headers map[string]string = map[string]string{
			"X-Public-Key": publicKey,
		}

		reqOrders *orders_exchange_service.RefundIdCoinsValidCommitRequest = &orders_exchange_service.RefundIdCoinsValidCommitRequest{
			OrderId: req.OrderId,
			PsbtRaw: req.PsbtRaw,
		}
		respOrders *orders_exchange_service.RefundIdCoinsValidCommitResp
		err        error
	)
	respOrders, err = orders_exchange_service.RefundIdCoinsInvalidLpCommit(reqOrders, headers)
	if err != nil {
		return nil, err
	}
	return &respond.RefundIdCoinsValidCommitResp{
		OrderId:       respOrders.OrderId,
		RefundAddress: respOrders.RefundAddress,
		RefundAmount:  respOrders.RefundAmount,
		TxId:          respOrders.TxId,
	}, nil
}
