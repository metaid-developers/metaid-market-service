package mrc20_op_service

import (
	"errors"
	"metaid-market-service/common"
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
	"metaid-market-service/models"

	"gorm.io/gorm"
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
	if !idCoinsLaunchEnabled() {
		return &respond.FetchIdCoinsDeployCheckResp{
			CanDeploy: false,
			Msg:       idCoinsUpgradeMessage,
		}, nil
	}
	if req.Address == "" {
		return nil, errors.New("address is empty")
	}
	msg, canDeploy, err := checkIdCoinsDeployer(req.Address)
	if err != nil {
		return nil, err
	}
	if canDeploy {
		if xpub := idCoinsLpVaultXpub(); xpub == "" {
			return &respond.FetchIdCoinsDeployCheckResp{CanDeploy: false, Msg: "launch is not configured"}, nil
		}
		signKey, _ := common.GetPlatformKeyForSignMsg()
		if signKey == "" {
			return &respond.FetchIdCoinsDeployCheckResp{CanDeploy: false, Msg: "launch is not configured"}, nil
		}
		// one launch per issuer (legacy rule)
		existing, err := models.IdCoinsDeployOrderModelDao().GetOne(&models.IdCoinsDeployOrderModel{IssuerAddress: req.Address, InscribeState: models.InscribeStateFinish})
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		if existing != nil {
			return &respond.FetchIdCoinsDeployCheckResp{CanDeploy: false, Msg: "issuer has deployed id coins"}, nil
		}
	}
	return &respond.FetchIdCoinsDeployCheckResp{
		CanDeploy: canDeploy,
		Msg:       msg,
	}, nil
}
