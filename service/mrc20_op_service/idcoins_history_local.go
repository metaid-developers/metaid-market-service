package mrc20_op_service

import (
	"errors"
	"fmt"
	"metaid-market-service/common"
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
	"metaid-market-service/major"
	"metaid-market-service/models"
	"metaid-market-service/service/man_service"
	"metaid-market-service/tool"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

type legacyIdCoinsDeployOrder struct {
	OrderId           string                   `gorm:"column:orderId"`
	InscribeState     int64                    `gorm:"column:inscribeState"`
	IssuerMetaId      string                   `gorm:"column:issuerMetaId"`
	IssuerAddress     string                   `gorm:"column:issuerAddress"`
	TickId            string                   `gorm:"column:tickId"`
	Tick              string                   `gorm:"column:tick"`
	TokenName         string                   `gorm:"column:tokenName"`
	Decimals          string                   `gorm:"column:decimals"`
	AmtPerMint        string                   `gorm:"column:amtPerMint"`
	MintCount         string                   `gorm:"column:mintCount"`
	PremineCount      string                   `gorm:"column:premineCount"`
	StartBlockHeight  string                   `gorm:"column:startBlockHeight"`
	EndBlockHeight    string                   `gorm:"column:endBlockHeight"`
	PayCheckAmount    int64                    `gorm:"column:payCheckAmount"`
	TxId              string                   `gorm:"column:txId"`
	BlockHeight       int64                    `gorm:"column:blockHeight"`
	ConfirmationState models.ConfirmationState `gorm:"column:confirmationState"`
	Timestamp         int64                    `gorm:"column:timestamp"`
	CreateTime        int64                    `gorm:"column:createTime"`
}

type legacyIdCoinsMintOrder struct {
	Id                int64                    `gorm:"column:id"`
	OrderId           string                   `gorm:"column:orderId"`
	InscribeState     int64                    `gorm:"column:inscribeState"`
	Address           string                   `gorm:"column:address"`
	TickId            string                   `gorm:"column:tickId"`
	PayToAmount       int64                    `gorm:"column:payToAmount"`
	MintPins          string                   `gorm:"column:mintPins"`
	RevealMintTxId    string                   `gorm:"column:revealMintTxId"`
	BlockHeight       int64                    `gorm:"column:blockHeight"`
	ConfirmationState models.ConfirmationState `gorm:"column:confirmationState"`
	Timestamp         int64                    `gorm:"column:timestamp"`
	CreateTime        int64                    `gorm:"column:createTime"`
}

type legacyBookMintUtxoRefund struct {
	FromOrderId string `gorm:"column:fromOrderId"`
	RefundTxId  string `gorm:"column:refundTxId"`
}

const (
	legacyInscribeStateFinish    int64 = 3
	legacyInscribeStateRefund    int64 = 11
	legacyDeployOrderReadColumns       = "orderId,inscribeState,issuerMetaId,issuerAddress,tickId,tick,tokenName,decimals,amtPerMint,mintCount,premineCount,startBlockHeight,endBlockHeight,payCheckAmount,txId,blockHeight,confirmationState,timestamp,createTime"
	legacyMintOrderReadColumns         = "id,orderId,inscribeState,address,tickId,payToAmount,mintPins,revealMintTxId,blockHeight,confirmationState,timestamp,createTime"
)

func legacyDBTable(table string) string {
	return fmt.Sprintf("%s.%s", idCoinsLegacyDB(), table)
}

func legacySqlDB() (*gorm.DB, error) {
	db := major.GetSqlDB()
	if db == nil {
		return nil, errors.New("db is not initialized")
	}
	return db, nil
}

func legacyDeployOrderFilter(query *gorm.DB, req *request.FetchIdCoinsOpOrdersRequest) *gorm.DB {
	query = query.Where("issuerAddress = ? AND inscribeState = ?", req.Address, legacyInscribeStateFinish)
	if req.TickId != "" {
		query = query.Where("tickId = ?", req.TickId)
	}
	if req.Confirmation != 0 {
		query = query.Where("confirmationState = ?", req.Confirmation)
	}
	return query
}

func legacyDeployOrderCountQuery(db *gorm.DB, req *request.FetchIdCoinsOpOrdersRequest) *gorm.DB {
	return legacyDeployOrderFilter(db.Table(legacyDBTable("tb_id_coins_deploy_order")), req)
}

func legacyDeployOrderListQuery(db *gorm.DB, req *request.FetchIdCoinsOpOrdersRequest) *gorm.DB {
	query := legacyDeployOrderFilter(db.Table(legacyDBTable("tb_id_coins_deploy_order")).Select(legacyDeployOrderReadColumns), req)
	return query.Order("timestamp desc").Limit(int(req.Size)).Offset(int(req.Cursor))
}

func countLegacyDeployOrders(req *request.FetchIdCoinsOpOrdersRequest) (int64, error) {
	db, err := legacySqlDB()
	if err != nil {
		return 0, err
	}
	var total int64
	err = legacyDeployOrderCountQuery(db, req).Count(&total).Error
	return total, err
}

func listLegacyDeployOrders(req *request.FetchIdCoinsOpOrdersRequest) ([]*legacyIdCoinsDeployOrder, error) {
	db, err := legacySqlDB()
	if err != nil {
		return nil, err
	}
	list := make([]*legacyIdCoinsDeployOrder, 0)
	err = legacyDeployOrderListQuery(db, req).Find(&list).Error
	return list, err
}

func legacyMintOrderFilter(query *gorm.DB, req *request.FetchIdCoinsOpOrdersRequest) *gorm.DB {
	query = query.Where("address = ?", req.Address).Where("inscribeState IN ?", []int64{legacyInscribeStateFinish, legacyInscribeStateRefund})
	if req.TickId != "" {
		query = query.Where("tickId = ?", req.TickId)
	}
	return query
}

func legacyMintOrderCountQuery(db *gorm.DB, req *request.FetchIdCoinsOpOrdersRequest) *gorm.DB {
	return legacyMintOrderFilter(db.Table(legacyDBTable("tb_id_coins_mint_order")), req)
}

func legacyMintOrderListQuery(db *gorm.DB, req *request.FetchIdCoinsOpOrdersRequest) *gorm.DB {
	query := legacyMintOrderFilter(db.Table(legacyDBTable("tb_id_coins_mint_order")).Select(legacyMintOrderReadColumns), req)
	return query.Order("timestamp desc").Limit(int(req.Size)).Offset(int(req.Cursor))
}

func countLegacyMintOrders(req *request.FetchIdCoinsOpOrdersRequest) (int64, error) {
	db, err := legacySqlDB()
	if err != nil {
		return 0, err
	}
	var total int64
	err = legacyMintOrderCountQuery(db, req).Count(&total).Error
	return total, err
}

func listLegacyMintOrders(req *request.FetchIdCoinsOpOrdersRequest) ([]*legacyIdCoinsMintOrder, error) {
	db, err := legacySqlDB()
	if err != nil {
		return nil, err
	}
	list := make([]*legacyIdCoinsMintOrder, 0)
	err = legacyMintOrderListQuery(db, req).Find(&list).Error
	return list, err
}

// legacyMintOrderGetOneQuery matches the retired service's GetOne semantics:
// First() on a model whose primary key is id picks the oldest matching row,
// so order explicitly by id ascending.
func legacyMintOrderGetOneQuery(db *gorm.DB, address, tickId string) *gorm.DB {
	return db.Table(legacyDBTable("tb_id_coins_mint_order")).Select(legacyMintOrderReadColumns).Where("address = ? AND tickId = ? AND inscribeState = ?", address, tickId, legacyInscribeStateFinish).Order("id asc")
}

func legacyRefundTxIdQuery(db *gorm.DB, fromOrderId string) *gorm.DB {
	return db.Table(legacyDBTable("tb_book_mint_utxo")).Select("fromOrderId,refundTxId").Where("fromOrderId = ?", fromOrderId)
}

func getLegacyMintOrder(address, tickId string) (*legacyIdCoinsMintOrder, error) {
	db, err := legacySqlDB()
	if err != nil {
		return nil, err
	}
	item := &legacyIdCoinsMintOrder{}
	err = legacyMintOrderGetOneQuery(db, address, tickId).First(item).Error
	if err != nil {
		return nil, err
	}
	return item, nil
}

func getLegacyRefundTxId(fromOrderId string) string {
	db, err := legacySqlDB()
	if err != nil {
		return ""
	}
	item := &legacyBookMintUtxoRefund{}
	err = legacyRefundTxIdQuery(db, fromOrderId).First(item).Error
	if err != nil {
		return ""
	}
	return item.RefundTxId
}

func convertLegacyTickStrings(info *man_service.Mrc20TickInfo) (mintCount, premineCount, totalMinted string) {
	if info == nil {
		return "", "", ""
	}
	return strconv.FormatInt(info.MintCount, 10), strconv.FormatInt(info.PremineCount, 10), strconv.FormatInt(info.TotalMinted, 10)
}

func buildLegacyUsedPins(mintPins string) []string {
	pins := make([]string, 0)
	if mintPins != "" {
		pins = strings.Split(mintPins, ",")
	}
	newMintPins := make([]string, 0)
	for _, pinId := range pins {
		if pinId == "" {
			continue
		}
		popLv := fetchIdCoinPinPopCached(pinId)
		pinId = fmt.Sprintf("%s-%d", pinId, popLv)
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
	}
	return newMintPins
}

func fetchLegacyIdCoinsDeployOrders(req *request.FetchIdCoinsOpOrdersRequest) (*respond.FetchIdCoinsOpOrdersResp, error) {
	if req.Address == "" {
		return nil, errors.New("address is empty")
	}
	total, err := countLegacyDeployOrders(req)
	if err != nil {
		return nil, err
	}
	entityList, err := listLegacyDeployOrders(req)
	if err != nil {
		return nil, err
	}
	tickInfoMap := make(map[string]*man_service.Mrc20TickInfo)
	list := make([]*respond.IdCoinsOpOrderInfoResp, 0, len(entityList))
	for _, v := range entityList {
		if _, ok := tickInfoMap[v.TickId]; !ok {
			tickInfo, _ := fetchIdCoinTickInfoCached(v.TickId)
			tickInfoMap[v.TickId] = tickInfo
		}
		tickInfo := tickInfoMap[v.TickId]
		if tickInfo == nil {
			continue
		}
		pinCheck := tool.AnyToStr(tickInfo.PinCheck)
		payCheck := tool.AnyToStr(tickInfo.PayCheck)
		deployState := 0
		if v.ConfirmationState == models.ConfirmationStateConfirmed {
			deployState = 1
		}
		list = append(list, &respond.IdCoinsOpOrderInfoResp{
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
			FollowersLimit:    v.MintCount,
			LiquidityPerMint:  v.PayCheckAmount,
			Qual:              pinCheck,
			PinCheck:          pinCheck,
			PayCheck:          payCheck,
			UsedPins:          nil,
			Holders:           tickInfo.Holders,
			TxId:              v.TxId,
			BlockHeight:       v.BlockHeight,
			ConfirmationState: v.ConfirmationState,
			Timestamp:         v.Timestamp,
			DeployerAddress:   v.IssuerAddress,
			DeployerMetaId:    common.GetMetaIdByAddress(v.IssuerAddress),
			DeployerUserInfo:  fetchIdCoinUserInfoCached(v.IssuerAddress),
			MetaData:          tickInfo.Metadata,
		})
	}
	return &respond.FetchIdCoinsOpOrdersResp{Total: total, List: list}, nil
}

func legacyMintState(entity *legacyIdCoinsMintOrder) (int, string) {
	mintState := 0
	refundTxId := ""
	if entity.InscribeState == legacyInscribeStateRefund {
		mintState = 3
		refundTxId = getLegacyRefundTxId(entity.OrderId)
		return mintState, refundTxId
	}
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
	return mintState, refundTxId
}

func buildLegacyMintOrderResponse(entity *legacyIdCoinsMintOrder, tickInfo *man_service.Mrc20TickInfo) *respond.IdCoinsOpOrderInfoResp {
	mintCount, premineCount, totalMinted := convertLegacyTickStrings(tickInfo)
	newMintPins := buildLegacyUsedPins(entity.MintPins)
	mintState, refundTxId := legacyMintState(entity)
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

func fetchLegacyIdCoinsMintOrders(req *request.FetchIdCoinsOpOrdersRequest) (*respond.FetchIdCoinsOpOrdersResp, error) {
	if req.Address == "" {
		return nil, errors.New("address is empty")
	}
	total, err := countLegacyMintOrders(req)
	if err != nil {
		return nil, err
	}
	entityList, err := listLegacyMintOrders(req)
	if err != nil {
		return nil, err
	}
	tickInfoMap := make(map[string]*man_service.Mrc20TickInfo)
	list := make([]*respond.IdCoinsOpOrderInfoResp, 0, len(entityList))
	for _, entity := range entityList {
		if _, ok := tickInfoMap[entity.TickId]; !ok {
			tickInfo, _ := fetchIdCoinTickInfoCached(entity.TickId)
			tickInfoMap[entity.TickId] = tickInfo
		}
		tickInfo := tickInfoMap[entity.TickId]
		if tickInfo == nil {
			continue
		}
		list = append(list, buildLegacyMintOrderResponse(entity, tickInfo))
	}
	return &respond.FetchIdCoinsOpOrdersResp{Total: total, List: list}, nil
}

func fetchLegacyIdCoinsOpOrders(req *request.FetchIdCoinsOpOrdersRequest) (*respond.FetchIdCoinsOpOrdersResp, error) {
	switch req.OpOrderType {
	case "deploy":
		return fetchLegacyIdCoinsDeployOrders(req)
	case "mint":
		return fetchLegacyIdCoinsMintOrders(req)
	default:
		return nil, errors.New("op order type is invalid")
	}
}

func fetchLegacyAddressIdCoinsMintOrder(req *request.FetchIdCoinsMintOrderRequest) (*respond.FetchOneIdCoinsMintOrderResp, error) {
	if req.Address == "" {
		return nil, errors.New("address is empty")
	}
	if req.TickId == "" {
		return nil, errors.New("tickId is empty")
	}
	// The retired service returned its GetOne error (gorm "record not found") when the
	// address has no finished mint order; callers treat addressMintState=1 as "already minted".
	entity, err := getLegacyMintOrder(req.Address, req.TickId)
	if err != nil {
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
	mintState := 0
	mintPins := make([]string, 0)
	if entity.MintPins != "" {
		mintPins = strings.Split(entity.MintPins, ",")
	}
	if entity.ConfirmationState == models.ConfirmationStateConfirmed {
		if fetchIdCoinTxVerifiedCached(entity.RevealMintTxId, int64(len(mintPins)+1)) {
			mintState = 1
		} else {
			mintState = 2
		}
	}
	return &respond.FetchOneIdCoinsMintOrderResp{
		AddressMintState:  1,
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
		Qual:              tool.AnyToStr(tickInfo.PinCheck),
		PinCheck:          tool.AnyToStr(tickInfo.PinCheck),
		PayCheck:          tool.AnyToStr(tickInfo.PayCheck),
		TxId:              entity.RevealMintTxId,
		BlockHeight:       entity.BlockHeight,
		ConfirmationState: entity.ConfirmationState,
		Timestamp:         entity.CreateTime,
		DeployerAddress:   tickInfo.Address,
		DeployerMetaId:    common.GetMetaIdByAddress(tickInfo.Address),
		DeployerUserInfo:  fetchIdCoinUserInfoCached(tickInfo.Address),
		MetaData:          tickInfo.Metadata,
	}, nil
}
