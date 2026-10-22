package example

import (
	"github.com/btcsuite/btcd/txscript"
	"metaid-market-service/common"
	"metaid-market-service/controller/request"
	"metaid-market-service/tool"
	"testing"
)

func TestIdCoinsMintPre(t *testing.T) {
	var (
		priKeyHex = "d4e34195c8a62d5773bfb2ce65056defe30c4cbc8ef4ac8a4074728e04ff5c34"
		address   = "tb1qe6r2e0d7kl92ul489g7r7kfm4hz66qndtr88ft"
		tickId    = "5bff13d5602857af613a9f0259b494b3bde53b747f820871f964591bdf972e3fi0"

		url string = domain + "/api/v1/id-coins/mint/pre"
		err error

		networkFeeRate int64 = 100

		req = &request.IdCoinsMintPreRequest{
			NetworkFeeRate: networkFeeRate,
			TickId:         tickId,
			OutAddress:     address,
			OutValue:       546,
		}
	)

	resp, err := tool.PostUrl(url, req, addHeaders(priKeyHex))
	if err != nil {
		t.Fatalf("PostUrl() error = %v,", err)
	}
	t.Logf("resp = %v,", resp)
	/*
	   id_coins_mint_test.go:34: resp = {"code":0,"message":"success","processingTime":28,"data":{"orderId":"b9a8e097bccc24132e11d68ff9206c16b623e67c72cc57f114eeacc0506c2210","totalFee":64437,"revealInscribeFee":14846,"revealMintFee":46592,"revealInscribeAddress":"tb1pvp8n8gsk396qj5tyc76edwnu53sre2jjny535u9tlvkduxet4uqq4dvvmk","revealMintAddress":"tb1pjgd7nq68j3htqu9k284zdtd0dll0cnvcm4hrsp60sepy7yrfetwqw3cwzp","serviceFee":2999,"serviceAddress":"","extra":{}}},
	*/
}

func TestIdCoinsMintCommit(t *testing.T) {
	var (
		net           = "testnet"
		priKeyHex     = "d4e34195c8a62d5773bfb2ce65056defe30c4cbc8ef4ac8a4074728e04ff5c34"
		address       = "tb1qe6r2e0d7kl92ul489g7r7kfm4hz66qndtr88ft"
		pkScript, _   = common.AddressToPkScript(net, address)
		changeAddress = address

		url string = domain + "/api/v1/id-coins/mint/commit"
		err error

		orderId               string = "b9a8e097bccc24132e11d68ff9206c16b623e67c72cc57f114eeacc0506c2210"
		revealInscribeFee     int64  = 14846
		revealMintFee         int64  = 46592
		revealInscribeAddress string = "tb1pvp8n8gsk396qj5tyc76edwnu53sre2jjny535u9tlvkduxet4uqq4dvvmk"
		revealMintAddress     string = "tb1pjgd7nq68j3htqu9k284zdtd0dll0cnvcm4hrsp60sepy7yrfetwqw3cwzp"
		serviceFee            int64  = 2999

		req = &request.IdCoinsMintCommitRequest{
			OrderId:                  orderId,
			CommitTxRaw:              "",
			CommitTxOutInscribeIndex: 0,
			CommitTxOutMintIndex:     1,
		}

		utxoOutTxId string = "c8b8d475021dbb8df3e5a1a4303d4962f0084c4773b299537158e04d4d58d1db"
		utxoIndex   uint32 = 1
		utxoAmount  int64  = 2668308

		commitPsbtBuilder *common.PsbtBuilder
		inputs            []common.Input
		outputs           []common.Output
		inSigns           []*common.InputSign

		minerFee       int64 = 0
		txSize         int64 = 0
		networkFeeRate int64 = 100
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
			Address: revealInscribeAddress,
			Amount:  uint64(revealInscribeFee),
			//Script:  "",
		},
		{
			Address: revealMintAddress,
			Amount:  uint64(revealMintFee),
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

	commitPsbtBuilder.PsbtUpdater.Upsbt.UnsignedTx.TxOut[len(outputs)-1].Value = int64(utxoAmount - revealInscribeFee - revealMintFee - serviceFee - minerFee)
	if commitPsbtBuilder.PsbtUpdater.Upsbt.UnsignedTx.TxOut[len(outputs)-1].Value < 546 {
		commitPsbtBuilder.PsbtUpdater.Upsbt.UnsignedTx.TxOut = commitPsbtBuilder.PsbtUpdater.Upsbt.UnsignedTx.TxOut[:len(outputs)-1]
		commitPsbtBuilder.PsbtUpdater.Upsbt.Outputs = commitPsbtBuilder.PsbtUpdater.Upsbt.Outputs[:len(outputs)-1]
	}

	if err = commitPsbtBuilder.UpdateAndSignInput(inSigns); err != nil {
		t.Fatalf("UpdateAndAddInputWitness() error = %v,", err)
	}

	req.CommitTxRaw, err = commitPsbtBuilder.ExtractPsbtTransaction()
	if err != nil {
		t.Fatalf("ExtractPsbtTransaction() error = %v,", err)
	}
	req.CommitTxOutInscribeIndex = 0
	req.CommitTxOutMintIndex = 1

	resp, err := tool.PostUrl(url, req, addHeaders(priKeyHex))
	if err != nil {
		t.Fatalf("PostUrl() error = %v,", err)
	}
	t.Logf("resp = %v,", resp)
	/*
		":"success","processingTime":20,"data":{"orderId":"c3ca396d5f3b89ddaabeb540c667839e58474081e43d142935707f1fbc076c46","commitTxId":"aff024e3d5bb22805d2c1f0f9155da1bcf870b121b09866d965a09d0bdfb92b2","revealInscribeTxId":"ef55e7c34d21f60efec480386fd654b2cd287b53529c806d3a4735084b464fb0","revealMintTxId":"de9cfc3410653c52b323e9bb5b0ca9209119264c3eac9b9cf614e5415f3d1bf9"}},
	*/
}

func TestIdCoinsMintPreL(t *testing.T) {
	var (
		priKeyHex = "c92b5412f75905c1b78af96d61b4c12b3a53d2f6d5a74cd57a32de90596e77cb"
		address   = "bc1q0xmpakvxxtrp2t40rz2p457s2sy9hw937kyyct"
		tickId    = "8ce590918a3d493631ab9d9e3bbc89e322f3dd08354af87f97c07218818b78f4i0"

		url string = domain + "/api/v1/id-coins/mint/pre"
		err error

		networkFeeRate int64 = 6

		req = &request.IdCoinsMintPreRequest{
			NetworkFeeRate: networkFeeRate,
			TickId:         tickId,
			OutAddress:     address,
			OutValue:       546,
		}
	)

	resp, err := tool.PostUrl(url, req, addHeaders(priKeyHex))
	if err != nil {
		t.Fatalf("PostUrl() error = %v,", err)
	}
	t.Logf("resp = %v,", resp)
	/*
	   id_coins_mint_test.go:34: resp = {"code":0,"message":"success","processingTime":28,"data":{"orderId":"b9a8e097bccc24132e11d68ff9206c16b623e67c72cc57f114eeacc0506c2210","totalFee":64437,"revealInscribeFee":14846,"revealMintFee":46592,"revealInscribeAddress":"tb1pvp8n8gsk396qj5tyc76edwnu53sre2jjny535u9tlvkduxet4uqq4dvvmk","revealMintAddress":"tb1pjgd7nq68j3htqu9k284zdtd0dll0cnvcm4hrsp60sepy7yrfetwqw3cwzp","serviceFee":2999,"serviceAddress":"","extra":{}}},
	*/
}

func TestIdCoinsMintCommitL(t *testing.T) {
	var (
		net           = "livenet"
		priKeyHex     = "c92b5412f75905c1b78af96d61b4c12b3a53d2f6d5a74cd57a32de90596e77cb"
		address       = "bc1q0xmpakvxxtrp2t40rz2p457s2sy9hw937kyyct"
		pkScript, _   = common.AddressToPkScript(net, address)
		changeAddress = address

		url string = domain + "/api/v1/id-coins/mint/commit"
		err error

		orderId               string = "dba5a3bc833715ad1df03671287582255fb617415c0a9ea54a8a1f9ebd0cd664"
		revealInscribeFee     int64  = 1410
		revealMintFee         int64  = 105227
		revealInscribeAddress string = "bc1ptezzsqepp7rs2dys2k669puwg0yxd69yjzfs3fgd4xhqpce0a4nqr9xvwh"
		revealMintAddress     string = "bc1pqc4qx43a7fssfwhh3vdcsuj0mu78gy57968t72dt7l49yut4q2esgkg34w"
		serviceFee            int64  = 1999

		req = &request.IdCoinsMintCommitRequest{
			OrderId:                  orderId,
			CommitTxRaw:              "",
			CommitTxOutInscribeIndex: 0,
			CommitTxOutMintIndex:     1,
		}

		utxoOutTxId string = "ee3a944cf174141c5b9fa5f27793338f01b1f47d827c55dcdf6a857c23eb42bc"
		utxoIndex   uint32 = 0
		utxoAmount  int64  = 108000

		commitPsbtBuilder *common.PsbtBuilder
		inputs            []common.Input
		outputs           []common.Output
		inSigns           []*common.InputSign

		minerFee       int64 = 0
		txSize         int64 = 0
		networkFeeRate int64 = 10
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
			Address: revealInscribeAddress,
			Amount:  uint64(revealInscribeFee),
			//Script:  "",
		},
		{
			Address: revealMintAddress,
			Amount:  uint64(revealMintFee),
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

	commitPsbtBuilder.PsbtUpdater.Upsbt.UnsignedTx.TxOut[len(outputs)-1].Value = int64(utxoAmount - revealInscribeFee - revealMintFee - serviceFee - minerFee)
	if commitPsbtBuilder.PsbtUpdater.Upsbt.UnsignedTx.TxOut[len(outputs)-1].Value < 546 {
		commitPsbtBuilder.PsbtUpdater.Upsbt.UnsignedTx.TxOut = commitPsbtBuilder.PsbtUpdater.Upsbt.UnsignedTx.TxOut[:len(outputs)-1]
		commitPsbtBuilder.PsbtUpdater.Upsbt.Outputs = commitPsbtBuilder.PsbtUpdater.Upsbt.Outputs[:len(outputs)-1]
	}

	if err = commitPsbtBuilder.UpdateAndSignInput(inSigns); err != nil {
		t.Fatalf("UpdateAndAddInputWitness() error = %v,", err)
	}

	req.CommitTxRaw, err = commitPsbtBuilder.ExtractPsbtTransaction()
	if err != nil {
		t.Fatalf("ExtractPsbtTransaction() error = %v,", err)
	}
	req.CommitTxOutInscribeIndex = 0
	req.CommitTxOutMintIndex = 1

	resp, err := tool.PostUrl(url, req, addHeaders(priKeyHex))
	if err != nil {
		t.Fatalf("PostUrl() error = %v,", err)
	}
	t.Logf("resp = %v,", resp)
	/*
		":"success","processingTime":20,"data":{"orderId":"c3ca396d5f3b89ddaabeb540c667839e58474081e43d142935707f1fbc076c46","commitTxId":"aff024e3d5bb22805d2c1f0f9155da1bcf870b121b09866d965a09d0bdfb92b2","revealInscribeTxId":"ef55e7c34d21f60efec480386fd654b2cd287b53529c806d3a4735084b464fb0","revealMintTxId":"de9cfc3410653c52b323e9bb5b0ca9209119264c3eac9b9cf614e5415f3d1bf9"}},
	*/
}
