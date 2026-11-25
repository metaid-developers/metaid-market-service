package risk_service

import (
	"bytes"
	"encoding/hex"
	"metaid-market-service/common"
	"metaid-market-service/conf"
	"metaid-market-service/models"
	"testing"

	"github.com/btcsuite/btcd/btcutil/psbt"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

func TestRiskEngine_SeedDirectAndRescueWhitelist(t *testing.T) {
	txMap := map[string]*ChainTx{
		// Downstream transfer from Victim #1's rescued UTXO ac30da2b..._1
		"aaaa000000000000000000000000000000000000000000000000000000000001": {
			TxId:        "aaaa000000000000000000000000000000000000000000000000000000000001",
			Confirmed:   true,
			BlockHeight: 977060,
			Inputs: []ChainTxInput{
				{
					TxId:    "ac30da2b8dc59d2a19d9bd83b64761d74a17e76a4a9573f38e99291731b76cb8",
					Vout:    1,
					Address: "13TzdVkgh8WxpKwc6P5S2oRfW3U5LqE1xS",
					Value:   546,
				},
			},
			Outputs: []ChainTxOutput{
				{Address: "13TzdVkgh8WxpKwc6P5S2oRfW3U5LqE1xS", Value: 546},
			},
		},
	}

	eng := NewRiskEngine(
		func(chain, txId string) (*ChainTx, error) { return txMap[txId], nil },
		func(txId string) (*CompletedMarketTrade, error) { return nil, nil },
		nil,
	)
	eng.LoadSeedEntriesInMemory()

	// 1. Seeded thief addresses & stolen UTXOs must be blocked
	for _, thiefAddr := range []string{
		"bc1pe2vr873pqlufgeh2vlr4yf04z92ndvkjhnqmt8p7jp9wuevpuzjs8z2evg",
		"BC1PE2VR873PQLUFGEH2VLR4YF04Z92NDVKJHNQMT8P7JP9WUEVPUZJS8Z2EVG", // case-insensitive bech32
		"1JeiAHnCcqtYiZr1EPYogeKh67fkJCFmUA",
		"1PCzk7TqRTnrDbVjZkRqsRfAT6EPVj5N74",
		"1Gx23Z2EByLmHVSdUXhgN2fDNxcHckK6vG",
		"1MTEVpcigyzKU9AhKQvfqNbSUHiZggALZ7",
	} {
		if blocked, _ := eng.CheckUtxoAndAddress("btc", thiefAddr, "", 546, nil); !blocked {
			t.Fatalf("expected thief address %s to be blocked", thiefAddr)
		}
	}

	// Stolen MC/METASO/SHABI/METALD UTXOs must be blocked even if moved to a fresh address
	if blocked, _ := eng.CheckUtxoAndAddress("btc", "1FreshMuleAddrX", "b6994a0b4c520ed9ecea489cd462d92387108ab042bf2aeb6424c0943e033718:1", 546, nil); !blocked {
		t.Fatalf("expected stolen MC utxo to be blocked")
	}

	// 2. Victim #1 clean self-rescue wallets & rescued UTXOs must NOT be blocked
	if blocked, reason := eng.CheckUtxoAndAddress("btc", "13TzdVkgh8WxpKwc6P5S2oRfW3U5LqE1xS", "ac30da2b8dc59d2a19d9bd83b64761d74a17e76a4a9573f38e99291731b76cb8_1", 546, nil); blocked {
		t.Fatalf("expected rescued wallet 13Tzd... to be clean, got blocked: %s", reason)
	}
	if blocked, reason := eng.CheckUtxoAndAddress("btc", "1AkEdzTXgDUckLb19yoGspBTMwxjvxHKzY", "815aae66564f3349f2a80e740b596a39988f793259fd3c48554079bfcc7cf6ca_1", 546, nil); blocked {
		t.Fatalf("expected rescued wallet 1AkEdz... to be clean, got blocked: %s", reason)
	}
	if blocked, reason := eng.CheckUtxoAndAddress("btc", "13TzdVkgh8WxpKwc6P5S2oRfW3U5LqE1xS", "aaaa000000000000000000000000000000000000000000000000000000000001_0", 546, nil); blocked {
		t.Fatalf("expected downstream transfer of rescued UTXO to be clean, got blocked: %s", reason)
	}
}

func TestRiskEngine_MultiHopSybilRotationAndGasTaint(t *testing.T) {
	txMap := map[string]*ChainTx{
		// Hop 1: 1JeiAHn... -> 1SybilA
		"tx_hop_1": {
			TxId: "tx_hop_1", Confirmed: true, BlockHeight: 977040,
			Inputs:  []ChainTxInput{{TxId: "533173fbcbdc1fc9800a301a992faf54d27a3950928be2c02e4bf719add6548f", Vout: 1, Address: "1JeiAHnCcqtYiZr1EPYogeKh67fkJCFmUA", Value: 546}},
			Outputs: []ChainTxOutput{{Address: "1SybilA", Value: 546}},
		},
		// Hop 2: 1SybilA -> 1SybilB
		"tx_hop_2": {
			TxId: "tx_hop_2", Confirmed: true, BlockHeight: 977045,
			Inputs:  []ChainTxInput{{TxId: "tx_hop_1", Vout: 0, Address: "1SybilA", Value: 546}},
			Outputs: []ChainTxOutput{{Address: "1SybilB", Value: 546}},
		},
		// Hop 3: 1SybilB -> Taproot commit+reveal -> 1SybilC (unconfirmed in mempool)
		"tx_commit_3": {
			TxId: "tx_commit_3", Confirmed: false, BlockHeight: 0,
			Inputs:  []ChainTxInput{{TxId: "tx_gas_clean", Vout: 0, Address: "1SybilB", Value: 5000}},
			Outputs: []ChainTxOutput{{Address: "bc1p_ephemeral_reveal", Value: 1500}},
		},
		"tx_reveal_3": {
			TxId: "tx_reveal_3", Confirmed: false, BlockHeight: 0,
			Inputs: []ChainTxInput{
				{TxId: "tx_hop_2", Vout: 0, Address: "1SybilB", Value: 546},
				{TxId: "tx_commit_3", Vout: 0, Address: "bc1p_ephemeral_reveal", Value: 1500},
			},
			Outputs: []ChainTxOutput{{Address: "1SybilC", Value: 546}},
		},
		// Unknown Future Victim #4 drained using gas funded by bc1pe2vr873p...
		"tx_victim4_old_token": {
			TxId: "tx_victim4_old_token", Confirmed: true, BlockHeight: 910000,
			Inputs:  []ChainTxInput{{TxId: "tx_genesis", Vout: 0, Address: "1OldClean", Value: 546}},
			Outputs: []ChainTxOutput{{Address: "1FutureVictim4", Value: 546}},
		},
		"tx_hub_gas_to_v4": {
			TxId: "tx_hub_gas_to_v4", Confirmed: true, BlockHeight: 977070,
			Inputs:  []ChainTxInput{{TxId: "tx_hub_utxo", Vout: 0, Address: "bc1pe2vr873pqlufgeh2vlr4yf04z92ndvkjhnqmt8p7jp9wuevpuzjs8z2evg", Value: 200000}},
			Outputs: []ChainTxOutput{{Address: "1FutureMule4", Value: 10000}},
		},
		"tx_v4_commit": {
			TxId: "tx_v4_commit", Confirmed: false,
			Inputs:  []ChainTxInput{{TxId: "tx_hub_gas_to_v4", Vout: 0, Address: "1FutureMule4", Value: 10000}},
			Outputs: []ChainTxOutput{{Address: "bc1p_v4_reveal", Value: 1600}},
		},
		"tx_v4_reveal": {
			TxId: "tx_v4_reveal", Confirmed: false,
			Inputs: []ChainTxInput{
				{TxId: "tx_victim4_old_token", Vout: 0, Address: "1FutureVictim4", Value: 546},
				{TxId: "tx_v4_commit", Vout: 0, Address: "bc1p_v4_reveal", Value: 1600},
			},
			Outputs: []ChainTxOutput{{Address: "1FutureMule4", Value: 546}},
		},
	}

	var persisted []*models.MarketBlacklistModel
	eng := NewRiskEngine(
		func(chain, txId string) (*ChainTx, error) { return txMap[txId], nil },
		func(txId string) (*CompletedMarketTrade, error) { return nil, nil },
		func(entry *models.MarketBlacklistModel) error {
			persisted = append(persisted, entry)
			return nil
		},
	)
	eng.LoadSeedEntriesInMemory()

	// 3-hop Sybil rotation must be caught and outpoint-blacklisted (without auto-blacklisting recipient address)
	blocked, reason := eng.CheckUtxoAndAddress("btc", "1SybilC", "tx_reveal_3_0", 546, nil)
	if !blocked {
		t.Fatalf("expected 3-hop SybilC listing to be blocked")
	}
	t.Logf("SybilC blocked reason: %s (persisted %d auto-taint records)", reason, len(persisted))
	if !eng.IsUtxoTaintedFast("btc", "tx_reveal_3_0", 0) || !eng.IsOrderBlockedFast("1SybilC", "tx_reveal_3_0") {
		t.Fatalf("expected tainted UTXO tx_reveal_3_0 to be outpoint-blacklisted after taint detection")
	}
	if addrBlocked, _ := eng.IsAddressBlacklisted("1SybilC"); addrBlocked {
		t.Fatalf("expected recipient address 1SybilC NOT to be auto-blacklisted to prevent dusting/cascade collateral damage")
	}

	// Future Victim #4 drained using hub gas must be caught automatically via commit gas ancestry
	blockedV4, reasonV4 := eng.CheckUtxoAndAddress("btc", "1FutureMule4", "tx_v4_reveal_0", 546, nil)
	if !blockedV4 {
		t.Fatalf("expected FutureMule4 funded by attacker hub gas to be blocked")
	}
	t.Logf("FutureMule4 blocked reason: %s", reasonV4)
}

func TestRiskEngine_InnocentMarketBuyerExempt_ThiefProceedsStillTainted(t *testing.T) {
	conf.PlatformAddressDummyAsk = "bc1q0hzgkuls62egjpqxxhzs3cky7auyz596252p78"
	conf.PlatformServiceFeeConfigData = &conf.ServiceFeeConfig{
		ServiceAddress: "bc1pf4gj2uyjfytzzx3h06m977tvw6pl6hgy6kmgyle2757pcrrukr8s2srvy5",
	}

	txMap := map[string]*ChainTx{
		// Completed market trade where 1CleanBuyer bought 500 METAID from thief 1GtPDnwu...
		"tx_market_trade_1": {
			TxId: "tx_market_trade_1", Confirmed: true, BlockHeight: 977045,
			Inputs: []ChainTxInput{
				{TxId: "tx_dummy_prev", Vout: 2, Address: "bc1q0hzgkuls62egjpqxxhzs3cky7auyz596252p78", Value: 600},
				{TxId: "dd51318238798de987a77b433c3234769267079388496c598c4d94f3977c2604", Vout: 1, Address: "1GtPDnwuWkULR36euBCYJTTGQuicVhsre4", Value: 546},
				{TxId: "tx_buyer_btc", Vout: 0, Address: "1CleanBuyer", Value: 1000000},
			},
			Outputs: []ChainTxOutput{
				{Address: "1CleanBuyer", Value: 546},                                                     // vout 0: buyer MRC20
				{Address: "1GtPDnwuWkULR36euBCYJTTGQuicVhsre4", Value: 450000},                           // vout 1: thief BTC proceeds
				{Address: "bc1q0hzgkuls62egjpqxxhzs3cky7auyz596252p78", Value: 600},                      // vout 2: platform dummy
				{Address: "bc1pf4gj2uyjfytzzx3h06m977tvw6pl6hgy6kmgyle2757pcrrukr8s2srvy5", Value: 2250}, // vout 3: platform fee
				{Address: "1CleanBuyer", Value: 546000},                                                  // vout 4: buyer BTC change
			},
		},
		// Innocent buyer later transfers purchased MRC20 + BTC change to their second clean wallet
		"tx_buyer_resell_prep": {
			TxId: "tx_buyer_resell_prep", Confirmed: true, BlockHeight: 977055,
			Inputs: []ChainTxInput{
				{TxId: "tx_market_trade_1", Vout: 0, Address: "1CleanBuyer", Value: 546},
				{TxId: "tx_market_trade_1", Vout: 4, Address: "1CleanBuyer", Value: 546000},
			},
			Outputs: []ChainTxOutput{
				{Address: "1CleanBuyerVault2", Value: 546},
				{Address: "1CleanBuyerVault2", Value: 544000},
			},
		},
		// Thief uses BTC proceeds (tx_market_trade_1:1) to fund a brand-new split to 1ProceedsMule
		"tx_thief_reinvest_commit": {
			TxId: "tx_thief_reinvest_commit", Confirmed: true, BlockHeight: 977056,
			Inputs:  []ChainTxInput{{TxId: "tx_market_trade_1", Vout: 1, Address: "1GtPDnwuWkULR36euBCYJTTGQuicVhsre4", Value: 450000}},
			Outputs: []ChainTxOutput{{Address: "bc1p_thief_reveal", Value: 1500}},
		},
		"tx_thief_reinvest_reveal": {
			TxId: "tx_thief_reinvest_reveal", Confirmed: true, BlockHeight: 977056,
			Inputs: []ChainTxInput{
				{TxId: "tx_clean_looking_token", Vout: 0, Address: "1UnknownVictim", Value: 546},
				{TxId: "tx_thief_reinvest_commit", Vout: 0, Address: "bc1p_thief_reveal", Value: 1500},
			},
			Outputs: []ChainTxOutput{{Address: "1ProceedsMule", Value: 546}},
		},
		// Wash trade where thief buys own order with another thief address 17vGB948xbkSvsEerJPXQaSsQAA3xJqgqC
		"tx_wash_trade": {
			TxId: "tx_wash_trade", Confirmed: true, BlockHeight: 977058,
			Inputs: []ChainTxInput{
				{TxId: "dd51318238798de987a77b433c3234769267079388496c598c4d94f3977c2604", Vout: 0, Address: "1GtPDnwuWkULR36euBCYJTTGQuicVhsre4", Value: 546},
			},
			Outputs: []ChainTxOutput{
				{Address: "17vGB948xbkSvsEerJPXQaSsQAA3xJqgqC", Value: 546},
			},
		},
	}

	trades := map[string]*CompletedMarketTrade{
		"tx_market_trade_1": {
			TxId:          "tx_market_trade_1",
			SellerAddress: "1GtPDnwuWkULR36euBCYJTTGQuicVhsre4",
			BuyerAddress:  "1CleanBuyer",
		},
		"tx_wash_trade": {
			TxId:          "tx_wash_trade",
			SellerAddress: "1GtPDnwuWkULR36euBCYJTTGQuicVhsre4",
			BuyerAddress:  "17vGB948xbkSvsEerJPXQaSsQAA3xJqgqC",
		},
	}

	eng := NewRiskEngine(
		func(chain, txId string) (*ChainTx, error) { return txMap[txId], nil },
		func(txId string) (*CompletedMarketTrade, error) { return trades[txId], nil },
		nil,
	)
	eng.LoadSeedEntriesInMemory()

	// 1. Innocent buyer listing tx_market_trade_1:0 directly -> CLEAN
	if blocked, reason := eng.CheckUtxoAndAddress("btc", "1CleanBuyer", "tx_market_trade_1_0", 546, nil); blocked {
		t.Fatalf("expected innocent buyer output tx_market_trade_1_0 to be exempt, got blocked: %s", reason)
	}

	// 2. Innocent buyer transferring purchased UTXO + BTC change to second wallet and listing -> CLEAN
	if blocked, reason := eng.CheckUtxoAndAddress("btc", "1CleanBuyerVault2", "tx_buyer_resell_prep_0", 546, nil); blocked {
		t.Fatalf("expected innocent buyer downstream transfer tx_buyer_resell_prep_0 to be clean, got blocked: %s", reason)
	}

	// 3. Thief spending seller proceeds output tx_market_trade_1:1 -> STILL TAINTED & BLOCKED
	if blocked, _ := eng.CheckUtxoAndAddress("btc", "1ProceedsMule", "tx_thief_reinvest_reveal_0", 546, nil); !blocked {
		t.Fatalf("expected thief proceeds reinvestment tx_thief_reinvest_reveal_0 to be blocked")
	}

	// 4. Thief wash-trading with own mule -> NOT EXEMPT, STILL BLOCKED
	if blocked, _ := eng.CheckUtxoAndAddress("btc", "17vGB948xbkSvsEerJPXQaSsQAA3xJqgqC", "tx_wash_trade_0", 546, nil); !blocked {
		t.Fatalf("expected thief self-wash-trade to be blocked")
	}
}

func buildTestPsbtBuilder(t *testing.T, outpoints []wire.OutPoint) *common.PsbtBuilder {
	t.Helper()
	unsignedTx := wire.NewMsgTx(2)
	for _, op := range outpoints {
		unsignedTx.AddTxIn(wire.NewTxIn(&op, nil, nil))
	}
	unsignedTx.AddTxOut(wire.NewTxOut(546, []byte{0x51, 0x01, 0x01}))
	pkt, err := psbt.NewFromUnsignedTx(unsignedTx)
	if err != nil {
		t.Fatalf("NewFromUnsignedTx err: %v", err)
	}
	var buf bytes.Buffer
	if err := pkt.Serialize(&buf); err != nil {
		t.Fatalf("pkt.Serialize err: %v", err)
	}
	pb, err := common.NewPsbtBuilder(&chaincfg.MainNetParams, hex.EncodeToString(buf.Bytes()))
	if err != nil {
		t.Fatalf("NewPsbtBuilder err: %v", err)
	}
	return pb
}

func mustHash(t *testing.T, hexTxId string) chainhash.Hash {
	t.Helper()
	h, err := chainhash.NewHashFromStr(hexTxId)
	if err != nil {
		t.Fatalf("invalid hash %s: %v", hexTxId, err)
	}
	return *h
}

func serializeMsgTxHex(t *testing.T, tx *wire.MsgTx) string {
	t.Helper()
	var buf bytes.Buffer
	if err := tx.Serialize(&buf); err != nil {
		t.Fatalf("tx.Serialize err: %v", err)
	}
	return hex.EncodeToString(buf.Bytes())
}

func TestRiskEngine_DirtyBtcProceedsBuyerTakeBroadcastAndAntiDusting(t *testing.T) {
	conf.Net = "mainnet"
	conf.PlatformAddressDummyAsk = "bc1q0hzgkuls62egjpqxxhzs3cky7auyz596252p78"
	conf.PlatformServiceFeeConfigData = &conf.ServiceFeeConfig{
		ServiceAddress: "bc1pf4gj2uyjfytzzx3h06m977tvw6pl6hgy6kmgyle2757pcrrukr8s2srvy5",
	}

	const (
		cleanDummyTxId       = "1111111111111111111111111111111111111111111111111111111111111111"
		cleanSellerAssetTxId = "2222222222222222222222222222222222222222222222222222222222222222"
		cleanUserBtcTxId     = "3333333333333333333333333333333333333333333333333333333333333333"
		completedTradeTxId   = "7a25a6ad613f5f4152ca40e4d9d925a1e1276de25a6c27a6bfdbe5f5f46d2549"
		hub2SplitHop1TxId    = "4444444444444444444444444444444444444444444444444444444444444444"
		hub2SplitHop2TxId    = "5555555555555555555555555555555555555555555555555555555555555555"
		proceedsHopTxId      = "6666666666666666666666666666666666666666666666666666666666666666"
		dustAttackTxId       = "7777777777777777777777777777777777777777777777777777777777777777"
	)

	txMap := map[string]*ChainTx{
		cleanUserBtcTxId: {
			TxId: cleanUserBtcTxId, Confirmed: true, BlockHeight: 900000,
			Inputs:  []ChainTxInput{{TxId: cleanDummyTxId, Vout: 0, Address: "1OldCleanGenesis", Value: 2000000}},
			Outputs: []ChainTxOutput{{Address: "1InnocentUser", Value: 1500000}},
		},
		completedTradeTxId: {
			TxId: completedTradeTxId, Confirmed: true, BlockHeight: 977045,
			Inputs: []ChainTxInput{
				{TxId: cleanDummyTxId, Vout: 0, Address: conf.PlatformAddressDummyAsk, Value: 600},
				{TxId: "dd51318238798de987a77b433c3234769267079388496c598c4d94f3977c2604", Vout: 1, Address: "1GtPDnwuWkULR36euBCYJTTGQuicVhsre4", Value: 546},
				{TxId: cleanUserBtcTxId, Vout: 0, Address: "1KfU2XZw2X5bzQ1VkSURP4wmUmSgy9Demv", Value: 1500000},
			},
			Outputs: []ChainTxOutput{
				{Address: "1KfU2XZw2X5bzQ1VkSURP4wmUmSgy9Demv", Value: 546},              // vout 0: innocent buyer MRC20
				{Address: "1GtPDnwuWkULR36euBCYJTTGQuicVhsre4", Value: 1000000},          // vout 1: thief BTC sale proceeds
				{Address: conf.PlatformAddressDummyAsk, Value: 600},                      // vout 2: dummy
				{Address: conf.PlatformServiceFeeConfigData.ServiceAddress, Value: 5000}, // vout 3: fee
				{Address: "1KfU2XZw2X5bzQ1VkSURP4wmUmSgy9Demv", Value: 493000},           // vout 4: innocent buyer BTC change
			},
		},
		// Thief splits Hub #2 0.0724 BTC across 2 fresh Sybil wallets to buy clean assets on Market
		hub2SplitHop1TxId: {
			TxId: hub2SplitHop1TxId, Confirmed: true, BlockHeight: 977068,
			Inputs:  []ChainTxInput{{TxId: "1ef5460150b9e2522b9fa1143714f684735ad9a1e5d63b3a7c8e61a697fae346", Vout: 1, Address: "bc1pky6zpsx8gpv257kr3w43rvl2dre0j7phndxlf5muetzcyane8cyq2cys66", Value: 6338854}},
			Outputs: []ChainTxOutput{{Address: "1FreshBtcHop1", Value: 6330000}},
		},
		hub2SplitHop2TxId: {
			TxId: hub2SplitHop2TxId, Confirmed: false, BlockHeight: 0,
			Inputs:  []ChainTxInput{{TxId: hub2SplitHop1TxId, Vout: 0, Address: "1FreshBtcHop1", Value: 6330000}},
			Outputs: []ChainTxOutput{{Address: "1FreshSybilBuyer", Value: 6320000}},
		},
		// Thief moves Market sale BTC proceeds (vout 1) to a fresh wallet
		proceedsHopTxId: {
			TxId: proceedsHopTxId, Confirmed: false, BlockHeight: 0,
			Inputs:  []ChainTxInput{{TxId: completedTradeTxId, Vout: 1, Address: "1GtPDnwuWkULR36euBCYJTTGQuicVhsre4", Value: 1000000}},
			Outputs: []ChainTxOutput{{Address: "1FreshProceedsWallet", Value: 998000}},
		},
		// Thief sends unsolicited 546-sat dirty dust to an innocent community user
		dustAttackTxId: {
			TxId: dustAttackTxId, Confirmed: true, BlockHeight: 977070,
			Inputs:  []ChainTxInput{{TxId: "1ef5460150b9e2522b9fa1143714f684735ad9a1e5d63b3a7c8e61a697fae346", Vout: 1, Address: "bc1pky6zpsx8gpv257kr3w43rvl2dre0j7phndxlf5muetzcyane8cyq2cys66", Value: 6338854}},
			Outputs: []ChainTxOutput{{Address: "1InnocentUser", Value: 546}},
		},
	}

	trades := map[string]*CompletedMarketTrade{
		completedTradeTxId: {
			TxId:          completedTradeTxId,
			SellerAddress: "1GtPDnwuWkULR36euBCYJTTGQuicVhsre4",
			BuyerAddress:  "1KfU2XZw2X5bzQ1VkSURP4wmUmSgy9Demv",
		},
	}

	eng := NewRiskEngine(
		func(chain, txId string) (*ChainTx, error) { return txMap[txId], nil },
		func(txId string) (*CompletedMarketTrade, error) { return trades[txId], nil },
		nil,
	)
	eng.LoadSeedEntriesInMemory()

	// 0. Newly rotated blk 977060 mules must be directly seeded
	for _, rotated := range []string{
		"1K1xQjTAw8BmBt8n9KS6FLKoe2i31V8unb",
		"1CjSkrT5zEowwGtckmyqsqLE68GJZ8Ziu1",
		"18CaqsRn3DvPMPssfF7g1ERL1S7Rp2DNsG",
		"1DrcaSDzmmuqkNTFj2UuYcfjmJ1BXKSTAo",
		"1AVyhwDZGe3FVUKkxzXG4Nm3TWmc541Wza",
	} {
		if blocked, _ := eng.IsAddressBlacklisted(rotated); !blocked {
			t.Fatalf("expected rotated mule %s to be seeded in blacklist", rotated)
		}
	}

	// 1. Fresh Sybil buyer funded by Hub #2 dirty BTC (2 hops away) trying to buy clean MRC20 on Market -> BLOCKED
	dirtyBuyPsbt := buildTestPsbtBuilder(t, []wire.OutPoint{
		{Hash: mustHash(t, cleanDummyTxId), Index: 0},       // input 0: platform dummy
		{Hash: mustHash(t, cleanSellerAssetTxId), Index: 0}, // input 1: clean seller MRC20
		{Hash: mustHash(t, hub2SplitHop2TxId), Index: 0},    // input 2: buyer dirty BTC from Hub #2
	})
	if blocked, reason := eng.CheckBuyerTakePsbt("btc", "1FreshSybilBuyer", dirtyBuyPsbt, 2); !blocked {
		t.Fatalf("expected Hub #2 dirty BTC buyer PSBT to be blocked")
	} else {
		t.Logf("Blocked Hub #2 dirty BTC buyer take: %s", reason)
	}

	// 2. Fresh wallet funded by Market sale BTC proceeds (completedTradeTxId:1) -> BLOCKED on both Take and Raw Broadcast
	proceedsBuyPsbt := buildTestPsbtBuilder(t, []wire.OutPoint{
		{Hash: mustHash(t, cleanDummyTxId), Index: 0},
		{Hash: mustHash(t, cleanSellerAssetTxId), Index: 0},
		{Hash: mustHash(t, proceedsHopTxId), Index: 0},
	})
	if blocked, _ := eng.CheckBuyerTakePsbt("btc", "1FreshProceedsWallet", proceedsBuyPsbt, 2); !blocked {
		t.Fatalf("expected Market sale proceeds BTC buyer PSBT to be blocked")
	}

	rawProceedsTx := wire.NewMsgTx(2)
	hProceeds := mustHash(t, proceedsHopTxId)
	rawProceedsTx.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: hProceeds, Index: 0}, nil, nil))
	rawProceedsTx.AddTxOut(wire.NewTxOut(990000, []byte{0x51, 0x01, 0x01}))
	if blocked, reason := eng.CheckRawBtcTx("btc", serializeMsgTxHex(t, rawProceedsTx)); !blocked {
		t.Fatalf("expected raw broadcast spending Market sale proceeds BTC to be blocked")
	} else {
		t.Logf("Blocked raw broadcast of sale proceeds BTC: %s", reason)
	}

	// 3. Innocent Market buyer spending their BTC change (completedTradeTxId:4) -> CLEAN & ALLOWED
	cleanBuyerChangePsbt := buildTestPsbtBuilder(t, []wire.OutPoint{
		{Hash: mustHash(t, cleanDummyTxId), Index: 0},
		{Hash: mustHash(t, cleanSellerAssetTxId), Index: 0},
		{Hash: mustHash(t, completedTradeTxId), Index: 4},
	})
	if blocked, reason := eng.CheckBuyerTakePsbt("btc", "1KfU2XZw2X5bzQ1VkSURP4wmUmSgy9Demv", cleanBuyerChangePsbt, 2); blocked {
		t.Fatalf("expected innocent buyer BTC change (vout 4) to be allowed, got blocked: %s", reason)
	}

	// 4. Draining a brand-new phishing victim's clean UTXO TO a known thief hub/mule via BroadcastTx -> BLOCKED
	thiefPkScriptHex, err := common.AddressToPkScript("mainnet", "btc", "1CjSkrT5zEowwGtckmyqsqLE68GJZ8Ziu1")
	if err != nil {
		t.Fatalf("AddressToPkScript err: %v", err)
	}
	thiefPkScript, _ := hex.DecodeString(thiefPkScriptHex)
	drainTx := wire.NewMsgTx(2)
	hClean := mustHash(t, cleanUserBtcTxId)
	drainTx.AddTxIn(wire.NewTxIn(&wire.OutPoint{Hash: hClean, Index: 0}, nil, nil))
	drainTx.AddTxOut(wire.NewTxOut(546, thiefPkScript))
	if blocked, reason := eng.CheckRawBtcTx("btc", serializeMsgTxHex(t, drainTx)); !blocked {
		t.Fatalf("expected raw tx paying blacklisted thief address to be blocked")
	} else {
		t.Logf("Blocked drain tx to blacklisted thief address: %s", reason)
	}

	// 5. Anti-dusting: dirty dust UTXO sent to 1InnocentUser is blocked/filtered, but 1InnocentUser's address stays CLEAN!
	if !eng.IsUtxoTainted("btc", dustAttackTxId+"_0", 546, 977070) {
		t.Fatalf("expected dirty dust UTXO to be flagged tainted")
	}
	dustedPsbt := buildTestPsbtBuilder(t, []wire.OutPoint{
		{Hash: mustHash(t, cleanDummyTxId), Index: 0},
		{Hash: mustHash(t, cleanSellerAssetTxId), Index: 0},
		{Hash: mustHash(t, dustAttackTxId), Index: 0},
	})
	if blocked, _ := eng.CheckBuyerTakePsbt("btc", "1InnocentUser", dustedPsbt, 2); !blocked {
		t.Fatalf("expected accidental spend of dirty dust UTXO to be rejected")
	}
	if addrBlocked, _ := eng.IsAddressBlacklisted("1InnocentUser"); addrBlocked {
		t.Fatalf("ANTI-DUSTING VIOLATION: 1InnocentUser address must NEVER be auto-blacklisted by unsolicited dust!")
	}
	cleanUserPsbt := buildTestPsbtBuilder(t, []wire.OutPoint{
		{Hash: mustHash(t, cleanDummyTxId), Index: 0},
		{Hash: mustHash(t, cleanSellerAssetTxId), Index: 0},
		{Hash: mustHash(t, cleanUserBtcTxId), Index: 0},
	})
	if blocked, reason := eng.CheckBuyerTakePsbt("btc", "1InnocentUser", cleanUserPsbt, 2); blocked {
		t.Fatalf("expected 1InnocentUser clean BTC UTXO purchase to succeed, got blocked: %s", reason)
	}
}

func TestRiskEngine_TradeCacheCleanCacheStabilityAndFastCheck(t *testing.T) {
	txLookups := 0
	tradeLookups := 0
	const txClean = "aaaa111111111111111111111111111111111111111111111111111111111111"
	const txTrade = "bbbb222222222222222222222222222222222222222222222222222222222222"

	eng := NewRiskEngine(
		func(chain, txId string) (*ChainTx, error) {
			txLookups++
			return &ChainTx{
				TxId:        txId,
				Confirmed:   true,
				BlockHeight: 977100,
				Inputs:      []ChainTxInput{{TxId: "0000000000000000000000000000000000000000000000000000000000000001", Vout: 0, Address: "1OldClean", Value: 546}},
				Outputs:     []ChainTxOutput{{Address: "1UserA", Value: 546}},
			}, nil
		},
		func(txId string) (*CompletedMarketTrade, error) {
			tradeLookups++
			if txId == txTrade {
				return &CompletedMarketTrade{TxId: txTrade, SellerAddress: "1Seller", BuyerAddress: "1Buyer"}, nil
			}
			return nil, nil
		},
		nil,
	)

	seed := DefaultSeedEntries()
	eng.ApplyEntries(seed)

	// 1. Trade cache caches both hit and miss without hitting DB again
	if _, err := eng.lookupTradeCached(txClean); err != nil {
		t.Fatalf("lookupTradeCached miss err: %v", err)
	}
	if _, err := eng.lookupTradeCached(txClean); err != nil {
		t.Fatalf("lookupTradeCached miss2 err: %v", err)
	}
	if _, err := eng.lookupTradeCached(txTrade); err != nil {
		t.Fatalf("lookupTradeCached hit err: %v", err)
	}
	if _, err := eng.lookupTradeCached(txTrade); err != nil {
		t.Fatalf("lookupTradeCached hit2 err: %v", err)
	}
	if tradeLookups != 2 {
		t.Fatalf("expected 2 tradeLookup calls (1 miss + 1 hit), got %d", tradeLookups)
	}

	// 2. ApplyEntries with identical list must preserve cleanOutpoints cache
	if eng.IsUtxoTainted("btc", txClean+"_0", 546, 977100) {
		t.Fatalf("expected clean UTXO")
	}
	firstTxLookups := txLookups
	eng.ApplyEntries(seed) // identical entries -> must NOT wipe cleanOutpoints
	if eng.IsUtxoTainted("btc", txClean+"_0", 546, 977100) {
		t.Fatalf("expected clean UTXO cached")
	}
	if txLookups != firstTxLookups {
		t.Fatalf("expected cleanOutpoints preserved on unchanged ApplyEntries, txLookups went from %d to %d", firstTxLookups, txLookups)
	}

	// Adding a new blacklist entry must invalidate cleanOutpoints
	updated := append(seed, &models.MarketBlacklistModel{
		EntryType:  models.BlacklistTypeAddress,
		EntryValue: "1SomeNewThief",
		Reason:     "test",
		State:      models.STATE_EXIST,
	})
	eng.ApplyEntries(updated)
	if len(eng.cleanOutpoints) != 0 {
		t.Fatalf("expected cleanOutpoints cleared when entries changed")
	}

	// 3. IsUtxoTaintedFast must block direct blacklisted UTXOs and never invoke txFetcher
	txBeforeFast := txLookups
	if !eng.IsUtxoTaintedFast("btc", "b6994a0b4c520ed9ecea489cd462d92387108ab042bf2aeb6424c0943e033718_1", 977100) {
		t.Fatalf("expected stolen MC UTXO blocked by IsUtxoTaintedFast")
	}
	if eng.IsUtxoTaintedFast("btc", "ac30da2b8dc59d2a19d9bd83b64761d74a17e76a4a9573f38e99291731b76cb8_1", 977100) {
		t.Fatalf("expected whitelisted rescued UTXO allowed by IsUtxoTaintedFast")
	}
	if eng.IsUtxoTaintedFast("btc", txClean+"_0", 977100) {
		t.Fatalf("expected normal UTXO allowed by IsUtxoTaintedFast")
	}
	if txLookups != txBeforeFast {
		t.Fatalf("IsUtxoTaintedFast must not invoke external txFetcher")
	}
}

func TestRiskEngine_NoFalsePositiveOnPreIncidentVictimTradesDummyPoolOrExchangeConsolidation(t *testing.T) {
	conf.PlatformAddressDummyAsk = "bc1q0hzgkuls62egjpqxxhzs3cky7auyz596252p78"
	conf.PlatformServiceFeeConfigData = &conf.ServiceFeeConfig{
		ServiceAddress: "bc1pf4gj2uyjfytzzx3h06m977tvw6pl6hgy6kmgyle2757pcrrukr8s2srvy5",
	}

	const (
		preIncidentVictimTradeTx = "19730fe6cba94663bc614f801e882d973426ac7054990f2d80ca1f1a419fd673" // blk 975887 (< 977025)
		sellerConsolidationTx    = "6e469c3a6347cccf4a2181763f00eae965ae8cfcc84c4fe54eb6a2f8b8c68762" // blk 977138
		unindexedThiefDummyTx    = "aaaa888888888888888888888888888888888888888888888888888888888888" // blk 977100
		userLegitSaleTx          = "d09c7b6dd7a169490d1d31f575b0635908c6b9c894037efaa99ad06c56560edc" // blk 977187
		userOldMintTx            = "ff8cb3a1577b16a64c27ebfd1c6ee5c7cf67054569e5082e460d5084b9d150be" // blk 856009
		preIncidentDirectTokenTx = "bbbb999999999999999999999999999999999999999999999999999999999999" // blk 975000
		exchangeSweepTx          = "cccc777777777777777777777777777777777777777777777777777777777777" // blk 977150
		exchangeWithdrawTx       = "2fa30aa977e50000000000000000000000000000000000000000000000000000" // blk 977180
		dirtyIndirectHop1Tx      = "dddd666666666666666666666666666666666666666666666666666666666666" // blk 977140
	)

	// Build a 12-depositor exchange consolidation where 1 input is indirectly tainted 2 hops away
	exchangeInputs := make([]ChainTxInput, 0, 12)
	exchangeInputs = append(exchangeInputs, ChainTxInput{
		TxId: dirtyIndirectHop1Tx, Vout: 0, Address: "1ExchangeUserDepositDirtyHop", Value: 500000,
	})
	for i := 0; i < 11; i++ {
		exchangeInputs = append(exchangeInputs, ChainTxInput{
			TxId: userOldMintTx, Vout: uint32(i + 2), Address: "1CleanDepositor_" + string(rune('A'+i)), Value: 1000000,
		})
	}

	txMap := map[string]*ChainTx{
		// 8 days before the phishing incident: 1CuCuXbg... legitimately bought from 1PqueWv1...
		preIncidentVictimTradeTx: {
			TxId: preIncidentVictimTradeTx, Confirmed: true, BlockHeight: 975887,
			Inputs: []ChainTxInput{
				{TxId: "2f7323d2702c8af9e942d55572993d776e9ff9379f53cc226e1d93e9b1d48c63", Vout: 2, Address: "bc1q0hzgkuls62egjpqxxhzs3cky7auyz596252p78", Value: 600},
				{TxId: "d348dfd20726e85d4bfcbc59bb472c8c8800e5b0fb4e8173f8fa5f67213cc6ae", Vout: 1, Address: "1PqueWv1aEpaY1jZ3C27efpB919cZNseXd", Value: 546},
				{TxId: "ed0360f1723fd982315c371e4b77735668a1cc336037b0ae6c80bd23afbb438b", Vout: 20, Address: "1CuCuXbgjpov1t54yB2YanN4yCYtGTS7Bq", Value: 105000},
			},
			Outputs: []ChainTxOutput{
				{Address: "1CuCuXbgjpov1t54yB2YanN4yCYtGTS7Bq", Value: 546},
				{Address: "1PqueWv1aEpaY1jZ3C27efpB919cZNseXd", Value: 82600},
				{Address: "bc1q0hzgkuls62egjpqxxhzs3cky7auyz596252p78", Value: 600},
				{Address: "1CuCuXbgjpov1t54yB2YanN4yCYtGTS7Bq", Value: 20585},
			},
		},
		// Post-incident consolidation by innocent active seller 1PqueWv1...
		sellerConsolidationTx: {
			TxId: sellerConsolidationTx, Confirmed: true, BlockHeight: 977138,
			Inputs: []ChainTxInput{
				{TxId: preIncidentVictimTradeTx, Vout: 1, Address: "1PqueWv1aEpaY1jZ3C27efpB919cZNseXd", Value: 82600},
			},
			Outputs: []ChainTxOutput{
				{Address: "bc1q2ngpqcxn6cvmualrj7nnhcku2vktyjeyp59s2h3flk45069s2qpqgxx7ps", Value: 8400000},
				{Address: "1PqueWv1aEpaY1jZ3C27efpB919cZNseXd", Value: 416692},
			},
		},
		// Platform 600-sat dummy UTXO recycled from a tx that touched a thief input (even if tradeLookup hasn't indexed it yet)
		unindexedThiefDummyTx: {
			TxId: unindexedThiefDummyTx, Confirmed: true, BlockHeight: 977100,
			Inputs: []ChainTxInput{
				{TxId: "dd51318238798de987a77b433c3234769267079388496c598c4d94f3977c2604", Vout: 1, Address: "1GtPDnwuWkULR36euBCYJTTGQuicVhsre4", Value: 546},
			},
			Outputs: []ChainTxOutput{
				{Address: "1SomeBuyer", Value: 546},
				{Address: "1GtPDnwuWkULR36euBCYJTTGQuicVhsre4", Value: 500000},
				{Address: "bc1q0hzgkuls62egjpqxxhzs3cky7auyz596252p78", Value: 600}, // vout 2: recycled platform dummy
			},
		},
		userOldMintTx: {
			TxId: userOldMintTx, Confirmed: true, BlockHeight: 856009,
			Inputs:  []ChainTxInput{{TxId: "0000000000000000000000000000000000000000000000000000000000000099", Vout: 0, Address: "17Co7Yc5kmyz7gbi1vV8AJxZuFCC8pJCGb", Value: 10000}},
			Outputs: []ChainTxOutput{{Address: "17Co7Yc5kmyz7gbi1vV8AJxZuFCC8pJCGb", Value: 546}, {Address: "17Co7Yc5kmyz7gbi1vV8AJxZuFCC8pJCGb", Value: 546}},
		},
		dirtyIndirectHop1Tx: {
			TxId: dirtyIndirectHop1Tx, Confirmed: true, BlockHeight: 977140,
			Inputs:  []ChainTxInput{{TxId: "1ef5460150b9e2522b9fa1143714f684735ad9a1e5d63b3a7c8e61a697fae346", Vout: 1, Address: "bc1pky6zpsx8gpv257kr3w43rvl2dre0j7phndxlf5muetzcyane8cyq2cys66", Value: 500000}},
			Outputs: []ChainTxOutput{{Address: "1ExchangeUserDepositDirtyHop", Value: 500000}},
		},
		exchangeSweepTx: {
			TxId: exchangeSweepTx, Confirmed: true, BlockHeight: 977150,
			Inputs:  exchangeInputs,
			Outputs: []ChainTxOutput{{Address: "bc1q_exchange_hot_wallet", Value: 11500000}},
		},
		exchangeWithdrawTx: {
			TxId: exchangeWithdrawTx, Confirmed: true, BlockHeight: 977180,
			Inputs:  []ChainTxInput{{TxId: exchangeSweepTx, Vout: 0, Address: "bc1q_exchange_hot_wallet", Value: 11500000}},
			Outputs: []ChainTxOutput{{Address: "1JUA2QXPKQVFHEiA59UJ9EAAzV66wp1dYh", Value: 1550000}},
		},
		// User 17Co7Yc5kmyz7gbi1vV8AJxZuFCC8pJCGb legitimate Market sale spending recycled dummy + buyer's exchange withdrawal
		userLegitSaleTx: {
			TxId: userLegitSaleTx, Confirmed: true, BlockHeight: 977187,
			Inputs: []ChainTxInput{
				{TxId: unindexedThiefDummyTx, Vout: 2, Address: "bc1q0hzgkuls62egjpqxxhzs3cky7auyz596252p78", Value: 600},
				{TxId: userOldMintTx, Vout: 1, Address: "17Co7Yc5kmyz7gbi1vV8AJxZuFCC8pJCGb", Value: 546},
				{TxId: exchangeWithdrawTx, Vout: 0, Address: "1JUA2QXPKQVFHEiA59UJ9EAAzV66wp1dYh", Value: 1550000},
			},
			Outputs: []ChainTxOutput{
				{Address: "1JUA2QXPKQVFHEiA59UJ9EAAzV66wp1dYh", Value: 546},
				{Address: "17Co7Yc5kmyz7gbi1vV8AJxZuFCC8pJCGb", Value: 1400000},
				{Address: "bc1q0hzgkuls62egjpqxxhzs3cky7auyz596252p78", Value: 600},
				{Address: "bc1pf4gj2uyjfytzzx3h06m977tvw6pl6hgy6kmgyle2757pcrrukr8s2srvy5", Value: 14000},
				{Address: "1JUA2QXPKQVFHEiA59UJ9EAAzV66wp1dYh", Value: 135147},
			},
		},
		// Token transferred FROM 1CuCuXbg... BEFORE the phishing incident (blk 975000 < 977025)
		preIncidentDirectTokenTx: {
			TxId: preIncidentDirectTokenTx, Confirmed: true, BlockHeight: 975000,
			Inputs:  []ChainTxInput{{TxId: "ed0360f1723fd982315c371e4b77735668a1cc336037b0ae6c80bd23afbb438b", Vout: 1, Address: "1CuCuXbgjpov1t54yB2YanN4yCYtGTS7Bq", Value: 546}},
			Outputs: []ChainTxOutput{{Address: "1InnocentEarlyHolder", Value: 546}},
		},
	}

	eng := NewRiskEngine(
		func(chain, txId string) (*ChainTx, error) { return txMap[txId], nil },
		func(txId string) (*CompletedMarketTrade, error) { return nil, nil },
		nil,
	)
	eng.LoadSeedEntriesInMemory()

	// 1. 1PqueWv1... consolidation of pre-incident (blk 975887) trade with 1CuCuXbg... MUST BE CLEAN
	if blocked, reason := eng.CheckUtxoAndAddress("btc", "1PqueWv1aEpaY1jZ3C27efpB919cZNseXd", sellerConsolidationTx+"_1", 0, nil); blocked {
		t.Fatalf("expected 1PqueWv1 consolidation of pre-incident trade to be clean, got blocked: %s", reason)
	}
	if addrBlocked, _ := eng.IsAddressBlacklisted("1PqueWv1aEpaY1jZ3C27efpB919cZNseXd"); addrBlocked {
		t.Fatalf("1PqueWv1aEpaY1jZ3C27efpB919cZNseXd must NOT be blacklisted")
	}

	// 2. User 17Co7Yc5kmyz7gbi1vV8AJxZuFCC8pJCGb sale proceeds (spending recycled platform dummy + exchange withdrawal) MUST BE CLEAN
	if blocked, reason := eng.CheckUtxoAndAddress("btc", "17Co7Yc5kmyz7gbi1vV8AJxZuFCC8pJCGb", userLegitSaleTx+"_1", 0, nil); blocked {
		t.Fatalf("expected 17Co7Yc5kmyz7gbi1vV8AJxZuFCC8pJCGb sale proceeds UTXO to be clean, got blocked: %s", reason)
	}
	if addrBlocked, _ := eng.IsAddressBlacklisted("17Co7Yc5kmyz7gbi1vV8AJxZuFCC8pJCGb"); addrBlocked {
		t.Fatalf("17Co7Yc5kmyz7gbi1vV8AJxZuFCC8pJCGb must NOT be blacklisted")
	}

	// 3. Pre-incident token with TaintHint{FromAddress: 1CuCuXbg...} confirmed at blk 975000 MUST BE CLEAN
	if blocked, reason := eng.CheckUtxoAndAddress("btc", "1InnocentEarlyHolder", preIncidentDirectTokenTx+"_0", 546, []TaintHint{{FromAddress: "1CuCuXbgjpov1t54yB2YanN4yCYtGTS7Bq"}}); blocked {
		t.Fatalf("expected pre-incident token from 1CuCuXbg... at blk 975000 to be clean, got blocked: %s", reason)
	}

	// 4. Post-incident / unconfirmed token from 1CuCuXbg... MUST STILL BE BLOCKED
	if blocked, _ := eng.CheckUtxoAndAddress("btc", "1PostHackMule", "ffff000000000000000000000000000000000000000000000000000000000000_0", 546, []TaintHint{{FromAddress: "1CuCuXbgjpov1t54yB2YanN4yCYtGTS7Bq"}}); !blocked {
		t.Fatalf("expected unconfirmed/post-incident transfer from compromised 1CuCuXbg... to be blocked")
	}
}

