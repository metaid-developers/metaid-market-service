package parse_service

import (
	"bytes"
	"encoding/hex"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"metaid-market-service/common"
	"metaid-market-service/conf"
)

func ParseTxPin(txRaw string) (bool, *PersonalInformationNode, []*wire.TxOut, error) {
	var (
		pin       *PersonalInformationNode
		outputs   []*wire.TxOut
		err       error
		txRawByte []byte
		tx        *wire.MsgTx = wire.NewMsgTx(2)
		indexer               = &Indexer{
			ChainParams: common.GetNetParams(conf.Net),
			Block:       nil,
		}
	)
	txRawByte, err = hex.DecodeString(txRaw)
	if err != nil {
		return false, nil, nil, err
	}
	err = tx.Deserialize(bytes.NewReader(txRawByte))
	if err != nil {
		return false, nil, nil, err
	}
	outputs = tx.TxOut
	for _, txIn := range tx.TxIn {
		if txIn.Witness != nil {
			//Witness length error
			if len(txIn.Witness) <= 1 {
				continue
			}
			//Witness length error,Taproot
			if len(txIn.Witness) == 2 && txIn.Witness[len(txIn.Witness)-1][0] == txscript.TaprootAnnexTag {
				continue
			}
			// If Taproot Annex data exists, take the last element of the witness as the script data, otherwise,
			// take the penultimate element of the witness as the script data
			var witnessScript []byte
			if txIn.Witness[len(txIn.Witness)-1][0] == txscript.TaprootAnnexTag {
				witnessScript = txIn.Witness[len(txIn.Witness)-1]
			} else {
				witnessScript = txIn.Witness[len(txIn.Witness)-2]
			}

			pin = indexer.ParsePin(witnessScript)
			if pin == nil {
				return false, nil, nil, nil
			}
		}
	}
	return true, pin, outputs, nil
}
