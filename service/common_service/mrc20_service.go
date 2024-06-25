package common_service

import (
	"errors"
	"github.com/godaddy-x/freego/utils/decimal"
	"metaid-market-service/common"
	"metaid-market-service/conf"
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
	"metaid-market-service/service/man_service"
	"strconv"
	"strings"
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
	if mrc20Resp == nil {
		return nil, errors.New("mrc20 not found")
	}
	totalSupply := "0"
	mintable := false
	remaining := "0"
	supply := "0"
	if mrc20Resp.AmtPerMint != "" && mrc20Resp.MintCount != 0 {
		totalMintedDe := decimal.New(mrc20Resp.TotalMinted, 0)
		premineCountDe := decimal.New(mrc20Resp.PremineCount, 0)
		amtPerMintDe, _ := decimal.NewFromString(mrc20Resp.AmtPerMint)
		mintCountDe := decimal.New(mrc20Resp.MintCount, 0)
		supplyDe := totalMintedDe.Add(premineCountDe).Mul(amtPerMintDe)
		supply = supplyDe.String()

		totalSupplyDe := mintCountDe.Mul(amtPerMintDe)
		totalSupply = totalSupplyDe.String()

		remainingDe := mintCountDe.Sub(totalMintedDe.Add(premineCountDe)).Mul(amtPerMintDe)
		remaining = remainingDe.String()
		if remainingDe.GreaterThan(decimal.Zero) {
			mintable = true
		}
	}

	price := "0"
	priceUsd := "0.00"
	change24h := "+0.00%"
	marketCap := "0"
	marketCapUsd := "0.00"

	return &respond.Mrc20TickInfo{
		Tick:             mrc20Resp.Tick,
		TokenName:        mrc20Resp.TokenName,
		Decimals:         mrc20Resp.Decimals,
		AmtPerMint:       mrc20Resp.AmtPerMint,
		MintCount:        strconv.FormatInt(mrc20Resp.MintCount, 10),
		PremineCount:     strconv.FormatInt(mrc20Resp.PremineCount, 10),
		BlockHeight:      mrc20Resp.BlockHeight,
		MetaData:         mrc20Resp.Metadata,
		Type:             mrc20Resp.Type,
		Qual:             mrc20Resp.Qual,
		TotalMinted:      strconv.FormatInt(mrc20Resp.TotalMinted, 10),
		Mrc20Id:          mrc20Resp.Mrc20Id,
		PinNumber:        mrc20Resp.PinNumber,
		Holders:          mrc20Resp.Holders,
		TxCount:          mrc20Resp.TxCount,
		DeployerMetaId:   mrc20Resp.MetaId,
		DeployerAddress:  mrc20Resp.Address,
		DeployTime:       mrc20Resp.DeployTime,
		DeployerUserInfo: common.FetchMetaIDUserInfo(mrc20Resp.Address),
		Price:            price,
		PriceUsd:         priceUsd,
		Change24h:        change24h,
		MarketCap:        marketCap,
		MarketCapUsd:     marketCapUsd,
		TotalSupply:      totalSupply,
		Supply:           supply,
		Mintable:         mintable,
		Remaining:        remaining,
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

func FetchMrc20TickAddressBalances(req *request.Mrc20AddressBalancesReq) (*respond.Mrc20BalanceInfoResp, error) {
	var (
		mrc20Resp *man_service.Mrc20BalanceResp
		err       error
		list      []*respond.Mrc20BalanceInfo = make([]*respond.Mrc20BalanceInfo, 0)
		total     int64                       = 0
	)
	mrc20Resp, err = man_service.FetchMrc20AddressBalanceList(req.Address, req.Cursor, req.Size)
	if err != nil {
		return nil, err
	}
	if mrc20Resp != nil {
		for _, v := range mrc20Resp.List {
			tickInfo, err := man_service.FetchMrc20TickInfo(v.Id)
			if err != nil {
				return nil, err
			}
			if tickInfo == nil || tickInfo.Mrc20Id == "" {
				return nil, errors.New("mrc20 not found")
			}

			item := &respond.Mrc20BalanceInfo{
				Tick:      tickInfo.Tick,
				TokenName: tickInfo.TokenName,
				Mrc20Id:   v.Id,
				Balance:   strconv.FormatInt(v.Balance, 10),
				Decimals:  tickInfo.Decimals,
			}
			list = append(list, item)
		}
		total = mrc20Resp.Total
	}
	return &respond.Mrc20BalanceInfoResp{
		Total: total,
		List:  list,
	}, nil
}

func FetchMrc20TickAddressUtxos(req *request.Mrc20AddressUtxosReq) (*respond.Mrc20UtxoResp, error) {
	var (
		tickInfo  *man_service.Mrc20TickInfo
		mrc20Resp *man_service.Mrc20UtxoResp
		err       error
		list      []*respond.Mrc20Utxo = make([]*respond.Mrc20Utxo, 0)
		total     int64                = 0
	)
	tickInfo, err = man_service.FetchMrc20TickInfo(req.TickId)
	if err != nil {
		return nil, err
	}
	if tickInfo == nil || tickInfo.Mrc20Id == "" {
		return nil, errors.New("mrc20 not found")
	}

	mrc20Resp, err = man_service.FetchMrc20AddressUtxoList(req.Address, req.TickId, req.Cursor, req.Size)
	if err != nil {
		return nil, err
	}
	if mrc20Resp != nil {
		for _, v := range mrc20Resp.List {

			if !v.Verify {
				continue
			}
			if v.AmtChange == 0 {
				continue
			}

			address := v.ToAddress
			txPoint := v.TxPoint
			txPointStrs := strings.Split(txPoint, ":")
			txId := ""
			vout := int64(0)
			if len(txPointStrs) == 2 {
				txId = txPointStrs[0]
				vout, _ = strconv.ParseInt(txPointStrs[1], 10, 64)
			}
			pkScript, _ := common.AddressToPkScript(conf.Net, address)
			satoshi := int64(546)
			if v.PointValue > 0 {
				satoshi = v.PointValue
			}

			mrc20s := make([]*respond.Mrc20Info, 0)
			mrc20s = append(mrc20s, &respond.Mrc20Info{
				Tick:     v.Tick,
				Mrc20Id:  v.Mrc20Id,
				TxPoint:  v.TxPoint,
				Amount:   strconv.FormatInt(v.AmtChange, 10),
				Decimals: tickInfo.Decimals,
			})

			txPointInfo, txPointTotal, err := FetchTxPointInfo(txId, vout, 0, 100)
			if err != nil {
				return nil, err
			}
			if txPointTotal > 1 {
				for _, p := range txPointInfo {
					if p.Mrc20Id == v.Mrc20Id {
						continue
					}
					mrc20s = append(mrc20s, &respond.Mrc20Info{
						Tick:     p.Tick,
						Mrc20Id:  p.Mrc20Id,
						TxPoint:  p.TxPoint,
						Amount:   strconv.FormatInt(p.AmtChange, 10),
						Decimals: tickInfo.Decimals,
					})
				}
			}

			list = append(list, &respond.Mrc20Utxo{
				Chain:       v.Chain,
				BlockHeight: v.BlockHeight,
				Address:     address,
				Satoshi:     satoshi,
				Satoshis:    satoshi,
				ScriptPk:    pkScript,
				TxId:        txId,
				Vout:        vout,
				OutputIndex: vout,
				Mrc20s:      mrc20s,
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

func FetchMrc20TickList(req *request.FetchMrc20TickListReq) (*respond.Mrc20TickListResp, error) {
	var (
		mrc20Resp *man_service.Mrc20TickListResp
		err       error
		list      []*respond.Mrc20TickInfo = make([]*respond.Mrc20TickInfo, 0)
		total     int64                    = 0
	)
	mrc20Resp, err = man_service.FetchMrc20TickList(req.Cursor, req.Size, req.Completed, req.OrderBy)
	if err != nil {
		return nil, err
	}
	if mrc20Resp != nil {
		for _, v := range mrc20Resp.List {
			totalSupply := "0"
			mintable := false
			remaining := "0"
			supply := "0"
			if v.AmtPerMint != "" && v.MintCount != 0 {
				totalMintedDe := decimal.New(v.TotalMinted, 0)
				premineCountDe := decimal.New(v.PremineCount, 0)
				amtPerMintDe, _ := decimal.NewFromString(v.AmtPerMint)
				mintCountDe := decimal.New(v.MintCount, 0)

				supplyDe := totalMintedDe.Add(premineCountDe).Mul(amtPerMintDe)
				supply = supplyDe.String()

				totalSupplyDe := mintCountDe.Mul(amtPerMintDe)
				totalSupply = totalSupplyDe.String()

				remainingDe := mintCountDe.Sub(totalMintedDe.Add(premineCountDe)).Mul(amtPerMintDe)
				remaining = remainingDe.String()
				if remainingDe.GreaterThan(decimal.Zero) {
					mintable = true
				}
			}

			price := "0"
			priceUsd := "0.00"
			change24h := "+0.00%"
			marketCap := "0"
			marketCapUsd := "0.00"

			item := &respond.Mrc20TickInfo{
				Tick:             v.Tick,
				TokenName:        v.TokenName,
				Decimals:         v.Decimals,
				AmtPerMint:       v.AmtPerMint,
				MintCount:        strconv.FormatInt(v.MintCount, 10),
				PremineCount:     strconv.FormatInt(v.PremineCount, 10),
				BlockHeight:      v.BlockHeight,
				MetaData:         v.Metadata,
				Type:             v.Type,
				Qual:             v.Qual,
				TotalMinted:      strconv.FormatInt(v.TotalMinted, 10),
				Mrc20Id:          v.Mrc20Id,
				PinNumber:        v.PinNumber,
				Holders:          v.Holders,
				TxCount:          v.TxCount,
				DeployerMetaId:   v.MetaId,
				DeployerAddress:  v.Address,
				DeployTime:       v.DeployTime,
				DeployerUserInfo: common.FetchMetaIDUserInfo(v.Address),
				Price:            price,
				PriceUsd:         priceUsd,
				Change24h:        change24h,
				MarketCap:        marketCap,
				MarketCapUsd:     marketCapUsd,
				TotalSupply:      totalSupply,
				Supply:           supply,
				Mintable:         mintable,
				Remaining:        remaining,
			}

			list = append(list, item)
		}
		total = mrc20Resp.Total
	}
	return &respond.Mrc20TickListResp{
		Total: total,
		List:  list,
	}, nil
}
