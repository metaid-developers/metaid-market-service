package mrc20_op_service

import (
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
)

func RefundIdCoinsInvalidLpPre(req *request.RefundIdCoinsValidPreRequest, publicKey, ip string) (*respond.RefundIdCoinsValidPreResp, error) {
	return nil, errIdCoinsMaintenance
}

func RefundIdCoinsInvalidLpCommit(req *request.RefundIdCoinsValidCommitRequest, publicKey, ip string) (*respond.RefundIdCoinsValidCommitResp, error) {
	return nil, errIdCoinsMaintenance
}
