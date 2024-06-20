package common_service

import (
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
	"metaid-market-service/service/man_service"
)

func FetchMrc20TickInfo(req *request.FetchMrc20TickInfoReq) (*respond.Mrc20TickInfo, error) {
	var (
		mrc20Resp *man_service.Mrc20TickInfo
		err       error
	)
	mrc20Resp, err = man_service.FetchMrc20TickInfo(req.TickId)
	if err != nil {
		return nil, err
	}
	return &respond.Mrc20TickInfo{
		Tick:        mrc20Resp.Tick,
		TokenName:   mrc20Resp.TokenName,
		Decimals:    mrc20Resp.Decimals,
		AmtPerMint:  mrc20Resp.AmtPerMint,
		MintCount:   mrc20Resp.MintCount,
		Blockheight: mrc20Resp.Blockheight,
		Metadata:    mrc20Resp.Metadata,
		Type:        mrc20Resp.Type,
		Qual:        mrc20Resp.Qual,
		TotalMinted: mrc20Resp.TotalMinted,
		Mrc20Id:     mrc20Resp.Mrc20Id,
		PinNumber:   mrc20Resp.PinNumber,
	}, nil
}

func FetchMrc20TickAddressShovels(req *request.Mrc20AddressShovelsReq) (*respond.Mrc20ShovelResp, error) {
	var (
		mrc20Resp *man_service.Mrc20ShovelResp
		err       error
		list      []*respond.ShovelInfo = make([]*respond.ShovelInfo, 0)
		total     int64                 = 0
	)
	mrc20Resp, err = man_service.FetchMrc20AddressShovelList(req.Address, req.TickId, req.Cursor, req.Size)
	if err != nil {
		return nil, err
	}
	if mrc20Resp != nil {
		for _, v := range mrc20Resp.List {
			list = append(list, &respond.ShovelInfo{
				Id:                 v.Id,
				Number:             v.Number,
				Metaid:             v.Metaid,
				Address:            v.Address,
				Creator:            v.Creator,
				InitialOwner:       v.InitialOwner,
				Output:             v.Output,
				OutputValue:        v.OutputValue,
				Timestamp:          v.Timestamp,
				GenesisFee:         v.GenesisFee,
				GenesisHeight:      v.GenesisHeight,
				GenesisTransaction: v.GenesisTransaction,
				TxIndex:            v.TxIndex,
				TxInIndex:          v.TxInIndex,
				Offset:             v.Offset,
				Location:           v.Location,
				Operation:          v.Operation,
				Path:               v.Path,
				ParentPath:         v.ParentPath,
				OriginalPath:       v.OriginalId,
				Encryption:         v.Encryption,
				Version:            v.Version,
				ContentType:        v.Content,
				ContentTypeDetect:  v.ContentTypeDetect,
				ContentBody:        v.ContentBody,
				ContentLength:      v.ContentLength,
				ContentSummary:     v.ContentSummary,
				Status:             v.Status,
				OriginalId:         v.OriginalId,
				IsTransfered:       v.IsTransfered,
				Preview:            v.Preview,
				Content:            v.Content,
				Pop:                v.Pop,
				PopLv:              v.PopLv,
				ChainName:          v.ChainName,
				DataValue:          v.DataValue,
				Mrc20Minted:        v.Mrc20Minted,
				Mrc20MintPin:       v.Mrc20MintPin,
			})
		}
		total = mrc20Resp.Total
	}
	return &respond.Mrc20ShovelResp{
		Total: total,
		List:  list,
	}, nil
}

func FetchMrc20TickAddressUtxos(req *request.Mrc20AddressUtxosReq) (*respond.Mrc20UtxoResp, error) {
	var (
		mrc20Resp *man_service.Mrc20UtxoResp
		err       error
		list      []*respond.Mrc20Utxo = make([]*respond.Mrc20Utxo, 0)
		total     int64                = 0
	)
	mrc20Resp, err = man_service.FetchMrc20AddressUtxoList(req.Address, req.TickId, req.Cursor, req.Size)
	if err != nil {
		return nil, err
	}
	if mrc20Resp != nil {
		for _, v := range mrc20Resp.List {
			list = append(list, &respond.Mrc20Utxo{
				Tick:        v.Tick,
				Mrc20Id:     v.Mrc20Id,
				TxPoint:     v.TxPoint,
				PinId:       v.PinId,
				PinContent:  v.PinContent,
				Verify:      v.Verify,
				BlockHeight: v.BlockHeight,
				MrcOption:   v.MrcOption,
				FromAddress: v.FromAddress,
				ToAddress:   v.ToAddress,
				ErrorMsg:    v.ErrorMsg,
				AmtChange:   v.AmtChange,
				Status:      v.Status,
				Chain:       v.Chain,
				Index:       v.Index,
				Timestamp:   v.Timestamp,
			})
		}
		total = mrc20Resp.Total
	}
	return &respond.Mrc20UtxoResp{
		Total: total,
		List:  list,
	}, nil
}
