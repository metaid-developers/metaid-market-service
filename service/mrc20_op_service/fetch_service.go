package mrc20_op_service

import (
	"errors"
	"fmt"
	"metaid-market-service/common"
	"metaid-market-service/conf"
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
	"metaid-market-service/models"
	"metaid-market-service/service/common_service"
	"metaid-market-service/service/man_service"
	"metaid-market-service/tool"
	"strconv"
	"strings"
)

func FetchMrc20OpOrders(req *request.FetchMrc20OpOrdersRequest, publicKey, ip string) (*respond.FetchMrc20OpOrdersResp, error) {
	switch req.OpOrderType {
	case "deploy":
		return fetchMrc20DeployOrders(req)
	case "mint":
		return fetchMrc20MintOrders(req)
	case "transfer":
		return fetchMrc20TransferOrders(req)
	default:
		return nil, errors.New("op order type is invalid")
	}
}

func fetchMrc20DeployOrders(req *request.FetchMrc20OpOrdersRequest) (*respond.FetchMrc20OpOrdersResp, error) {
	var (
		entityList []*models.Mrc20DeployOrderModel
		total      int64                         = 0
		list       []*respond.OpOrderInfoResp    = make([]*respond.OpOrderInfoResp, 0)
		filter     *models.Mrc20DeployOrderModel = &models.Mrc20DeployOrderModel{
			Address:       req.Address,
			InscribeState: models.InscribeStateFinish,
		}
		tickInfoMap map[string]*man_service.Mrc20TickInfo = make(map[string]*man_service.Mrc20TickInfo)
	)
	if req.Address == "" {
		return nil, errors.New("address is empty")
	}
	if req.TickId != "" {
		filter.TickId = req.TickId
	}
	total, _ = models.Mrc20DeployOrderModelDao().Count(filter)
	entityList, _ = models.Mrc20DeployOrderModelDao().GetList(filter, req.Cursor, req.Size)
	for _, v := range entityList {
		if _, ok := tickInfoMap[v.TickId]; !ok {
			tickInfo, _ := man_service.FetchMrc20TickInfo(v.TickId, "")
			if tickInfo == nil {
				continue
			}
			tickInfoMap[v.TickId] = tickInfo
		}
		pinCheck := ""
		payCheck := ""
		metaData := ""
		deployState := 0
		holders := int64(0)

		if tickInfoMap[v.TickId] != nil {
			metaData = tickInfoMap[v.TickId].Metadata
			pinCheck = tool.AnyToStr(tickInfoMap[v.TickId].PinCheck)
			payCheck = tool.AnyToStr(tickInfoMap[v.TickId].PayCheck)
			holders = tickInfoMap[v.TickId].Holders
		}
		if v.ConfirmationState == models.ConfirmationStateConfirmed {
			if tickInfoMap[v.TickId] != nil {
				deployState = 1
			} else {
				deployState = 2
			}
		}
		list = append(list, &respond.OpOrderInfoResp{
			OpOrderType:       "deploy",
			OrderId:           v.OrderId,
			TickId:            v.TickId,
			Tick:              v.Tick,
			TickName:          v.TokenName,
			Decimals:          v.Decimals,
			AmtPerMint:        v.AmtPerMint,
			DeployState:       deployState,
			MintCount:         v.MintCount,
			PremineCount:      v.PremineCount,
			TotalMinted:       v.MintCount,
			StartBlockHeight:  v.StartBlockHeight,
			EndBlockHeight:    v.EndBlockHeight,
			Qual:              pinCheck,
			PinCheck:          pinCheck,
			PayCheck:          payCheck,
			UsedPins:          nil,
			Holders:           holders,
			TxId:              v.TxId,
			BlockHeight:       v.BlockHeight,
			ConfirmationState: v.ConfirmationState,
			Timestamp:         v.Timestamp,
			DeployerAddress:   v.Address,
			DeployerMetaId:    common.GetMetaIdByAddress(v.Address),
			DeployerUserInfo:  common.FetchMetaIDUserInfo(v.Address),
			MetaData:          metaData,
		})
	}
	return &respond.FetchMrc20OpOrdersResp{
		Total: total,
		List:  list,
	}, nil
}

func fetchMrc20MintOrders(req *request.FetchMrc20OpOrdersRequest) (*respond.FetchMrc20OpOrdersResp, error) {
	var (
		entityList []*models.Mrc20MintOrderModel
		total      int64                       = 0
		list       []*respond.OpOrderInfoResp  = make([]*respond.OpOrderInfoResp, 0)
		filter     *models.Mrc20MintOrderModel = &models.Mrc20MintOrderModel{
			Address:       req.Address,
			InscribeState: models.InscribeStateFinish,
		}
		tickInfoMap map[string]*man_service.Mrc20TickInfo = make(map[string]*man_service.Mrc20TickInfo)
	)
	if req.Address == "" {
		return nil, errors.New("address is empty")
	}
	if req.TickId != "" {
		filter.TickId = req.TickId
	}

	if req.Confirmation != 0 {
		filter.ConfirmationState = req.Confirmation
	}

	total, _ = models.Mrc20MintOrderModelDao().Count(filter)
	entityList, _ = models.Mrc20MintOrderModelDao().GetList(filter, req.Cursor, req.Size)
	for _, v := range entityList {

		mintPins := make([]string, 0)
		if v.MintPins != "" {
			mintPins = strings.Split(v.MintPins, ",")
		}
		mintIndex := 0
		newMintPins := make([]string, 0)
		for _, pinId := range mintPins {
			pinInfo, _ := man_service.FetchPinInfo(conf.Net, pinId)
			if pinInfo != nil {
				pinId = fmt.Sprintf("%s-%d", pinId, pinInfo.PopLv)
			} else {
				pinId = fmt.Sprintf("%s-%d", pinId, 0)
			}
			has := false
			for _, pin := range newMintPins {
				if strings.Contains(pin, pinId) {
					has = true
					break
				}
			}
			if has {
				continue
			}
			newMintPins = append(newMintPins, pinId)
			mintIndex++
		}

		tick := ""
		tokenName := ""
		decimals := ""
		amtPerMint := ""
		mintCount := ""
		premineCount := ""
		totalMinted := ""
		startBlockHeight := ""
		deployerAddress := ""
		metaData := ""
		var pinCheck, payCheck interface{}
		if _, ok := tickInfoMap[v.TickId]; !ok {
			tickInfo, _ := man_service.FetchMrc20TickInfo(v.TickId, "")
			if tickInfo == nil {
				continue
			}
			tickInfoMap[v.TickId] = tickInfo
		}
		if tickInfoMap[v.TickId] != nil {
			tick = tickInfoMap[v.TickId].Tick
			tokenName = tickInfoMap[v.TickId].TokenName
			decimals = tickInfoMap[v.TickId].Decimals
			amtPerMint = tickInfoMap[v.TickId].AmtPerMint
			mintCount = strconv.FormatInt(tickInfoMap[v.TickId].MintCount, 10)
			premineCount = strconv.FormatInt(tickInfoMap[v.TickId].PremineCount, 10)
			totalMinted = strconv.FormatInt(tickInfoMap[v.TickId].TotalMinted, 10)
			startBlockHeight = tickInfoMap[v.TickId].BlockHeight
			pinCheck = tickInfoMap[v.TickId].PinCheck
			payCheck = tickInfoMap[v.TickId].PayCheck
			deployerAddress = tickInfoMap[v.TickId].Address
			metaData = tickInfoMap[v.TickId].Metadata
		}
		mintState := 0
		if v.ConfirmationState == models.ConfirmationStateConfirmed {
			txPointInfo, _, _ := common_service.FetchTxPointInfo(v.TxId, int64(mintIndex)+1, 0, 100)
			if txPointInfo == nil || len(txPointInfo) == 0 {
				txPointInfo, _, _ = common_service.FetchTxPointInfo(v.TxId, int64(mintIndex), 0, 100)
			}
			if txPointInfo != nil && len(txPointInfo) > 0 && txPointInfo[0].Verify {
				mintState = 1
			} else {
				mintState = 2
			}
		}

		list = append(list, &respond.OpOrderInfoResp{
			OpOrderType:       "mint",
			OrderId:           v.OrderId,
			TickId:            v.TickId,
			Tick:              tick,
			TickName:          tokenName,
			Decimals:          decimals,
			AmtPerMint:        amtPerMint,
			MintCount:         mintCount,
			PremineCount:      premineCount,
			TotalMinted:       totalMinted,
			MintState:         mintState,
			StartBlockHeight:  startBlockHeight,
			Qual:              tool.AnyToStr(pinCheck),
			PinCheck:          tool.AnyToStr(pinCheck),
			PayCheck:          tool.AnyToStr(payCheck),
			UsedPins:          newMintPins,
			TxId:              v.TxId,
			BlockHeight:       v.BlockHeight,
			ConfirmationState: v.ConfirmationState,
			Timestamp:         v.CreateTime,
			DeployerAddress:   deployerAddress,
			DeployerMetaId:    common.GetMetaIdByAddress(deployerAddress),
			DeployerUserInfo:  common.FetchMetaIDUserInfo(deployerAddress),
			MetaData:          metaData,
		})
	}
	return &respond.FetchMrc20OpOrdersResp{
		Total: total,
		List:  list,
	}, nil
}

func fetchMrc20TransferOrders(req *request.FetchMrc20OpOrdersRequest) (*respond.FetchMrc20OpOrdersResp, error) {
	var (
		entityList []*models.Mrc20TransferOrderModel
		total      int64                           = 0
		list       []*respond.OpOrderInfoResp      = make([]*respond.OpOrderInfoResp, 0)
		filter     *models.Mrc20TransferOrderModel = &models.Mrc20TransferOrderModel{
			Address:       req.Address,
			InscribeState: models.InscribeStateFinish,
		}
		tickInfoMap map[string]*man_service.Mrc20TickInfo = make(map[string]*man_service.Mrc20TickInfo)
	)
	if req.Address == "" {
		return nil, errors.New("address is empty")
	}
	if req.TickId != "" {
		filter.TickId = req.TickId
	}

	total, _ = models.Mrc20TransferOrderModelDao().Count(filter)
	entityList, _ = models.Mrc20TransferOrderModelDao().GetList(filter, req.Cursor, req.Size)
	for _, v := range entityList {

		tick := ""
		tokenName := ""
		deployerAddress := ""
		metaData := ""
		if _, ok := tickInfoMap[v.TickId]; !ok {
			tickInfo, _ := man_service.FetchMrc20TickInfo(v.TickId, "")
			if tickInfo == nil {
				continue
			}
			tickInfoMap[v.TickId] = tickInfo
		}
		if tickInfoMap[v.TickId] != nil {
			tick = tickInfoMap[v.TickId].Tick
			tokenName = tickInfoMap[v.TickId].TokenName
			deployerAddress = tickInfoMap[v.TickId].Address
			metaData = tickInfoMap[v.TickId].Metadata
		}

		list = append(list, &respond.OpOrderInfoResp{
			OpOrderType:       "transfer",
			OrderId:           v.OrderId,
			TickId:            v.TickId,
			Tick:              tick,
			TickName:          tokenName,
			TxId:              v.TxId,
			BlockHeight:       v.BlockHeight,
			ConfirmationState: v.ConfirmationState,
			Timestamp:         v.Timestamp,
			DeployerAddress:   deployerAddress,
			DeployerMetaId:    common.GetMetaIdByAddress(deployerAddress),
			DeployerUserInfo:  common.FetchMetaIDUserInfo(deployerAddress),
			MetaData:          metaData,
		})
	}
	return &respond.FetchMrc20OpOrdersResp{
		Total: total,
		List:  list,
	}, nil
}
