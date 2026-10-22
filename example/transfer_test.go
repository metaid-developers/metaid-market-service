package example

import (
	"bytes"
	"encoding/hex"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"metaid-market-service/common"
	"metaid-market-service/controller/request"
	"metaid-market-service/tool"
	"testing"
)

func TestMrc20TransferPreForLegacy(t *testing.T) {
	var (
		net         = "testnet"
		priKeyHex   = "8e2a95af06f8e03f1ea2a2b0b85e35757bbd27b8bd03f8cae934bf6f90fa4f71"
		address     = "mnVinwawKEtqyFJk3BHqmaEGkKNGrxYRh9"
		pkScript, _ = common.AddressToPkScript(net, address)

		toAddrss1      = "tb1qg28jmtz7cqpu493mlmg2g2ukuqnu383c5s0mz6"
		toPkScript1, _ = common.AddressToPkScript(net, toAddrss1)
		toAmount       = "200"

		url string = domain + "/api/v1/inscribe/mrc20/transfer/pre"
		err error

		networkFeeRate int64  = 1000
		tickerId       string = "8e659899275b1d06db870fbee9b293bc73d25e063cc86860a6d52c1e11091e9bi0"

		req = &request.Mrc20TransferPreRequest{
			NetworkFeeRate: networkFeeRate,
			TickerId:       tickerId,
			Transfers: []*request.TransferMrc20Utxo{
				{
					UtxoTxId:     "f4456256c0f75f6da01d5c68bfd8796071fe2d9bde6281ae452bdaff0e554292",
					UtxoIndex:    0,
					UtxoOutValue: 546,
					TickerId:     tickerId,
					Amount:       "800",
					Address:      address,
					PkScript:     pkScript,
				},
			},
			ChangeAddress:  address,
			ChangeOutValue: 546,
			Mrc20Outs: []*request.Mrc20OutInfo{
				{
					Amount:   toAmount,
					Address:  toAddrss1,
					PkScript: toPkScript1,
					OutValue: 546,
				},
			},
		}
	)

	resp, err := tool.PostUrl(url, req, addHeaders(priKeyHex))
	if err != nil {
		t.Fatalf("PostUrl() error = %v,", err)
	}
	t.Logf("resp = %v,", resp)
	/*
	   transfer_test.go:62: resp = {"code":0,"message":"success","processingTime":8,"data":{"orderId":"2a133b84bf9c47e83edc203126ce46bbc3e4a7fb024dc18de5395087690ee00d","totalFee":341546,"revealFee":341546,"revealAddress":"tb1p2rdy02lcjeyj2l68p8cw5r3d7vkgsrsgj66j78nx9w9pgmra6mtqsayyff","serviceFee":0,"serviceAddress":"","revealPrePsbtRaw":"70736274ff01009d02000000029242550effda2b45ae8162de9b2dfe716079d8bf685c1da06d5ff7c0566245f40000000000ffffffff00000000000000000000000000000000000000000000000000000000000000000000000000ffffffff0222020000000000001976a9144c8d3194f32e5c94478fe6e88cacc91f5384203f88ac2202000000000000160014428f2dac5ec003ca963bfed0a42b96e027c89e3800000000000100fd2f020200000000010222c3e855938f52a14b1a8b990500ad4201c1d2dea15ae73c71f80c0240635f95010000006b4830450221009b964890be124e02174a5ff270ca01868d87a1d25250b951d2cdeaa52dca376d02205a3f02f1feed065d1a99aabb6f49dd12177fe5bd12f0470e24314dc47c5262120121022fd11f8a9015c4eb5ba62bfecfda7cb37b71c2a316a1e979392079bb4e16bf65ffffffff9d9f3b8c233fb1dbfc791a72a419b5349cdf0629476238771659f1e9bd63df8b0000000000ffffffff0222020000000000001976a9144c8d3194f32e5c94478fe6e88cacc91f5384203f88ac2202000000000000160014428f2dac5ec003ca963bfed0a42b96e027c89e38000341a137552706fa4d4188c0e5b781e172cec0f48b2bbd5d7d3edf451078d7b4332d8a7c0c806e4d361c3756ee74f88888c535c94b094be1e987d4288799d4aa3dc901be201c3b4e0d72f4fed0e6f76ba4e1c984d88c746538b3cf26d8d7b4e5c1c2ef4a93ac0063067465737469640468696465122f66742f6d726332302f7472616e736665720000106170706c69636174696f6e2f6a736f6e4c655b7b226964223a22386536353938393932373562316430366462383730666265653962323933626337336432356530363363633836383630613664353263316531313039316539626930222c22616d6f756e74223a22323030222c22766f7574223a317d5d6821c11c3b4e0d72f4fed0e6f76ba4e1c984d88c746538b3cf26d8d7b4e5c1c2ef4a9300000000010304010000000001012b2a3605000000000022512050da47abf89649257f4709f0ea0e2df32c880e0896b52f1e662b8a146c7dd6d601030401000000000000","revealInputIndex":1,"extra":{"controlBlockWitness":"c1f8fbedc74baa4617d048871d840760b2390d6e8cedc4da03c1ebec5c1ebcf999","redeemScript":"20f8fbedc74baa4617d048871d840760b2390d6e8cedc4da03c1ebec5c1ebcf999ac0063067465737469640468696465122f66742f6d726332302f7472616e736665720000106170706c69636174696f6e2f6a736f6e4c655b7b226964223a22386536353938393932373562316430366462383730666265653962323933626337336432356530363363633836383630613664353263316531313039316539626930222c22616d6f756e74223a22323030222c22766f7574223a317d5d68","xOnlyPubKey":"f8fbedc74baa4617d048871d840760b2390d6e8cedc4da03c1ebec5c1ebcf999"}}},
	*/
}

func TestMrc20TransferCommitForLegacy(t *testing.T) {
	var (
		net           = "testnet"
		priKeyHex     = "8e2a95af06f8e03f1ea2a2b0b85e35757bbd27b8bd03f8cae934bf6f90fa4f71"
		address       = "mnVinwawKEtqyFJk3BHqmaEGkKNGrxYRh9"
		pkScript, _   = common.AddressToPkScript(net, address)
		changeAddress = address

		url string = domain + "/api/v1/inscribe/mrc20/transfer/commit"
		err error

		orderId          string = "2a133b84bf9c47e83edc203126ce46bbc3e4a7fb024dc18de5395087690ee00d"                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     //from respond of preApi
		revealPrePsbtRaw string = "70736274ff01009d02000000029242550effda2b45ae8162de9b2dfe716079d8bf685c1da06d5ff7c0566245f40000000000ffffffff00000000000000000000000000000000000000000000000000000000000000000000000000ffffffff0222020000000000001976a9144c8d3194f32e5c94478fe6e88cacc91f5384203f88ac2202000000000000160014428f2dac5ec003ca963bfed0a42b96e027c89e3800000000000100fd2f020200000000010222c3e855938f52a14b1a8b990500ad4201c1d2dea15ae73c71f80c0240635f95010000006b4830450221009b964890be124e02174a5ff270ca01868d87a1d25250b951d2cdeaa52dca376d02205a3f02f1feed065d1a99aabb6f49dd12177fe5bd12f0470e24314dc47c5262120121022fd11f8a9015c4eb5ba62bfecfda7cb37b71c2a316a1e979392079bb4e16bf65ffffffff9d9f3b8c233fb1dbfc791a72a419b5349cdf0629476238771659f1e9bd63df8b0000000000ffffffff0222020000000000001976a9144c8d3194f32e5c94478fe6e88cacc91f5384203f88ac2202000000000000160014428f2dac5ec003ca963bfed0a42b96e027c89e38000341a137552706fa4d4188c0e5b781e172cec0f48b2bbd5d7d3edf451078d7b4332d8a7c0c806e4d361c3756ee74f88888c535c94b094be1e987d4288799d4aa3dc901be201c3b4e0d72f4fed0e6f76ba4e1c984d88c746538b3cf26d8d7b4e5c1c2ef4a93ac0063067465737469640468696465122f66742f6d726332302f7472616e736665720000106170706c69636174696f6e2f6a736f6e4c655b7b226964223a22386536353938393932373562316430366462383730666265653962323933626337336432356530363363633836383630613664353263316531313039316539626930222c22616d6f756e74223a22323030222c22766f7574223a317d5d6821c11c3b4e0d72f4fed0e6f76ba4e1c984d88c746538b3cf26d8d7b4e5c1c2ef4a9300000000010304010000000001012b2a3605000000000022512050da47abf89649257f4709f0ea0e2df32c880e0896b52f1e662b8a146c7dd6d601030401000000000000" //from respond of preApi
		revealFee        int64  = 341546                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 //from respond of preApi
		revealInputIndex int64  = 1                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      //from respond of preApi
		revealAddress    string = "tb1p2rdy02lcjeyj2l68p8cw5r3d7vkgsrsgj66j78nx9w9pgmra6mtqsayyff"                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       //from respond of preApi
		serviceFee       int64  = 0                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      //from respond of preApi

		req = &request.Mrc20TransferCommitRequest{
			OrderId:          orderId,
			CommitTxRaw:      "",
			CommitTxOutIndex: 0,
			RevealPrePsbtRaw: "",
		}

		utxoOutTxId string = "8bdf63bde9f15916773862472906df9c34b519a4721a79fcdbb13f238c3b9f9d"
		utxoIndex   uint32 = 1
		utxoAmount  int64  = 256800
		utxoOutRaw  string = "02000000017ea4a0aa54c3af54524fb2a9f559c2c7eb8fa640b95475e1890f395cdead41d2000000006b483045022100c027c088ec7fc7f9d449f75cc883e3c61c5a4700894e303b257c6d81bea0b8280220477e12f9592ebf4a5fde89b938b05b491b20ce1accdefdf26b6249aa89056a450121022fd11f8a9015c4eb5ba62bfecfda7cb37b71c2a316a1e979392079bb4e16bf65ffffffff0240c2020000000000225120df6d32d7ff6011aca719e69117b2f447911429fe2f765e7008a396da237581ca809e0900000000001976a9144c8d3194f32e5c94478fe6e88cacc91f5384203f88ac00000000"

		tickerId          string = "8e659899275b1d06db870fbee9b293bc73d25e063cc86860a6d52c1e11091e9bi0"
		mrc20UtxoTxId     string = "955f6340020cf8713ce75aa1ded2c10142ad0005998b1a4ba1528f9355e8c322"
		mrc20UtxoIndex    uint32 = 1
		mrc20UtxoOutValue int64  = 546
		mrc20UtxoOutRaw   string = "0200000000010218e1d460894294930a5e3fbb12c7c01e1a9df46a4f660c9376ab416798da22e7000000006a473044022063f74a32f94cb87de84abde9e054cd7ee9c7de5cdfe5c04bac660ce3c685cbce02200102e75b25aa8a1445cf6b680e316db07768e7cae6376260e1620deb2d9f730c0121022fd11f8a9015c4eb5ba62bfecfda7cb37b71c2a316a1e979392079bb4e16bf65ffffffff1288faa99a89991bccbc6e27bccdac3f497aeccb5a2eb95fee54fd54ee5909600000000000ffffffff0222020000000000001976a9144c8d3194f32e5c94478fe6e88cacc91f5384203f88ac22020000000000001976a9144c8d3194f32e5c94478fe6e88cacc91f5384203f88ac0003419be920f201de2ba49a90a776f2adefd359f96f4a126aaa746dd34adf9ed7f9b1438f9cb364c5491383177bd82edcb99e65e8ea681302d4ce42de270dfbf27460019f20fb42fc0b0c3d06c27f0024d306ae6d57bcb93504e5fdb490772f7fc89e1643c5ac00630674657374696404686964650e2f66742f6d726332302f6d696e740000106170706c69636174696f6e2f6a736f6e4b7b226964223a22386536353938393932373562316430366462383730666265653962323933626337336432356530363363633836383630613664353263316531313039316539626930227d6821c0fb42fc0b0c3d06c27f0024d306ae6d57bcb93504e5fdb490772f7fc89e1643c500000000"

		mrc20Utxos = []*request.TransferMrc20Utxo{
			{
				UtxoTxId:     mrc20UtxoTxId,
				UtxoIndex:    mrc20UtxoIndex,
				UtxoOutValue: mrc20UtxoOutValue,
				TickerId:     tickerId,
				Amount:       "1000",
				Address:      address,
				PkScript:     pkScript,
				OutRaw:       mrc20UtxoOutRaw,
			},
		}

		revealPsbtBuilder *common.PsbtBuilder
		commitPsbtBuilder *common.PsbtBuilder
		inputs            []common.Input
		outputs           []common.Output
		inSigns           []*common.InputSign
		revealPinSigners  []*common.InputSign = make([]*common.InputSign, 0)

		minerFee       int64 = 0
		txSize         int64 = 0
		networkFeeRate int64 = 800
		txRawByte      []byte
		commitTx       = wire.NewMsgTx(2)
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
			UtxoType: common.NonWitness,
			Index:    0,
			OutRaw:   utxoOutRaw,
			//PkScript: pkScript,
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

	txRawByte, _ = hex.DecodeString(req.CommitTxRaw)
	err = commitTx.Deserialize(bytes.NewReader(txRawByte))
	if err != nil {
		t.Fatalf("Deserialize() error = %v,", err)
	}

	//sign reveal for pin input
	revealPsbtBuilder, err = common.NewPsbtBuilder(common.GetNetParams(net), revealPrePsbtRaw)
	if err != nil {
		t.Fatalf("NewPsbtBuilder() error = %v,", err)
	}

	txHash := commitTx.TxHash()
	commitPreOutPoint := wire.NewOutPoint(&txHash, 0)
	revealPsbtBuilder.PsbtUpdater.Upsbt.UnsignedTx.TxIn[revealInputIndex].PreviousOutPoint = *commitPreOutPoint

	for i, v := range mrc20Utxos {
		inSigner := &common.InputSign{
			UtxoType: common.NonWitness,
			Index:    i,
			OutRaw:   v.OutRaw,
			//PkScript:     "",
			RedeemScript: "",
			Amount:       uint64(v.UtxoOutValue),
			SighashType:  txscript.SigHashAll,
			PriHex:       priKeyHex,
			//MultiSigScript: "",
			//PreSigScript:   "",
		}
		revealPinSigners = append(revealPinSigners, inSigner)
	}

	err = revealPsbtBuilder.UpdateAndSignInput(revealPinSigners)
	if err != nil {
		t.Fatalf("UpdateAndSignInput() error = %v,", err)
	}

	req.RevealPrePsbtRaw, err = revealPsbtBuilder.ToString()
	if err != nil {
		t.Fatalf("ToString() error = %v,", err)
	}

	resp, err := tool.PostUrl(url, req, addHeaders(priKeyHex))
	if err != nil {
		t.Fatalf("PostUrl() error = %v,", err)
	}
	t.Logf("resp = %v,", resp)
	/*
	   transfer_test.go:247: resp = {"code":0,"message":"success","processingTime":14,"data":{"orderId":"d0e977e778c61dc0ec656eab2144b0c452bdb2a3679dcecde613acdb47ab1d93","commitTxId":"8bdf63bde9f15916773862472906df9c34b519a4721a79fcdbb13f238c3b9f9d","revealTxId":"f4456256c0f75f6da01d5c68bfd8796071fe2d9bde6281ae452bdaff0e554292"}},
	*/
}

//2024/06/24 14:14:28 new priviate key f84e6d3053881c4f254a782ca5368ef39bb5072a35ef58db94be2b4521e2837c
//2024/06/24 14:14:28 new public key 032d4fa2ecbf05035b4e4c84d099deb88cc2b91ca321c8b5b971a7c9829e1661fa
//2024/06/24 14:14:28 new legacy address mxhbxVVxEqPD17SzWESfjeRxkJN2nXPxNR
//2024/06/24 14:14:28 new native segwit address tb1qh37mew26wyggvr7q75d60m5urnvvuwhysw5syk
//2024/06/24 14:14:28 new nest segwit address 2NDbZJVoHraBCpwoQHtA6SH94txssiGErzm
//2024/06/24 14:14:28 new taproot address tb1pslytra085eljsj5sf9sjfes5gjmghzy7xznwrpr6qy0qrlkfh5hqaydnuj
//2024/06/24 14:14:28 restore taproot address tb1pslytra085eljsj5sf9sjfes5gjmghzy7xznwrpr6qy0qrlkfh5hqaydnuj

func TestMrc20TransferPreForSigwet(t *testing.T) {
	var (
		net         = "testnet"
		priKeyHex   = "d4e34195c8a62d5773bfb2ce65056defe30c4cbc8ef4ac8a4074728e04ff5c34"
		address     = "tb1qe6r2e0d7kl92ul489g7r7kfm4hz66qndtr88ft"
		pkScript, _ = common.AddressToPkScript(net, address)

		toAddrss1      = "tb1qg28jmtz7cqpu493mlmg2g2ukuqnu383c5s0mz6"
		toPkScript1, _ = common.AddressToPkScript(net, toAddrss1)
		toAmount       = "800"

		url string = domain + "/api/v1/inscribe/mrc20/transfer/pre"
		err error

		networkFeeRate int64  = 1000
		tickerId       string = "8e659899275b1d06db870fbee9b293bc73d25e063cc86860a6d52c1e11091e9bi0"

		req = &request.Mrc20TransferPreRequest{
			NetworkFeeRate: networkFeeRate,
			TickerId:       tickerId,
			Transfers: []*request.TransferMrc20Utxo{
				{
					UtxoTxId:     "02cae6084f5a4ccf0c08b866869fb81aa97cb792abbb4d9111e1350886ff432d",
					UtxoIndex:    0,
					UtxoOutValue: 546,
					TickerId:     tickerId,
					Amount:       "800",
					Address:      address,
					PkScript:     pkScript,
				},
			},
			ChangeAddress:  address,
			ChangeOutValue: 546,
			Mrc20Outs: []*request.Mrc20OutInfo{
				{
					Amount:   toAmount,
					Address:  toAddrss1,
					PkScript: toPkScript1,
					OutValue: 546,
				},
			},
		}
	)

	resp, err := tool.PostUrl(url, req, addHeaders(priKeyHex))
	if err != nil {
		t.Fatalf("PostUrl() error = %v,", err)
	}
	t.Logf("resp = %v,", resp)
	/*
	   transfer_test.go:309: resp = {"code":0,"message":"success","processingTime":6,"data":{"orderId":"843d764c5a8b95c1ad1a768e0cc4ad04de742851aefdf9bf99f320ad74f5108a","totalFee":254546,"revealFee":254546,"revealAddress":"tb1prqcp9fyt5uy43060qrd7mzfz4h6s32djxp0llzzvc66lds9ru76sp3j65e","serviceFee":0,"serviceAddress":"","revealPrePsbtRaw":"70736274ff01009a02000000022d43ff860835e111914dbbab92b77ca91ab89f8666b8080ccf4c5a4f08e6ca020000000000ffffffff00000000000000000000000000000000000000000000000000000000000000000000000000ffffffff022202000000000000160014ce86acbdbeb7caae7ea72a3c3f593badc5ad026d2202000000000000160014428f2dac5ec003ca963bfed0a42b96e027c89e38000000000001011f2202000000000000160014ce86acbdbeb7caae7ea72a3c3f593badc5ad026d010304010000000001012b52e2030000000000225120183012a48ba70958bf4f00dbed8922adf508a9b2305fff884cc6b5f6c0a3e7b501030401000000000000","revealInputIndex":1,"extra":{"controlBlockWitness":"c0d7a8ace3187b3e12c737eb378b5533c878f8431b36ad000cd33e2bcb1e5759d7","redeemScript":"20d7a8ace3187b3e12c737eb378b5533c878f8431b36ad000cd33e2bcb1e5759d7ac0063067465737469640468696465122f66742f6d726332302f7472616e736665720000106170706c69636174696f6e2f6a736f6e4c655b7b226964223a22386536353938393932373562316430366462383730666265653962323933626337336432356530363363633836383630613664353263316531313039316539626930222c22616d6f756e74223a22383030222c22766f7574223a317d5d68","xOnlyPubKey":"d7a8ace3187b3e12c737eb378b5533c878f8431b36ad000cd33e2bcb1e5759d7"}}},
	*/
}

func TestMrc20TransferCommitForSigwet(t *testing.T) {
	var (
		net           = "testnet"
		priKeyHex     = "d4e34195c8a62d5773bfb2ce65056defe30c4cbc8ef4ac8a4074728e04ff5c34"
		address       = "tb1qe6r2e0d7kl92ul489g7r7kfm4hz66qndtr88ft"
		pkScript, _   = common.AddressToPkScript(net, address)
		changeAddress = address

		url string = domain + "/api/v1/inscribe/mrc20/transfer/commit"
		err error

		orderId          string = "843d764c5a8b95c1ad1a768e0cc4ad04de742851aefdf9bf99f320ad74f5108a"                                                                                                                                                                                                                                                                                                                                                                                                                                                                           //from respond of preApi
		revealPrePsbtRaw string = "70736274ff01009a02000000022d43ff860835e111914dbbab92b77ca91ab89f8666b8080ccf4c5a4f08e6ca020000000000ffffffff00000000000000000000000000000000000000000000000000000000000000000000000000ffffffff022202000000000000160014ce86acbdbeb7caae7ea72a3c3f593badc5ad026d2202000000000000160014428f2dac5ec003ca963bfed0a42b96e027c89e38000000000001011f2202000000000000160014ce86acbdbeb7caae7ea72a3c3f593badc5ad026d010304010000000001012b52e2030000000000225120183012a48ba70958bf4f00dbed8922adf508a9b2305fff884cc6b5f6c0a3e7b501030401000000000000" //from respond of preApi
		revealFee        int64  = 254546                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       //from respond of preApi
		revealInputIndex int64  = 1                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            //from respond of preApi
		revealAddress    string = "tb1prqcp9fyt5uy43060qrd7mzfz4h6s32djxp0llzzvc66lds9ru76sp3j65e"                                                                                                                                                                                                                                                                                                                                                                                                                                                                             //from respond of preApi
		serviceFee       int64  = 0                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            //from respond of preApi

		req = &request.Mrc20TransferCommitRequest{
			OrderId:          orderId,
			CommitTxRaw:      "",
			CommitTxOutIndex: 0,
			RevealPrePsbtRaw: "",
		}

		utxoOutTxId string = "97444921b07e00ddf495cb16c07d330d7761893aee2b6f6211b2397baeb9f110"
		utxoIndex   uint32 = 0
		utxoAmount  int64  = 5000000

		tickerId          string = "8e659899275b1d06db870fbee9b293bc73d25e063cc86860a6d52c1e11091e9bi0"
		mrc20UtxoTxId     string = "02cae6084f5a4ccf0c08b866869fb81aa97cb792abbb4d9111e1350886ff432d"
		mrc20UtxoIndex    uint32 = 0
		mrc20UtxoOutValue int64  = 546
		mrc20UtxoOutRaw   string = ""

		mrc20Utxos = []*request.TransferMrc20Utxo{
			{
				UtxoTxId:     mrc20UtxoTxId,
				UtxoIndex:    mrc20UtxoIndex,
				UtxoOutValue: mrc20UtxoOutValue,
				TickerId:     tickerId,
				Amount:       "800",
				Address:      address,
				PkScript:     pkScript,
				OutRaw:       mrc20UtxoOutRaw,
			},
		}

		revealPsbtBuilder *common.PsbtBuilder
		commitPsbtBuilder *common.PsbtBuilder
		inputs            []common.Input
		outputs           []common.Output
		inSigns           []*common.InputSign
		revealPinSigners  []*common.InputSign = make([]*common.InputSign, 0)

		minerFee       int64 = 0
		txSize         int64 = 0
		networkFeeRate int64 = 1000
		txRawByte      []byte
		commitTx       = wire.NewMsgTx(2)
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
			//OutRaw:   utxoOutRaw,
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

	txRawByte, _ = hex.DecodeString(req.CommitTxRaw)
	err = commitTx.Deserialize(bytes.NewReader(txRawByte))
	if err != nil {
		t.Fatalf("Deserialize() error = %v,", err)
	}

	//sign reveal for pin input
	revealPsbtBuilder, err = common.NewPsbtBuilder(common.GetNetParams(net), revealPrePsbtRaw)
	if err != nil {
		t.Fatalf("NewPsbtBuilder() error = %v,", err)
	}

	txHash := commitTx.TxHash()
	commitPreOutPoint := wire.NewOutPoint(&txHash, 0)
	revealPsbtBuilder.PsbtUpdater.Upsbt.UnsignedTx.TxIn[revealInputIndex].PreviousOutPoint = *commitPreOutPoint

	for i, v := range mrc20Utxos {
		inSigner := &common.InputSign{
			UtxoType: common.Witness,
			Index:    i,
			//OutRaw:   v.OutRaw,
			PkScript:     v.PkScript,
			RedeemScript: "",
			Amount:       uint64(v.UtxoOutValue),
			SighashType:  txscript.SigHashAll,
			PriHex:       priKeyHex,
			//MultiSigScript: "",
			//PreSigScript:   "",
		}
		revealPinSigners = append(revealPinSigners, inSigner)
	}

	err = revealPsbtBuilder.UpdateAndSignInput(revealPinSigners)
	if err != nil {
		t.Fatalf("UpdateAndSignInput() error = %v,", err)
	}

	req.RevealPrePsbtRaw, err = revealPsbtBuilder.ToString()
	if err != nil {
		t.Fatalf("ToString() error = %v,", err)
	}

	resp, err := tool.PostUrl(url, req, addHeaders(priKeyHex))
	if err != nil {
		t.Fatalf("PostUrl() error = %v,", err)
	}
	t.Logf("resp = %v,", resp)
	/*
	   transfer_test.go:493: resp = {"code":0,"message":"success","processingTime":16,"data":{"orderId":"843d764c5a8b95c1ad1a768e0cc4ad04de742851aefdf9bf99f320ad74f5108a","commitTxId":"3860d96c2005cb58909dd44290a86f04565ea80a518055a2d909ca445ee5cded","revealTxId":"f09e96653309e26552353f2011b9ec4449ce9c5fe3ad58cc93816c6154a75147"}},
	*/
}
