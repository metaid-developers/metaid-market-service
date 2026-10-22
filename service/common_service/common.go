package common_service

import (
	"fmt"
	"metaid-market-service/service/grpc_service"
	"metaid-market-service/service/man_service"
	"strconv"
	"strings"
)

func FetchTxPointInfo(chain, txId string, index, cursor, size int64) ([]*man_service.Mrc20Utxo, int64, error) {
	var (
		list  []*man_service.Mrc20Utxo = make([]*man_service.Mrc20Utxo, 0)
		total int64                    = 0
	)
	// 原 man_service 实现，已改为 gRPC
	//txPointInfo, err := man_service.FetchMrc20txPointList(txId, index, cursor, size)
	//if err != nil && !strings.Contains(err.Error(), "no data found") {
	//	return nil, 0, err
	//}
	//if txPointInfo != nil {
	//	total = txPointInfo.Total
	//	for _, v := range txPointInfo.List {
	//		if !v.Verify {
	//			total--
	//			continue
	//		}
	//		if v.AmtChange == "0" || v.AmtChange == "" {
	//			total--
	//			continue
	//		}
	//		list = append(list, v)
	//	}
	//}

	conn, err := grpc_service.GetMrc20BaseConn()
	if err != nil {
		return nil, 0, err
	}

	source := ""
	if chain == "doge" || chain == "mrc20-v2" {
		source = "mrc20-v2"
	}
	fmt.Printf("FetchTxPointInfo source:%s\n", source)
	grpcResp, err := conn.FetchMrc20UtxoTxPoint(txId, source, index, cursor, size)
	if err != nil && !strings.Contains(err.Error(), "no data found") {
		return nil, 0, err
	}
	if grpcResp != nil {
		total = grpcResp.Total
		for _, v := range grpcResp.GetDetail() {
			if !v.GetVerify() {
				total--
				continue
			}
			if v.GetAmtChange() == "0" || v.GetAmtChange() == "" {
				total--
				continue
			}
			list = append(list, &man_service.Mrc20Utxo{
				Tick:        v.GetTick(),
				Mrc20Id:     v.GetMrc20Id(),
				TxPoint:     v.GetTxPoint(),
				PinId:       v.GetPinId(),
				PinContent:  v.GetPinContent(),
				Verify:      v.GetVerify(),
				BlockHeight: v.GetBlockHeight(),
				MrcOption:   v.GetMrcOption(),
				FromAddress: v.GetFromAddress(),
				ToAddress:   v.GetToAddress(),
				ErrorMsg:    v.GetMsg(),
				AmtChange:   v.GetAmtChange(),
				Status:      v.GetStatus(),
				Chain:       v.GetChain(),
				Index:       v.GetIndex(),
				Timestamp:   v.GetTimestamp(),
				PointValue:  v.GetPointValue(),
			})
		}
	}
	return list, total, nil
}

type TickInfo struct {
	Tick         string      `json:"tick"`
	TokenName    string      `json:"tokenName"`
	Decimals     string      `json:"decimals"`
	AmtPerMint   string      `json:"amtPerMint"`
	MintCount    string      `json:"mintCount"`
	PremineCount string      `json:"premineCount"`
	BeginHeight  string      `json:"beginheight"`
	EndHeight    string      `json:"endheight"`
	MetaData     string      `json:"metadata"`
	Type         string      `json:"type"`
	Qual         interface{} `json:"qual"`
	PinCheck     interface{} `json:"pinCheck"`
	PayCheck     *PayCheck   `json:"payCheck"`
	TotalMinted  string      `json:"totalMinted"`
	Mrc20Id      string      `json:"mrc20Id"`
	PinNumber    int64       `json:"pinNumber"`
	Chain        string      `json:"chain"`
	Holders      int64       `json:"holders"`
	TxCount      int64       `json:"txCount"`
	MetaId       string      `json:"metaId"`
	Address      string      `json:"address"`
	DeployTime   int64       `json:"deployTime"`
}

type PayCheck struct {
	PayTo     string `json:"payTo"`
	PayAmount string `json:"payAmount"`
}

type PinCheck struct {
	Creator string `json:"creator"`
	Path    string `json:"path"`
	Lvl     string `json:"lvl"`
	Count   string `json:"count"`
}

func GetMrc20TickInfo(tickId, tick string) (*TickInfo, error) {
	var (
		mrc20Resp *man_service.Mrc20TickInfo
		err       error
	)
	mrc20Resp, err = man_service.FetchMrc20TickInfo(tickId, tick)
	if err != nil {
		return nil, err
	}
	payCheck := &PayCheck{}
	if mrc20Resp.PayCheck != nil {
		payCheck.PayTo = mrc20Resp.PayCheck.PayTo
		payCheck.PayAmount = mrc20Resp.PayCheck.PayAmount
	}

	return &TickInfo{
		Tick:         mrc20Resp.Tick,
		TokenName:    mrc20Resp.TokenName,
		Decimals:     mrc20Resp.Decimals,
		AmtPerMint:   mrc20Resp.AmtPerMint,
		MintCount:    strconv.FormatInt(mrc20Resp.MintCount, 10),
		PremineCount: strconv.FormatInt(mrc20Resp.PremineCount, 10),
		BeginHeight:  mrc20Resp.BeginHeight,
		EndHeight:    mrc20Resp.EndHeight,
		MetaData:     mrc20Resp.Metadata,
		Type:         mrc20Resp.Type,
		Qual:         mrc20Resp.PinCheck,
		PinCheck:     mrc20Resp.PinCheck,
		PayCheck:     payCheck,
		TotalMinted:  strconv.FormatInt(mrc20Resp.TotalMinted, 10),
		Mrc20Id:      mrc20Resp.Mrc20Id,
		PinNumber:    mrc20Resp.PinNumber,
		Chain:        mrc20Resp.Chain,
		Holders:      mrc20Resp.Holders,
		TxCount:      mrc20Resp.TxCount,
		MetaId:       mrc20Resp.MetaId,
		Address:      mrc20Resp.Address,
		DeployTime:   mrc20Resp.DeployTime,
	}, nil
}

func FetchTickUsedShove(tickId string) []string {
	var (
		shovelList []string
		mrc20Info  *man_service.Mrc20TickUsedShovelResp
	)
	mrc20Info, _ = man_service.FetchMrc20TickUsedShovelList(tickId, 0, 1000)
	if mrc20Info == nil {
		return nil
	}
	shovelList = mrc20Info.List
	if mrc20Info.Total > 1000 {
		mrc20Info, _ = man_service.FetchMrc20TickUsedShovelList(tickId, 0, mrc20Info.Total)
		if mrc20Info == nil {
			return nil
		}
		shovelList = mrc20Info.List
	}

	return shovelList
}
