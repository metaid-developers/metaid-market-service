package mrc20_service

import (
	"bytes"
	"encoding/hex"
	"metaid-market-service/common"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

func testPrivKeyHex(t *testing.T, seed byte) (string, string) {
	t.Helper()
	keyBytes := make([]byte, 32)
	for i := range keyBytes {
		keyBytes[i] = seed
	}
	keyBytes[31] = seed + 1
	priv, _ := btcec.PrivKeyFromBytes(keyBytes)
	addr, err := btcutil.NewAddressWitnessPubKeyHash(btcutil.Hash160(priv.PubKey().SerializeCompressed()), &chaincfg.MainNetParams)
	if err != nil {
		t.Fatalf("address: %v", err)
	}
	return hex.EncodeToString(priv.Serialize()), addr.EncodeAddress()
}

func newTestIdCoinsOpRequest(t *testing.T, userAddr, payToAddr string, payToAmount int64) *Mrc20OpRequest {
	t.Helper()
	return &Mrc20OpRequest{
		Net:                 &chaincfg.MainNetParams,
		MetaIdFlag:          common.GetMetaIdFlag(),
		Op:                  "mint",
		OpPayload:           `{"id":"sunny-tick-id", "vout":"2"}`,
		PayTos:              []*PayTo{{Address: payToAddr, Amount: payToAmount}},
		Mrc20OutValue:       546,
		Mrc20OutAddressList: []string{userAddr, userAddr},
		PinOutAddress:       userAddr,
		PinOutValue:         546,
		ChangeAddress:       userAddr,
	}
}

func TestIdCoinsMintBuilderLayout(t *testing.T) {
	_, userAddr := testPrivKeyHex(t, 1)
	_, payToAddr := testPrivKeyHex(t, 2)
	payToAmount := int64(1000 + 10*110) // payCheck 1000 + sweepFee at 10 sat/vB
	req := newTestIdCoinsOpRequest(t, userAddr, payToAddr, payToAmount)

	builder, inscribeFee, mintFee, err := IdCoinsMintBuilder(req, "deployer-metaid", 10)
	if err != nil {
		t.Fatalf("builder: %v", err)
	}
	if builder == nil || builder.RevealInscribePsbtBuilder == nil || builder.RevealPsbtBuilder == nil {
		t.Fatal("both reveal psbts must be built")
	}

	// reveal-inscribe: 1 input (taproot placeholder), 1 output: pinOutValue to pin address
	inscTx := builder.RevealInscribePsbtBuilder
	if len(inscTx.GetInputs()) != 1 {
		t.Fatalf("reveal-inscribe inputs = %d, want 1", len(inscTx.GetInputs()))
	}
	inscOuts := inscTx.GetOutputs()
	if len(inscOuts) != 1 || inscOuts[0].Value != 546 {
		t.Fatalf("reveal-inscribe outputs mismatch: %+v", inscOuts)
	}
	pinPkScript, err := common.AddressToPkScript("mainnet", "btc", builder.PinAddress)
	if err != nil {
		t.Fatalf("pin pkscript: %v", err)
	}
	if hex.EncodeToString(inscOuts[0].PkScript) != pinPkScript {
		t.Fatal("reveal-inscribe output must pay the pin address")
	}
	if builder.PinPrivateKeyHex == "" || builder.PinAddress == "" {
		t.Fatal("pin key material must be retained for commit-time signing")
	}

	// reveal-mint: 2 inputs (pin at 0, taproot placeholder at 1)
	mintTx := builder.RevealPsbtBuilder
	if len(mintTx.GetInputs()) != 2 {
		t.Fatalf("reveal-mint inputs = %d, want 2", len(mintTx.GetInputs()))
	}

	// output layout must match the legacy mainnet layout (cf. Sunny reveal-mint
	// tx aea801ecf3493f43…): pin re-creation 546 to the user, 2x546 mrc20
	// outputs to the user, then the payTo output carrying payAmount+sweepFee.
	mintOuts := mintTx.GetOutputs()
	if len(mintOuts) != 4 {
		t.Fatalf("reveal-mint outputs = %d, want 4 (3x546 user + payTo)", len(mintOuts))
	}
	userPkScript, err := common.AddressToPkScript("mainnet", "btc", userAddr)
	if err != nil {
		t.Fatalf("user pkscript: %v", err)
	}
	for i := 0; i < 3; i++ {
		if mintOuts[i].Value != 546 {
			t.Fatalf("output %d value = %d, want 546", i, mintOuts[i].Value)
		}
		if hex.EncodeToString(mintOuts[i].PkScript) != userPkScript {
			t.Fatalf("output %d must pay the user", i)
		}
	}
	if mintOuts[3].Value != payToAmount {
		t.Fatalf("payTo output = %d, want %d (payAmount+sweepFee)", mintOuts[3].Value, payToAmount)
	}
	payToPkScript, _ := common.AddressToPkScript("mainnet", "btc", payToAddr)
	if hex.EncodeToString(mintOuts[3].PkScript) != payToPkScript {
		t.Fatal("payTo output must pay the pool address")
	}

	// fees: each commit output must cover the outputs carried inside its reveal
	if inscribeFee <= 546 {
		t.Fatalf("inscribeFee %d must exceed the pin output value", inscribeFee)
	}
	if mintFee <= 546*3+payToAmount {
		t.Fatalf("mintFee %d must exceed the mint output sum", mintFee)
	}

	// fee math scales with the fee rate
	_, inscribeFee2, mintFee2, err := IdCoinsMintBuilder(req, "deployer-metaid", 20)
	if err != nil {
		t.Fatalf("builder r2: %v", err)
	}
	if inscribeFee2 <= inscribeFee || mintFee2 <= mintFee {
		t.Fatalf("fees must grow with fee rate: %d→%d / %d→%d", inscribeFee, inscribeFee2, mintFee, mintFee2)
	}
}

// TestIdCoinsTwoRevealChaining reproduces the commit-time wiring: commit tx →
// reveal-inscribe → reveal-mint, and asserts the extracted transactions chain
// exactly like the legacy flow.
func TestIdCoinsTwoRevealChaining(t *testing.T) {
	_, userAddr := testPrivKeyHex(t, 1)
	_, payToAddr := testPrivKeyHex(t, 2)
	funderPrivHex, funderAddr := testPrivKeyHex(t, 3)
	req := newTestIdCoinsOpRequest(t, userAddr, payToAddr, 3200)

	builder, inscribeFee, mintFee, err := IdCoinsMintBuilder(req, "deployer-metaid", 10)
	if err != nil {
		t.Fatalf("builder: %v", err)
	}

	// fake commit tx: outputs [inscribeFee→inscribeAddr, mintFee→mintAddr, change]
	commitTx := wire.NewMsgTx(2)
	zeroHash := wire.OutPoint{}.Hash
	commitTx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&zeroHash, 0), nil, nil))
	inscribeAddr, err := common.PkScriptToAddress("mainnet", "btc", hex.EncodeToString(builder.InscribeTxCtxData.CommitTxAddressPkScript))
	if err != nil {
		t.Fatalf("inscribe addr: %v", err)
	}
	mintAddr, err := common.PkScriptToAddress("mainnet", "btc", hex.EncodeToString(builder.TxCtxData.CommitTxAddressPkScript))
	if err != nil {
		t.Fatalf("mint addr: %v", err)
	}
	inscribePkScriptBytes, _ := hex.DecodeString(mustAddrToScript(t, inscribeAddr))
	mintPkScriptBytes, _ := hex.DecodeString(mustAddrToScript(t, mintAddr))
	changePkScriptBytes, _ := hex.DecodeString(mustAddrToScript(t, funderAddr))
	commitTx.AddTxOut(wire.NewTxOut(inscribeFee, inscribePkScriptBytes))
	commitTx.AddTxOut(wire.NewTxOut(mintFee, mintPkScriptBytes))
	commitTx.AddTxOut(wire.NewTxOut(9000, changePkScriptBytes))
	commitTxId := commitTx.TxHash()
	_ = funderPrivHex

	// --- wire + sign reveal-inscribe (input 0 ← commit output 0) ---
	inscBuilder, err := common.NewPsbtBuilder(&chaincfg.MainNetParams, mustPsbtString(t, builder.RevealInscribePsbtBuilder))
	if err != nil {
		t.Fatalf("inscribe psbt reload: %v", err)
	}
	inscBuilder.PsbtUpdater.Upsbt.UnsignedTx.TxIn[0].PreviousOutPoint = wire.OutPoint{Hash: commitTxId, Index: 0}
	err = inscBuilder.UpdateAndSignTaprootInput([]*common.InputSign{{
		UtxoType:            common.Taproot,
		Index:               0,
		PkScript:            mustAddrToScript(t, inscribeAddr),
		RedeemScript:        hex.EncodeToString(builder.InscribeTxCtxData.InscriptionScript),
		ControlBlockWitness: hex.EncodeToString(builder.InscribeTxCtxData.ControlBlockWitness),
		Amount:              uint64(inscribeFee),
		SighashType:         txscript.SigHashAll,
		PriHex:              builder.InscribeTxCtxData.RecoveryPrivateKeyHex,
	}})
	if err != nil {
		t.Fatalf("sign inscribe: %v", err)
	}
	inscribeRaw, err := inscBuilder.ExtractPsbtTransaction()
	if err != nil {
		t.Fatalf("extract inscribe (signing incomplete?): %v", err)
	}
	inscribeTx := wire.NewMsgTx(2)
	rawBytes, _ := hex.DecodeString(inscribeRaw)
	if err := inscribeTx.Deserialize(bytes.NewReader(rawBytes)); err != nil {
		t.Fatalf("decode inscribe: %v", err)
	}
	inscribeTxId := inscribeTx.TxHash()
	if inscribeTx.TxIn[0].PreviousOutPoint.Hash != commitTxId || inscribeTx.TxIn[0].PreviousOutPoint.Index != 0 {
		t.Fatal("reveal-inscribe must spend commit output 0")
	}

	// --- wire + sign reveal-mint (input 0 ← inscribe output 0 pin; input 1 ← commit output 1) ---
	mintBuilder, err := common.NewPsbtBuilder(&chaincfg.MainNetParams, mustPsbtString(t, builder.RevealPsbtBuilder))
	if err != nil {
		t.Fatalf("mint psbt reload: %v", err)
	}
	mintBuilder.PsbtUpdater.Upsbt.UnsignedTx.TxIn[0].PreviousOutPoint = wire.OutPoint{Hash: inscribeTxId, Index: 0}
	mintBuilder.PsbtUpdater.Upsbt.UnsignedTx.TxIn[1].PreviousOutPoint = wire.OutPoint{Hash: commitTxId, Index: 1}

	pinPkScript := mustAddrToScript(t, builder.PinAddress)
	err = mintBuilder.UpdateAndSignInput([]*common.InputSign{{
		UtxoType:    common.Witness,
		Index:       0,
		PkScript:    pinPkScript,
		Amount:      uint64(546),
		SighashType: txscript.SigHashAll,
		PriHex:      builder.PinPrivateKeyHex,
	}})
	if err != nil {
		t.Fatalf("sign pin input: %v", err)
	}
	err = mintBuilder.UpdateAndSignTaprootInput([]*common.InputSign{{
		UtxoType:            common.Taproot,
		Index:               1,
		PkScript:            mustAddrToScript(t, mintAddr),
		RedeemScript:        hex.EncodeToString(builder.TxCtxData.InscriptionScript),
		ControlBlockWitness: hex.EncodeToString(builder.TxCtxData.ControlBlockWitness),
		Amount:              uint64(mintFee),
		SighashType:         txscript.SigHashAll,
		PriHex:              builder.TxCtxData.RecoveryPrivateKeyHex,
	}})
	if err != nil {
		t.Fatalf("sign mint taproot: %v", err)
	}
	mintRaw, err := mintBuilder.ExtractPsbtTransaction()
	if err != nil {
		t.Fatalf("extract mint (signing incomplete?): %v", err)
	}
	mintTx := wire.NewMsgTx(2)
	mintRawBytes, _ := hex.DecodeString(mintRaw)
	if err := mintTx.Deserialize(bytes.NewReader(mintRawBytes)); err != nil {
		t.Fatalf("decode mint: %v", err)
	}

	// chaining assertions
	if mintTx.TxIn[0].PreviousOutPoint.Hash != inscribeTxId || mintTx.TxIn[0].PreviousOutPoint.Index != 0 {
		t.Fatal("reveal-mint input 0 must spend the reveal-inscribe pin output")
	}
	if mintTx.TxIn[1].PreviousOutPoint.Hash != commitTxId || mintTx.TxIn[1].PreviousOutPoint.Index != 1 {
		t.Fatal("reveal-mint input 1 must spend commit output 1")
	}

	// structural comparison with the legacy mainnet layout (Sunny reveal-mint
	// tx aea801ecf3493f43…): 2 inputs; outputs 0..2 = 546 sat each to the
	// user, output 3 = payTo.
	if len(mintTx.TxIn) != 2 {
		t.Fatalf("legacy layout has 2 inputs, got %d", len(mintTx.TxIn))
	}
	if len(mintTx.TxOut) != 4 {
		t.Fatalf("legacy layout has 4 outputs, got %d", len(mintTx.TxOut))
	}
	userPkScript := mustAddrToScript(t, userAddr)
	for i := 0; i < 3; i++ {
		if mintTx.TxOut[i].Value != 546 || hex.EncodeToString(mintTx.TxOut[i].PkScript) != userPkScript {
			t.Fatalf("legacy layout mismatch at output %d", i)
		}
	}
	if mintTx.TxOut[3].Value != 3200 {
		t.Fatalf("payTo output mismatch: %d", mintTx.TxOut[3].Value)
	}
}

func mustPsbtString(t *testing.T, b *common.PsbtBuilder) string {
	t.Helper()
	s, err := b.ToString()
	if err != nil {
		t.Fatalf("psbt tostring: %v", err)
	}
	return s
}

func mustAddrToScript(t *testing.T, addr string) string {
	t.Helper()
	s, err := common.AddressToPkScript("mainnet", "btc", addr)
	if err != nil {
		t.Fatalf("addr2script %s: %v", addr, err)
	}
	return s
}
