package mrc20_op_service

import (
	"errors"
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
	"metaid-market-service/models"
	"metaid-market-service/service/man_service"
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
			Address: req.Address,
		}
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
		list = append(list, &respond.OpOrderInfoResp{
			OpOrderType:       "deploy",
			OrderId:           v.OrderId,
			TickId:            v.TickId,
			Tick:              v.Tick,
			TickName:          v.TokenName,
			TxId:              v.Address,
			BlockHeight:       v.BlockHeight,
			ConfirmationState: v.ConfirmationState,
			Timestamp:         v.Timestamp,
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
			Address: req.Address,
		}
		tickInfoMap map[string]*man_service.Mrc20TickInfo = make(map[string]*man_service.Mrc20TickInfo)
	)
	if req.Address == "" {
		return nil, errors.New("address is empty")
	}
	if req.TickId != "" {
		filter.TickId = req.TickId
	}

	total, _ = models.Mrc20MintOrderModelDao().Count(filter)
	entityList, _ = models.Mrc20MintOrderModelDao().GetList(filter, req.Cursor, req.Size)
	for _, v := range entityList {

		tick := ""
		tokenName := ""
		if _, ok := tickInfoMap[v.TickId]; !ok {
			tickInfo, _ := man_service.FetchMrc20TickInfo(v.TickId)
			if tickInfo == nil {
				continue
			}
			tickInfoMap[v.TickId] = tickInfo
		}
		if tickInfoMap[v.TickId] != nil {
			tick = tickInfoMap[v.TickId].Tick
			tokenName = tickInfoMap[v.TickId].TokenName
		}

		list = append(list, &respond.OpOrderInfoResp{
			OpOrderType:       "deploy",
			OrderId:           v.OrderId,
			TickId:            v.TickId,
			Tick:              tick,
			TickName:          tokenName,
			TxId:              v.Address,
			BlockHeight:       v.BlockHeight,
			ConfirmationState: v.ConfirmationState,
			Timestamp:         v.Timestamp,
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
			Address: req.Address,
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
		if _, ok := tickInfoMap[v.TickId]; !ok {
			tickInfo, _ := man_service.FetchMrc20TickInfo(v.TickId)
			if tickInfo == nil {
				continue
			}
			tickInfoMap[v.TickId] = tickInfo
		}
		if tickInfoMap[v.TickId] != nil {
			tick = tickInfoMap[v.TickId].Tick
			tokenName = tickInfoMap[v.TickId].TokenName
		}

		list = append(list, &respond.OpOrderInfoResp{
			OpOrderType:       "deploy",
			OrderId:           v.OrderId,
			TickId:            v.TickId,
			Tick:              tick,
			TickName:          tokenName,
			TxId:              v.Address,
			BlockHeight:       v.BlockHeight,
			ConfirmationState: v.ConfirmationState,
			Timestamp:         v.Timestamp,
		})
	}
	return &respond.FetchMrc20OpOrdersResp{
		Total: total,
		List:  list,
	}, nil
}
