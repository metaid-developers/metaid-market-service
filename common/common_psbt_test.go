package common

import (
	"fmt"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"testing"
)

//priKeyHex: abb13a81345bcaa4cf0cc9cc16bd9b3950bc0037b84ab19334b27b3fe355ec45
//publicKey: 035e77304596e08dc07242a4ebdb49bc1352cc6953a098ab4d6bc701594afa8db6
//address: 12Pjewh2zceHTTtQFshpEkAmMyi1tpHEYJ
//txId: 0b76a6ff6e96227db7b5a435df5987712e0adde75154f060942a5b019c5f4228
//index: 0
//amount: 2000

//priKeyHex: 9e452e560160c18895882d244dff6ed55f11c2735416a3d8cdc3cdb287a65a9f
//publicKey: 0328851d05435fbe5f4743589c5cc4d873a35e998c1d8366f66fe7edc722547df7
//address: 15WWpP8n2zcHk7RQRqXj1ryPWYKyBCXaE6
//txId: 86690bd3811652af74492096dd40868c504a9172d9e5ed7a694ad4d850a48a2e
//index: 0
//amount: 3200

var (
	priKeyHex1 string = "abb13a81345bcaa4cf0cc9cc16bd9b3950bc0037b84ab19334b27b3fe355ec45"
	publicKey1 string = "035e77304596e08dc07242a4ebdb49bc1352cc6953a098ab4d6bc701594afa8db6"
	address1   string = "12Pjewh2zceHTTtQFshpEkAmMyi1tpHEYJ"
	pkScript1  string = "76a9140f44e2038c329c1a7d6fe4c8f0d6fbfab11a29f088ac"
	txId1      string = "0b76a6ff6e96227db7b5a435df5987712e0adde75154f060942a5b019c5f4228"
	txIndex1   uint32 = 0
	txValue1   uint64 = 2000

	priKeyHex2 string = "9e452e560160c18895882d244dff6ed55f11c2735416a3d8cdc3cdb287a65a9f"
	publicKey2 string = "0328851d05435fbe5f4743589c5cc4d873a35e998c1d8366f66fe7edc722547df7"
	address2   string = "15WWpP8n2zcHk7RQRqXj1ryPWYKyBCXaE6"
	pkScript2  string = "76a914317574b89fc07e9bcb71396e637a77d39d6b3ccf88ac"
	txId2      string = "86690bd3811652af74492096dd40868c504a9172d9e5ed7a694ad4d850a48a2e"
	txIndex2   uint32 = 0
	txValue2   uint64 = 3200

	outAddress string = "1F4Ga3Nbjehizh5ZMfKtZ6bP3WQuVRP7Xf"
	outAmount  int64  = 4000
)

func TestBuildMvcPsbt(t *testing.T) {
	netParams := &chaincfg.MainNetParams

	inputs := make([]Input, 0)
	inputs = append(inputs, Input{
		OutTxId:  txId1,
		OutIndex: uint32(txIndex1),
	})
	inputSigns := make([]*InputSign, 0)
	inputSigns = append(inputSigns, &InputSign{
		Index:       0,
		OutRaw:      "",
		PkScript:    pkScript1,
		SighashType: txscript.SigHashSingle | txscript.SigHashAnyOneCanPay,
		PriHex:      priKeyHex1,
		UtxoType:    Witness,
		Amount:      txValue1,
	})

	outputs := make([]Output, 0)
	outputs = append(outputs, Output{
		Address: outAddress,
		Amount:  uint64(outAmount),
	})

	builder, err := CreatePsbtBuilder(netParams, inputs, outputs)
	if err != nil {
		fmt.Printf("CreatePsbtBuilder Err: %s\n", err.Error())
		return
	}
	err = builder.UpdateAndSignInput(inputSigns)
	if err != nil {
		fmt.Printf("UpdateAndSignInput Err: %s\n", err.Error())
		return
	}
	psbtRaw, err := builder.ToString()
	if err != nil {
		fmt.Printf("ToString Err: %s\n", err.Error())
		return
	}
	fmt.Printf("psbtRaw: %s\n", psbtRaw)
}
