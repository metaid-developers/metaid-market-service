package common

import (
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"metaid-market-service/models"
)

func BatchMakeAskTakerPsbtRaw(net string, psbtRawList []string, preOutValueList []int64, buyerAddress string, buyerChangeAmount uint64, feeAmountForPlatform int64, isUnSign bool) (string, int64, error) {
	var (
		takerPsbtRaw string           = ""
		netParams    *chaincfg.Params = GetNetParams(net)

		askBuilderList                                      []*PsbtBuilder = make([]*PsbtBuilder, 0)
		builder                                             *PsbtBuilder
		utxoDummy1200List                                   []*models.MarketUtxoModel
		utxoDummyList                                       []*models.MarketUtxoModel
		err                                                 error
		platformPrivateKeyDummyAsk, platformAddressDummyAsk string = GetPlatformKeyAndAddressForDummyAsk()
		//feeAmountForPlatform                                int64  = GenerateAskTakerPlatformFeeOrderV2(orderId, buyerAddress)
		//feeAmountForPlatform         int64  = 2000
		_, platformAddressReceiveFee string = GetPlatformKeyAndAddressForReceiveFee(net)

		totalBuyerReceiveOutValue uint64 = 0
		dummyOutValue                    = uint64(0)
		newDummyOutValue          uint64 = 600
		offsetDummy1200InputIndex int    = 2
	)

	if len(psbtRawList) != len(preOutValueList) {
		return "", feeAmountForPlatform, errors.New("Wrong ask Psbt: psbtRawList and preOutValueList length not equal. ")
	}

	for i, psbtRaw := range psbtRawList {
		askBuilder, err := NewPsbtBuilder(netParams, psbtRaw)
		if err != nil {
			return "", feeAmountForPlatform, err
		}
		askPreOutList := askBuilder.GetInputs()
		if askPreOutList == nil || len(askPreOutList) == 0 {
			return "", feeAmountForPlatform, errors.New("Wrong ask Psbt: empty inputs in brc20 psbt. ")
		}
		askOutputList := askBuilder.GetOutputs()
		if askOutputList == nil || len(askOutputList) == 0 {
			return "", feeAmountForPlatform, errors.New("Wrong ask Psbt: empty outputs in brc20 psbt. ")
		}
		if len(askOutputList) != len(askPreOutList) && len(askOutputList) != 1 {
			return "", feeAmountForPlatform, errors.New("Wrong ask Psbt: outputs length not equal inputs length. ")
		}

		totalBuyerReceiveOutValue += uint64(preOutValueList[i])
		askBuilderList = append(askBuilderList, askBuilder)
	}

	//find dummy utxo
	utxoDummyList, err = GetUnoccupiedUtxoList(2, models.UtxoTypeDummy600)
	defer ReleaseUtxoList(utxoDummyList)
	if err != nil {
		return "", feeAmountForPlatform, err
	}
	utxoDummy1200List, err = GetUnoccupiedUtxoList(1, models.UtxoTypeDummy1200)
	defer ReleaseUtxoList(utxoDummy1200List)
	if err != nil {
		return "", feeAmountForPlatform, err
	}

	inputs := make([]Input, 0)
	//add dummy input: 0,1
	for _, dummy := range utxoDummyList {
		inputs = append(inputs, Input{
			OutTxId:  dummy.TxId,
			OutIndex: uint32(dummy.Index),
		})
		dummyOutValue = dummyOutValue + dummy.Amount
	}

	//add pin input: 2 - 2+n
	for _, askBuilder := range askBuilderList {
		offsetDummy1200InputIndex += 1
		askPreOutList := askBuilder.GetInputs()
		askInput := askPreOutList[0]
		inputs = append(inputs, Input{
			OutTxId:  askInput.PreviousOutPoint.Hash.String(),
			OutIndex: uint32(askInput.PreviousOutPoint.Index),
		})
	}

	//add dummy1200 input: 3
	for _, dummy := range utxoDummy1200List {
		inputs = append(inputs, Input{
			OutTxId:  dummy.TxId,
			OutIndex: uint32(dummy.Index),
		})
	}

	outputs := make([]Output, 0)
	// add dummy1200 output: 0
	outputs = append(outputs, Output{
		Address: platformAddressDummyAsk,
		Amount:  dummyOutValue,
	})

	// add buyer receive pin output: 1
	isTaproot, err := CheckTaprootAddressType(net, buyerAddress)
	if err != nil {
		return "", feeAmountForPlatform, err
	}
	if totalBuyerReceiveOutValue < 546 && !isTaproot {
		totalBuyerReceiveOutValue = 546
	}
	outputs = append(outputs, Output{
		Address: buyerAddress,
		Amount:  uint64(totalBuyerReceiveOutValue),
	})

	// add receive btc output: 2 - 2+n
	for _, askBuilder := range askBuilderList {
		askOutputList := askBuilder.GetOutputs()
		askOutput := askOutputList[0]
		outputs = append(outputs, Output{
			Amount: uint64(askOutput.Value),
			Script: hex.EncodeToString(askOutput.PkScript),
		})
	}

	// add dummy output: 3,4
	dummyOut600 := Output{
		Address: platformAddressDummyAsk,
		Amount:  newDummyOutValue,
	}
	outputs = append(outputs, dummyOut600)
	outputs = append(outputs, dummyOut600)

	if feeAmountForPlatform > 546 {
		// add fee output: 5
		feeOut := Output{
			Address: platformAddressReceiveFee,
			Amount:  uint64(feeAmountForPlatform),
		}
		outputs = append(outputs, feeOut)
	}

	// add change output: 6
	if buyerChangeAmount > 0 {
		changeOut := Output{
			Address: buyerAddress,
			Amount:  buyerChangeAmount,
		}
		outputs = append(outputs, changeOut)
	}

	inputSigns := make([]*InputSign, 0)
	platformDummyPkScript, err := AddressToPkScript(net, platformAddressDummyAsk)
	if err != nil {
		return "", feeAmountForPlatform, errors.New("AddressToPkScript err: " + err.Error())
	}
	//add dummy inputSign: 0,1
	inputSigns = append(inputSigns, &InputSign{
		Index:       0,
		OutRaw:      "",
		PkScript:    platformDummyPkScript,
		SighashType: txscript.SigHashAll,
		PriHex:      platformPrivateKeyDummyAsk,
		UtxoType:    Witness,
		Amount:      600,
	})
	inputSigns = append(inputSigns, &InputSign{
		Index:       1,
		OutRaw:      "",
		PkScript:    platformDummyPkScript,
		SighashType: txscript.SigHashAll,
		PriHex:      platformPrivateKeyDummyAsk,
		UtxoType:    Witness,
		Amount:      600,
	})

	inputSigns = append(inputSigns, &InputSign{
		Index:       offsetDummy1200InputIndex,
		OutRaw:      "",
		PkScript:    platformDummyPkScript,
		SighashType: txscript.SigHashAll,
		PriHex:      platformPrivateKeyDummyAsk,
		UtxoType:    Witness,
		Amount:      1200,
	})

	builder, err = CreatePsbtBuilder(netParams, inputs, outputs)
	if err != nil {
		return "", feeAmountForPlatform, err
	}

	for i, askBuilder := range askBuilderList {
		//askPreOutList := askBuilder.GetInputs()
		//askInput := askPreOutList[0]

		if askBuilder.PsbtUpdater.Upsbt.Inputs[0].NonWitnessUtxo != nil {
			finalSigScript := askBuilder.PsbtUpdater.Upsbt.Inputs[0].FinalScriptSig
			nonWitnessUtxo := askBuilder.PsbtUpdater.Upsbt.Inputs[0].NonWitnessUtxo
			//witnessUtxo := askBuilder.PsbtUpdater.Upsbt.Inputs[0].WitnessUtxo
			partialSigs := askBuilder.PsbtUpdater.Upsbt.Inputs[0].PartialSigs
			sighashType := askBuilder.PsbtUpdater.Upsbt.Inputs[0].SighashType
			err = builder.AddSigInForNonWitnessUtxo(nonWitnessUtxo, partialSigs, sighashType, finalSigScript, 2+i)
			if err != nil {
				return "", feeAmountForPlatform, errors.New(fmt.Sprintf("PSBT(Ask): AddPartialSigIn err:%s", err.Error()))
			}
		} else {
			finalScriptWitness := askBuilder.PsbtUpdater.Upsbt.Inputs[0].FinalScriptWitness
			witnessUtxo := askBuilder.PsbtUpdater.Upsbt.Inputs[0].WitnessUtxo
			finalScriptSig := askBuilder.PsbtUpdater.Upsbt.Inputs[0].FinalScriptSig
			sighashType := askBuilder.PsbtUpdater.Upsbt.Inputs[0].SighashType
			err = builder.AddSigIn(witnessUtxo, sighashType, finalScriptWitness, finalScriptSig, 2+i)
			if err != nil {
				return "", feeAmountForPlatform, errors.New(fmt.Sprintf("PSBT(Ask): AddPartialSigIn err:%s", err.Error()))
			}

		}
	}

	if !isUnSign {
		err = builder.UpdateAndSignInput(inputSigns)
		//err = builder.UpdateAndSignInputNoFinalize(inputSigns)
		if err != nil {
			return "", feeAmountForPlatform, err
		}
	} else {
		err = builder.UpdateAndAddInputWitness(inputSigns)
		if err != nil {
			return "", feeAmountForPlatform, err
		}
	}

	takerPsbtRaw, err = builder.ToString()
	if err != nil {
		return "", feeAmountForPlatform, err
	}
	return takerPsbtRaw, feeAmountForPlatform, nil
}

func BatchSignAskTakerPsbtRawInDummy(net string, unSignDummyAskPsbtBuilder *PsbtBuilder, askOrderInputCount int) error {
	var (
		platformPrivateKeyDummyAsk, platformAddressDummyAsk string       = GetPlatformKeyAndAddressForDummyAsk()
		inputSigns                                          []*InputSign = make([]*InputSign, 0)
		offsetDummy1200InputIndex                           int          = 2
	)
	offsetDummy1200InputIndex = offsetDummy1200InputIndex + askOrderInputCount
	platformDummyPkScript, err := AddressToPkScript(net, platformAddressDummyAsk)
	if err != nil {
		return errors.New("AddressToPkScript err: " + err.Error())
	}
	//add dummy inputSign: 0,1
	inputSigns = append(inputSigns, &InputSign{
		Index:       0,
		OutRaw:      "",
		PkScript:    platformDummyPkScript,
		SighashType: txscript.SigHashAll,
		PriHex:      platformPrivateKeyDummyAsk,
		UtxoType:    Witness,
		Amount:      600,
	})
	inputSigns = append(inputSigns, &InputSign{
		Index:       1,
		OutRaw:      "",
		PkScript:    platformDummyPkScript,
		SighashType: txscript.SigHashAll,
		PriHex:      platformPrivateKeyDummyAsk,
		UtxoType:    Witness,
		Amount:      600,
	})

	inputSigns = append(inputSigns, &InputSign{
		Index:       offsetDummy1200InputIndex,
		OutRaw:      "",
		PkScript:    platformDummyPkScript,
		SighashType: txscript.SigHashAll,
		PriHex:      platformPrivateKeyDummyAsk,
		UtxoType:    Witness,
		Amount:      1200,
	})
	err = unSignDummyAskPsbtBuilder.UpdateAndSignInput(inputSigns)
	if err != nil {
		return err
	}
	return nil
}

func MakeAskTakerPsbtRaw(net, orderId, psbtRaw string, preOutValue int64, buyerAddress string, buyerChangeAmount uint64, feeAmountForPlatform int64, isUnSign bool) (string, int64, error) {
	var (
		takerPsbtRaw                                        string           = ""
		netParams                                           *chaincfg.Params = GetNetParams(net)
		askBuilder                                          *PsbtBuilder
		builder                                             *PsbtBuilder
		utxoDummy1200List                                   []*models.MarketUtxoModel
		utxoDummyList                                       []*models.MarketUtxoModel
		err                                                 error
		platformPrivateKeyDummyAsk, platformAddressDummyAsk string = GetPlatformKeyAndAddressForDummyAsk()
		//feeAmountForPlatform                                int64  = GenerateAskTakerPlatformFeeOrderV2(orderId, buyerAddress)
		//feeAmountForPlatform         int64  = 2000
		_, platformAddressReceiveFee string = GetPlatformKeyAndAddressForReceiveFee(net)

		dummyOutValue           = uint64(0)
		newDummyOutValue uint64 = 600
	)

	askBuilder, err = NewPsbtBuilder(netParams, psbtRaw)
	if err != nil {
		return "", feeAmountForPlatform, err
	}
	askPreOutList := askBuilder.GetInputs()
	if askPreOutList == nil || len(askPreOutList) == 0 {
		return "", feeAmountForPlatform, errors.New("Wrong ask Psbt: empty inputs in brc20 psbt. ")
	}
	askInput := askPreOutList[0]
	askOutputList := askBuilder.GetOutputs()
	if askOutputList == nil || len(askOutputList) == 0 {
		return "", feeAmountForPlatform, errors.New("Wrong ask Psbt: empty outputs in brc20 psbt. ")
	}
	askOutput := askOutputList[0]

	//find dummy utxo
	utxoDummyList, err = GetUnoccupiedUtxoList(2, models.UtxoTypeDummy600)
	defer ReleaseUtxoList(utxoDummyList)
	if err != nil {
		return "", feeAmountForPlatform, err
	}
	utxoDummy1200List, err = GetUnoccupiedUtxoList(1, models.UtxoTypeDummy1200)
	defer ReleaseUtxoList(utxoDummy1200List)
	if err != nil {
		return "", feeAmountForPlatform, err
	}

	inputs := make([]Input, 0)
	//add dummy input: 0,1
	for _, dummy := range utxoDummyList {
		inputs = append(inputs, Input{
			OutTxId:  dummy.TxId,
			OutIndex: uint32(dummy.Index),
		})
		dummyOutValue = dummyOutValue + dummy.Amount
	}

	//add brc20 input: 2
	inputs = append(inputs, Input{
		OutTxId:  askInput.PreviousOutPoint.Hash.String(),
		OutIndex: uint32(askInput.PreviousOutPoint.Index),
	})

	//add dummy1200 input: 3
	for _, dummy := range utxoDummy1200List {
		inputs = append(inputs, Input{
			OutTxId:  dummy.TxId,
			OutIndex: uint32(dummy.Index),
		})
	}

	outputs := make([]Output, 0)
	// add dummy1200 output: 0
	outputs = append(outputs, Output{
		Address: platformAddressDummyAsk,
		Amount:  dummyOutValue,
	})

	// add buyer receive brc20 output: 1
	isTaproot, err := CheckTaprootAddressType(net, buyerAddress)
	if err != nil {
		return "", feeAmountForPlatform, err
	}
	if preOutValue < 546 && !isTaproot {
		preOutValue = 546
	}
	outputs = append(outputs, Output{
		Address: buyerAddress,
		Amount:  uint64(preOutValue),
	})

	// add receive btc output: 2
	outputs = append(outputs, Output{
		Amount: uint64(askOutput.Value),
		Script: hex.EncodeToString(askOutput.PkScript),
	})

	// add dummy output: 3,4
	dummyOut600 := Output{
		Address: platformAddressDummyAsk,
		Amount:  newDummyOutValue,
	}
	outputs = append(outputs, dummyOut600)
	outputs = append(outputs, dummyOut600)

	if feeAmountForPlatform > 546 {
		// add fee output: 5
		feeOut := Output{
			Address: platformAddressReceiveFee,
			Amount:  uint64(feeAmountForPlatform),
		}
		outputs = append(outputs, feeOut)
	}

	// add change output: 6
	if buyerChangeAmount > 0 {
		changeOut := Output{
			Address: buyerAddress,
			Amount:  buyerChangeAmount,
		}
		outputs = append(outputs, changeOut)
	}

	inputSigns := make([]*InputSign, 0)
	platformDummyPkScript, err := AddressToPkScript(net, platformAddressDummyAsk)
	if err != nil {
		return "", feeAmountForPlatform, errors.New("AddressToPkScript err: " + err.Error())
	}
	//add dummy inputSign: 0,1
	inputSigns = append(inputSigns, &InputSign{
		Index:       0,
		OutRaw:      "",
		PkScript:    platformDummyPkScript,
		SighashType: txscript.SigHashAll,
		PriHex:      platformPrivateKeyDummyAsk,
		UtxoType:    Witness,
		Amount:      600,
	})
	inputSigns = append(inputSigns, &InputSign{
		Index:       1,
		OutRaw:      "",
		PkScript:    platformDummyPkScript,
		SighashType: txscript.SigHashAll,
		PriHex:      platformPrivateKeyDummyAsk,
		UtxoType:    Witness,
		Amount:      600,
	})

	inputSigns = append(inputSigns, &InputSign{
		Index:       3,
		OutRaw:      "",
		PkScript:    platformDummyPkScript,
		SighashType: txscript.SigHashAll,
		PriHex:      platformPrivateKeyDummyAsk,
		UtxoType:    Witness,
		Amount:      1200,
	})

	builder, err = CreatePsbtBuilder(netParams, inputs, outputs)
	if err != nil {
		return "", feeAmountForPlatform, err
	}

	if askBuilder.PsbtUpdater.Upsbt.Inputs[0].NonWitnessUtxo != nil {
		//fmt.Printf("NonWitnessUtxo:%+v\n", askBuilder.PsbtUpdater.Upsbt.Inputs[0].NonWitnessUtxo)
		finalSigScript := askBuilder.PsbtUpdater.Upsbt.Inputs[0].FinalScriptSig
		nonWitnessUtxo := askBuilder.PsbtUpdater.Upsbt.Inputs[0].NonWitnessUtxo
		partialSigs := askBuilder.PsbtUpdater.Upsbt.Inputs[0].PartialSigs
		sighashType := askBuilder.PsbtUpdater.Upsbt.Inputs[0].SighashType
		err = builder.AddSigInForNonWitnessUtxo(nonWitnessUtxo, partialSigs, sighashType, finalSigScript, 2)
		if err != nil {
			return "", feeAmountForPlatform, errors.New(fmt.Sprintf("PSBT(Ask): AddPartialSigIn err:%s", err.Error()))
		}
	} else {
		finalScriptWitness := askBuilder.PsbtUpdater.Upsbt.Inputs[0].FinalScriptWitness
		finalScriptSig := askBuilder.PsbtUpdater.Upsbt.Inputs[0].FinalScriptSig
		witnessUtxo := askBuilder.PsbtUpdater.Upsbt.Inputs[0].WitnessUtxo
		//redeemScript := askBuilder.PsbtUpdater.Upsbt.Inputs[0].RedeemScript
		sighashType := askBuilder.PsbtUpdater.Upsbt.Inputs[0].SighashType
		err = builder.AddSigIn(witnessUtxo, sighashType, finalScriptWitness, finalScriptSig, 2)
		if err != nil {
			return "", feeAmountForPlatform, errors.New(fmt.Sprintf("PSBT(Ask): AddPartialSigIn err:%s", err.Error()))
		}
	}

	if !isUnSign {
		err = builder.UpdateAndSignInput(inputSigns)
		//err = builder.UpdateAndSignInputNoFinalize(inputSigns)
		if err != nil {
			return "", feeAmountForPlatform, err
		}
	} else {
		err = builder.UpdateAndAddInputWitness(inputSigns)
		if err != nil {
			return "", feeAmountForPlatform, err
		}
	}

	takerPsbtRaw, err = builder.ToString()
	if err != nil {
		return "", feeAmountForPlatform, err
	}

	return takerPsbtRaw, feeAmountForPlatform, nil
}

func SignAskTakerPsbtRawInDummy(net string, unSignDummyAskPsbtBuilder *PsbtBuilder) error {
	var (
		platformPrivateKeyDummyAsk, platformAddressDummyAsk string       = GetPlatformKeyAndAddressForDummyAsk()
		inputSigns                                          []*InputSign = make([]*InputSign, 0)
	)

	platformDummyPkScript, err := AddressToPkScript(net, platformAddressDummyAsk)
	if err != nil {
		return errors.New("AddressToPkScript err: " + err.Error())
	}
	//add dummy inputSign: 0,1
	inputSigns = append(inputSigns, &InputSign{
		Index:       0,
		OutRaw:      "",
		PkScript:    platformDummyPkScript,
		SighashType: txscript.SigHashAll,
		PriHex:      platformPrivateKeyDummyAsk,
		UtxoType:    Witness,
		Amount:      600,
	})
	inputSigns = append(inputSigns, &InputSign{
		Index:       1,
		OutRaw:      "",
		PkScript:    platformDummyPkScript,
		SighashType: txscript.SigHashAll,
		PriHex:      platformPrivateKeyDummyAsk,
		UtxoType:    Witness,
		Amount:      600,
	})

	inputSigns = append(inputSigns, &InputSign{
		Index:       3,
		OutRaw:      "",
		PkScript:    platformDummyPkScript,
		SighashType: txscript.SigHashAll,
		PriHex:      platformPrivateKeyDummyAsk,
		UtxoType:    Witness,
		Amount:      1200,
	})
	err = unSignDummyAskPsbtBuilder.UpdateAndSignInput(inputSigns)
	if err != nil {
		return err
	}
	return nil
}

func MakeAskTakerPsbtRawForPreMake(net, orderId, psbtRaw string, preOutValue int64, buyerAddress string, buyerChangeAmount uint64, feeAmountForPlatform int64, isUnSign bool) (string, int64, error) {
	var (
		takerPsbtRaw string           = ""
		netParams    *chaincfg.Params = GetNetParams(net)
		askBuilder   *PsbtBuilder
		//builder                                             *PsbtBuilder
		utxoDummy1200List                                   []*models.MarketUtxoModel
		utxoDummyList                                       []*models.MarketUtxoModel
		err                                                 error
		platformPrivateKeyDummyAsk, platformAddressDummyAsk string = GetPlatformKeyAndAddressForDummyAsk()
		//feeAmountForPlatform                                int64  = GenerateAskTakerPlatformFeeOrderV2(orderId, buyerAddress)
		//feeAmountForPlatform         int64  = 2000
		_, platformAddressReceiveFee string = GetPlatformKeyAndAddressForReceiveFee(net)

		dummyOutValue           = uint64(0)
		newDummyOutValue uint64 = 600
	)

	askBuilder, err = NewPsbtBuilder(netParams, psbtRaw)
	if err != nil {
		return "", feeAmountForPlatform, err
	}
	askUpsbtList := askBuilder.GetUpsbtInputs()
	if askUpsbtList == nil || len(askUpsbtList) == 0 {
		return "", feeAmountForPlatform, errors.New("Wrong ask Psbt: empty askUpsbtList in psbt. ")
	}
	askPreOutList := askBuilder.GetInputs()
	if askPreOutList == nil || len(askPreOutList) == 0 {
		return "", feeAmountForPlatform, errors.New("Wrong ask Psbt: empty inputs in psbt. ")
	}
	askOutputList := askBuilder.GetOutputs()
	if askOutputList == nil || len(askOutputList) == 0 {
		return "", feeAmountForPlatform, errors.New("Wrong ask Psbt: empty outputs in psbt. ")
	}

	//find dummy utxo
	utxoDummyList, err = GetUnoccupiedUtxoList(2, models.UtxoTypeDummy600)
	defer ReleaseUtxoList(utxoDummyList)
	if err != nil {
		return "", feeAmountForPlatform, err
	}
	utxoDummy1200List, err = GetUnoccupiedUtxoList(1, models.UtxoTypeDummy1200)
	defer ReleaseUtxoList(utxoDummy1200List)
	if err != nil {
		return "", feeAmountForPlatform, err
	}

	inputs := make([]Input, 0)
	//add dummy input: 0,1
	for i, dummy := range utxoDummyList {
		dummyTxHash, err := chainhash.NewHashFromStr(dummy.TxId)
		if err != nil {
			return "", 0, err
		}

		askPreOutList[i].PreviousOutPoint = *wire.NewOutPoint(dummyTxHash, uint32(dummy.Index))
		dummyOutValue = dummyOutValue + dummy.Amount

		askUpsbtList[i].WitnessUtxo = wire.NewTxOut(int64(dummy.Amount), askUpsbtList[i].WitnessUtxo.PkScript)
		//signAll
	}
	askBuilder.PsbtUpdater.Upsbt.UnsignedTx.TxIn = askPreOutList
	askBuilder.PsbtUpdater.Upsbt.Inputs = askUpsbtList

	outputs := make([]Output, 0)
	// add dummy1200 output: 0
	dummyAskPkScript, err := AddressToPkScript(net, platformAddressDummyAsk)
	if err != nil {
		return "", feeAmountForPlatform, errors.New("AddressToPkScript err: " + err.Error())
	}
	dummyAskPkScriptBytes, err := hex.DecodeString(dummyAskPkScript)
	if err != nil {
		return "", feeAmountForPlatform, errors.New("DecodeString err: " + err.Error())
	}
	askOutputList[0] = wire.NewTxOut(int64(dummyOutValue), dummyAskPkScriptBytes)

	// add buyer receive brc20 output: 1
	isTaproot, err := CheckTaprootAddressType(net, buyerAddress)
	if err != nil {
		return "", feeAmountForPlatform, err
	}
	if preOutValue < 546 && !isTaproot {
		preOutValue = 546
	}

	buyerAddressPkScript, err := AddressToPkScript(net, buyerAddress)
	if err != nil {
		return "", feeAmountForPlatform, errors.New("AddressToPkScript err: " + err.Error())
	}
	buyerAddressPkScriptBytes, err := hex.DecodeString(buyerAddressPkScript)
	if err != nil {
		return "", feeAmountForPlatform, errors.New("DecodeString err: " + err.Error())
	}
	askOutputList[1] = wire.NewTxOut(int64(preOutValue), buyerAddressPkScriptBytes)
	askBuilder.PsbtUpdater.Upsbt.UnsignedTx.TxOut = askOutputList

	//add dummy1200 input: 3
	for _, dummy := range utxoDummy1200List {
		inputs = append(inputs, Input{
			OutTxId:  dummy.TxId,
			OutIndex: uint32(dummy.Index),
		})
	}

	for _, in := range inputs {
		err = askBuilder.AddInputOnly(in)
		if err != nil {
			return "", 0, err
		}
	}

	// add dummy output: 3,4
	dummyOut600 := Output{
		Address: platformAddressDummyAsk,
		Amount:  newDummyOutValue,
	}
	outputs = append(outputs, dummyOut600)
	outputs = append(outputs, dummyOut600)

	if feeAmountForPlatform > 546 {
		// add fee output: 5
		feeOut := Output{
			Address: platformAddressReceiveFee,
			Amount:  uint64(feeAmountForPlatform),
		}
		outputs = append(outputs, feeOut)
	}

	// add change output: 6
	if buyerChangeAmount > 0 {
		changeOut := Output{
			Address: buyerAddress,
			Amount:  buyerChangeAmount,
		}
		outputs = append(outputs, changeOut)
	}

	err = askBuilder.AddOutput(outputs)
	if err != nil {
		return "", 0, err
	}

	inputSigns := make([]*InputSign, 0)
	platformDummyPkScript, err := AddressToPkScript(net, platformAddressDummyAsk)
	if err != nil {
		return "", feeAmountForPlatform, errors.New("AddressToPkScript err: " + err.Error())
	}
	//add dummy inputSign: 0,1
	//inputSigns = append(inputSigns, &InputSign{
	//	Index:       0,
	//	OutRaw:      "",
	//	PkScript:    platformDummyPkScript,
	//	SighashType: txscript.SigHashAll,
	//	PriHex:      platformPrivateKeyDummyAsk,
	//	UtxoType:    Witness,
	//	Amount:      600,
	//})
	//inputSigns = append(inputSigns, &InputSign{
	//	Index:       1,
	//	OutRaw:      "",
	//	PkScript:    platformDummyPkScript,
	//	SighashType: txscript.SigHashAll,
	//	PriHex:      platformPrivateKeyDummyAsk,
	//	UtxoType:    Witness,
	//	Amount:      600,
	//})

	inputSigns = append(inputSigns, &InputSign{
		Index:       3,
		OutRaw:      "",
		PkScript:    platformDummyPkScript,
		SighashType: txscript.SigHashAll,
		PriHex:      platformPrivateKeyDummyAsk,
		UtxoType:    Witness,
		Amount:      1200,
	})

	if !isUnSign {
		err = askBuilder.UpdateAndSignInput(inputSigns)
		//err = builder.UpdateAndSignInputNoFinalize(inputSigns)
		if err != nil {
			return "", feeAmountForPlatform, err
		}
	} else {
		err = askBuilder.UpdateAndAddInputWitness(inputSigns)
		if err != nil {
			return "", feeAmountForPlatform, err
		}
	}

	takerPsbtRaw, err = askBuilder.ToString()
	if err != nil {
		return "", feeAmountForPlatform, err
	}

	return takerPsbtRaw, feeAmountForPlatform, nil
}

// Mrc20 orders
func MakeMrc20AskTakerPsbtRaw(net, orderId, psbtRaw string, preOutValue int64, buyerAddress string, buyerChangeAmount uint64, feeAmountForPlatform int64, isUnSign bool) (string, int64, error) {
	var (
		takerPsbtRaw                                        string           = ""
		netParams                                           *chaincfg.Params = GetNetParams(net)
		askBuilder                                          *PsbtBuilder
		builder                                             *PsbtBuilder
		utxoDummyList                                       []*models.MarketUtxoModel
		err                                                 error
		platformPrivateKeyDummyAsk, platformAddressDummyAsk string = GetPlatformKeyAndAddressForDummyAsk()
		//feeAmountForPlatform                                int64  = GenerateAskTakerPlatformFeeOrderV2(orderId, buyerAddress)
		//feeAmountForPlatform         int64  = 2000
		_, platformAddressReceiveFee string = GetPlatformKeyAndAddressForReceiveFee(net)

		dummyOutValue           = uint64(0)
		newDummyOutValue uint64 = 600
	)

	askBuilder, err = NewPsbtBuilder(netParams, psbtRaw)
	if err != nil {
		return "", feeAmountForPlatform, err
	}
	askPreOutList := askBuilder.GetInputs()
	if askPreOutList == nil || len(askPreOutList) == 0 {
		return "", feeAmountForPlatform, errors.New("Wrong ask Psbt: empty inputs in brc20 psbt. ")
	}
	askInput := askPreOutList[0]
	askOutputList := askBuilder.GetOutputs()
	if askOutputList == nil || len(askOutputList) == 0 {
		return "", feeAmountForPlatform, errors.New("Wrong ask Psbt: empty outputs in brc20 psbt. ")
	}
	askOutput := askOutputList[0]

	//find dummy utxo
	utxoDummyList, err = GetUnoccupiedUtxoList(1, models.UtxoTypeDummy600)
	defer ReleaseUtxoList(utxoDummyList)
	if err != nil {
		return "", feeAmountForPlatform, err
	}

	// make inputs
	inputs := make([]Input, 0)
	//add dummy input: 0
	for _, dummy := range utxoDummyList {
		inputs = append(inputs, Input{
			OutTxId:  dummy.TxId,
			OutIndex: uint32(dummy.Index),
		})
		dummyOutValue = dummyOutValue + dummy.Amount
	}

	//add brc20 input: 1
	inputs = append(inputs, Input{
		OutTxId:  askInput.PreviousOutPoint.Hash.String(),
		OutIndex: uint32(askInput.PreviousOutPoint.Index),
	})

	// make outputs
	outputs := make([]Output, 0)
	// add buyer receive brc20 output: 0
	isTaproot, err := CheckTaprootAddressType(net, buyerAddress)
	if err != nil {
		return "", feeAmountForPlatform, err
	}
	if preOutValue < 546 && !isTaproot {
		preOutValue = 546
	}
	outputs = append(outputs, Output{
		Address: buyerAddress,
		Amount:  uint64(preOutValue),
	})

	// add receive btc output: 1
	outputs = append(outputs, Output{
		Amount: uint64(askOutput.Value),
		Script: hex.EncodeToString(askOutput.PkScript),
	})

	// add dummy output: 2
	dummyOut600 := Output{
		Address: platformAddressDummyAsk,
		Amount:  newDummyOutValue,
	}
	outputs = append(outputs, dummyOut600)

	if feeAmountForPlatform > 546 {
		// add fee output: 3
		feeOut := Output{
			Address: platformAddressReceiveFee,
			Amount:  uint64(feeAmountForPlatform),
		}
		outputs = append(outputs, feeOut)
	}

	// add change output: 4
	if buyerChangeAmount > 0 {
		changeOut := Output{
			Address: buyerAddress,
			Amount:  buyerChangeAmount,
		}
		outputs = append(outputs, changeOut)
	}

	inputSigns := make([]*InputSign, 0)
	platformDummyPkScript, err := AddressToPkScript(net, platformAddressDummyAsk)
	if err != nil {
		return "", feeAmountForPlatform, errors.New("AddressToPkScript err: " + err.Error())
	}
	//add dummy inputSign: 0
	inputSigns = append(inputSigns, &InputSign{
		Index:       0,
		OutRaw:      "",
		PkScript:    platformDummyPkScript,
		SighashType: txscript.SigHashAll,
		PriHex:      platformPrivateKeyDummyAsk,
		UtxoType:    Witness,
		Amount:      600,
	})

	builder, err = CreatePsbtBuilder(netParams, inputs, outputs)
	if err != nil {
		return "", feeAmountForPlatform, err
	}

	if askBuilder.PsbtUpdater.Upsbt.Inputs[0].NonWitnessUtxo != nil {
		//fmt.Printf("NonWitnessUtxo:%+v\n", askBuilder.PsbtUpdater.Upsbt.Inputs[0].NonWitnessUtxo)
		finalSigScript := askBuilder.PsbtUpdater.Upsbt.Inputs[0].FinalScriptSig
		nonWitnessUtxo := askBuilder.PsbtUpdater.Upsbt.Inputs[0].NonWitnessUtxo
		partialSigs := askBuilder.PsbtUpdater.Upsbt.Inputs[0].PartialSigs
		sighashType := askBuilder.PsbtUpdater.Upsbt.Inputs[0].SighashType
		err = builder.AddSigInForNonWitnessUtxo(nonWitnessUtxo, partialSigs, sighashType, finalSigScript, 1)
		if err != nil {
			return "", feeAmountForPlatform, errors.New(fmt.Sprintf("PSBT(Ask): AddPartialSigIn err:%s", err.Error()))
		}
	} else {
		finalScriptWitness := askBuilder.PsbtUpdater.Upsbt.Inputs[0].FinalScriptWitness
		finalScriptSig := askBuilder.PsbtUpdater.Upsbt.Inputs[0].FinalScriptSig
		witnessUtxo := askBuilder.PsbtUpdater.Upsbt.Inputs[0].WitnessUtxo
		//redeemScript := askBuilder.PsbtUpdater.Upsbt.Inputs[0].RedeemScript
		sighashType := askBuilder.PsbtUpdater.Upsbt.Inputs[0].SighashType
		err = builder.AddSigIn(witnessUtxo, sighashType, finalScriptWitness, finalScriptSig, 1)
		if err != nil {
			return "", feeAmountForPlatform, errors.New(fmt.Sprintf("PSBT(Ask): AddPartialSigIn err:%s", err.Error()))
		}
	}

	if !isUnSign {
		err = builder.UpdateAndSignInput(inputSigns)
		//err = builder.UpdateAndSignInputNoFinalize(inputSigns)
		if err != nil {
			return "", feeAmountForPlatform, err
		}
	} else {
		err = builder.UpdateAndAddInputWitness(inputSigns)
		if err != nil {
			return "", feeAmountForPlatform, err
		}
	}

	takerPsbtRaw, err = builder.ToString()
	if err != nil {
		return "", feeAmountForPlatform, err
	}

	return takerPsbtRaw, feeAmountForPlatform, nil
}

func SignMrc20AskTakerPsbtRawInDummy(net string, unSignDummyAskPsbtBuilder *PsbtBuilder) error {
	var (
		platformPrivateKeyDummyAsk, platformAddressDummyAsk string       = GetPlatformKeyAndAddressForDummyAsk()
		inputSigns                                          []*InputSign = make([]*InputSign, 0)
	)

	platformDummyPkScript, err := AddressToPkScript(net, platformAddressDummyAsk)
	if err != nil {
		return errors.New("AddressToPkScript err: " + err.Error())
	}
	//add dummy inputSign: 0,1
	inputSigns = append(inputSigns, &InputSign{
		Index:       0,
		OutRaw:      "",
		PkScript:    platformDummyPkScript,
		SighashType: txscript.SigHashAll,
		PriHex:      platformPrivateKeyDummyAsk,
		UtxoType:    Witness,
		Amount:      600,
	})
	err = unSignDummyAskPsbtBuilder.UpdateAndSignInput(inputSigns)
	if err != nil {
		return err
	}
	return nil
}

func MakeMrc20AskTakerPsbtRawForPreMake(net, orderId, psbtRaw string, preOutValue int64, buyerAddress string, buyerChangeAmount uint64, feeAmountForPlatform int64, isUnSign bool) (string, int64, error) {
	var (
		takerPsbtRaw string           = ""
		netParams    *chaincfg.Params = GetNetParams(net)
		askBuilder   *PsbtBuilder
		//builder                                             *PsbtBuilder
		utxoDummyList              []*models.MarketUtxoModel
		err                        error
		_, platformAddressDummyAsk string = GetPlatformKeyAndAddressForDummyAsk()
		//feeAmountForPlatform                                int64  = GenerateAskTakerPlatformFeeOrderV2(orderId, buyerAddress)
		//feeAmountForPlatform         int64  = 2000
		_, platformAddressReceiveFee string = GetPlatformKeyAndAddressForReceiveFee(net)

		dummyOutValue           = uint64(0)
		newDummyOutValue uint64 = 600
	)

	askBuilder, err = NewPsbtBuilder(netParams, psbtRaw)
	if err != nil {
		return "", feeAmountForPlatform, err
	}
	askUpsbtList := askBuilder.GetUpsbtInputs()
	if askUpsbtList == nil || len(askUpsbtList) == 0 {
		return "", feeAmountForPlatform, errors.New("Wrong ask Psbt: empty askUpsbtList in psbt. ")
	}
	askPreOutList := askBuilder.GetInputs()
	if askPreOutList == nil || len(askPreOutList) == 0 {
		return "", feeAmountForPlatform, errors.New("Wrong ask Psbt: empty inputs in psbt. ")
	}
	askOutputList := askBuilder.GetOutputs()
	if askOutputList == nil || len(askOutputList) == 0 {
		return "", feeAmountForPlatform, errors.New("Wrong ask Psbt: empty outputs in psbt. ")
	}

	//find dummy utxo
	utxoDummyList, err = GetUnoccupiedUtxoList(1, models.UtxoTypeDummy600)
	defer ReleaseUtxoList(utxoDummyList)
	if err != nil {
		return "", feeAmountForPlatform, err
	}

	//add dummy input: 0
	for i, dummy := range utxoDummyList {
		dummyTxHash, err := chainhash.NewHashFromStr(dummy.TxId)
		if err != nil {
			return "", 0, err
		}

		askPreOutList[i].PreviousOutPoint = *wire.NewOutPoint(dummyTxHash, uint32(dummy.Index))
		dummyOutValue = dummyOutValue + dummy.Amount

		askUpsbtList[i].WitnessUtxo = wire.NewTxOut(int64(dummy.Amount), askUpsbtList[i].WitnessUtxo.PkScript)
		//signAll
	}
	askBuilder.PsbtUpdater.Upsbt.UnsignedTx.TxIn = askPreOutList
	askBuilder.PsbtUpdater.Upsbt.Inputs = askUpsbtList

	outputs := make([]Output, 0)
	// add buyer receive brc20 output: 0
	isTaproot, err := CheckTaprootAddressType(net, buyerAddress)
	if err != nil {
		return "", feeAmountForPlatform, err
	}
	if preOutValue < 546 && !isTaproot {
		preOutValue = 546
	}
	buyerAddressPkScript, err := AddressToPkScript(net, buyerAddress)
	if err != nil {
		return "", feeAmountForPlatform, errors.New("AddressToPkScript err: " + err.Error())
	}
	buyerAddressPkScriptBytes, err := hex.DecodeString(buyerAddressPkScript)
	if err != nil {
		return "", feeAmountForPlatform, errors.New("DecodeString err: " + err.Error())
	}
	askOutputList[0] = wire.NewTxOut(int64(preOutValue), buyerAddressPkScriptBytes)
	askBuilder.PsbtUpdater.Upsbt.UnsignedTx.TxOut = askOutputList

	// add dummy output: 2
	dummyOut600 := Output{
		Address: platformAddressDummyAsk,
		Amount:  newDummyOutValue,
	}
	outputs = append(outputs, dummyOut600)

	if feeAmountForPlatform > 546 {
		// add fee output: 3
		feeOut := Output{
			Address: platformAddressReceiveFee,
			Amount:  uint64(feeAmountForPlatform),
		}
		outputs = append(outputs, feeOut)
	}

	// add change output: 4
	if buyerChangeAmount > 0 {
		changeOut := Output{
			Address: buyerAddress,
			Amount:  buyerChangeAmount,
		}
		outputs = append(outputs, changeOut)
	}

	err = askBuilder.AddOutput(outputs)
	if err != nil {
		return "", 0, err
	}

	takerPsbtRaw, err = askBuilder.ToString()
	if err != nil {
		return "", feeAmountForPlatform, err
	}

	return takerPsbtRaw, feeAmountForPlatform, nil
}
