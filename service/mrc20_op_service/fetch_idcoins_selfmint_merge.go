package mrc20_op_service

import (
	"errors"
	"metaid-market-service/common"
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
	"metaid-market-service/models"
	"metaid-market-service/service/man_service"
	"metaid-market-service/tool"
	"sort"
	"strings"

	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/btcsuite/btcd/wire"

	"gorm.io/gorm"
)

// ---- self-hosted mint order → response conversion (same shape + semantics as
// the legacy order_book_db rows) ----

func selfMintState(entity *models.IdCoinsMintOrderModel) (int, string) {
	mintState := 0
	if entity.ConfirmationState == models.ConfirmationStateConfirmed {
		mintPinCount := 0
		if entity.MintPins != "" {
			mintPinCount = len(strings.Split(entity.MintPins, ","))
		}
		if fetchIdCoinTxVerifiedCached(entity.RevealMintTxId, int64(mintPinCount+1)) {
			mintState = 1
		} else {
			mintState = 2
		}
	}
	return mintState, ""
}

func buildSelfMintOrderResponse(entity *models.IdCoinsMintOrderModel, tickInfo *man_service.Mrc20TickInfo) *respond.IdCoinsOpOrderInfoResp {
	mintCount, premineCount, totalMinted := convertLegacyTickStrings(tickInfo)
	newMintPins := buildLegacyUsedPins(entity.MintPins)
	mintState, refundTxId := selfMintState(entity)
	deployerAddress := ""
	metaData := ""
	if tickInfo != nil {
		deployerAddress = tickInfo.Address
		metaData = tickInfo.Metadata
	}
	return &respond.IdCoinsOpOrderInfoResp{
		OpOrderType:       "mint",
		OrderId:           entity.OrderId,
		TickId:            entity.TickId,
		Tick:              tickInfo.Tick,
		TickName:          tickInfo.TokenName,
		Decimals:          tickInfo.Decimals,
		AmtPerMint:        tickInfo.AmtPerMint,
		MintCount:         mintCount,
		PremineCount:      premineCount,
		TotalMinted:       totalMinted,
		MintState:         mintState,
		StartBlockHeight:  tickInfo.BlockHeight,
		LiquidityPerMint:  entity.PayToAmount,
		FollowersLimit:    mintCount,
		Qual:              tool.AnyToStr(tickInfo.PinCheck),
		PinCheck:          tool.AnyToStr(tickInfo.PinCheck),
		PayCheck:          tool.AnyToStr(tickInfo.PayCheck),
		UsedPins:          newMintPins,
		TxId:              entity.RevealMintTxId,
		RefundTxId:        refundTxId,
		BlockHeight:       entity.BlockHeight,
		ConfirmationState: entity.ConfirmationState,
		Timestamp:         entity.CreateTime,
		DeployerAddress:   deployerAddress,
		DeployerMetaId:    common.GetMetaIdByAddress(deployerAddress),
		DeployerUserInfo:  fetchIdCoinUserInfoCached(deployerAddress),
		MetaData:          metaData,
	}
}

// selfMintOrderFilter mirrors legacyMintOrderFilter: same address filter, same
// finish/refund state filter, same optional tickId filter.
func selfMintOrderFilter(query *gorm.DB, address, tickId string) *gorm.DB {
	query = query.Where("address = ?", address).Where("inscribeState IN ?", []int64{int64(models.InscribeStateFinish), legacyInscribeStateRefund})
	if tickId != "" {
		query = query.Where("tickId = ?", tickId)
	}
	return query
}

func listSelfMintOrders(db *gorm.DB, address, tickId string) ([]*models.IdCoinsMintOrderModel, error) {
	list := make([]*models.IdCoinsMintOrderModel, 0)
	err := selfMintOrderFilter(db.Model(&models.IdCoinsMintOrderModel{}), address, tickId).Order("timestamp desc").Find(&list).Error
	return list, err
}

func countSelfMintOrders(db *gorm.DB, address, tickId string) (int64, error) {
	var total int64
	err := selfMintOrderFilter(db.Model(&models.IdCoinsMintOrderModel{}), address, tickId).Count(&total).Error
	return total, err
}

// fetchMergedIdCoinsMintOrders merges legacy order_book_db rows with the new
// self-hosted table, sorted by response timestamp desc, with the total and the
// cursor/size window applied across BOTH sources.
func fetchMergedIdCoinsMintOrders(req *request.FetchIdCoinsOpOrdersRequest) (*respond.FetchIdCoinsOpOrdersResp, error) {
	if req.Address == "" {
		return nil, errors.New("address is empty")
	}
	db, err := legacySqlDB()
	if err != nil {
		return nil, err
	}

	// legacy rows, unpaginated (per-address mint history is small)
	legacyList := make([]*legacyIdCoinsMintOrder, 0)
	err = legacyMintOrderFilter(db.Table(legacyDBTable("tb_id_coins_mint_order")).Select(legacyMintOrderReadColumns), req).
		Order("timestamp desc").Find(&legacyList).Error
	if err != nil {
		return nil, err
	}

	selfList, err := listSelfMintOrders(db, req.Address, req.TickId)
	if err != nil {
		return nil, err
	}

	tickInfoMap := make(map[string]*man_service.Mrc20TickInfo)
	merged := make([]*respond.IdCoinsOpOrderInfoResp, 0, len(legacyList)+len(selfList))
	getTick := func(tickId string) *man_service.Mrc20TickInfo {
		if _, ok := tickInfoMap[tickId]; !ok {
			tickInfo, _ := fetchIdCoinTickInfoCached(tickId)
			tickInfoMap[tickId] = tickInfo
		}
		return tickInfoMap[tickId]
	}
	for _, entity := range legacyList {
		tickInfo := getTick(entity.TickId)
		if tickInfo == nil {
			continue
		}
		merged = append(merged, buildLegacyMintOrderResponse(entity, tickInfo))
	}
	for _, entity := range selfList {
		tickInfo := getTick(entity.TickId)
		if tickInfo == nil {
			continue
		}
		merged = append(merged, buildSelfMintOrderResponse(entity, tickInfo))
	}

	page, total := mergeAndWindowMintOrders(merged, req.Cursor, req.Size)
	return &respond.FetchIdCoinsOpOrdersResp{Total: total, List: page}, nil
}

// mergeAndWindowMintOrders sorts the combined legacy+self-hosted rows by
// response timestamp desc and applies the cursor/size window across both
// sources. Pure function — unit-tested directly.
func mergeAndWindowMintOrders(merged []*respond.IdCoinsOpOrderInfoResp, cursor, size int64) ([]*respond.IdCoinsOpOrderInfoResp, int64) {
	sort.SliceStable(merged, func(i, j int) bool {
		return merged[i].Timestamp > merged[j].Timestamp
	})
	total := int64(len(merged))
	start := cursor
	if start < 0 {
		start = 0
	}
	if start > total {
		start = total
	}
	end := total
	if size > 0 && start+size < end {
		end = start + size
	}
	page := merged[start:end]
	if page == nil {
		page = []*respond.IdCoinsOpOrderInfoResp{}
	}
	return page, total
}

// fetchMergedAddressIdCoinsMintOrder answers the single-order lookup: a
// finished self-hosted mint order for this address+tickId wins; otherwise fall
// back to the legacy row (exact legacy semantics there).
func fetchMergedAddressIdCoinsMintOrder(req *request.FetchIdCoinsMintOrderRequest) (*respond.FetchOneIdCoinsMintOrderResp, error) {
	if req.Address == "" {
		return nil, errors.New("address is empty")
	}
	if req.TickId == "" {
		return nil, errors.New("tickId is empty")
	}
	db, err := legacySqlDB()
	if err != nil {
		return nil, err
	}
	self := &models.IdCoinsMintOrderModel{}
	err = db.Model(&models.IdCoinsMintOrderModel{}).
		Where("address = ? AND tickId = ? AND inscribeState = ?", req.Address, req.TickId, int64(models.InscribeStateFinish)).
		Order("id asc").First(self).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fetchLegacyAddressIdCoinsMintOrder(req)
		}
		return nil, err
	}

	tickInfo, err := fetchIdCoinTickInfoCached(req.TickId)
	if err != nil {
		return nil, err
	}
	if tickInfo == nil {
		return nil, errors.New("no record")
	}
	mintCount, premineCount, totalMinted := convertLegacyTickStrings(tickInfo)
	mintState, _ := selfMintState(self)
	return &respond.FetchOneIdCoinsMintOrderResp{
		AddressMintState:  1,
		OpOrderType:       "mint",
		OrderId:           self.OrderId,
		TickId:            self.TickId,
		Tick:              tickInfo.Tick,
		TickName:          tickInfo.TokenName,
		Decimals:          tickInfo.Decimals,
		AmtPerMint:        tickInfo.AmtPerMint,
		MintCount:         mintCount,
		PremineCount:      premineCount,
		TotalMinted:       totalMinted,
		MintState:         mintState,
		StartBlockHeight:  tickInfo.BlockHeight,
		Qual:              tool.AnyToStr(tickInfo.PinCheck),
		PinCheck:          tool.AnyToStr(tickInfo.PinCheck),
		PayCheck:          tool.AnyToStr(tickInfo.PayCheck),
		TxId:              self.RevealMintTxId,
		BlockHeight:       self.BlockHeight,
		ConfirmationState: self.ConfirmationState,
		Timestamp:         self.CreateTime,
		DeployerAddress:   tickInfo.Address,
		DeployerMetaId:    common.GetMetaIdByAddress(tickInfo.Address),
		DeployerUserInfo:  fetchIdCoinUserInfoCached(tickInfo.Address),
		MetaData:          tickInfo.Metadata,
	}, nil
}

// ParseRevealMintOutputs decodes a reveal-mint tx for structural verification
// in tests.
func ParseRevealMintOutputs(rawHex string) ([]*wire.TxOut, error) {
	tx := wire.NewMsgTx(2)
	b, err := hex.DecodeString(rawHex)
	if err != nil {
		return nil, fmt.Errorf("decode hex: %w", err)
	}
	if err := tx.Deserialize(bytes.NewReader(b)); err != nil {
		return nil, fmt.Errorf("deserialize: %w", err)
	}
	return tx.TxOut, nil
}
