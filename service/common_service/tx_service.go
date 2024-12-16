package common_service

import (
	"metaid-market-service/common"
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
	"metaid-market-service/service/mempool_space_service"
	"metaid-market-service/tool"
)

func BroadcastTx(req *request.BroadcastTxReq) (*respond.BroadcastTxResp, error) {
	var (
		txId string
		err  error
	)
	txId, err = common.BroadcastTx(req.TxHex)
	if err != nil {
		return nil, err
	}
	return &respond.BroadcastTxResp{
		TxId: txId,
	}, nil
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

func GetCoinRate() {

}
