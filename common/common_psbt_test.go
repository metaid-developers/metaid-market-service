package common

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
)

func psbtTestPrivateKey(label string) string {
	hash := sha256.Sum256([]byte(label))
	return hex.EncodeToString(hash[:])
}

var (
	priKeyHex1 string = psbtTestPrivateKey("psbt-test-key-1")
	pkScript1  string
	txId1      string = "0b76a6ff6e96227db7b5a435df5987712e0adde75154f060942a5b019c5f4228"
	txIndex1   uint32 = 0
	txValue1   uint64 = 2000

	outAddress string = "1F4Ga3Nbjehizh5ZMfKtZ6bP3WQuVRP7Xf"
	outAmount  int64  = 4000
)

func TestBuildMvcPsbt(t *testing.T) {
	netParams := &chaincfg.MainNetParams
	privateKeyBytes, err := hex.DecodeString(priKeyHex1)
	if err != nil {
		t.Fatal(err)
	}
	privateKey, _ := btcec.PrivKeyFromBytes(privateKeyBytes)
	address, err := GetSegwitAddressFromPublicKey(netParams, hex.EncodeToString(privateKey.PubKey().SerializeCompressed()))
	if err != nil {
		t.Fatal(err)
	}
	pkScript1, err = AddressToPkScript("mainnet", "btc", address)
	if err != nil {
		t.Fatal(err)
	}
	inputs := []Input{{OutTxId: txId1, OutIndex: txIndex1}}
	inputSigns := []*InputSign{{
		Index:       0,
		PkScript:    pkScript1,
		SighashType: txscript.SigHashSingle | txscript.SigHashAnyOneCanPay,
		PriHex:      priKeyHex1,
		UtxoType:    Witness,
		Amount:      txValue1,
	}}
	outputs := []Output{{Address: outAddress, Amount: uint64(outAmount)}}
	builder, err := CreatePsbtBuilder(netParams, inputs, outputs)
	if err != nil {
		t.Fatal(err)
	}
	if err := builder.UpdateAndSignInput(inputSigns); err != nil {
		t.Fatal(err)
	}
	if _, err := builder.ToString(); err != nil {
		t.Fatal(err)
	}
}
