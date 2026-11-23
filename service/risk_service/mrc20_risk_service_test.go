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

	// 3-hop Sybil rotation must be caught and auto-blacklisted
	blocked, reason := eng.CheckUtxoAndAddress("btc", "1SybilC", "tx_reveal_3_0", 546, nil)
	if !blocked {
		t.Fatalf("expected 3-hop SybilC listing to be blocked")
	}
	t.Logf("SybilC blocked reason: %s (persisted %d auto-taint records)", reason, len(persisted))
	if addrBlocked, _ := eng.IsAddressBlacklisted("1SybilC"); !addrBlocked {
		t.Fatalf("expected 1SybilC to be auto-blacklisted after taint detection")
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
