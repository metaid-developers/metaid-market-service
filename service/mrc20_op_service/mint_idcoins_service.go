package mrc20_op_service

import (
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
)

func IdCoinsMintPre(req *request.IdCoinsMintPreRequest, publicKey, ip string) (*respond.IdCoinsMintPreResp, error) {
	return nil, errIdCoinsMaintenance
}

func IdCoinsMintCommit(req *request.IdCoinsMintCommitRequest, publicKey, ip string) (*respond.IdCoinsMintCommitResp, error) {
	return nil, errIdCoinsMaintenance
}
