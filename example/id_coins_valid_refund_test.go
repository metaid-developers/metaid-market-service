package example

import (
	"github.com/btcsuite/btcd/txscript"
	"metaid-market-service/common"
	"metaid-market-service/controller/request"
	"metaid-market-service/tool"
	"testing"
)

func TestIdCoinsValidRefundPre(t *testing.T) {
	var (
		//priKeyHex = "d4e34195c8a62d5773bfb2ce65056defe30c4cbc8ef4ac8a4074728e04ff5c34"
		//address   = "tb1qe6r2e0d7kl92ul489g7r7kfm4hz66qndtr88ft"
		//orderId   = "b9a8e097bccc24132e11d68ff9206c16b623e67c72cc57f114eeacc0506c2210"
		//orderId = "id_coins_mint_outside_5bff13d5602857af613a9f0259b494b3bde53b747f820871f964591bdf"

		//orderId = "id_coins_mint_outside_dba5a3bc833715ad1df03671287582255fb617415c0a9ea54a8a1f9ebd0cd664"
		orderId = "dba5a3bc833715ad1df03671287582255fb617415c0a9ea54a8a1f9ebd0cd664"

		priKeyHex = "c92b5412f75905c1b78af96d61b4c12b3a53d2f6d5a74cd57a32de90596e77cb"
		address   = "bc1q0xmpakvxxtrp2t40rz2p457s2sy9hw937kyyct"

		url string = domain + "/api/v1/id-coins/invalid/refund/pre"
		err error

		req = &request.RefundIdCoinsValidPreRequest{
			Address: address,
			OrderId: orderId,
		}
	)

	resp, err := tool.PostUrl(url, req, addHeaders(priKeyHex))
	if err != nil {
		t.Fatalf("PostUrl() error = %v,", err)
	}
	t.Logf("resp = %v,", resp)
	/*
	   id_coins_valid_refund_test.go:31: resp = {"code":0,"message":"success","processingTime":11,"data":{"orderId":"id_coins_mint_outside_5bff13d5602857af613a9f0259b494b3bde53b747f820871f964591bdf","refundAddress":"tb1qe6r2e0d7kl92ul489g7r7kfm4hz66qndtr88ft","refundAmount":10000,"psbtRaw":"70736274ff01005202000000019f3bf723e86b5b62f4a68a19cbb229211d1ac9e145318c7fff1d2a5c5c3296210300000000ffffffff011027000000000000160014ce86acbdbeb7caae7ea72a3c3f593badc5ad026d000000000001011f1027000000000000160014df5bdae6ee21386d273d3bf503e36f6568154c99010304010000000000"}},
	*/
}

func TestIdCoinsValidRefundCommit(t *testing.T) {
	type feeUtxo struct {
		TxId   string
		Index  uint32
		Amount uint64
	}
	var (
		//net           = "testnet"
		//priKeyHex     = "d4e34195c8a62d5773bfb2ce65056defe30c4cbc8ef4ac8a4074728e04ff5c34"
		//address       = "tb1qe6r2e0d7kl92ul489g7r7kfm4hz66qndtr88ft"

		net           = "livenet"
		priKeyHex     = "c92b5412f75905c1b78af96d61b4c12b3a53d2f6d5a74cd57a32de90596e77cb"
		address       = "bc1q0xmpakvxxtrp2t40rz2p457s2sy9hw937kyyct"
		pkScript, _   = common.AddressToPkScript(net, address)
		changeAddress = address

		url string = domain + "/api/v1/id-coins/invalid/refund/commit"
		err error

		//orderId    string = "b9a8e097bccc24132e11d68ff9206c16b623e67c72cc57f114eeacc0506c2210"
		//orderId    string = "id_coins_mint_outside_5bff13d5602857af613a9f0259b494b3bde53b747f820871f964591bdf"
		//orderId    string = "id_coins_mint_outside_dba5a3bc833715ad1df03671287582255fb617415c0a9ea54a8a1f9ebd0cd664"
		orderId    string = "dba5a3bc833715ad1df03671287582255fb617415c0a9ea54a8a1f9ebd0cd664"
		psbtPreRaw string = "70736274ff0100520200000001998dbf5466f9e94f27d629ee08bb25aba16924c0dde154ca8684789a57590e480300000000ffffffff01a08601000000000016001479b61ed98632c6152eaf18941ad3d054085bb8b1000000000001011fa086010000000000160014d52236dd70e9c9b16612236c3be39b0bf3c6cfec010304010000000000"

		req = &request.RefundIdCoinsValidCommitRequest{
			OrderId: orderId,
			PsbtRaw: "",
		}

		utxos []feeUtxo = []feeUtxo{
			{
				TxId:   "c87b7a0b5d332f4a7425da556dc5bfc10e823b27f653e01846d974eae5a47749",
				Index:  0,
				Amount: 8000,
			},
		}

		takePsbtBuilder *common.PsbtBuilder
		outputs         []common.Output
		inSigners       []*common.InputSign

		utxoAmount     int64 = 0
		outAmount      int64 = 0
		minerFee       int64 = 0
		txSize         int64 = 0
		networkFeeRate int64 = 6
	)

	takePsbtBuilder, err = common.NewPsbtBuilder(common.GetNetParams(net), psbtPreRaw)
	if err != nil {
		t.Errorf("NewPsbtBuilder() error = %v,", err)
		return
	}
	d := len(takePsbtBuilder.GetInputs())

	for i, utxo := range utxos {
		utxoAmount += int64(utxo.Amount)
		input := common.Input{
			OutTxId:  utxo.TxId,
			OutIndex: utxo.Index,
		}
		inSigner := &common.InputSign{
			UtxoType:    common.Witness,
			Index:       d + i,
			PkScript:    pkScript,
			Amount:      utxo.Amount,
			SighashType: txscript.SigHashAll,
			PriHex:      priKeyHex,
		}
		err = takePsbtBuilder.AddInputOnly(input)
		if err != nil {
			t.Errorf("AddInputOnly() error = %v,", err)
			return
		}
		inSigners = append(inSigners, inSigner)
	}

	err = takePsbtBuilder.UpdateAndAddInputWitness(inSigners)
	if err != nil {
		t.Errorf("UpdateAndAddInputWitness() error = %v,", err)
		return
	}

	outputs = append(outputs, common.Output{
		Address: changeAddress,
		Amount:  0,
	})
	if len(outputs) > 0 {
		err = takePsbtBuilder.AddOutput(outputs)
		if err != nil {
			t.Errorf("AddOutputOnly() error = %v,", err)
			return
		}
	}

	for _, out := range takePsbtBuilder.PsbtUpdater.Upsbt.UnsignedTx.TxOut {
		outAmount += out.Value
	}

	txSize, err = takePsbtBuilder.CalTxSize()
	if err != nil {
		t.Fatalf("CalTxSize() error = %v,", err)
	}
	minerFee = txSize * networkFeeRate

	changeOutIndex := len(takePsbtBuilder.PsbtUpdater.Upsbt.UnsignedTx.TxOut) - 1
	takePsbtBuilder.PsbtUpdater.Upsbt.UnsignedTx.TxOut[changeOutIndex].Value = int64(utxoAmount - outAmount - minerFee)
	if takePsbtBuilder.PsbtUpdater.Upsbt.UnsignedTx.TxOut[changeOutIndex].Value < 546 {
		takePsbtBuilder.PsbtUpdater.Upsbt.UnsignedTx.TxOut = takePsbtBuilder.PsbtUpdater.Upsbt.UnsignedTx.TxOut[:changeOutIndex]
		takePsbtBuilder.PsbtUpdater.Upsbt.Outputs = takePsbtBuilder.PsbtUpdater.Upsbt.Outputs[:changeOutIndex]
	}

	err = takePsbtBuilder.UpdateAndSignInput(inSigners)
	//err = takePsbtBuilder.UpdateAndSignInputNoFinalize(inSigners)
	if err != nil {
		t.Errorf("UpdateAndSignInput() error = %v,", err)
		return
	}

	req.PsbtRaw, err = takePsbtBuilder.ToString()
	if err != nil {
		t.Errorf("ToString() error = %v,", err)
		return
	}

	resp, err := tool.PostUrl(url, req, addHeaders(priKeyHex))
	if err != nil {
		t.Fatalf("PostUrl() error = %v,", err)
	}
	t.Logf("resp = %v,", resp)
	/*
	   id_coins_valid_refund_test.go:162: resp = {"code":0,"message":"success","processingTime":299,"data":{"orderId":"id_coins_mint_outside_5bff13d5602857af613a9f0259b494b3bde53b747f820871f964591bdf","refundAddress":"tb1qe6r2e0d7kl92ul489g7r7kfm4hz66qndtr88ft","refundAmount":10000,"txId":"1fb771588cb2bffc221b32c75fd99291b9e9fc5aad00e637d1829172c774af5f"}},
	*/
}
