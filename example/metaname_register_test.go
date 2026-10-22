package example

import (
	"github.com/btcsuite/btcd/txscript"
	"metaid-market-service/common"
	"metaid-market-service/controller/request"
	"metaid-market-service/tool"
	"testing"
)

func TestMetaNameRegisterPre(t *testing.T) {
	var (
		priKeyHex = "d4e34195c8a62d5773bfb2ce65056defe30c4cbc8ef4ac8a4074728e04ff5c34"
		address   = "tb1qe6r2e0d7kl92ul489g7r7kfm4hz66qndtr88ft"

		url string = domain + "/api/v1/metaname/register/pre"
		err error

		networkFeeRate int64 = 6000

		req = &request.RegisterMetaNamePreRequest{
			MetaName:       "aaldd.metaid",
			Address:        address,
			NetworkFeeRate: networkFeeRate,
		}
	)

	resp, err := tool.PostUrl(url, req, addHeaders(priKeyHex))
	if err != nil {
		t.Fatalf("PostUrl() error = %v,", err)
	}
	t.Logf("resp = %v,", resp)
	/*
	   metaname_register_test.go:32: resp = {"code":0,"message":"success","processingTime":21,"data":{"orderId":"5f4d93525323b9bfd69060ab323661943f1370c4906cdcf73f31dce2f86e96f7","totalFee":924000,"receiveAddress":"tb1p85l4swp3lnw48mghrvq5zqzsh955gk5l7vmrvr7lq36ly5fke52sqfmadw","minerFee":924000,"minerGas":924000,"minerOutValue":0,"serviceFee":0,"metaName":"aaldd.metaid"}},
	*/
}

func TestMetaNameRegisterCommit(t *testing.T) {
	var (
		net           = "testnet"
		priKeyHex     = "d4e34195c8a62d5773bfb2ce65056defe30c4cbc8ef4ac8a4074728e04ff5c34"
		address       = "tb1qe6r2e0d7kl92ul489g7r7kfm4hz66qndtr88ft"
		pkScript, _   = common.AddressToPkScript(net, address)
		changeAddress = address

		url string = domain + "/api/v1/metaname/register/commit"
		err error

		orderId       string = "5f4d93525323b9bfd69060ab323661943f1370c4906cdcf73f31dce2f86e96f7"
		revealFee     int64  = 924000
		revealAddress string = "tb1p85l4swp3lnw48mghrvq5zqzsh955gk5l7vmrvr7lq36ly5fke52sqfmadw"
		serviceFee    int64  = 0

		req = &request.RegisterMetaNameCommitRequest{
			OrderId:          orderId,
			CommitTxRaw:      "",
			CommitTxOutIndex: 0,
		}

		utxoOutTxId string = "c06209bc0216f4c20e9ed0e93aa69d85b9c1ba474d4be31c33c04da1a8c053b3"
		utxoIndex   uint32 = 0
		utxoAmount  int64  = 10000000

		commitPsbtBuilder *common.PsbtBuilder
		inputs            []common.Input
		outputs           []common.Output
		inSigns           []*common.InputSign

		minerFee       int64 = 0
		txSize         int64 = 0
		networkFeeRate int64 = 6000
	)

	//make commit tx
	inputs = []common.Input{
		{
			OutTxId:  utxoOutTxId,
			OutIndex: utxoIndex,
		},
	}
	outputs = []common.Output{
		{
			Address: revealAddress,
			Amount:  uint64(revealFee),
			//Script:  "",
		},
		//{
		//	Address: outServiceAddress,
		//	Amount:  uint64(serviceFee),
		//	//Script:  "",
		//},
		{
			Address: changeAddress,
			Amount:  0,
		},
	}

	commitPsbtBuilder, err = common.CreatePsbtBuilder(common.GetNetParams(net), inputs, outputs)
	if err != nil {
		t.Fatalf("CreatePsbtBuilder() error = %v,", err)
	}

	inSigns = []*common.InputSign{
		{
			UtxoType: common.Witness,
			Index:    0,
			//OutRaw:         "",
			PkScript: pkScript,
			//RedeemScript:   "",
			Amount:      uint64(utxoAmount),
			SighashType: txscript.SigHashAll,
			PriHex:      priKeyHex,
			//MultiSigScript: "",
			//PreSigScript:   "",
		},
	}

	if err = commitPsbtBuilder.UpdateAndAddInputWitness(inSigns); err != nil {
		t.Fatalf("UpdateAndAddInputWitness() error = %v,", err)
	}

	txSize, err = commitPsbtBuilder.CalTxSize()
	if err != nil {
		t.Fatalf("CalTxSize() error = %v,", err)
	}
	minerFee = txSize * networkFeeRate

	commitPsbtBuilder.PsbtUpdater.Upsbt.UnsignedTx.TxOut[1].Value = int64(utxoAmount - revealFee - serviceFee - minerFee)
	if commitPsbtBuilder.PsbtUpdater.Upsbt.UnsignedTx.TxOut[1].Value < 546 {
		commitPsbtBuilder.PsbtUpdater.Upsbt.UnsignedTx.TxOut = commitPsbtBuilder.PsbtUpdater.Upsbt.UnsignedTx.TxOut[:1]
		commitPsbtBuilder.PsbtUpdater.Upsbt.Outputs = commitPsbtBuilder.PsbtUpdater.Upsbt.Outputs[:1]
	}

	if err = commitPsbtBuilder.UpdateAndSignInput(inSigns); err != nil {
		t.Fatalf("UpdateAndAddInputWitness() error = %v,", err)
	}

	req.CommitTxRaw, err = commitPsbtBuilder.ExtractPsbtTransaction()
	if err != nil {
		t.Fatalf("ExtractPsbtTransaction() error = %v,", err)
	}
	req.CommitTxOutIndex = 0

	resp, err := tool.PostUrl(url, req, addHeaders(priKeyHex))
	if err != nil {
		t.Fatalf("PostUrl() error = %v,", err)
	}
	t.Logf("resp = %v,", resp)
	/*
	   metaname_register_test.go:148: resp = {"code":0,"message":"success","processingTime":45,"data":{"orderId":"5f4d93525323b9bfd69060ab323661943f1370c4906cdcf73f31dce2f86e96f7","commitTxId":"0fa01e5d7529d6a51eadd51396ae0aa0415761d2b428ee2275853689c737730f","revealTxId":"9481d08573ea133ab5874f8a6b7bb388cb2d7577e0e6f53a0c2d47ad36391eb2","pinId":"9481d08573ea133ab5874f8a6b7bb388cb2d7577e0e6f53a0c2d47ad36391eb2i0","txId":"9481d08573ea133ab5874f8a6b7bb388cb2d7577e0e6f53a0c2d47ad36391eb2","metaName":"aaldd.metaid"}},
	*/
}
