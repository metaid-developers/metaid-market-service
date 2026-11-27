package common

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"metaid-market-service/conf"
	"metaid-market-service/models"
	"strings"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

func GetMinOrderPriceAmount(chain string) int64 {
	if strings.EqualFold(chain, "doge") {
		if conf.MinOrderPriceAmountDoge > 1000000 {
			return conf.MinOrderPriceAmountDoge
		}
		return 1000000
	}
	if conf.MinOrderPriceAmountBtc > 546 {
		return conf.MinOrderPriceAmountBtc
	}
	return 546
}

func MinOutputDustLimit(net, chain, address string) int64 {
	if strings.EqualFold(chain, "doge") {
		return 1000000
	}
	if address == "" {
		return 546
	}
	addr, err := btcutil.DecodeAddress(address, GetNetParams(net, chain))
	if err != nil {
		return 546
	}
	switch addr.(type) {
	case *btcutil.AddressTaproot, *btcutil.AddressWitnessScriptHash:
		return 330
	case *btcutil.AddressWitnessPubKeyHash:
		return 294
	default:
		return 546
	}
}

func MinScriptDustLimit(net, chain string, pkScript []byte) int64 {
	if strings.EqualFold(chain, "doge") {
		return 1000000
	}
	addr, err := PkScriptToAddress(net, chain, hex.EncodeToString(pkScript))
	if err != nil {
		return 546
	}
	return MinOutputDustLimit(net, chain, addr)
}

var checkUtxoSpendForValidation = CheckUtxoSpendWithMempool

func takeInputID(txIn *wire.TxIn) string {
	return fmt.Sprintf("%s_%d", txIn.PreviousOutPoint.Hash.String(), txIn.PreviousOutPoint.Index)
}

func takeInputTxOut(builder *PsbtBuilder, index int) (*wire.TxOut, error) {
	if builder == nil || builder.PsbtUpdater == nil || index < 0 || index >= len(builder.PsbtUpdater.Upsbt.Inputs) {
		return nil, errors.New("PSBT input is missing")
	}
	input := builder.PsbtUpdater.Upsbt.Inputs[index]
	if input.WitnessUtxo != nil {
		return input.WitnessUtxo, nil
	}
	if input.NonWitnessUtxo != nil {
		outPoint := builder.PsbtUpdater.Upsbt.UnsignedTx.TxIn[index].PreviousOutPoint
		if input.NonWitnessUtxo.TxHash() != outPoint.Hash {
			return nil, errors.New("non-witness transaction does not match input outpoint")
		}
		if int(outPoint.Index) >= len(input.NonWitnessUtxo.TxOut) {
			return nil, errors.New("non-witness output index is out of range")
		}
		return input.NonWitnessUtxo.TxOut[outPoint.Index], nil
	}
	return nil, fmt.Errorf("PSBT input %d has no previous output", index)
}

func takeOutputAddress(net, chain string, output *wire.TxOut) (string, error) {
	if output == nil || output.Value <= 0 {
		return "", errors.New("transaction output has an invalid value")
	}
	return PkScriptToAddress(net, chain, fmt.Sprintf("%x", output.PkScript))
}

func takeInputAddress(net, chain string, builder *PsbtBuilder, index int) (string, error) {
	output, err := takeInputTxOut(builder, index)
	if err != nil {
		return "", err
	}
	return takeOutputAddress(net, chain, output)
}

func validateUniqueInputs(builder *PsbtBuilder) error {
	seen := make(map[string]struct{}, len(builder.GetInputs()))
	for _, input := range builder.GetInputs() {
		id := takeInputID(input)
		if _, ok := seen[id]; ok {
			return fmt.Errorf("duplicate transaction input: %s", id)
		}
		seen[id] = struct{}{}
	}
	return nil
}

func signatureHasSighash(signature []byte, sighash txscript.SigHashType) bool {
	return len(signature) > 1 && signature[0] == 0x30 && signature[len(signature)-1] == byte(sighash)
}

func witnessHasSighash(finalScriptWitness []byte, sighash txscript.SigHashType) bool {
	reader := bytes.NewReader(finalScriptWitness)
	count, err := wire.ReadVarInt(reader, 0)
	if err != nil || count == 0 {
		return false
	}
	for i := uint64(0); i < count; i++ {
		size, err := wire.ReadVarInt(reader, 0)
		if err != nil || size == 0 {
			return false
		}
		item := make([]byte, size)
		if _, err := reader.Read(item); err != nil {
			return false
		}
		if signatureHasSighash(item, sighash) || ((len(item) == 64 || len(item) == 65) && item[len(item)-1] == byte(sighash)) {
			return true
		}
	}
	return false
}

func inputHasSighash(builder *PsbtBuilder, index int, sighash txscript.SigHashType) bool {
	input := builder.PsbtUpdater.Upsbt.Inputs[index]
	for _, partialSig := range input.PartialSigs {
		if signatureHasSighash(partialSig.Signature, sighash) {
			return true
		}
	}
	if witnessHasSighash(input.FinalScriptWitness, sighash) {
		return true
	}
	if len(input.FinalScriptSig) > 0 {
		pushes, err := txscript.PushedData(input.FinalScriptSig)
		if err == nil {
			for _, push := range pushes {
				if signatureHasSighash(push, sighash) {
					return true
				}
			}
		}
	}
	if len(input.TaprootKeySpendSig) > 0 && input.TaprootKeySpendSig[len(input.TaprootKeySpendSig)-1] == byte(sighash) {
		return true
	}
	for _, sig := range input.TaprootScriptSpendSig {
		if len(sig.Signature) > 0 && sig.Signature[len(sig.Signature)-1] == byte(sighash) {
			return true
		}
	}
	return false
}

func validateInputSighash(builder *PsbtBuilder, index int, sighash txscript.SigHashType) error {
	input := builder.PsbtUpdater.Upsbt.Inputs[index]
	if input.SighashType == sighash || (input.SighashType == 0 && inputHasSighash(builder, index, sighash)) {
		return nil
	}
	return fmt.Errorf("unexpected sighash at input %d: %d, want %d", index, input.SighashType, sighash)
}

func validateExpectedInput(builder *PsbtBuilder, index int, expectedID string, sighash txscript.SigHashType) error {
	if index < 0 || index >= len(builder.GetInputs()) {
		return fmt.Errorf("missing transaction input %d", index)
	}
	if got := takeInputID(builder.GetInputs()[index]); got != expectedID {
		return fmt.Errorf("unexpected input %d: %s, want %s", index, got, expectedID)
	}
	return validateInputSighash(builder, index, sighash)
}

func validateExpectedOutput(net, chain string, builder *PsbtBuilder, index int, address string, value int64) error {
	if index < 0 || index >= len(builder.GetOutputs()) {
		return fmt.Errorf("missing transaction output %d", index)
	}
	output := builder.GetOutputs()[index]
	if output.Value != value {
		return fmt.Errorf("unexpected value at output %d: %d, want %d", index, output.Value, value)
	}
	got, err := takeOutputAddress(net, chain, output)
	if err != nil {
		return err
	}
	if got != address {
		return fmt.Errorf("unexpected address at output %d: %s, want %s", index, got, address)
	}
	return nil
}

func validateDummyInput(net, chain string, builder *PsbtBuilder, index int, dummy *models.MarketUtxoModel, expectedType models.UtxoType) error {
	if index < 0 || index >= len(builder.GetInputs()) {
		return fmt.Errorf("missing dummy input %d", index)
	}
	if dummy == nil {
		return fmt.Errorf("dummy UTXO is not registered at input %d", index)
	}
	if dummy.UtxoId != takeInputID(builder.GetInputs()[index]) {
		return errors.New("dummy UTXO record does not match input")
	}
	if dummy.UsedState != models.UsedNo || dummy.UtxoType != expectedType {
		return errors.New("dummy UTXO is unavailable or has the wrong type")
	}
	_, platformAddress := GetPlatformKeyAndAddressForDummyAsk(chain)
	if dummy.Address != platformAddress || (dummy.Chain != "" && dummy.Chain != chain) {
		return errors.New("dummy UTXO belongs to an unexpected address or chain")
	}
	address, err := takeInputAddress(net, chain, builder, index)
	if err != nil {
		return err
	}
	if address != dummy.Address {
		return errors.New("dummy input previous output address does not match")
	}
	output, err := takeInputTxOut(builder, index)
	if err != nil {
		return err
	}
	if output.Value != int64(dummy.Amount) {
		return errors.New("dummy input value does not match its record")
	}
	return nil
}

func validateBuyerInputs(net, chain string, builder *PsbtBuilder, first int, publicKey string) (string, error) {
	if first >= len(builder.GetInputs()) {
		return "", errors.New("buyer payment input is missing")
	}
	buyerAddress, err := takeInputAddress(net, chain, builder, first)
	if err != nil {
		return "", err
	}
	verified, err := CheckPublicKeyAddress(GetNetParams(net, chain), publicKey, buyerAddress)
	if err != nil || !verified {
		return "", errors.New("buyer address does not belong to the authenticated public key")
	}
	for i := first; i < len(builder.GetInputs()); i++ {
		address, err := takeInputAddress(net, chain, builder, i)
		if err != nil {
			return "", err
		}
		if address != buyerAddress {
			return "", errors.New("all buyer payment inputs must belong to the authenticated address")
		}
		if builder.PsbtUpdater.Upsbt.Inputs[i].SighashType != txscript.SigHashAll {
			return "", fmt.Errorf("buyer input %d must use SIGHASH_ALL", i)
		}
		input := builder.GetInputs()[i]
		spent, _ := checkUtxoSpendForValidation(net, chain, input.PreviousOutPoint.Hash.String(), int64(input.PreviousOutPoint.Index))
		if spent {
			return "", fmt.Errorf("buyer input is already spent: %s", takeInputID(input))
		}
	}
	return buyerAddress, nil
}

func validateChangeAndFees(net, chain string, builder *PsbtBuilder, buyerAddress string, baseOutputs int) error {
	if len(builder.GetOutputs()) == baseOutputs+1 {
		last := len(builder.GetOutputs()) - 1
		address, err := takeOutputAddress(net, chain, builder.GetOutputs()[last])
		if err != nil {
			return err
		}
		if address != buyerAddress {
			return errors.New("the only optional output must be buyer change")
		}
	} else if len(builder.GetOutputs()) != baseOutputs {
		return fmt.Errorf("unexpected output count: %d, want %d plus optional buyer change", len(builder.GetOutputs()), baseOutputs)
	}
	var inputSum, outputSum int64
	for i := range builder.GetInputs() {
		output, err := takeInputTxOut(builder, i)
		if err != nil {
			return err
		}
		inputSum += output.Value
	}
	for i, output := range builder.GetOutputs() {
		if output.Value <= 0 {
			return errors.New("transaction contains a non-positive output")
		}
		dustLimit := MinScriptDustLimit(net, chain, output.PkScript)
		if output.Value < dustLimit {
			return fmt.Errorf("transaction output %d value %d is below dust limit %d", i, output.Value, dustLimit)
		}
		outputSum += output.Value
	}
	if outputSum > inputSum {
		return errors.New("transaction outputs exceed inputs")
	}
	return nil
}

func adjustedAssetOutputValue(net, chain, buyerAddress string, value int64) (int64, error) {
	if value <= 0 {
		return 0, errors.New("asset output value is invalid")
	}
	isTaproot, err := CheckTaprootAddressType(net, chain, buyerAddress)
	if err != nil {
		return 0, err
	}
	limit := int64(546)
	if isTaproot {
		limit = 330
	}
	if strings.EqualFold(chain, "doge") {
		limit = 1000000
	}
	if value < limit {
		value = limit
	}
	return value, nil
}

func ValidateMrc20TakePsbt(net, chain string, entity *models.MarketMrc20OrderModel, builder *PsbtBuilder, publicKey string, dummyUtxos map[string]*models.MarketUtxoModel) (string, error) {
	if entity == nil || builder == nil {
		return "", errors.New("order or PSBT is missing")
	}
	if len(builder.GetInputs()) < 3 {
		return "", errors.New("not enough transaction inputs")
	}
	if err := validateUniqueInputs(builder); err != nil {
		return "", err
	}
	if err := validateExpectedInput(builder, 1, entity.UtxoId, txscript.SigHashSingle|txscript.SigHashAnyOneCanPay); err != nil {
		return "", err
	}
	if err := validateDummyInput(net, chain, builder, 0, dummyUtxos[takeInputID(builder.GetInputs()[0])], models.UtxoTypeDummy600); err != nil {
		return "", err
	}
	buyerAddress, err := validateBuyerInputs(net, chain, builder, 2, publicKey)
	if err != nil {
		return "", err
	}
	assetValue, err := adjustedAssetOutputValue(net, chain, buyerAddress, entity.OutValue)
	if err != nil {
		return "", err
	}
	if err := validateExpectedOutput(net, chain, builder, 0, buyerAddress, assetValue); err != nil {
		return "", err
	}
	sellerDust := MinOutputDustLimit(net, chain, entity.SellerAddress)
	if entity.PriceAmount < sellerDust {
		return "", fmt.Errorf("seller payout %d is below dust limit %d", entity.PriceAmount, sellerDust)
	}
	if err := validateExpectedOutput(net, chain, builder, 1, entity.SellerAddress, entity.PriceAmount); err != nil {
		return "", err
	}
	_, platformDummyAddress := GetPlatformKeyAndAddressForDummyAsk(chain)
	dummyValue := int64(600)
	feeLimit := int64(546)
	if chain == "doge" {
		dummyValue = 1000000
		feeLimit = 1000000
	}
	if err := validateExpectedOutput(net, chain, builder, 2, platformDummyAddress, dummyValue); err != nil {
		return "", err
	}
	baseOutputs := 3
	feeAmount, _, _, feeAddress := GetPlatformMrc20TradeServiceFee(chain, entity.PriceAmount)
	if feeAmount >= feeLimit {
		if err := validateExpectedOutput(net, chain, builder, 3, feeAddress, feeAmount); err != nil {
			return "", err
		}
		baseOutputs++
	}
	if err := validateChangeAndFees(net, chain, builder, buyerAddress, baseOutputs); err != nil {
		return "", err
	}
	return buyerAddress, nil
}

func ValidatePinTakePsbt(net, chain string, entity *models.MarketOrderModel, builder *PsbtBuilder, publicKey string, dummyUtxos map[string]*models.MarketUtxoModel) (string, error) {
	if entity == nil || builder == nil {
		return "", errors.New("order or PSBT is missing")
	}
	if len(builder.GetInputs()) < 5 {
		return "", errors.New("not enough transaction inputs")
	}
	if err := validateUniqueInputs(builder); err != nil {
		return "", err
	}
	if err := validateExpectedInput(builder, 2, entity.UtxoId, txscript.SigHashSingle|txscript.SigHashAnyOneCanPay); err != nil {
		return "", err
	}
	for _, i := range []int{0, 1, 3} {
		type_ := models.UtxoTypeDummy600
		if i == 3 {
			type_ = models.UtxoTypeDummy1200
		}
		if err := validateDummyInput(net, chain, builder, i, dummyUtxos[takeInputID(builder.GetInputs()[i])], type_); err != nil {
			return "", err
		}
	}
	buyerAddress, err := validateBuyerInputs(net, chain, builder, 4, publicKey)
	if err != nil {
		return "", err
	}
	_, platformDummyAddress := GetPlatformKeyAndAddressForDummyAsk(chain)
	dummy0 := dummyUtxos[takeInputID(builder.GetInputs()[0])]
	dummy1 := dummyUtxos[takeInputID(builder.GetInputs()[1])]
	if err := validateExpectedOutput(net, chain, builder, 0, platformDummyAddress, int64(dummy0.Amount+dummy1.Amount)); err != nil {
		return "", err
	}
	assetValue, err := adjustedAssetOutputValue(net, chain, buyerAddress, entity.OutValue)
	if err != nil {
		return "", err
	}
	if err := validateExpectedOutput(net, chain, builder, 1, buyerAddress, assetValue); err != nil {
		return "", err
	}
	sellerDust := MinOutputDustLimit(net, chain, entity.SellerAddress)
	if entity.SellPriceAmount < sellerDust {
		return "", fmt.Errorf("seller payout %d is below dust limit %d", entity.SellPriceAmount, sellerDust)
	}
	if err := validateExpectedOutput(net, chain, builder, 2, entity.SellerAddress, entity.SellPriceAmount); err != nil {
		return "", err
	}
	dummyValue := int64(600)
	feeLimit := int64(546)
	if chain == "doge" {
		dummyValue = 1000000
		feeLimit = 1000000
	}
	for _, i := range []int{3, 4} {
		if err := validateExpectedOutput(net, chain, builder, i, platformDummyAddress, dummyValue); err != nil {
			return "", err
		}
	}
	baseOutputs := 5
	feeAmount, _, _, feeAddress := GetPlatformPinTradeServiceFee(chain, entity.SellPriceAmount)
	if feeAmount >= feeLimit {
		if err := validateExpectedOutput(net, chain, builder, 5, feeAddress, feeAmount); err != nil {
			return "", err
		}
		baseOutputs++
	}
	if err := validateChangeAndFees(net, chain, builder, buyerAddress, baseOutputs); err != nil {
		return "", err
	}
	return buyerAddress, nil
}

func ValidateBatchPinTakePsbt(net, chain string, entities []*models.MarketOrderModel, builder *PsbtBuilder, publicKey string, dummyUtxos map[string]*models.MarketUtxoModel) (string, error) {
	if len(entities) == 0 || builder == nil {
		return "", errors.New("orders or PSBT is missing")
	}
	n := len(entities)
	if len(builder.GetInputs()) < n+4 {
		return "", errors.New("not enough transaction inputs")
	}
	if err := validateUniqueInputs(builder); err != nil {
		return "", err
	}
	var totalAssetValue int64
	for i, entity := range entities {
		if entity == nil || entity.Chain != chain {
			return "", errors.New("batch orders must use one chain")
		}
		if err := validateExpectedInput(builder, 2+i, entity.UtxoId, txscript.SigHashSingle|txscript.SigHashAnyOneCanPay); err != nil {
			return "", err
		}
		totalAssetValue += entity.OutValue
	}
	for _, i := range []int{0, 1, n + 2} {
		type_ := models.UtxoTypeDummy600
		if i == n+2 {
			type_ = models.UtxoTypeDummy1200
		}
		if err := validateDummyInput(net, chain, builder, i, dummyUtxos[takeInputID(builder.GetInputs()[i])], type_); err != nil {
			return "", err
		}
	}
	buyerAddress, err := validateBuyerInputs(net, chain, builder, n+3, publicKey)
	if err != nil {
		return "", err
	}
	_, platformDummyAddress := GetPlatformKeyAndAddressForDummyAsk(chain)
	dummy0 := dummyUtxos[takeInputID(builder.GetInputs()[0])]
	dummy1 := dummyUtxos[takeInputID(builder.GetInputs()[1])]
	if err := validateExpectedOutput(net, chain, builder, 0, platformDummyAddress, int64(dummy0.Amount+dummy1.Amount)); err != nil {
		return "", err
	}
	assetValue, err := adjustedAssetOutputValue(net, chain, buyerAddress, totalAssetValue)
	if err != nil {
		return "", err
	}
	if err := validateExpectedOutput(net, chain, builder, 1, buyerAddress, assetValue); err != nil {
		return "", err
	}
	for i, entity := range entities {
		sellerDust := MinOutputDustLimit(net, chain, entity.SellerAddress)
		if entity.SellPriceAmount < sellerDust {
			return "", fmt.Errorf("seller payout %d is below dust limit %d", entity.SellPriceAmount, sellerDust)
		}
		if err := validateExpectedOutput(net, chain, builder, 2+i, entity.SellerAddress, entity.SellPriceAmount); err != nil {
			return "", err
		}
	}
	dummyValue := int64(600)
	if chain == "doge" {
		dummyValue = 1000000
	}
	for _, i := range []int{n + 2, n + 3} {
		if err := validateExpectedOutput(net, chain, builder, i, platformDummyAddress, dummyValue); err != nil {
			return "", err
		}
	}
	if err := validateChangeAndFees(net, chain, builder, buyerAddress, n+4); err != nil {
		return "", err
	}
	return buyerAddress, nil
}

func ValidateMakerAssetInput(net, chain string, builder *PsbtBuilder, index int, sellerAddress string) error {
	if builder == nil || index < 0 || index >= len(builder.PsbtUpdater.Upsbt.Inputs) {
		return errors.New("maker asset input is missing")
	}
	input := builder.PsbtUpdater.Upsbt.Inputs[index]
	if err := validateInputSighash(builder, index, txscript.SigHashSingle|txscript.SigHashAnyOneCanPay); err != nil {
		return err
	}
	if len(input.PartialSigs) == 0 && len(input.FinalScriptWitness) == 0 && len(input.FinalScriptSig) == 0 && len(input.TaprootKeySpendSig) == 0 && len(input.TaprootScriptSpendSig) == 0 {
		return errors.New("maker asset input is not signed")
	}
	address, err := takeInputAddress(net, chain, builder, index)
	if err != nil {
		return err
	}
	if address != sellerAddress {
		return errors.New("maker asset input does not belong to the seller")
	}
	return nil
}
