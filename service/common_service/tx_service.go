package common_service

import (
	"errors"
	"fmt"
	"metaid-market-service/common"
	"metaid-market-service/conf"
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
	"metaid-market-service/protobuf/btc_service"
	"metaid-market-service/protobuf/doge_service"
	"metaid-market-service/service/grpc_service"
	"metaid-market-service/service/mempool_space_service"
	"metaid-market-service/tool"
	"strings"
)

func BroadcastTx(req *request.BroadcastTxReq) (*respond.BroadcastTxResp, error) {
	var (
		txId string
		err  error
	)
	txId, err = common.BroadcastTx(req.Chain, req.TxHex)
	if err != nil {
		return nil, err
	}
	return &respond.BroadcastTxResp{
		TxId: txId,
	}, nil
}

func FetchRawTx(txId string) (*respond.FetchRawTxResp, error) {
	// gRPC FetchTxRaw's upstream is broken (returns corrupt JSON), GetBtcTxRaw
	// goes through a different, healthy upstream; mempool.space is the fallback.
	client, err := grpc_service.GetBtcBaseConn()
	if err == nil {
		grpcResp, grpcErr := client.GetBtcTxRaw(&btc_service.BtcTxRawReq{TxId: txId})
		if grpcErr == nil && grpcResp != nil && grpcResp.TxRaw != "" {
			return &respond.FetchRawTxResp{RawTx: grpcResp.TxRaw}, nil
		}
	}
	rawTx, _, rawErr := tool.GetUrlAndCode(fmt.Sprintf("%s/api/tx/%s/hex", conf.MempoolSpace, txId), nil, nil)
	if rawErr == nil && rawTx != "" && !strings.HasPrefix(rawTx, "{") {
		return &respond.FetchRawTxResp{RawTx: strings.TrimSpace(rawTx)}, nil
	}
	return nil, errors.New("raw tx not found")
}

// fee recommend
var (
	CurrentFeeRecommend *mempool_space_service.FeeRecommended
	UpdateFeeTime       int64 = 0
)

func GetFeeSummary() (*respond.FeeRecommended, error) {
	var (
		feeRecommended      *mempool_space_service.FeeRecommended
		nowTime             int64 = tool.MakeTimestamp()
		currentFeeRecommend *mempool_space_service.FeeRecommended
	)
	if CurrentFeeRecommend == nil || nowTime-UpdateFeeTime >= 1000*60*5 {
		feeRecommended, _ = mempool_space_service.GetFeeRecommended()
		if feeRecommended != nil {
			CurrentFeeRecommend = feeRecommended
			UpdateFeeTime = nowTime
		} else {
			CurrentFeeRecommend = &mempool_space_service.FeeRecommended{}
		}
	}
	currentFeeRecommend = CurrentFeeRecommend

	fast := int64(0)
	if currentFeeRecommend.MinimumFee == currentFeeRecommend.FastestFee {
		currentFeeRecommend.FastestFee += 1
	}
	if currentFeeRecommend.MinimumFee == currentFeeRecommend.HalfHourFee {
		currentFeeRecommend.HalfHourFee += 1
	}
	if currentFeeRecommend.MinimumFee == currentFeeRecommend.HourFee {
		currentFeeRecommend.HourFee += 1
	}
	return &respond.FeeRecommended{
		FastestFee:  currentFeeRecommend.FastestFee + fast,
		HalfHourFee: currentFeeRecommend.HalfHourFee + fast,
		HourFee:     currentFeeRecommend.HourFee + fast,
		EconomyFee:  currentFeeRecommend.EconomyFee + fast,
		MinimumFee:  currentFeeRecommend.MinimumFee + fast,
	}, nil
}

type CheckTxInfo struct {
	TxHash    string `json:"tx_hash"`
	Height    int64  `json:"height"`
	Confirmed bool   `json:"confirmed"`
}

func GetTxInfo(chain, txId string) (*CheckTxInfo, error) {
	var (
		txHash    string
		height    int64
		confirmed bool
	)

	if chain == "doge" {
		// 使用 Doge gRPC 服务获取交易信息
		client, err := grpc_service.GetDogeBaseConn()
		if err != nil {
			return nil, err
		}
		req := &doge_service.DogeTxInfoReq{
			TxId: txId,
		}
		resp, err := client.GetTxInfo(req)
		if err != nil {
			return nil, err
		}
		if resp == nil {
			return nil, errors.New("doge get tx info response is nil")
		}
		txHash = resp.GetTxHash()
		height = int64(resp.GetHeight())
		confirmed = resp.GetConfirmed()
	} else {
		// 使用 BTC gRPC 服务获取交易信息
		client, err := grpc_service.GetBtcBaseConn()
		if err != nil {
			return nil, err
		}
		req := &btc_service.BtcTxInfoReq{
			TxId: txId,
		}
		resp, err := client.GetTxInfo(req)
		if err != nil {
			return nil, err
		}
		if resp == nil {
			return nil, errors.New("btc get tx info response is nil")
		}
		txHash = resp.GetTxHash()
		height = int64(resp.GetHeight())
		confirmed = resp.GetConfirmed()
	}

	return &CheckTxInfo{
		TxHash:    txHash,
		Height:    height,
		Confirmed: confirmed,
	}, nil
}
