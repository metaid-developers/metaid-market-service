package mrc20_op_service

import (
	"database/sql"
	"metaid-market-service/common"
	"metaid-market-service/controller/request"
	"metaid-market-service/models"
	"metaid-market-service/service/man_service"
	"strings"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestDeriveIdCoinValues(t *testing.T) {
	info := &man_service.Mrc20TickInfo{
		AmtPerMint:  "10000",
		MintCount:   500,
		TotalMinted: 500,
		PayCheck:    &man_service.PayCheck{PayTo: "bc1qexample", PayAmount: "100000"},
	}
	values := deriveIdCoinValues(info, 2)
	if values.price != "10.00" {
		t.Fatalf("price mismatch: %s", values.price)
	}
	if values.pool != 50000000 {
		t.Fatalf("pool mismatch: %d", values.pool)
	}
	if values.supply != "5000000" || values.totalSupply != "5000000" || values.remaining != "0" {
		t.Fatalf("supply values mismatch: %+v", values)
	}
	if values.mintable {
		t.Fatal("fully minted coin must not be mintable")
	}

	info.TotalMinted = 12
	values = deriveIdCoinValues(info, 2)
	if values.pool != 1200000 || values.supply != "120000" || values.remaining != "4880000" || !values.mintable {
		t.Fatalf("partially minted values mismatch: %+v", values)
	}

	info.TotalMinted = 600
	values = deriveIdCoinValues(info, 2)
	if values.pool != 50000000 || values.supply != "5000000" || values.remaining != "-1000000" {
		t.Fatalf("over-minted clamp mismatch: %+v", values)
	}
}

func TestIdCoinSnapshotResolution(t *testing.T) {
	if coin := findIdCoinSnapshotByTick("metaid"); coin == nil || coin.Tick != "METAID" {
		t.Fatalf("case-insensitive tick lookup failed: %+v", coin)
	}
	if coin := findIdCoinSnapshotByTick("MetaWeb"); coin == nil || coin.Tick != "MetaWeb" {
		t.Fatalf("original-case tick lookup failed: %+v", coin)
	}
	if coin := findIdCoinSnapshotByTickId("8ce590918a3d493631ab9d9e3bbc89e322f3dd08354af87f97c07218818b78f4i0"); coin == nil || coin.Tick != "METAID" {
		t.Fatalf("tickId lookup failed: %+v", coin)
	}
	if coin := findIdCoinSnapshotByTick("not-a-coin"); coin != nil {
		t.Fatalf("unexpected coin: %+v", coin)
	}
}

func TestIdCoinsSimpleInfoOrderingAndPagination(t *testing.T) {
	entries := []*idCoinListEntry{
		{coin: common.IdCoinSnapshotCoin{Tick: "new", TickId: "new-id"}, info: &man_service.Mrc20TickInfo{DeployTime: 30, MetaId: "m3", Address: "a3"}},
		{coin: common.IdCoinSnapshotCoin{Tick: "old", TickId: "old-id"}, info: &man_service.Mrc20TickInfo{DeployTime: 10, MetaId: "m1", Address: "a1"}},
		{coin: common.IdCoinSnapshotCoin{Tick: "mid", TickId: "mid-id"}, info: &man_service.Mrc20TickInfo{DeployTime: 20, MetaId: "m2", Address: "a2"}},
	}
	// input order is the snapshot order (newest first); output reverses it
	page := idCoinsSimpleInfoPage(entries, 1, 1)
	if page.Total != 3 || len(page.List) != 1 || page.List[0].Tick != "old" {
		t.Fatalf("unexpected page: %+v", page)
	}
	page = idCoinsSimpleInfoPage(entries, 0, 2)
	if len(page.List) != 2 || page.List[0].Tick != "mid" || page.List[1].Tick != "old" {
		t.Fatalf("unexpected ordering: %+v", page.List)
	}
	page = idCoinsSimpleInfoPage(entries, 3, 10)
	if page.Total != 3 || len(page.List) != 0 {
		t.Fatalf("unexpected out-of-range page: %+v", page)
	}
}

func TestLegacyMintOrderMapping(t *testing.T) {
	entity := &legacyIdCoinsMintOrder{
		OrderId:           "order-1",
		InscribeState:     legacyInscribeStateFinish,
		Address:           "minter",
		TickId:            "tick-id",
		PayToAmount:       10000,
		MintPins:          "pin1,pin2,pin1",
		RevealMintTxId:    "txid",
		BlockHeight:       123,
		ConfirmationState: models.ConfirmationStateConfirmed,
		CreateTime:        456,
	}
	tickInfo := &man_service.Mrc20TickInfo{
		Tick:         "TICK",
		TokenName:    "Token",
		Decimals:     "8",
		AmtPerMint:   "100",
		MintCount:    10,
		PremineCount: 0,
		TotalMinted:  2,
		BeginHeight:  "100",
		PinCheck:     map[string]interface{}{"count": "1"},
		PayCheck:     &man_service.PayCheck{PayTo: "pay", PayAmount: "10000"},
		Address:      "deployer",
		Metadata:     "metadata",
	}
	idCoinsPinPopCacheMu.Lock()
	idCoinsPinPopCache["pin1"] = 7
	idCoinsPinPopCache["pin2"] = 3
	idCoinsPinPopCacheMu.Unlock()
	idCoinsTxVerifyCacheMu.Lock()
	idCoinsTxVerifyCache["txid:4"] = true
	idCoinsTxVerifyCacheMu.Unlock()

	resp := buildLegacyMintOrderResponse(entity, tickInfo)
	if resp.MintState != 1 {
		t.Fatalf("mintState mismatch: %d", resp.MintState)
	}
	if len(resp.UsedPins) != 2 || resp.UsedPins[0] != "pin1-7" || resp.UsedPins[1] != "pin2-3" {
		t.Fatalf("usedPins mismatch: %+v", resp.UsedPins)
	}
	if resp.TxId != "txid" || resp.Timestamp != 456 || resp.DeployerAddress != "deployer" || resp.LiquidityPerMint != 10000 {
		t.Fatalf("response mismatch: %+v", resp)
	}

	entity.InscribeState = legacyInscribeStateRefund
	resp = buildLegacyMintOrderResponse(entity, tickInfo)
	if resp.MintState != 3 {
		t.Fatalf("refund mintState mismatch: %d", resp.MintState)
	}
}

func TestIdCoinsMaintenanceErrors(t *testing.T) {
	calls := []error{}
	_, err1 := BuildIdCoinsPre(&request.BuildIdCoinsPreRequest{}, "", "")
	_, err2 := BuildIdCoinsCommit(&request.BuildIdCoinsCommitRequest{}, "", "")
	_, err3 := IdCoinsMintPre(&request.IdCoinsMintPreRequest{}, "", "")
	_, err4 := IdCoinsMintCommit(&request.IdCoinsMintCommitRequest{}, "", "")
	_, err5 := RefundIdCoinsInvalidLpPre(&request.RefundIdCoinsValidPreRequest{}, "", "")
	_, err6 := RefundIdCoinsInvalidLpCommit(&request.RefundIdCoinsValidCommitRequest{}, "", "")
	_, err7 := BookOrderTakeMintBidPreview(&request.BookTakeMintBidPreviewReq{}, "", "")
	_, err8 := BookOrderTakeMintBidPre(&request.BookTakeMintBidPreReq{}, "", "")
	_, err9 := BookOrderTakeMintBidCommit(&request.BookTakeMintBidCommitReq{}, "", "")
	calls = append(calls, err1, err2, err3, err4, err5, err6, err7, err8, err9)
	for i, err := range calls {
		if err == nil || err.Error() != idCoinsMaintenanceError {
			t.Fatalf("call %d error mismatch: %v", i, err)
		}
	}
	check, err := FetchIdCoinsDeployCheckInfo(&request.FetchIdCoinsDeployCheckRequest{}, "", "")
	if err != nil || check.CanDeploy || check.Msg != idCoinsUpgradeMessage {
		t.Fatalf("deploy check mismatch: %+v, %v", check, err)
	}
}

func legacyDryRunDB(t *testing.T) *gorm.DB {
	t.Helper()
	// sql.Open is lazy and DryRun never executes, so no real server is needed.
	sqlDB, err := sql.Open("mysql", "root@tcp(127.0.0.1:1)/order_book_db")
	if err != nil {
		t.Fatalf("open driver: %v", err)
	}
	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("open dry run db: %v", err)
	}
	return db
}

func assertLegacySQL(t *testing.T, name, sql string, mustContain, mustNotContain []string) {
	t.Helper()
	for _, want := range mustContain {
		if !strings.Contains(sql, want) {
			t.Errorf("%s: generated SQL missing %q:\n%s", name, want, sql)
		}
	}
	for _, unwanted := range mustNotContain {
		if strings.Contains(sql, unwanted) {
			t.Errorf("%s: generated SQL must not contain %q:\n%s", name, unwanted, sql)
		}
	}
}

func assertLegacyVars(t *testing.T, name string, vars []interface{}, want []interface{}) {
	t.Helper()
	if len(vars) != len(want) {
		t.Errorf("%s: vars count mismatch: %v", name, vars)
		return
	}
	for i := range want {
		if vars[i] != want[i] {
			t.Errorf("%s: var %d mismatch: got %v want %v (all: %v)", name, i, vars[i], want[i], vars)
		}
	}
}

var legacySecretColumns = []string{
	"revealTxPrivateKey", "pinPrivateKey", "revealInscribeTxPrivateKey", "revealMintTxPrivateKey",
	"PsbtRaw", "PsbtHex", "commitTxRaw",
}

func TestLegacyHistorySQL(t *testing.T) {
	db := legacyDryRunDB(t)
	req := &request.FetchIdCoinsOpOrdersRequest{
		OpOrderType:  "deploy",
		Address:      "addr",
		TickId:       "tick-id",
		Cursor:       5,
		Size:         10,
		Confirmation: models.ConfirmationStateConfirmed,
	}

	var total int64
	tx := legacyDeployOrderCountQuery(db, req).Count(&total)
	assertLegacySQL(t, "deploy-count", tx.Statement.SQL.String(),
		[]string{"`order_book_db`.`tb_id_coins_deploy_order`", "issuerAddress = ?", "inscribeState = ?", "tickId = ?", "confirmationState = ?", "count(*)"},
		legacySecretColumns)

	list := make([]*legacyIdCoinsDeployOrder, 0)
	tx = legacyDeployOrderListQuery(db, req).Find(&list)
	assertLegacySQL(t, "deploy-list", tx.Statement.SQL.String(),
		[]string{"SELECT orderId,inscribeState,issuerMetaId,issuerAddress,tickId", "`order_book_db`.`tb_id_coins_deploy_order`", "issuerAddress = ?", "inscribeState = ?", "tickId = ?", "confirmationState = ?", "ORDER BY timestamp desc LIMIT ? OFFSET ?"},
		legacySecretColumns)
	assertLegacyVars(t, "deploy-list", tx.Statement.Vars, []interface{}{"addr", int64(3), "tick-id", models.ConfirmationStateConfirmed, 10, 5})

	reqNoOpt := &request.FetchIdCoinsOpOrdersRequest{Address: "addr", Cursor: 0, Size: 20}
	tx = legacyDeployOrderListQuery(db, reqNoOpt).Find(&list)
	if strings.Contains(tx.Statement.SQL.String(), "tickId = ?") || strings.Contains(tx.Statement.SQL.String(), "confirmationState = ?") {
		t.Errorf("deploy-list without optional filters must omit them:\n%s", tx.Statement.SQL.String())
	}

	req.OpOrderType = "mint"
	tx = legacyMintOrderCountQuery(db, req).Count(&total)
	assertLegacySQL(t, "mint-count", tx.Statement.SQL.String(),
		[]string{"`order_book_db`.`tb_id_coins_mint_order`", "address = ?", "inscribeState IN (?,?)", "tickId = ?", "count(*)"},
		legacySecretColumns)

	mintList := make([]*legacyIdCoinsMintOrder, 0)
	tx = legacyMintOrderListQuery(db, req).Find(&mintList)
	assertLegacySQL(t, "mint-list", tx.Statement.SQL.String(),
		[]string{"SELECT id,orderId,inscribeState,address,tickId", "`order_book_db`.`tb_id_coins_mint_order`", "address = ?", "inscribeState IN (?,?)", "tickId = ?", "ORDER BY timestamp desc LIMIT ? OFFSET ?"},
		legacySecretColumns)
	assertLegacyVars(t, "mint-list", tx.Statement.Vars, []interface{}{"addr", int64(3), int64(11), "tick-id", 10, 5})

	one := &legacyIdCoinsMintOrder{}
	tx = legacyMintOrderGetOneQuery(db, "addr", "tick-id").Find(one)
	assertLegacySQL(t, "mint-getOne", tx.Statement.SQL.String(),
		[]string{"SELECT id,orderId,inscribeState,address,tickId", "`order_book_db`.`tb_id_coins_mint_order`", "address = ? AND tickId = ? AND inscribeState = ?", "ORDER BY id asc"},
		legacySecretColumns)

	refund := &legacyBookMintUtxoRefund{}
	tx = legacyRefundTxIdQuery(db, "order-1").Find(refund)
	assertLegacySQL(t, "refund-lookup", tx.Statement.SQL.String(),
		[]string{"SELECT fromOrderId,refundTxId", "`order_book_db`.`tb_book_mint_utxo`", "fromOrderId = ?"},
		legacySecretColumns)
}
