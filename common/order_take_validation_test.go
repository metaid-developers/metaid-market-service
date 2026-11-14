package common

import (
	"encoding/hex"
	"fmt"
	"metaid-market-service/conf"
	"metaid-market-service/models"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil/psbt"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

func testWallet(t *testing.T) (string, string, string) {
	t.Helper()
	privateKey, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	publicKey := hex.EncodeToString(privateKey.PubKey().SerializeCompressed())
	address, err := GetSegwitAddressFromPublicKey(GetNetParams("testnet", "btc"), publicKey)
	if err != nil {
		t.Fatal(err)
	}
	script, err := AddressToPkScript("testnet", "btc", address)
	if err != nil {
		t.Fatal(err)
	}
	return publicKey, address, script
}

func testOutpoint(id int, index uint32) Input {
	return Input{OutTxId: fmt.Sprintf("%064x", id), OutIndex: index}
}

func testSigner(index int, script string, amount uint64, sighash txscript.SigHashType) *InputSign {
	return &InputSign{Index: index, UtxoType: Witness, PkScript: script, Amount: amount, SighashType: sighash}
}

func setupTakeValidation(t *testing.T) (string, string, string, string) {
	t.Helper()
	previousNet := conf.Net
	previousDummyAddress := conf.PlatformAddressDummyAsk
	previousFees := conf.PlatformServiceFeeConfigData
	previousCheck := checkUtxoSpendForValidation
	t.Cleanup(func() {
		conf.Net = previousNet
		conf.PlatformAddressDummyAsk = previousDummyAddress
		conf.PlatformServiceFeeConfigData = previousFees
		checkUtxoSpendForValidation = previousCheck
	})
	conf.Net = "testnet"
	checkUtxoSpendForValidation = func(string, string, string, int64) (bool, string) { return false, "" }
	_, dummyAddress, _ := testWallet(t)
	_, feeAddress, _ := testWallet(t)
	conf.PlatformAddressDummyAsk = dummyAddress
	conf.PlatformServiceFeeConfigData = &conf.ServiceFeeConfig{ServiceAddress: feeAddress, Mrc20TradeFeeRate: 60, Mrc20TradeFeeMin: 1800, PinTradeFeeRate: 60, PinTradeFeeMin: 1800}
	buyerPublicKey, buyerAddress, _ := testWallet(t)
	_, sellerAddress, _ := testWallet(t)
	return buyerPublicKey, buyerAddress, sellerAddress, dummyAddress
}

func validMrc20Take(t *testing.T, buyerPublicKey, buyerAddress, sellerAddress, dummyAddress string) (*models.MarketMrc20OrderModel, *PsbtBuilder, map[string]*models.MarketUtxoModel) {
	t.Helper()
	buyerScript, _ := AddressToPkScript("testnet", "btc", buyerAddress)
	sellerScript, _ := AddressToPkScript("testnet", "btc", sellerAddress)
	dummyScript, _ := AddressToPkScript("testnet", "btc", dummyAddress)
	feeScript, _ := AddressToPkScript("testnet", "btc", conf.PlatformServiceFeeConfigData.ServiceAddress)
	dummyID := testOutpoint(1, 0)
	sellerID := testOutpoint(2, 1)
	buyerID := testOutpoint(3, 0)
	dummy := &models.MarketUtxoModel{UtxoId: fmt.Sprintf("%s_%d", dummyID.OutTxId, dummyID.OutIndex), TxId: dummyID.OutTxId, Index: 0, Amount: 600, Address: dummyAddress, Chain: "btc", UsedState: models.UsedNo, UtxoType: models.UtxoTypeDummy600}
	inputs := []Input{dummyID, sellerID, buyerID}
	outputs := []Output{{Address: buyerAddress, Amount: 546}, {Address: sellerAddress, Amount: 1000000}, {Address: dummyAddress, Amount: 600}, {Script: feeScript, Amount: 6000}}
	builder, err := CreatePsbtBuilder(GetNetParams("testnet", "btc"), inputs, outputs)
	if err != nil {
		t.Fatal(err)
	}
	if err := builder.UpdateAndAddInputWitness([]*InputSign{
		testSigner(0, dummyScript, 600, txscript.SigHashAll),
		testSigner(1, sellerScript, 546, txscript.SigHashSingle|txscript.SigHashAnyOneCanPay),
		testSigner(2, buyerScript, 2000000, txscript.SigHashAll),
	}); err != nil {
		t.Fatal(err)
	}
	entity := &models.MarketMrc20OrderModel{UtxoId: fmt.Sprintf("%s_%d", sellerID.OutTxId, sellerID.OutIndex), SellerAddress: sellerAddress, OutValue: 546, PriceAmount: 1000000, Chain: "btc"}
	return entity, builder, map[string]*models.MarketUtxoModel{dummy.UtxoId: dummy}
}

func TestValidateMrc20TakePsbt(t *testing.T) {
	buyerPublicKey, buyerAddress, sellerAddress, dummyAddress := setupTakeValidation(t)
	entity, builder, dummies := validMrc20Take(t, buyerPublicKey, buyerAddress, sellerAddress, dummyAddress)
	if _, err := ValidateMrc20TakePsbt("testnet", "btc", entity, builder, buyerPublicKey, dummies); err != nil {
		t.Fatalf("valid transaction rejected: %v", err)
	}
}

func TestValidateMrc20TakePsbtRejectsTampering(t *testing.T) {
	buyerPublicKey, buyerAddress, sellerAddress, dummyAddress := setupTakeValidation(t)
	tests := []struct {
		name   string
		mutate func(*PsbtBuilder)
	}{
		{name: "platform fee removed", mutate: func(builder *PsbtBuilder) {
			builder.PsbtUpdater.Upsbt.UnsignedTx.TxOut = builder.PsbtUpdater.Upsbt.UnsignedTx.TxOut[:3]
		}},
		{name: "seller payout replaced", mutate: func(builder *PsbtBuilder) {
			builder.PsbtUpdater.Upsbt.UnsignedTx.TxOut[1].Value--
		}},
		{name: "asset input replaced", mutate: func(builder *PsbtBuilder) {
			builder.PsbtUpdater.Upsbt.UnsignedTx.TxIn[1].PreviousOutPoint.Index++
		}},
		{name: "extra output", mutate: func(builder *PsbtBuilder) {
			builder.PsbtUpdater.Upsbt.UnsignedTx.TxOut = append(builder.PsbtUpdater.Upsbt.UnsignedTx.TxOut, wire.NewTxOut(1000, []byte{0x51}))
		}},
		{name: "buyer asset output replaced", mutate: func(builder *PsbtBuilder) {
			builder.PsbtUpdater.Upsbt.UnsignedTx.TxOut[0].Value++
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entity, builder, dummies := validMrc20Take(t, buyerPublicKey, buyerAddress, sellerAddress, dummyAddress)
			test.mutate(builder)
			if _, err := ValidateMrc20TakePsbt("testnet", "btc", entity, builder, buyerPublicKey, dummies); err == nil {
				t.Fatal("tampered transaction was accepted")
			}
		})
	}
}

func validPinTake(t *testing.T, buyerPublicKey, buyerAddress, sellerAddress, dummyAddress string) (*models.MarketOrderModel, *PsbtBuilder, map[string]*models.MarketUtxoModel) {
	t.Helper()
	buyerScript, _ := AddressToPkScript("testnet", "btc", buyerAddress)
	sellerScript, _ := AddressToPkScript("testnet", "btc", sellerAddress)
	dummyScript, _ := AddressToPkScript("testnet", "btc", dummyAddress)
	feeScript, _ := AddressToPkScript("testnet", "btc", conf.PlatformServiceFeeConfigData.ServiceAddress)
	inputs := []Input{testOutpoint(10, 0), testOutpoint(11, 0), testOutpoint(12, 1), testOutpoint(13, 0), testOutpoint(14, 0)}
	outputs := []Output{{Address: dummyAddress, Amount: 1200}, {Address: buyerAddress, Amount: 546}, {Address: sellerAddress, Amount: 1000000}, {Address: dummyAddress, Amount: 600}, {Address: dummyAddress, Amount: 600}, {Script: feeScript, Amount: 6000}}
	builder, err := CreatePsbtBuilder(GetNetParams("testnet", "btc"), inputs, outputs)
	if err != nil {
		t.Fatal(err)
	}
	if err := builder.UpdateAndAddInputWitness([]*InputSign{
		testSigner(0, dummyScript, 600, txscript.SigHashAll),
		testSigner(1, dummyScript, 600, txscript.SigHashAll),
		testSigner(2, sellerScript, 546, txscript.SigHashSingle|txscript.SigHashAnyOneCanPay),
		testSigner(3, dummyScript, 1200, txscript.SigHashAll),
		testSigner(4, buyerScript, 2000000, txscript.SigHashAll),
	}); err != nil {
		t.Fatal(err)
	}
	dummies := make(map[string]*models.MarketUtxoModel)
	for i, input := range inputs {
		if i == 2 || i == 4 {
			continue
		}
		type_ := models.UtxoTypeDummy600
		amount := uint64(600)
		if i == 3 {
			type_ = models.UtxoTypeDummy1200
			amount = 1200
		}
		id := fmt.Sprintf("%s_%d", input.OutTxId, input.OutIndex)
		dummies[id] = &models.MarketUtxoModel{UtxoId: id, TxId: input.OutTxId, Index: int64(input.OutIndex), Amount: amount, Address: dummyAddress, Chain: "btc", UsedState: models.UsedNo, UtxoType: type_}
	}
	entity := &models.MarketOrderModel{UtxoId: fmt.Sprintf("%s_%d", inputs[2].OutTxId, inputs[2].OutIndex), SellerAddress: sellerAddress, OutValue: 546, SellPriceAmount: 1000000, Chain: "btc"}
	return entity, builder, dummies
}

func TestValidatePinTakePsbt(t *testing.T) {
	buyerPublicKey, buyerAddress, sellerAddress, dummyAddress := setupTakeValidation(t)
	entity, builder, dummies := validPinTake(t, buyerPublicKey, buyerAddress, sellerAddress, dummyAddress)
	if _, err := ValidatePinTakePsbt("testnet", "btc", entity, builder, buyerPublicKey, dummies); err != nil {
		t.Fatalf("valid PIN transaction rejected: %v", err)
	}
	builder.PsbtUpdater.Upsbt.UnsignedTx.TxOut[2].Value--
	if _, err := ValidatePinTakePsbt("testnet", "btc", entity, builder, buyerPublicKey, dummies); err == nil {
		t.Fatal("PIN transaction with changed seller payout was accepted")
	}
}

func TestValidateMakerAssetInput(t *testing.T) {
	buyerPublicKey, buyerAddress, sellerAddress, dummyAddress := setupTakeValidation(t)
	_, builder, _ := validMrc20Take(t, buyerPublicKey, buyerAddress, sellerAddress, dummyAddress)
	builder.PsbtUpdater.Upsbt.Inputs[1].PartialSigs = []*psbt.PartialSig{{PubKey: []byte{1}, Signature: []byte{2}}}
	if err := ValidateMakerAssetInput("testnet", "btc", builder, 1, sellerAddress); err != nil {
		t.Fatalf("signed maker input rejected: %v", err)
	}
	builder.PsbtUpdater.Upsbt.Inputs[1].PartialSigs = nil
	if err := ValidateMakerAssetInput("testnet", "btc", builder, 1, sellerAddress); err == nil {
		t.Fatal("unsigned maker input was accepted")
	}
	builder.PsbtUpdater.Upsbt.Inputs[1].PartialSigs = []*psbt.PartialSig{{PubKey: []byte{1}, Signature: []byte{2}}}
	builder.PsbtUpdater.Upsbt.Inputs[1].SighashType = txscript.SigHashAll
	if err := ValidateMakerAssetInput("testnet", "btc", builder, 1, sellerAddress); err == nil {
		t.Fatal("maker input with wrong sighash was accepted")
	}
}

func TestValidateBatchPinTakePsbt(t *testing.T) {
	buyerPublicKey, buyerAddress, sellerAddress, dummyAddress := setupTakeValidation(t)
	first, builder, dummies := validPinTake(t, buyerPublicKey, buyerAddress, sellerAddress, dummyAddress)
	secondInput := testOutpoint(15, 1)
	input := wire.NewTxIn(wire.NewOutPoint(secondInputMustHash(t, secondInput.OutTxId), secondInput.OutIndex), nil, nil)
	builder.PsbtUpdater.Upsbt.UnsignedTx.TxIn = append(builder.PsbtUpdater.Upsbt.UnsignedTx.TxIn[:3], append([]*wire.TxIn{input}, builder.PsbtUpdater.Upsbt.UnsignedTx.TxIn[3:]...)...)
	sellerScript, _ := AddressToPkScript("testnet", "btc", sellerAddress)
	scriptBytes, _ := hex.DecodeString(sellerScript)
	builder.PsbtUpdater.Upsbt.Inputs = append(builder.PsbtUpdater.Upsbt.Inputs[:3], append([]psbt.PInput{{WitnessUtxo: wire.NewTxOut(546, scriptBytes), SighashType: txscript.SigHashSingle | txscript.SigHashAnyOneCanPay}}, builder.PsbtUpdater.Upsbt.Inputs[3:]...)...)
	builder.PsbtUpdater.Upsbt.UnsignedTx.TxOut = append(builder.PsbtUpdater.Upsbt.UnsignedTx.TxOut[:3], append([]*wire.TxOut{wire.NewTxOut(1000000, scriptBytes)}, builder.PsbtUpdater.Upsbt.UnsignedTx.TxOut[3:]...)...)
	builder.PsbtUpdater.Upsbt.UnsignedTx.TxOut = builder.PsbtUpdater.Upsbt.UnsignedTx.TxOut[:6]
	builder.PsbtUpdater.Upsbt.UnsignedTx.TxOut[1].Value = 1092
	second := &models.MarketOrderModel{UtxoId: secondInputID(secondInput), SellerAddress: sellerAddress, OutValue: 546, SellPriceAmount: 1000000, Chain: "btc"}
	if _, err := ValidateBatchPinTakePsbt("testnet", "btc", []*models.MarketOrderModel{first, second}, builder, buyerPublicKey, dummies); err != nil {
		t.Fatalf("valid batch transaction rejected: %v", err)
	}
	builder.PsbtUpdater.Upsbt.UnsignedTx.TxOut[3].Value--
	if _, err := ValidateBatchPinTakePsbt("testnet", "btc", []*models.MarketOrderModel{first, second}, builder, buyerPublicKey, dummies); err == nil {
		t.Fatal("batch transaction with changed seller payout was accepted")
	}
}

func secondInputMustHash(t *testing.T, id string) *chainhash.Hash {
	t.Helper()
	hash, err := chainhash.NewHashFromStr(id)
	if err != nil {
		t.Fatal(err)
	}
	return hash
}

func secondInputID(input Input) string {
	return input.OutTxId + "_1"
}
