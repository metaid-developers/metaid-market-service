package mrc20_service

import (
	"encoding/hex"
	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"metaid-market-service/common"
)

type MetaIdBuilder struct {
	Net        *chaincfg.Params
	MetaIdData *MetaIdData

	PayTos    []*PayTo
	FeeRate   int64
	OtherOuts []*OtherOut

	RevealPrivateKeyHex string
	RevealAddress       string

	revealTaprootDataInputIndex uint32
	TxCtxData                   *inscriptionTxCtxData
	RevealPsbtBuilder           *common.PsbtBuilder
	revealTx                    *wire.MsgTx

	metaIdOutValue   int64
	metaIdOutAddress string
}

func (m *MetaIdBuilder) buildEmptyRevealPsbt() error {
	var (
		revealPsbtBuilder     *common.PsbtBuilder
		inputs                []common.Input      = make([]common.Input, 0)
		inSigners             []*common.InputSign = make([]*common.InputSign, 0)
		outputs               []common.Output     = make([]common.Output, 0)
		taprootDataInputIndex uint32              = 0
		err                   error
	)

	emptyTxId := "0000000000000000000000000000000000000000000000000000000000000000"
	taprootDataIn := common.Input{
		OutTxId:  emptyTxId,
		OutIndex: 0,
	}

	inputs = append(inputs, taprootDataIn)

	outPin := common.Output{
		Address: m.metaIdOutAddress,
		Amount:  uint64(m.metaIdOutValue),
	}
	outputs = append(outputs, outPin)

	if m.OtherOuts != nil && len(m.OtherOuts) != 0 {
		for _, v := range m.OtherOuts {
			out := common.Output{
				Address: v.Address,
				Amount:  uint64(v.Amount),
				Script:  v.Script,
			}
			outputs = append(outputs, out)
		}
	}
	revealPsbtBuilder, err = common.CreatePsbtBuilder(m.Net, inputs, outputs)
	if err != nil {
		return err
	}
	m.RevealPsbtBuilder = revealPsbtBuilder

	taprootDataInSigner := &common.InputSign{
		UtxoType: common.Taproot,
		Index:    int(taprootDataInputIndex),
		//OutRaw:         "",
		PkScript:            hex.EncodeToString(m.TxCtxData.CommitTxAddressPkScript),
		RedeemScript:        hex.EncodeToString(m.TxCtxData.InscriptionScript),
		ControlBlockWitness: hex.EncodeToString(m.TxCtxData.ControlBlockWitness),
		Amount:              uint64(m.CalRevealPsbtFee(m.FeeRate)),
		SighashType:         txscript.SigHashAll,
		PriHex:              "",
		//MultiSigScript: "",
		//PreSigScript:   "",
	}
	inSigners = append(inSigners, taprootDataInSigner)

	err = revealPsbtBuilder.UpdateAndAddInputWitness(inSigners)
	if err != nil {
		return err
	}

	m.RevealPsbtBuilder = revealPsbtBuilder
	m.revealTaprootDataInputIndex = taprootDataInputIndex
	m.TxCtxData.revealTxPrevOutput = &wire.TxOut{
		PkScript: m.TxCtxData.CommitTxAddressPkScript,
		Value:    m.CalRevealPsbtFee(m.FeeRate),
	}
	return nil
}

func (m *MetaIdBuilder) CalRevealPsbtFee(feeRate int64) int64 {
	var (
		tx          *wire.MsgTx = m.RevealPsbtBuilder.PsbtUpdater.Upsbt.UnsignedTx
		txTotalSize int         = tx.SerializeSize()
		txBaseSize  int         = tx.SerializeSizeStripped()
		txFee       int64       = 0
		weight      int64       = 0
		vSize       int64       = 0

		revealOutValues = int64(0)
	)

	if m.OtherOuts != nil && len(m.OtherOuts) > 0 {
		for _, v := range m.OtherOuts {
			revealOutValues += v.Amount
		}
	}

	emptySignature := make([]byte, 64)
	emptyControlBlockWitness := make([]byte, 33)
	txTotalSize += wire.TxWitness{emptySignature, m.TxCtxData.InscriptionScript, emptyControlBlockWitness}.SerializeSize()

	weight = int64(txBaseSize*3 + txTotalSize)
	vSize = (weight + (blockchain.WitnessScaleFactor - 1)) / blockchain.WitnessScaleFactor
	vSize = vSize + 1
	txFee = vSize * feeRate
	//fmt.Printf("weight:%d, vSize:%d, txFee:%d\n", weight, vSize, txFee)
	//fmt.Printf("revealOutValues:%d, totalMinerFee:%d\n", revealOutValues, txFee+revealOutValues)
	return txFee + revealOutValues
}

func (m *MetaIdBuilder) completeRevealPsbt(commitTxId string, commitTxOutIndex uint32) error {
	var (
		commitPreOutPoint *wire.OutPoint
		txHash            *chainhash.Hash
		err               error
	)
	txHash, err = chainhash.NewHashFromStr(commitTxId)
	if err != nil {
		return err
	}
	commitPreOutPoint = wire.NewOutPoint(txHash, commitTxOutIndex)
	m.RevealPsbtBuilder.PsbtUpdater.Upsbt.UnsignedTx.TxIn[m.revealTaprootDataInputIndex].PreviousOutPoint = *commitPreOutPoint
	return nil
}

func (m *MetaIdBuilder) signRevealPsbt(taprootInSigner *common.InputSign) error {
	var (
		revealSigners        []*common.InputSign = make([]*common.InputSign, 0)
		revealTaprootSigners []*common.InputSign = make([]*common.InputSign, 0)
		err                  error
	)

	err = m.RevealPsbtBuilder.UpdateAndSignInput(revealSigners)
	if err != nil {
		return err
	}

	if taprootInSigner == nil {
		taprootInSigner = &common.InputSign{
			UtxoType: common.Taproot,
			Index:    int(m.revealTaprootDataInputIndex),
			//OutRaw:         "",
			PkScript:            hex.EncodeToString(m.TxCtxData.CommitTxAddressPkScript),
			RedeemScript:        hex.EncodeToString(m.TxCtxData.InscriptionScript),
			ControlBlockWitness: hex.EncodeToString(m.TxCtxData.ControlBlockWitness),
			Amount:              uint64(m.CalRevealPsbtFee(m.FeeRate)),
			SighashType:         txscript.SigHashAll,
			PriHex:              m.TxCtxData.RecoveryPrivateKeyHex,
			//MultiSigScript: "",
			//PreSigScript:   "",
		}
		revealTaprootSigners = append(revealTaprootSigners, taprootInSigner)
	}

	err = m.RevealPsbtBuilder.UpdateAndSignTaprootInput(revealTaprootSigners)
	if err != nil {
		return err
	}

	return nil
}

func (m *MetaIdBuilder) ExtractRevealTransaction() (string, string, error) {
	var (
		commitTxHex string
		revealTxHex string
		err         error
	)

	revealTxHex, err = m.RevealPsbtBuilder.ExtractPsbtTransaction()
	if err != nil {
		return "", "", err
	}
	return commitTxHex, revealTxHex, nil
}
