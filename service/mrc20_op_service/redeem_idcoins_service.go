package mrc20_op_service

import (
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
	"metaid-market-service/service/orders_exchange_service"
)

func BookOrderTakeMintBidPreview(req *request.BookTakeMintBidPreviewReq, publicKey, ip string) (*respond.BookMintOrderTakePreviewResp, error) {
	return BookOrderTakeMintBidPreviewFromOrders(req, publicKey, ip)
}

func BookOrderTakeMintBidPre(req *request.BookTakeMintBidPreReq, publicKey, ip string) (*respond.BookMintOrderTakePreResp, error) {
	return BookOrderTakeMintBidPreFromOrders(req, publicKey, ip)
}

func BookOrderTakeMintBidCommit(req *request.BookTakeMintBidCommitReq, publicKey, ip string) (*respond.BookMintOrderTakeCommitResp, error) {
	return BookOrderTakeMintBidCommitFromOrder(req, publicKey, ip)
}

func BookOrderTakeMintBidPreFromOrders(req *request.BookTakeMintBidPreReq, publicKey, ip string) (*respond.BookMintOrderTakePreResp, error) {
	var (
		headers map[string]string = map[string]string{
			"X-Public-Key": publicKey,
		}

		reqOrders *orders_exchange_service.BookTakeMintBidPreReq = &orders_exchange_service.BookTakeMintBidPreReq{
			TickId:         req.TickId,
			AssetUtxoIds:   req.AssetUtxoIds,
			SellCoinAmount: req.SellCoinAmount,
			SellerAddress:  req.SellerAddress,
			NetworkFeeRate: req.NetworkFeeRate,
		}
		respOrders *orders_exchange_service.BookMintOrderTakePreResp
		err        error
	)
	respOrders, err = orders_exchange_service.BookOrderTakeMintBidPre(reqOrders, headers)
	if err != nil {
		return nil, err
	}
	return &respond.BookMintOrderTakePreResp{
		OrderId:          respOrders.OrderId,
		TotalAmount:      respOrders.TotalAmount,
		ReceiveAddress:   respOrders.ReceiveAddress,
		PriceAmount:      respOrders.PriceAmount,
		TotalFee:         respOrders.TotalFee,
		MinerFee:         respOrders.MinerFee,
		ServiceFee:       respOrders.ServiceFee,
		PsbtRaw:          respOrders.PsbtRaw,
		RevealInputIndex: respOrders.RevealInputIndex,
	}, nil
}

func BookOrderTakeMintBidCommitFromOrder(req *request.BookTakeMintBidCommitReq, publicKey, ip string) (*respond.BookMintOrderTakeCommitResp, error) {
	var (
		headers map[string]string = map[string]string{
			"X-Public-Key": publicKey,
		}

		reqOrders *orders_exchange_service.BookTakeMintBidCommitReq = &orders_exchange_service.BookTakeMintBidCommitReq{
			OrderId:          req.OrderId,
			CommitTxRaw:      req.CommitTxRaw,
			CommitTxOutIndex: req.CommitTxOutIndex,
			RevealPrePsbtRaw: req.RevealPrePsbtRaw,
		}
		respOrders *orders_exchange_service.BookMintOrderTakeCommitResp
		err        error
	)
	respOrders, err = orders_exchange_service.BookOrderTakeMintBidCommit(reqOrders, headers)
	if err != nil {
		return nil, err
	}
	return &respond.BookMintOrderTakeCommitResp{
		OrderId:    respOrders.OrderId,
		TxId:       respOrders.TxId,
		CommitTxId: respOrders.CommitTxId,
		RevealTxId: respOrders.RevealTxId,
		TickId:     respOrders.TickId,
	}, nil
}

func BookOrderTakeMintBidPreviewFromOrders(req *request.BookTakeMintBidPreviewReq, publicKey, ip string) (*respond.BookMintOrderTakePreviewResp, error) {
	var (
		headers map[string]string = map[string]string{
			"X-Public-Key": publicKey,
		}

		reqOrders *orders_exchange_service.BookTakeMintBidPreviewReq = &orders_exchange_service.BookTakeMintBidPreviewReq{
			TickId:         req.TickId,
			AssetUtxoIds:   req.AssetUtxoIds,
			SellerAddress:  req.SellerAddress,
			NetworkFeeRate: req.NetworkFeeRate,
		}
		respOrders *orders_exchange_service.BookMintOrderTakePreviewResp
		err        error
	)
	respOrders, err = orders_exchange_service.BookOrderTakeMintBidPreview(reqOrders, headers)
	if err != nil {
		return nil, err
	}
	return &respond.BookMintOrderTakePreviewResp{
		AssetCoinList: respOrders.AssetCoinList,
	}, nil
}
