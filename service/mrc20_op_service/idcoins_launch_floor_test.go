package mrc20_op_service

import (
	"encoding/hex"
	"metaid-market-service/common"
	"metaid-market-service/conf"
	"metaid-market-service/controller/auth"
	"metaid-market-service/service/orderbook_service"
	"strings"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/shopspring/decimal"
	"github.com/spf13/viper"
)

func testKeyPairHex(seed byte) (privHex, pubHex string) {
	keyBytes := make([]byte, 32)
	for i := range keyBytes {
		keyBytes[i] = seed
	}
	keyBytes[31] = seed + 3
	priv, _ := btcec.PrivKeyFromBytes(keyBytes)
	return hex.EncodeToString(priv.Serialize()), hex.EncodeToString(priv.PubKey().SerializeCompressed())
}

func TestTickSignRoundtripBothPubkeys(t *testing.T) {
	platformPriv, platformPub := testKeyPairHex(31)
	_, legacyPub := testKeyPairHex(47)

	// save + restore conf
	origPriv, origPub := conf.PlatformPrivateKeySignMsg, conf.PlatformPublicKeySignMsg
	origLegacy, origTs := conf.IdCoinsSignPublicKey, conf.IdCoinsSignTimestamp
	defer func() {
		conf.PlatformPrivateKeySignMsg, conf.PlatformPublicKeySignMsg = origPriv, origPub
		conf.IdCoinsSignPublicKey, conf.IdCoinsSignTimestamp = origLegacy, origTs
	}()
	conf.PlatformPrivateKeySignMsg = platformPriv
	conf.PlatformPublicKeySignMsg = platformPub
	conf.IdCoinsSignPublicKey = legacyPub
	conf.IdCoinsSignTimestamp = 0

	// sign UPPER(tick) with the platform key, exactly like the launch pre flow
	sign, err := auth.SignTextMessage(strings.ToUpper("mycoin"), platformPriv)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	metaData := `{"message":"hi","tickSign":"` + sign + `"}`
	if got := common.CheckIdCoins("mycoin", metaData, 1000); got != "id-coins" {
		t.Fatal("platform-key tickSign must be accepted")
	}
	if got := common.CheckIdCoins("MYCOIN", metaData, 1000); got != "id-coins" {
		t.Fatal("tick case must not matter")
	}

	// a signature from an unrelated key must be rejected
	otherPriv, _ := testKeyPairHex(99)
	badSign, _ := auth.SignTextMessage(strings.ToUpper("mycoin"), otherPriv)
	badMeta := `{"tickSign":"` + badSign + `"}`
	if got := common.CheckIdCoins("mycoin", badMeta, 1000); got != "" {
		t.Fatal("foreign-key tickSign must be rejected")
	}

	// legacy pubkey path still works: sign with the legacy key
	legacyPriv, legacyPub2 := testKeyPairHex(47)
	conf.IdCoinsSignPublicKey = legacyPub2
	legacySign, _ := auth.SignTextMessage(strings.ToUpper("mycoin"), legacyPriv)
	legacyMeta := `{"tickSign":"` + legacySign + `"}`
	if got := common.CheckIdCoins("mycoin", legacyMeta, 1000); got != "id-coins" {
		t.Fatal("legacy-key tickSign must still be accepted")
	}
}

func TestIdCoinLaunchGatingAndRules(t *testing.T) {
	viper.Set("idcoins.launch_enabled", false)
	if idCoinsLaunchEnabled() {
		t.Fatal("default/unset must be disabled")
	}
	viper.Set("idcoins.launch_enabled", true)
	if !idCoinsLaunchEnabled() {
		t.Fatal("explicit true must enable")
	}
	viper.Set("idcoins.launch_enabled", nil)

	// tick uniqueness: snapshot tick is taken without touching network/DB
	taken, err := idCoinTickTaken("metaid")
	if err != nil || !taken {
		t.Fatalf("snapshot tick must be taken: %v %v", taken, err)
	}
	taken, err = idCoinTickTaken("SUNNY")
	if err != nil || !taken {
		t.Fatalf("snapshot tick case-insensitive must be taken: %v %v", taken, err)
	}
	if _, err := idCoinTickTaken(""); err == nil {
		t.Fatal("empty tick must error")
	}

	// min liquidity floor
	viper.Set("idcoins.min_liquidity_per_mint", 100000)
	if err := checkIdCoinLaunchLiquidity(99999); err == nil {
		t.Fatal("below floor must be rejected")
	}
	if err := checkIdCoinLaunchLiquidity(100000); err != nil {
		t.Fatalf("at floor must pass: %v", err)
	}
	viper.Set("idcoins.min_liquidity_per_mint", 0)
	if err := checkIdCoinLaunchLiquidity(1); err != nil {
		t.Fatalf("floor off must pass: %v", err)
	}

	// xpub fail-closed
	viper.Set("idcoins.lp_vault_xpub", "")
	if _, _, err := deriveIdCoinLpVault("TEST"); err == nil {
		t.Fatal("missing xpub must fail closed")
	}
	viper.Set("idcoins.lp_vault_xpub", "xpub6DDLXRDx6k34rNYcujmKp1xb92YSU4uAkEs1rhT6C3ytFbjU1YMaSt4gLQoB9cK9TTMRNrVVEZogsbPVxNQwui5TjKk7BMZrBwdBJdYktdg")
	viper.Set("net.net", "mainnet")
	pubHex, addr, err := deriveIdCoinLpVault("sunny")
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	if addr != "bc1qg0lt0nc4uja6fyhp8f0n2lfupd3hxld3vfeayf" {
		t.Fatalf("vault mismatch: %s", addr)
	}
	if len(pubHex) != 66 {
		t.Fatalf("pubkey hex length: %d", len(pubHex))
	}
	viper.Set("idcoins.lp_vault_xpub", nil)
}

func TestMergedCoinSetSnapshotOnly(t *testing.T) {
	// without a database the merged set must be exactly the snapshot
	merged := mergedIdCoinSet()
	snapshot := common.IdCoinsSnapshot()
	if len(merged) != len(snapshot) {
		t.Fatalf("merged len = %d, want %d", len(merged), len(snapshot))
	}
	for i := range snapshot {
		if merged[i] != snapshot[i] {
			t.Fatalf("snapshot order changed at %d", i)
		}
	}
	if coin := findMergedIdCoinByTickId(snapshot[0].TickId); coin == nil || coin.TickId != snapshot[0].TickId {
		t.Fatal("merged tickId lookup must find snapshot coins")
	}
	if coin := findMergedIdCoinByTickId("deadbeefdeadbeef"); coin != nil {
		t.Fatal("unknown tickId must not resolve")
	}
}

func TestIdCoinFloorData(t *testing.T) {
	const (
		tickId    = "aa11bb22cc33dd44"
		ftCH      = "1111111111111111111111111111111111111111"
		ftGenesis = "2222222222222222222222222222222222222222222222222222222222222222"
	)
	// seed the bridge asset cache directly (in-package seam)
	idCoinBridgeCacheMu.Lock()
	idCoinBridgeCache = []idCoinBridgeAsset{{
		Network:             "BTC",
		OriginTokenId:       tickId,
		TargetTokenCodeHash: ftCH,
		TargetTokenGenesis:  ftGenesis,
		Decimals:            8,
	}}
	idCoinBridgeCacheAt = time.Now()
	idCoinBridgeCacheMu.Unlock()
	defer func() {
		idCoinBridgeCacheMu.Lock()
		idCoinBridgeCache = nil
		idCoinBridgeCacheMu.Unlock()
	}()

	if ft := resolveIdCoinBridgeFT(tickId); ft == nil || ft.codeHash != ftCH || ft.genesis != ftGenesis || ft.decimals != 8 {
		t.Fatalf("ft resolution failed: %+v", ft)
	}

	// build a book with the orders in the book's own canonical coordinates
	pairKey, _, _, _, _, canonicalSide := orderbook_service.CanonicalizeTokenPair(idCoinWbtcCodeHash, idCoinWbtcTokenID, ftCH, ftGenesis)
	otherSide := uint8(orderbook_service.SideAsk)
	if canonicalSide == orderbook_service.SideAsk {
		otherSide = orderbook_service.SideBid
	}
	mk := func(outpoint string, side uint8, maker20 string, sellCH, sellID, buyCH, buyID string, remain, priceNum, priceDen uint64) *orderbook_service.LiveOrder {
		return &orderbook_service.LiveOrder{
			Outpoint:          outpoint,
			Txid:              outpoint[:8],
			PairKey:           pairKey,
			Side:              side,
			IsTokenOrder:      true,
			MakerHash20:       maker20,
			SellTokenCodeHash: sellCH,
			SellTokenID:       sellID,
			BuyTokenCodeHash:  buyCH,
			BuyTokenID:        buyID,
			PriceNum:          priceNum,
			PriceDen:          priceDen,
			RemainAmount:      remain,
		}
	}
	nonZeroMaker := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	book := orderbook_service.NewOrderBook("mainnet", orderbook_service.NewMemorySoftLocker())
	book.ResetSnapshot([]*orderbook_service.LiveOrder{
		// two genuine burn-maker bids: 500k + 300k sats locked
		mk("aaaa0001", canonicalSide, idCoinBurnMaker20, idCoinWbtcCodeHash, idCoinWbtcTokenID, ftCH, ftGenesis, 500000, 1, 5000000), // 20 sats/coin
		mk("aaaa0002", canonicalSide, idCoinBurnMaker20, idCoinWbtcCodeHash, idCoinWbtcTokenID, ftCH, ftGenesis, 300000, 1, 4000000), // 25 sats/coin — best
		// decoys: non-zero maker, wrong side, wrong token pairing
		mk("bbbb0001", canonicalSide, nonZeroMaker, idCoinWbtcCodeHash, idCoinWbtcTokenID, ftCH, ftGenesis, 999999, 1, 1),
		mk("bbbb0002", otherSide, idCoinBurnMaker20, idCoinWbtcCodeHash, idCoinWbtcTokenID, ftCH, ftGenesis, 999999, 1, 1),
		mk("bbbb0003", canonicalSide, idCoinBurnMaker20, ftCH, ftGenesis, idCoinWbtcCodeHash, idCoinWbtcTokenID, 999999, 1, 1),
	})

	origBook := orderbookFloorBook
	orderbookFloorBook = func() *orderbook_service.OrderBook { return book }
	defer func() { orderbookFloorBook = origBook }()

	price, pool := idCoinFloorData(tickId)
	if pool != 800000 {
		t.Fatalf("pool = %d, want 800000 (burn bids only)", pool)
	}
	// best: 1/4000000 sats per raw unit x 10^8 = 25 sats per coin
	want := decimal.New(25, 0).StringFixed(8)
	if price != want {
		t.Fatalf("price = %s, want %s", price, want)
	}

	// no bridged FT -> zeros
	price, pool = idCoinFloorData("unknown-tick-id")
	if price != "0" || pool != 0 {
		t.Fatalf("unbridged coin must be 0/0: %s %d", price, pool)
	}

	// book without any burn bid -> zeros
	emptyBook := orderbook_service.NewOrderBook("mainnet", orderbook_service.NewMemorySoftLocker())
	emptyBook.ResetSnapshot([]*orderbook_service.LiveOrder{
		mk("cccc0001", canonicalSide, nonZeroMaker, idCoinWbtcCodeHash, idCoinWbtcTokenID, ftCH, ftGenesis, 999999, 1, 1),
	})
	orderbookFloorBook = func() *orderbook_service.OrderBook { return emptyBook }
	price, pool = idCoinFloorData(tickId)
	if price != "0" || pool != 0 {
		t.Fatalf("no burn bids must be 0/0: %s %d", price, pool)
	}
}
