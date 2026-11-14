package common

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"

	"github.com/btcsuite/btcd/wire"
)

func ValidateCommitOutput(net, chain string, commitTx *wire.MsgTx, outputIndex int64, revealAddress string, revealAmount int64) error {
	if commitTx == nil || len(commitTx.TxIn) == 0 {
		return errors.New("commit transaction has no inputs")
	}
	if revealAmount <= 0 {
		return errors.New("reveal amount must be positive")
	}
	if outputIndex < 0 || outputIndex >= int64(len(commitTx.TxOut)) {
		return errors.New("commit output index is out of range")
	}
	output := commitTx.TxOut[outputIndex]
	if output.Value != revealAmount {
		return fmt.Errorf("commit output value is %d, want %d", output.Value, revealAmount)
	}
	address, err := PkScriptToAddress(net, chain, fmt.Sprintf("%x", output.PkScript))
	if err != nil {
		return err
	}
	if address != revealAddress {
		return fmt.Errorf("commit output address is %s, want %s", address, revealAddress)
	}
	return nil
}

func ValidateRevealPsbtTemplate(net, chain, storedRaw, clientRaw string, commitTx *wire.MsgTx, commitOutIndex, revealInputIndex int64, revealAddress string, revealAmount int64) error {
	if err := ValidateCommitOutput(net, chain, commitTx, commitOutIndex, revealAddress, revealAmount); err != nil {
		return err
	}
	stored, err := NewPsbtBuilder(GetNetParams(net, chain), storedRaw)
	if err != nil {
		return fmt.Errorf("stored reveal PSBT is invalid: %w", err)
	}
	client, err := NewPsbtBuilder(GetNetParams(net, chain), clientRaw)
	if err != nil {
		return fmt.Errorf("client reveal PSBT is invalid: %w", err)
	}
	if len(stored.GetInputs()) != len(client.GetInputs()) || len(stored.GetOutputs()) != len(client.GetOutputs()) {
		return errors.New("reveal PSBT input or output count differs from the order template")
	}
	if revealInputIndex < 0 || revealInputIndex >= int64(len(client.GetInputs())) {
		return errors.New("reveal input index is out of range")
	}
	commitHash := commitTx.TxHash()
	for i, input := range client.GetInputs() {
		storedInput := stored.GetInputs()[i]
		if int64(i) == revealInputIndex {
			if input.PreviousOutPoint.Hash != commitHash || input.PreviousOutPoint.Index != uint32(commitOutIndex) {
				return errors.New("reveal input does not spend the expected commit output")
			}
		} else if input.PreviousOutPoint != storedInput.PreviousOutPoint {
			return fmt.Errorf("reveal input %d differs from the order template", i)
		}
		clientPrev, err := takeInputTxOut(client, i)
		if err != nil {
			return err
		}
		storedPrev, err := takeInputTxOut(stored, i)
		if err != nil {
			return err
		}
		if clientPrev.Value != storedPrev.Value || !bytes.Equal(clientPrev.PkScript, storedPrev.PkScript) {
			return fmt.Errorf("reveal input %d previous output differs from the order template", i)
		}
		clientMeta := client.PsbtUpdater.Upsbt.Inputs[i]
		storedMeta := stored.PsbtUpdater.Upsbt.Inputs[i]
		if clientMeta.SighashType != storedMeta.SighashType ||
			!bytes.Equal(clientMeta.RedeemScript, storedMeta.RedeemScript) ||
			!bytes.Equal(clientMeta.WitnessScript, storedMeta.WitnessScript) ||
			!bytes.Equal(clientMeta.TaprootInternalKey, storedMeta.TaprootInternalKey) ||
			!reflect.DeepEqual(clientMeta.TaprootLeafScript, storedMeta.TaprootLeafScript) {
			return fmt.Errorf("reveal input %d signing metadata differs from the order template", i)
		}
	}
	for i, output := range client.GetOutputs() {
		storedOutput := stored.GetOutputs()[i]
		if output.Value != storedOutput.Value || !bytes.Equal(output.PkScript, storedOutput.PkScript) {
			return fmt.Errorf("reveal output %d differs from the order template", i)
		}
	}
	return nil
}
