package mrc20_op_service

import (
	"encoding/hex"
	"metaid-market-service/common"
	"metaid-market-service/controller/respond"
	"metaid-market-service/models"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
	"github.com/spf13/viper"
)

func testTaprootAddress(t *testing.T, seed byte) string {
	t.Helper()
	keyBytes := make([]byte, 32)
	for i := range keyBytes {
		keyBytes[i] = seed
	}
	keyBytes[31] = seed + 7
	priv, _ := btcec.PrivKeyFromBytes(keyBytes)
	addr, err := btcutil.NewAddressTaproot(schnorr.SerializePubKey(priv.PubKey()), &chaincfg.MainNetParams)
	if err != nil {
		t.Fatalf("taproot addr: %v", err)
	}
	return addr.EncodeAddress()
}

func TestIdCoinMintEnabledGating(t *testing.T) {
	// snapshot tick id for METAID (from common/idcoins.go snapshot)
	const metaidTickId = "8ce590918a3d493631ab9d9e3bbc89e322f3dd08354af87f97c07218818b78f4i0"

	viper.Set("idcoins.mint_enabled_ticks", []string{})
	if idCoinMintEnabledmetaidHelper(metaidTickId) {
		t.Fatal("empty allowlist must disable everything")
	}

	viper.Set("idcoins.mint_enabled_ticks", []string{"metaid"}) // case-insensitive
	if !idCoinMintEnabledmetaidHelper(metaidTickId) {
		t.Fatal("allowlisted snapshot coin must be enabled")
	}

	viper.Set("idcoins.mint_enabled_ticks", []string{"SUNNY"})
	if idCoinMintEnabledmetaidHelper(metaidTickId) {
		t.Fatal("coin not in the allowlist must stay disabled")
	}
	if idCoinMintEnabledmetaidHelper("deadbeefnotasnapshotcoini0") {
		t.Fatal("non-snapshot coin must never be enabled")
	}
	viper.Set("idcoins.mint_enabled_ticks", nil)
}

// helper indirection keeps the test independent of the exact tick casing.
func idCoinMintEnabledmetaidHelper(tickId string) bool {
	return idCoinMintEnabled(tickId)
}

func TestIdCoinMintSweepFee(t *testing.T) {
	if got := idCoinMintSweepFee(10); got != 1100 {
		t.Fatalf("sweepFee(10) = %d, want 1100", got)
	}
	if got := idCoinMintSweepFee(1); got != 110 {
		t.Fatalf("sweepFee(1) = %d, want 110", got)
	}
	if got := idCoinMintSweepFee(0); got != 0 {
		t.Fatalf("sweepFee(0) = %d, want 0", got)
	}
}

func buildCommitTxForValidation(t *testing.T, inscribeAddr, mintAddr string, inscribeAmt, mintAmt int64) *wire.MsgTx {
	t.Helper()
	commitTx := wire.NewMsgTx(2)
	zeroHash := wire.OutPoint{}.Hash
	commitTx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&zeroHash, 0), nil, nil))
	for i, out := range []struct {
		addr string
		amt  int64
	}{{inscribeAddr, inscribeAmt}, {mintAddr, mintAmt}} {
		pkScript, err := common.AddressToPkScript("mainnet", "btc", out.addr)
		if err != nil {
			t.Fatalf("output %d script: %v", i, err)
		}
		pkScriptBytes, _ := hex.DecodeString(pkScript)
		commitTx.AddTxOut(wire.NewTxOut(out.amt, pkScriptBytes))
	}
	return commitTx
}

func TestValidateIdCoinsMintCommitTx(t *testing.T) {
	// two taproot-ish addresses; content irrelevant, distinctness matters
	inscribeAddr := testTaprootAddress(t, 11)
	mintAddr := testTaprootAddress(t, 22)
	order := &models.IdCoinsMintOrderModel{
		RevealInscribeTxAddress: inscribeAddr,
		RevealMintTxAddress:     mintAddr,
		InscribeMinerFee:        2000,
		MintMinerFee:            5000,
	}

	valid := buildCommitTxForValidation(t, inscribeAddr, mintAddr, 2000, 5000)
	if err := validateIdCoinsMintCommitTx("mainnet", "btc", valid, order, 0, 1); err != nil {
		t.Fatalf("honest commit must validate: %v", err)
	}

	// tampered amount
	bad := buildCommitTxForValidation(t, inscribeAddr, mintAddr, 1999, 5000)
	if err := validateIdCoinsMintCommitTx("mainnet", "btc", bad, order, 0, 1); err == nil {
		t.Fatal("underpaid inscribe output must be rejected")
	}
	bad = buildCommitTxForValidation(t, inscribeAddr, mintAddr, 2000, 4999)
	if err := validateIdCoinsMintCommitTx("mainnet", "btc", bad, order, 0, 1); err == nil {
		t.Fatal("underpaid mint output must be rejected")
	}

	// tampered address (outputs swapped)
	bad = buildCommitTxForValidation(t, mintAddr, inscribeAddr, 2000, 5000)
	if err := validateIdCoinsMintCommitTx("mainnet", "btc", bad, order, 0, 1); err == nil {
		t.Fatal("wrong recipient address must be rejected")
	}

	// tampered index: correct outputs but the request points at wrong indexes
	good := buildCommitTxForValidation(t, inscribeAddr, mintAddr, 2000, 5000)
	if err := validateIdCoinsMintCommitTx("mainnet", "btc", good, order, 1, 0); err == nil {
		t.Fatal("swapped output indexes must be rejected")
	}
	if err := validateIdCoinsMintCommitTx("mainnet", "btc", good, order, 0, 5); err == nil {
		t.Fatal("out-of-range index must be rejected")
	}
	if err := validateIdCoinsMintCommitTx("mainnet", "btc", good, order, 0, 0); err == nil {
		t.Fatal("identical indexes must be rejected")
	}
}

func TestMergeAndWindowMintOrdersPagination(t *testing.T) {
	mk := func(ts int64, orderId string) *respond.IdCoinsOpOrderInfoResp {
		return &respond.IdCoinsOpOrderInfoResp{Timestamp: ts, OrderId: orderId}
	}
	// interleaved timestamps across the two sources
	merged := []*respond.IdCoinsOpOrderInfoResp{
		mk(100, "legacy-1"),
		mk(400, "legacy-2"),
		mk(200, "self-1"),
		mk(500, "self-2"),
		mk(300, "legacy-3"),
	}
	page, total := mergeAndWindowMintOrders(merged, 0, 2)
	if total != 5 {
		t.Fatalf("total = %d, want 5", total)
	}
	if len(page) != 2 || page[0].OrderId != "self-2" || page[1].OrderId != "legacy-2" {
		t.Fatalf("page 1 mismatch: %+v", page)
	}
	page, total = mergeAndWindowMintOrders(merged, 2, 2)
	if len(page) != 2 || page[0].OrderId != "legacy-3" || page[1].OrderId != "self-1" {
		t.Fatalf("page 2 mismatch: %+v", page)
	}
	page, total = mergeAndWindowMintOrders(merged, 4, 2)
	if len(page) != 1 || page[0].OrderId != "legacy-1" {
		t.Fatalf("last page mismatch: %+v", page)
	}
	page, _ = mergeAndWindowMintOrders(merged, 5, 2)
	if len(page) != 0 {
		t.Fatalf("beyond-end page must be empty: %+v", page)
	}
	// size 0 = everything from cursor
	page, _ = mergeAndWindowMintOrders(merged, 1, 0)
	if len(page) != 4 || page[0].OrderId != "legacy-2" {
		t.Fatalf("size-0 window mismatch: %+v", page)
	}
}
