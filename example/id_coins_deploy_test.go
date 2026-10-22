package example

import (
	"github.com/btcsuite/btcd/txscript"
	"metaid-market-service/common"
	"metaid-market-service/controller/auth"
	"metaid-market-service/controller/request"
	"metaid-market-service/tool"
	"testing"
)

func TestIdCoinsDeployPre(t *testing.T) {
	var (
		priKeyHex         = "d4e34195c8a62d5773bfb2ce65056defe30c4cbc8ef4ac8a4074728e04ff5c34"
		address           = "tb1qe6r2e0d7kl92ul489g7r7kfm4hz66qndtr88ft"
		metaId            = common.GetMetaIdByAddress(address)
		issuerSign string = ""

		url string = domain + "/api/v1/id-coins/deploy/pre"
		err error

		networkFeeRate int64 = 100

		req = &request.BuildIdCoinsPreRequest{
			Tick:             "MYCOIN",
			TokenName:        "My-Test-Coin",
			IssuerMetaId:     metaId,
			IssuerAddress:    address,
			IssuerSign:       "",
			Message:          "This is a MOON",
			FollowersNum:     1000,
			AmountPerMint:    1000,
			LiquidityPerMint: 2000,
			NetworkFeeRate:   networkFeeRate,
		}
	)

	issuerSign, err = auth.SignTextMessage(req.Tick, priKeyHex)
	if err != nil {
		t.Fatalf("SignTextMessage() error = %v,", err)
	}
	req.IssuerSign = issuerSign

	resp, err := tool.PostUrl(url, req, addHeaders(priKeyHex))
	if err != nil {
		t.Fatalf("PostUrl() error = %v,", err)
	}
	t.Logf("resp = %v,", resp)
	/*
	   id_coins_deploy_test.go:48: resp = {"code":0,"message":"success","processingTime":105,"data":{"orderId":"fa768e125ffd78ee4d0ff00bff7946badb2aa671791f4553c71e0b12d6ed7d03","totalFee":26146,"receiveAddress":"tb1pm28v5830e9u3vmg5f4qrcy7jrmjhpkmk7h7tetusyaz2763px4kq03a0xm","minerFee":26146,"serviceFee":0}},
	*/
}

func TestIdCoinsDeployCommit(t *testing.T) {
	var (
		net           = "testnet"
		priKeyHex     = "d4e34195c8a62d5773bfb2ce65056defe30c4cbc8ef4ac8a4074728e04ff5c34"
		address       = "tb1qe6r2e0d7kl92ul489g7r7kfm4hz66qndtr88ft"
		pkScript, _   = common.AddressToPkScript(net, address)
		changeAddress = address

		url string = domain + "/api/v1/id-coins/deploy/commit"
		err error

		orderId       string = "fa768e125ffd78ee4d0ff00bff7946badb2aa671791f4553c71e0b12d6ed7d03"
		revealFee     int64  = 26146
		revealAddress string = "tb1pm28v5830e9u3vmg5f4qrcy7jrmjhpkmk7h7tetusyaz2763px4kq03a0xm"
		serviceFee    int64  = 0

		req = &request.BuildIdCoinsCommitRequest{
			OrderId:          orderId,
			CommitTxRaw:      "",
			CommitTxOutIndex: 0,
		}

		utxoOutTxId string = "5f9432a09b48a790b8a49d6f08f3f27957093b3eafc99a956446f26cc5ae8c64"
		utxoIndex   uint32 = 1
		utxoAmount  int64  = 3885824

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
	   id_coins_deploy_test.go:164: resp = {"code":0,"message":"success","processingTime":30017,"data":{"orderId":"fa768e125ffd78ee4d0ff00bff7946badb2aa671791f4553c71e0b12d6ed7d03","tickId":"2c541420b38c84f07ed433dc6687887005b0f2013123a9e7499387f02fbcaf58i0","commitTxId":"adfbffb3aa4abb6b185bb54459a61655d8df7d7c09b08a88a6499a76680ab0ed","revealTxId":"2c541420b38c84f07ed433dc6687887005b0f2013123a9e7499387f02fbcaf58","pinId":"2c541420b38c84f07ed433dc6687887005b0f2013123a9e7499387f02fbcaf58i0","txId":"2c541420b38c84f07ed433dc6687887005b0f2013123a9e7499387f02fbcaf58"}},
	*/
}
