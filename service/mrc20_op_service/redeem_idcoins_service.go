package mrc20_op_service

import (
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
)

func BookOrderTakeMintBidPreview(req *request.BookTakeMintBidPreviewReq, publicKey, ip string) (*respond.BookMintOrderTakePreviewResp, error) {
	return nil, errIdCoinsMaintenance
}

func BookOrderTakeMintBidPre(req *request.BookTakeMintBidPreReq, publicKey, ip string) (*respond.BookMintOrderTakePreResp, error) {
	return nil, errIdCoinsMaintenance
}

func BookOrderTakeMintBidCommit(req *request.BookTakeMintBidCommitReq, publicKey, ip string) (*respond.BookMintOrderTakeCommitResp, error) {
	return nil, errIdCoinsMaintenance
}
