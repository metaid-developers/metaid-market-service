package parse

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math"

	"metaid-market-service/common"

	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

func parseTx(net, chain, txRaw string) *wire.MsgTx {
	txRawByte, _ := hex.DecodeString(txRaw)
	tx := wire.NewMsgTx(2)
	err := tx.Deserialize(bytes.NewReader(txRawByte))
	if err != nil {
		fmt.Printf(fmt.Sprintf("PSBT(Y): txRawPsbtY Deserialize err:%s", err.Error()))
		return nil
	}

	//
	fmt.Printf("Tx:\n")
	fmt.Printf("%+v\n", tx)
	for k, in := range tx.TxIn {
		if in.Witness == nil {
			fmt.Printf("Input-[%d] OutPoint:%s, Witness: nil\n", k, in.PreviousOutPoint.String())
			continue
		}
		fmt.Printf("Input-[%d] OutPoint:%s, Witness:%s\n", k, in.PreviousOutPoint.String(), hex.EncodeToString(in.Witness[0]))
	}
	fmt.Printf("TxOutLen: %d\n", len(tx.TxOut))
	for k, out := range tx.TxOut {
		//fmt.Printf()

		_, addrs, _, err := txscript.ExtractPkScriptAddrs(out.PkScript, common.GetNetParams(net, chain))
		if err != nil {
			fmt.Printf(fmt.Sprintf("ExtractPkScriptAddrs err:%s", err.Error()))
			return nil
		}
		if len(addrs) == 0 {
			fmt.Printf("Output-[%d] Value:%d PkScript:%s\n", k, out.Value, hex.EncodeToString(out.PkScript))
		} else {
			receiveAddress := addrs[0].EncodeAddress()

			fmt.Printf("Output-[%d] Value:%d PkScript:%s, Address:%s\n", k, out.Value, hex.EncodeToString(out.PkScript), receiveAddress)

		}

	}
	fmt.Printf("\n")
	return tx
}

// func parseMvcTx(net, txRaw string) *wire2.MsgTx {
// 	txRawByte, _ := hex.DecodeString(txRaw)
// 	tx := wire2.NewMsgTx(2)
// 	err := tx.Deserialize(bytes.NewReader(txRawByte))
// 	if err != nil {
// 		fmt.Printf(fmt.Sprintf("PSBT(Y): txRawPsbtY Deserialize err:%s", err.Error()))
// 		return nil
// 	}

// 	//
// 	fmt.Printf("Tx:\n")
// 	fmt.Printf("%+v\n", tx)
// 	for k, in := range tx.TxIn {
// 		fmt.Printf("Input-[%d] OutPoint:%s\n", k, in.PreviousOutPoint.String())
// 		if in.SignatureScript != nil {
// 			fmt.Printf("Input-[%d] SignatureScript:%s\n", k, hex.EncodeToString(in.SignatureScript))
// 		}
// 	}
// 	fmt.Printf("TxOutLen: %d\n", len(tx.TxOut))
// 	for k, out := range tx.TxOut {
// 		//fmt.Printf()

// 		_, addrs, _, err := txscript2.ExtractPkScriptAddrs(out.PkScript, common.GetNetParams(net, chain))
// 		if err != nil {
// 			fmt.Printf(fmt.Sprintf("ExtractPkScriptAddrs err:%s", err.Error()))
// 			return nil
// 		}
// 		if len(addrs) == 0 {
// 			fmt.Printf("Output-[%d] Value:%d PkScript:%s\n", k, out.Value, hex.EncodeToString(out.PkScript))
// 		} else {
// 			receiveAddress := addrs[0].EncodeAddress()

// 			fmt.Printf("Output-[%d] Value:%d PkScript:%s, Address:%s\n", k, out.Value, hex.EncodeToString(out.PkScript), receiveAddress)

// 		}

// 	}
// 	fmt.Printf("\n")
// 	return tx
// }

func parsePsbt(net, chain, psbtRaw string) {
	psbtBuilder, err := common.NewPsbtBuilder(common.GetNetParams(net, chain), psbtRaw)
	if err != nil {
		fmt.Printf(fmt.Sprintf("NewPsbtBuilder err:%s", err.Error()))
		return
	}
	txRawPsbtY, err := psbtBuilder.ExtractPsbtTransaction()
	if err != nil {
		fmt.Printf(fmt.Sprintf("ExtractPsbtTransaction err:%s", err.Error()))
		return
	}
	fmt.Printf("Tx:%s\n", txRawPsbtY)
	parseTx(net, chain, txRawPsbtY)
}

func parseRuneOpReturn(pkScript string) {
	pkScriptByte, _ := hex.DecodeString(pkScript)
	if len(pkScriptByte) < 4 {
		fmt.Printf("pkScriptByte len < 4\n")
		return
	}
	index := 0
	if pkScriptByte[index] != txscript.OP_RETURN {
		fmt.Printf("pkScriptByte[0] != txscript.OP_RETURN\n")
		return
	}
	index += 1
	if pkScriptByte[index] != txscript.OP_13 {
		fmt.Printf("pkScriptByte[1] != txscript.OP_13\n")
		return
	}
	index += 1
	partByte := pkScriptByte[index : index+9]
	dataLen, indexLenth := decodeVarIntForScript(partByte)
	_ = indexLenth
	if len(pkScriptByte) < index+dataLen {
		fmt.Printf("pkScriptByte len < 2+dataLen\n")
		return
	}
	index += indexLenth
	data := pkScriptByte[index : index+dataLen]
	fmt.Printf("DataHex:%s\n", hex.EncodeToString(data))
	fmt.Printf("Data:%s\n", string(data))
}

func decodeVarIntForScript(buf []byte) (int, int) {
	if buf[0] < 0x4c { //N/A 1-75
		return int(buf[0]), 1
	} else if buf[0] == 0x4c { //OP_PUSHDATA1 - 76
		return int(buf[1]), 2
	} else if buf[0] == 0x4d { //op_pushdata2 - 77
		count :=
			(int(buf[2]) * int(math.Pow(256, 1))) +
				int(buf[1])
		return count, 3
	} else if buf[0] == 0x4e { //OP_PUSHDATA4 - 78
		count :=
			int(buf[4])*int(math.Pow(256, 3)) +
				int(buf[3])*int(math.Pow(256, 2)) +
				int(buf[2])*int(math.Pow(256, 1)) +
				int(buf[1])
		return count, 5
	}
	return -1, -1
}
