package mrc20_op_service

import (
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
)

func FetchIdCoinsOpOrders(req *request.FetchIdCoinsOpOrdersRequest, publicKey, ip string) (*respond.FetchIdCoinsOpOrdersResp, error) {
	if req.OpOrderType == "mint" {
		return fetchMergedIdCoinsMintOrders(req)
	}
	return fetchLegacyIdCoinsOpOrders(req)
}

func FetchIdCoinsAddressMintOrder(req *request.FetchIdCoinsMintOrderRequest, publicKey, ip string) (*respond.FetchOneIdCoinsMintOrderResp, error) {
	return fetchMergedAddressIdCoinsMintOrder(req)
}

func FetchIdCoinsList(req *request.FetchIdCoinsListRequest, publicKey, ip string) (*respond.FetchIdCoinsListResp, error) {
	return fetchIdCoinsListLocally(req)
}

func FetchOneIdCoinsInfo(req *request.FetchOneIdCoinsRequest, publicKey, ip string) (*respond.IdCoinsInfoResp, error) {
	return fetchIdCoinsInfoLocally(req)
}

func FetchAddressIdCoinsInfo(req *request.FetchAddressIdCoinInfoRequest, publicKey, ip string) (*respond.IdCoinsInfoResp, error) {
	return fetchAddressIdCoinsInfoLocally(req)
}

func FetchIdCoinsSimpleInfoList(cursor, size int64, publicKey, ip string) (*respond.FetchIdCoinsSimpleInfoListResp, error) {
	return fetchIdCoinsSimpleInfoListLocally(cursor, size)
}

func FetchIdCoinsDeployCheckInfo(req *request.FetchIdCoinsDeployCheckRequest, publicKey, ip string) (*respond.FetchIdCoinsDeployCheckResp, error) {
	return &respond.FetchIdCoinsDeployCheckResp{
		CanDeploy: false,
		Msg:       idCoinsUpgradeMessage,
	}, nil
}
