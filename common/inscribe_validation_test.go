package common

import (
	"encoding/hex"
	"testing"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

func TestValidateRevealPsbtTemplate(t *testing.T) {
	_, revealAddress, revealScript := testWallet(t)
	_, recipientAddress, _ := testWallet(t)
	_, _, feeScript := testWallet(t)
	commitTx := wire.NewMsgTx(2)
	commitTx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&chainhash.Hash{}, 0), nil, nil))
	scriptBytes, _ := hex.DecodeString(revealScript)
	commitTx.AddTxOut(wire.NewTxOut(100000, scriptBytes))
	stored, err := CreatePsbtBuilder(GetNetParams("testnet", "btc"), []Input{testOutpoint(0, 0)}, []Output{{Address: recipientAddress, Amount: 546}, {Script: feeScript, Amount: 1999}})
	if err != nil {
		t.Fatal(err)
	}
	if err := stored.UpdateAndAddInputWitness([]*InputSign{testSigner(0, revealScript, 100000, txscript.SigHashAll)}); err != nil {
		t.Fatal(err)
	}
	client, err := NewPsbtBuilder(GetNetParams("testnet", "btc"), mustPsbtString(t, stored))
	if err != nil {
		t.Fatal(err)
	}
	commitHash := commitTx.TxHash()
	client.PsbtUpdater.Upsbt.UnsignedTx.TxIn[0].PreviousOutPoint = *wire.NewOutPoint(&commitHash, 0)
	if err := ValidateRevealPsbtTemplate("testnet", "btc", mustPsbtString(t, stored), mustPsbtString(t, client), commitTx, 0, 0, revealAddress, 100000); err != nil {
		t.Fatalf("valid reveal template rejected: %v", err)
	}

	t.Run("changed reveal output", func(t *testing.T) {
		tampered, err := NewPsbtBuilder(GetNetParams("testnet", "btc"), mustPsbtString(t, client))
		if err != nil {
			t.Fatal(err)
		}
		tampered.PsbtUpdater.Upsbt.UnsignedTx.TxOut[1].Value++
		if err := ValidateRevealPsbtTemplate("testnet", "btc", mustPsbtString(t, stored), mustPsbtString(t, tampered), commitTx, 0, 0, revealAddress, 100000); err == nil {
			t.Fatal("changed reveal output was accepted")
		}
	})
	t.Run("wrong commit amount", func(t *testing.T) {
		wrongCommit := commitTx.Copy()
		wrongCommit.TxOut[0].Value--
		if err := ValidateRevealPsbtTemplate("testnet", "btc", mustPsbtString(t, stored), mustPsbtString(t, client), wrongCommit, 0, 0, revealAddress, 100000); err == nil {
			t.Fatal("wrong commit amount was accepted")
		}
	})
	t.Run("wrong reveal outpoint", func(t *testing.T) {
		tampered, err := NewPsbtBuilder(GetNetParams("testnet", "btc"), mustPsbtString(t, client))
		if err != nil {
			t.Fatal(err)
		}
		tampered.PsbtUpdater.Upsbt.UnsignedTx.TxIn[0].PreviousOutPoint.Index = 1
		if err := ValidateRevealPsbtTemplate("testnet", "btc", mustPsbtString(t, stored), mustPsbtString(t, tampered), commitTx, 0, 0, revealAddress, 100000); err == nil {
			t.Fatal("wrong reveal outpoint was accepted")
		}
	})
}

func mustPsbtString(t *testing.T, builder *PsbtBuilder) string {
	t.Helper()
	raw, err := builder.ToString()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
