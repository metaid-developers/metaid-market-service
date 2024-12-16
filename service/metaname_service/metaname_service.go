package metaname_service

import (
	"errors"
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
	"metaid-market-service/models"
)

func FetchAddressMetaNameOrder(req *request.FetchMetaNameOpOrdersRequest, publicKey, ip string) (*respond.FetchMetaNameOpOrdersResp, error) {
	var (
		total      int64 = 0
		entityList []*models.MetanameRegisterOrderModel
		filter     *models.MetanameRegisterOrderModel = &models.MetanameRegisterOrderModel{
			RegisterAddress: req.Address,
			InscribeState:   models.InscribeStateFinish,
		}

		list []*respond.MetaNameOpOrderInfoResp = make([]*respond.MetaNameOpOrderInfoResp, 0)
	)
	if req.Address == "" {
		return nil, errors.New("address is empty")
	}
	if req.Confirmation != 0 {
		filter.ConfirmationState = req.Confirmation
	}

	total, _ = models.MetanameRegisterOrderModelDao().Count(filter)
	entityList, _ = models.MetanameRegisterOrderModelDao().GetList(filter, req.Cursor, req.Size)
	for _, v := range entityList {
		item := &respond.MetaNameOpOrderInfoResp{
			OpOrderType:       "register",
			OrderId:           v.OrderId,
			MetaName:          v.Metaname,
			Name:              v.Name,
			Namespace:         v.Namespace,
			ReceiveAddress:    v.ReceiveAddress,
			RegisterAddress:   v.RegisterAddress,
			RegisterState:     0,
			TxId:              v.TxId,
			BlockHeight:       v.BlockHeight,
			ConfirmationState: v.ConfirmationState,
			Timestamp:         v.CreateTime,
			MetaData:          "",
		}
		list = append(list, item)
	}

	return &respond.FetchMetaNameOpOrdersResp{
		Total: total,
		List:  list,
	}, nil
}
