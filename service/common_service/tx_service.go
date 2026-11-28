package common_service

import (
	"errors"
	"fmt"
	"metaid-market-service/common"
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
	"metaid-market-service/protobuf/btc_service"
	"metaid-market-service/protobuf/doge_service"
	"metaid-market-service/service/grpc_service"
	"metaid-market-service/service/mempool_space_service"
	"metaid-market-service/service/risk_service"
	"strings"
	"sync"
	"time"
)

func BroadcastTx(req *request.BroadcastTxReq) (*respond.BroadcastTxResp, error) {
	var (
		txId string
		err  error
	)
	if err = risk_service.CheckRawBtcTx(req.Chain, req.TxHex); err != nil {
		return nil, err
	}
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
	rawTx, status, rawErr := mempool_space_service.Get(fmt.Sprintf("/api/tx/%s/hex", txId), mempool_space_service.Foreground)
	if rawErr == nil && status == 200 && rawTx != "" && !strings.HasPrefix(rawTx, "{") {
		return &respond.FetchRawTxResp{RawTx: strings.TrimSpace(rawTx)}, nil
	}
	return nil, errors.New("raw tx not found")
}

// fee recommend
var (
	feeFreshTTL     = 5 * time.Minute
	feeRetryBackoff = 60 * time.Second
	feeStaleTTL     = 30 * time.Minute
	feeNowFunc      = time.Now

	feeMu            sync.Mutex
	feeLastGood      *mempool_space_service.FeeRecommended
	feeFetchedAt     time.Time
	feeLastFailureAt time.Time
	feeRefreshing    bool
)

func copyFeeRecommended(fee *mempool_space_service.FeeRecommended) *mempool_space_service.FeeRecommended {
	if fee == nil {
		return nil
	}
	copy := *fee
	return &copy
}

func staleFeeLocked(now time.Time) *mempool_space_service.FeeRecommended {
	if feeLastGood == nil || now.Sub(feeFetchedAt) > feeStaleTTL {
		return nil
	}
	return copyFeeRecommended(feeLastGood)
}

func getFeeRecommendedCached() (*mempool_space_service.FeeRecommended, error) {
	now := feeNowFunc()
	feeMu.Lock()
	if feeLastGood != nil && now.Sub(feeFetchedAt) < feeFreshTTL {
		fee := copyFeeRecommended(feeLastGood)
		feeMu.Unlock()
		return fee, nil
	}
	if feeRefreshing || (!feeLastFailureAt.IsZero() && now.Sub(feeLastFailureAt) < feeRetryBackoff) {
		fee := staleFeeLocked(now)
		feeMu.Unlock()
		if fee != nil {
			return fee, nil
		}
		return nil, errors.New("fee rate temporarily unavailable")
	}
	feeRefreshing = true
	feeMu.Unlock()

	fee, fetchErr := mempool_space_service.GetFeeRecommended()

	feeMu.Lock()
	feeRefreshing = false
	if fetchErr == nil && fee != nil {
		feeLastGood = copyFeeRecommended(fee)
		feeFetchedAt = feeNowFunc()
		feeLastFailureAt = time.Time{}
	} else {
		feeLastFailureAt = feeNowFunc()
	}
	stale := staleFeeLocked(feeNowFunc())
	feeMu.Unlock()

	if fetchErr == nil && fee != nil {
		return copyFeeRecommended(fee), nil
	}
	if stale != nil {
		return stale, nil
	}
	return nil, errors.New("fee rate temporarily unavailable")
}

func GetFeeSummary() (*respond.FeeRecommended, error) {
	currentFeeRecommend, err := getFeeRecommendedCached()
	if err != nil {
		return nil, err
	}
	adjusted := *currentFeeRecommend
	if adjusted.MinimumFee == adjusted.FastestFee {
		adjusted.FastestFee += 1
	}
	if adjusted.MinimumFee == adjusted.HalfHourFee {
		adjusted.HalfHourFee += 1
	}
	if adjusted.MinimumFee == adjusted.HourFee {
		adjusted.HourFee += 1
	}
	return &respond.FeeRecommended{
		FastestFee:  adjusted.FastestFee,
		HalfHourFee: adjusted.HalfHourFee,
		HourFee:     adjusted.HourFee,
		EconomyFee:  adjusted.EconomyFee,
		MinimumFee:  adjusted.MinimumFee,
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
