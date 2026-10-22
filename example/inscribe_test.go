package example

import (
	"bytes"
	"encoding/hex"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"metaid-market-service/common"
	"metaid-market-service/controller/request"
	"metaid-market-service/service/mrc20_service"
	"metaid-market-service/tool"
	"testing"
)

func TestMrc20DeployPre(t *testing.T) {
	var (
		net         = "testnet"
		priKeyHex   = "d4e34195c8a62d5773bfb2ce65056defe30c4cbc8ef4ac8a4074728e04ff5c34"
		address     = "tb1qe6r2e0d7kl92ul489g7r7kfm4hz66qndtr88ft"
		pkScript, _ = common.AddressToPkScript(net, address)

		url string = domain + "/api/v1/inscribe/mrc20/deploy/pre"
		err error

		deployData *mrc20_service.Mrc20DeployData = &mrc20_service.Mrc20DeployData{
			Tick:         "TDD22",
			TokenName:    "test deploy",
			Decimals:     "2",
			AmtPerMint:   "100",
			MintCount:    "100",
			PremineCount: "10",
			BeginHeight:  "",
			EndHeight:    "",
			Metadata:     "test",
			PinCheck:     nil,
			PayCheck:     nil,
		}
		payload string = tool.AnyToStr(deployData)

		networkFeeRate int64 = 100

		req = &request.Mrc20DeployPreRequest{
			NetworkFeeRate: networkFeeRate,
			Payload:        payload,
			Address:        address,
		}
	)
	_ = pkScript
	resp, err := tool.PostUrl(url, req, addHeaders(priKeyHex))
	if err != nil {
		t.Fatalf("PostUrl() error = %v,", err)
	}
	t.Logf("resp = %v,", resp)
	/*
	   inscribe_test.go:55: resp = {"code":0,"message":"success","processingTime":15,"data":{"orderId":"2f0e8437d3b4864abcd35ebed729f2c55d9c4c7b840d15f5fb300bec4e043fae","totalFee":22092,"minerFee":22092,"serviceFee":0,"revealAddress":"tb1pkuya70ukrttejd4lqmd8sewhsa70z9qaznadyjekqwttg65wlezqls2t3x","extra":null}},
	*/
}

func TestMrc20DeployCommit(t *testing.T) {
	var (
		net           = "testnet"
		priKeyHex     = "d4e34195c8a62d5773bfb2ce65056defe30c4cbc8ef4ac8a4074728e04ff5c34"
		address       = "tb1qe6r2e0d7kl92ul489g7r7kfm4hz66qndtr88ft"
		pkScript, _   = common.AddressToPkScript(net, address)
		changeAddress = address

		url string = domain + "/api/v1/inscribe/mrc20/deploy/commit"
		err error

		orderId       string = "62b8cd8cdce88ee4b84b69c413334246f4fbd263cab7d84bbc8cf3b4c9265422"
		revealFee     int64  = 22092
		revealAddress string = "tb1pt6c0mz56qmws7jzue8429ca6a99es93gjeyj9t5as7sedfr2xngs59jfn8"

		req = &request.Mrc20DeployCommitRequest{
			OrderId:          orderId,
			CommitTxRaw:      "",
			CommitTxOutIndex: 0,
		}

		utxoOutTxId string = "477e0442bd3ecdb8d9fa6b7d5c2188d7db9071c46f08b32623b3fc96196897f2"
		utxoIndex   uint32 = 1
		utxoAmount  int64  = 2713600

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

	commitPsbtBuilder.PsbtUpdater.Upsbt.UnsignedTx.TxOut[1].Value = int64(utxoAmount - revealFee - minerFee)
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
	   inscribe_test.go:170: resp = {"code":0,"message":"success","processingTime":23,"data":{"orderId":"62b8cd8cdce88ee4b84b69c413334246f4fbd263cab7d84bbc8cf3b4c9265422","commitTxId":"c8b8d475021dbb8df3e5a1a4303d4962f0084c4773b299537158e04d4d58d1db","revealTxId":""}},
	*/
}

func TestMrc20MintPre(t *testing.T) {
	var (
		net         = "testnet"
		priKeyHex   = "d4e34195c8a62d5773bfb2ce65056defe30c4cbc8ef4ac8a4074728e04ff5c34"
		address     = "tb1qe6r2e0d7kl92ul489g7r7kfm4hz66qndtr88ft"
		pkScript, _ = common.AddressToPkScript(net, address)

		url string = domain + "/api/v1/inscribe/mrc20/mint/pre"
		err error

		networkFeeRate int64  = 100
		tickerId       string = "5bff13d5602857af613a9f0259b494b3bde53b747f820871f964591bdf972e3fi0"

		req = &request.Mrc20MintPreRequest{
			NetworkFeeRate: networkFeeRate,
			TickerId:       tickerId,
			MintPins: []*request.MintPinInfo{
				{
					PinId:           "141ba0aa80cf320ef359883bbc413a603b38a268cc24f998b3490ad4c909a861i0",
					PinUtxoTxId:     "2387aca280a7f709e0b8085141441d66b80048fb75828c6d4651a5abaa7de3ec",
					PinUtxoIndex:    0,
					PinUtxoOutValue: 546,
					Address:         address,
					PkScript:        pkScript,
				},
			},
			OutAddress: address,
			OutValue:   546,
		}
	)
	_ = pkScript
	resp, err := tool.PostUrl(url, req, addHeaders(priKeyHex))
	if err != nil {
		t.Fatalf("PostUrl() error = %v,", err)
	}
	t.Logf("resp = %v,", resp)
	/*
	   inscribe_test.go:211: resp = {"code":0,"message":"success","processingTime":18,"data":{"orderId":"105a923198c1dd4679f59db04a84d2071b50c26d426b4d09e51dbcfcbd3faf13","totalFee":42292,"revealFee":42292,"revealAddress":"tb1p6x8sr74svknehqrjucpl5ezut4k4jqhlaac5a6gpnrfdqhmqkkdqf9wtg8","serviceFee":0,"serviceAddress":"","revealPrePsbtRaw":"70736274ff0100d80200000002ece37daaaba551466d8c8275fb4800b8661d44415108b8e009f7a780a2ac87230000000000ffffffff00000000000000000000000000000000000000000000000000000000000000000000000000ffffffff042202000000000000160014ce86acbdbeb7caae7ea72a3c3f593badc5ad026d2202000000000000160014ce86acbdbeb7caae7ea72a3c3f593badc5ad026d2202000000000000160014ce86acbdbeb7caae7ea72a3c3f593badc5ad026d1027000000000000160014df5bdae6ee21386d273d3bf503e36f6568154c99000000000001011f2202000000000000160014ce86acbdbeb7caae7ea72a3c3f593badc5ad026d010304010000000001012b34a5000000000000225120d18f01fab065a79b8072e603fa645c5d6d5902ffef714ee90198d2d05f60b59a010304010000000000000000","revealInputIndex":1,"extra":{}}},
	*/
}

func TestMrc20MintCommit(t *testing.T) {
	var (
		net           = "testnet"
		priKeyHex     = "d4e34195c8a62d5773bfb2ce65056defe30c4cbc8ef4ac8a4074728e04ff5c34"
		address       = "tb1qe6r2e0d7kl92ul489g7r7kfm4hz66qndtr88ft"
		pkScript, _   = common.AddressToPkScript(net, address)
		changeAddress = address

		url string = domain + "/api/v1/inscribe/mrc20/mint/commit"
		err error

		orderId          string = "105a923198c1dd4679f59db04a84d2071b50c26d426b4d09e51dbcfcbd3faf13"
		revealPrePsbtRaw string = "70736274ff0100d80200000002ece37daaaba551466d8c8275fb4800b8661d44415108b8e009f7a780a2ac87230000000000ffffffff00000000000000000000000000000000000000000000000000000000000000000000000000ffffffff042202000000000000160014ce86acbdbeb7caae7ea72a3c3f593badc5ad026d2202000000000000160014ce86acbdbeb7caae7ea72a3c3f593badc5ad026d2202000000000000160014ce86acbdbeb7caae7ea72a3c3f593badc5ad026d1027000000000000160014df5bdae6ee21386d273d3bf503e36f6568154c99000000000001011f2202000000000000160014ce86acbdbeb7caae7ea72a3c3f593badc5ad026d010304010000000001012b34a5000000000000225120d18f01fab065a79b8072e603fa645c5d6d5902ffef714ee90198d2d05f60b59a010304010000000000000000"
		revealFee        int64  = 42292
		revealInputIndex int64  = 1
		revealAddress    string = "tb1p6x8sr74svknehqrjucpl5ezut4k4jqhlaac5a6gpnrfdqhmqkkdqf9wtg8"
		serviceFee       int64  = 0

		req = &request.Mrc20MintCommitRequest{
			OrderId:          orderId,
			CommitTxRaw:      "",
			CommitTxOutIndex: 0,
			RevealPrePsbtRaw: "",
		}

		utxoOutTxId string = "8c6c919563e6c5460428b91f85f85cfcc92897b59762b7fe17ba6994f4595fc0"
		utxoIndex   uint32 = 1
		utxoAmount  int64  = 2545571

		pinId           string = "141ba0aa80cf320ef359883bbc413a603b38a268cc24f998b3490ad4c909a861i0"
		pinUxtoTxId     string = "2387aca280a7f709e0b8085141441d66b80048fb75828c6d4651a5abaa7de3ec"
		pinUxtoIndex    uint32 = 0
		pinUtxoOutValue int64  = 546

		mintPins = []*request.MintPinInfo{
			{
				PinId:           pinId,
				PinUtxoTxId:     pinUxtoTxId,
				PinUtxoIndex:    pinUxtoIndex,
				PinUtxoOutValue: pinUtxoOutValue,
				Address:         address,
				PkScript:        pkScript,
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
		networkFeeRate int64 = 100
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

	for i, v := range mintPins {
		inSigner := &common.InputSign{
			UtxoType: common.Witness,
			Index:    i,
			//OutRaw:         "",
			PkScript:     v.PkScript,
			RedeemScript: "",
			Amount:       uint64(v.PinUtxoOutValue),
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
	   inscribe_test.go:392: resp = {"code":0,"message":"success","processingTime":32,"data":{"orderId":"105a923198c1dd4679f59db04a84d2071b50c26d426b4d09e51dbcfcbd3faf13","commitTxId":"0e83784be782fd181a45cc0e897be814c13c2bc8f609d3387f7febe5c583331f","revealTxId":"2196325c5c2a1dff7f8c3145e1c91a1d2129b2cb198aa6f4625b6be823f73b9f"}},
	*/
}

func TestMrc20MintPreForLegacy(t *testing.T) {
	var (
		net         = "testnet"
		priKeyHex   = "8e2a95af06f8e03f1ea2a2b0b85e35757bbd27b8bd03f8cae934bf6f90fa4f71"
		address     = "mnVinwawKEtqyFJk3BHqmaEGkKNGrxYRh9"
		pkScript, _ = common.AddressToPkScript(net, address)

		url string = domain + "/api/v1/inscribe/mrc20/mint/pre"
		err error

		networkFeeRate int64  = 800
		tickerId       string = "8e659899275b1d06db870fbee9b293bc73d25e063cc86860a6d52c1e11091e9bi0"

		req = &request.Mrc20MintPreRequest{
			NetworkFeeRate: networkFeeRate,
			TickerId:       tickerId,
			MintPins: []*request.MintPinInfo{
				{
					PinId:           "c98897e0d969a59454a31c07b5a1191567b1303c79b4544126ef42d05182d86fi0",
					PinUtxoTxId:     "e722da986741ab76930c664f6af49d1a1ec0c712bb3f5e0a9394428960d4e118",
					PinUtxoIndex:    0,
					PinUtxoOutValue: 546,
					Address:         address,
					PkScript:        pkScript,
				},
			},
			OutAddress: address,
			OutValue:   546,
		}
	)

	resp, err := tool.PostUrl(url, req, addHeaders(priKeyHex))
	if err != nil {
		t.Fatalf("PostUrl() error = %v,", err)
	}
	t.Logf("resp = %v,", resp)
	/*
	   inscribe_test.go:271: resp = {"code":0,"message":"success","processingTime":11,"data":{"orderId":"3aa60305f39c1f21f186010eff0bef25b583eda23d8ac31cc145342853150ba5","totalFee":180800,"revealFee":180800,"revealAddress":"tb1pmakn94llvqg6efceu6g30vh5g7g3g2079am9uuqg5wtd5gm4s89qa43wee","serviceFee":0,"serviceAddress":"","revealPrePsbtRaw":"70736274ff0100a0020000000218e1d460894294930a5e3fbb12c7c01e1a9df46a4f660c9376ab416798da22e70000000000ffffffff00000000000000000000000000000000000000000000000000000000000000000100000000ffffffff0222020000000000001976a9144c8d3194f32e5c94478fe6e88cacc91f5384203f88ac22020000000000001976a9144c8d3194f32e5c94478fe6e88cacc91f5384203f88ac00000000000100fd2f01020000000001026fd88251d042ef264154b4793c30b1671519a1b5071ca35494a569d9e09788c90000000000ffffffff5e17dedb500bc190a6af09f4f344f26b7f03801e52b029c6aac9e6a281c3a3470300000000ffffffff0222020000000000001976a9144c8d3194f32e5c94478fe6e88cacc91f5384203f88acbaaa9900000000002251200d98974ddaf02ae38567d7fce1c6569a20426c1cc0e7a61bb6f08335f5dbd4f40140c17cde9a2164a3fd5f3463b42e89baf48b8fb9acd2414d2e0335686bb6d4f0fdc2fb2994ce222e5efcc262ed9adeb37191402dde7fb91a15fb46e5c3f50594310140f2faa628861bae3eee5a392d441266049ff28ec04fb776088a7ceb8310ff8a659a5fdf9e0eecc5c9698fc63688778f596f83faa05bf2e1b20c1d015cfb56d3a300000000010304010000000001012b40c2020000000000225120df6d32d7ff6011aca719e69117b2f447911429fe2f765e7008a396da237581ca01030401000000000000","revealInputIndex":1}},
	*/
}

func TestMrc20MintCommitForLegacy(t *testing.T) {
	var (
		net           = "testnet"
		priKeyHex     = "8e2a95af06f8e03f1ea2a2b0b85e35757bbd27b8bd03f8cae934bf6f90fa4f71"
		address       = "mnVinwawKEtqyFJk3BHqmaEGkKNGrxYRh9"
		pkScript, _   = common.AddressToPkScript(net, address)
		changeAddress = address

		url string = domain + "/api/v1/inscribe/mrc20/mint/commit"
		err error

		orderId          string = "3aa60305f39c1f21f186010eff0bef25b583eda23d8ac31cc145342853150ba5"                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           //from respond of preApi
		revealPrePsbtRaw string = "70736274ff0100a0020000000218e1d460894294930a5e3fbb12c7c01e1a9df46a4f660c9376ab416798da22e70000000000ffffffff00000000000000000000000000000000000000000000000000000000000000000100000000ffffffff0222020000000000001976a9144c8d3194f32e5c94478fe6e88cacc91f5384203f88ac22020000000000001976a9144c8d3194f32e5c94478fe6e88cacc91f5384203f88ac00000000000100fd2f01020000000001026fd88251d042ef264154b4793c30b1671519a1b5071ca35494a569d9e09788c90000000000ffffffff5e17dedb500bc190a6af09f4f344f26b7f03801e52b029c6aac9e6a281c3a3470300000000ffffffff0222020000000000001976a9144c8d3194f32e5c94478fe6e88cacc91f5384203f88acbaaa9900000000002251200d98974ddaf02ae38567d7fce1c6569a20426c1cc0e7a61bb6f08335f5dbd4f40140c17cde9a2164a3fd5f3463b42e89baf48b8fb9acd2414d2e0335686bb6d4f0fdc2fb2994ce222e5efcc262ed9adeb37191402dde7fb91a15fb46e5c3f50594310140f2faa628861bae3eee5a392d441266049ff28ec04fb776088a7ceb8310ff8a659a5fdf9e0eecc5c9698fc63688778f596f83faa05bf2e1b20c1d015cfb56d3a300000000010304010000000001012b40c2020000000000225120df6d32d7ff6011aca719e69117b2f447911429fe2f765e7008a396da237581ca01030401000000000000" //from respond of preApi
		revealFee        int64  = 180800                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       //from respond of preApi
		revealInputIndex int64  = 1                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            //from respond of preApi
		revealAddress    string = "tb1pmakn94llvqg6efceu6g30vh5g7g3g2079am9uuqg5wtd5gm4s89qa43wee"                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             //from respond of preApi
		serviceFee       int64  = 0                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            //from respond of preApi

		req = &request.Mrc20MintCommitRequest{
			OrderId:          orderId,
			CommitTxRaw:      "",
			CommitTxOutIndex: 0,
			RevealPrePsbtRaw: "",
		}

		utxoOutTxId string = "d241adde5c390f89e17554b940a68febc7c259f5a9b24f5254afc354aaa0a47e"
		utxoIndex   uint32 = 0
		utxoAmount  int64  = 1000000
		utxoOutRaw  string = "02000000000101eb70769e0897c451b872223f90960c5f617c68602be75cbeaeab81ad0dbd8b2b0100000000ffffffff0240420f00000000001976a9144c8d3194f32e5c94478fe6e88cacc91f5384203f88ac1a968500000000002251200d98974ddaf02ae38567d7fce1c6569a20426c1cc0e7a61bb6f08335f5dbd4f401402977dac7d153a6b64c252020c08517e5ebf42a87b36031533db79f31cdda71b94b3d720b1bd98c5cfe3a38d88a5671b0ea8eaa24c0a5e4cfc9ca674a379699e200000000"

		pinId           string = "c98897e0d969a59454a31c07b5a1191567b1303c79b4544126ef42d05182d86fi0"
		pinUxtoTxId     string = "e722da986741ab76930c664f6af49d1a1ec0c712bb3f5e0a9394428960d4e118"
		pinUxtoIndex    uint32 = 0
		pinUtxoOutValue int64  = 546
		pinUtxoOutRaw   string = "020000000001026fd88251d042ef264154b4793c30b1671519a1b5071ca35494a569d9e09788c90000000000ffffffff5e17dedb500bc190a6af09f4f344f26b7f03801e52b029c6aac9e6a281c3a3470300000000ffffffff0222020000000000001976a9144c8d3194f32e5c94478fe6e88cacc91f5384203f88acbaaa9900000000002251200d98974ddaf02ae38567d7fce1c6569a20426c1cc0e7a61bb6f08335f5dbd4f40140c17cde9a2164a3fd5f3463b42e89baf48b8fb9acd2414d2e0335686bb6d4f0fdc2fb2994ce222e5efcc262ed9adeb37191402dde7fb91a15fb46e5c3f50594310140f2faa628861bae3eee5a392d441266049ff28ec04fb776088a7ceb8310ff8a659a5fdf9e0eecc5c9698fc63688778f596f83faa05bf2e1b20c1d015cfb56d3a300000000"

		mintPins = []*request.MintPinInfo{
			{
				PinId:           pinId,
				PinUtxoTxId:     pinUxtoTxId,
				PinUtxoIndex:    pinUxtoIndex,
				PinUtxoOutValue: pinUtxoOutValue,
				Address:         address,
				PkScript:        pkScript,
				OutRaw:          pinUtxoOutRaw,
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

	for i, v := range mintPins {
		inSigner := &common.InputSign{
			UtxoType: common.NonWitness,
			Index:    i,
			OutRaw:   v.OutRaw,
			//PkScript:     "",
			RedeemScript: "",
			Amount:       uint64(v.PinUtxoOutValue),
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
	   inscribe_test.go:455: resp = {"code":0,"message":"success","processingTime":14,"data":{"orderId":"3aa60305f39c1f21f186010eff0bef25b583eda23d8ac31cc145342853150ba5","commitTxId":"600959ee54fd54ee5fb92e5acbec7a493faccdbc276ebccc1b99899aa9fa8812","revealTxId":"955f6340020cf8713ce75aa1ded2c10142ad0005998b1a4ba1528f9355e8c322"}},
	*/
}

func TestMrc20MintPreForNestSegwit(t *testing.T) {
	var (
		net         = "testnet"
		priKeyHex   = "ddecb5d9064b202f6b3f4c154a3f09750ea7b5341134373cc0c679a10cb397bf"
		address     = "2NBB3b8PwTTMZRobry8AWkq99s5WBMEJ87D"
		pkScript, _ = common.AddressToPkScript(net, address)

		url string = domain + "/api/v1/inscribe/mrc20/mint/pre"
		err error

		networkFeeRate int64  = 400
		tickerId       string = "8e659899275b1d06db870fbee9b293bc73d25e063cc86860a6d52c1e11091e9bi0"

		req = &request.Mrc20MintPreRequest{
			NetworkFeeRate: networkFeeRate,
			TickerId:       tickerId,
			MintPins: []*request.MintPinInfo{
				{
					PinId:           "c32453be098187bdb394932b4a5f34a94090a3ef7d8a71a2d6739c6ea9e211b1i0",
					PinUtxoTxId:     "78dc5062dccd8e5595764568a192c3ba36cbfaa29cb11461598fe86de0ad8541",
					PinUtxoIndex:    0,
					PinUtxoOutValue: 546,
					Address:         address,
					PkScript:        pkScript,
				},
			},
			OutAddress: address,
			OutValue:   546,
		}
	)

	resp, err := tool.PostUrl(url, req, addHeaders(priKeyHex))
	if err != nil {
		t.Fatalf("PostUrl() error = %v,", err)
	}
	t.Logf("resp = %v,", resp)
	/*
	   inscribe_test.go:498: resp = {"code":0,"message":"success","processingTime":6,"data":{"orderId":"c55048d1a863f9830f539c9785f1c8d17ec2b43ffa98880015b4e24b624acff7","totalFee":88800,"revealFee":88800,"revealAddress":"tb1p4hmvqh0fvnwygfzkrktatl3cgjdwk6u5jclajkky3zue9cxa7lustgpc2y","serviceFee":0,"serviceAddress":"","revealPrePsbtRaw":"70736274ff01009c02000000024185ade06de88f596114b19ca2facb36bac392a168457695558ecddc6250dc780000000000ffffffff00000000000000000000000000000000000000000000000000000000000000000100000000ffffffff02220200000000000017a914c4a7eb5ee40bac7082a08fdcc4e085311243e9b287220200000000000017a914c4a7eb5ee40bac7082a08fdcc4e085311243e9b2870000000000010120220200000000000017a914c4a7eb5ee40bac7082a08fdcc4e085311243e9b287010304010000000001012be05a010000000000225120adf6c05de964dc4424561d97d5fe38449aeb6b94963fd95ac488b992e0ddf7f901030401000000000000","revealInputIndex":1}},
	*/
}

func TestMrc20MintCommitForNestSegwit(t *testing.T) {
	var (
		net           = "testnet"
		priKeyHex     = "ddecb5d9064b202f6b3f4c154a3f09750ea7b5341134373cc0c679a10cb397bf"
		address       = "2NBB3b8PwTTMZRobry8AWkq99s5WBMEJ87D"
		pkScript, _   = common.AddressToPkScript(net, address)
		redeemScript  = ""
		changeAddress = address

		url string = domain + "/api/v1/inscribe/mrc20/mint/commit"
		err error

		orderId          string = "c55048d1a863f9830f539c9785f1c8d17ec2b43ffa98880015b4e24b624acff7"
		revealPrePsbtRaw string = "70736274ff01009c02000000024185ade06de88f596114b19ca2facb36bac392a168457695558ecddc6250dc780000000000ffffffff00000000000000000000000000000000000000000000000000000000000000000100000000ffffffff02220200000000000017a914c4a7eb5ee40bac7082a08fdcc4e085311243e9b287220200000000000017a914c4a7eb5ee40bac7082a08fdcc4e085311243e9b2870000000000010120220200000000000017a914c4a7eb5ee40bac7082a08fdcc4e085311243e9b287010304010000000001012be05a010000000000225120adf6c05de964dc4424561d97d5fe38449aeb6b94963fd95ac488b992e0ddf7f901030401000000000000"
		revealFee        int64  = 88800
		revealInputIndex int64  = 1
		revealAddress    string = "tb1p4hmvqh0fvnwygfzkrktatl3cgjdwk6u5jclajkky3zue9cxa7lustgpc2y"
		serviceFee       int64  = 0

		req = &request.Mrc20MintCommitRequest{
			OrderId:          orderId,
			CommitTxRaw:      "",
			CommitTxOutIndex: 0,
			RevealPrePsbtRaw: "",
		}

		utxoOutTxId string = "26b953ed5a2d3ce769ad69a1ee1ec5bf0f8983d6386045d9ee6303c107d8aa01"
		utxoIndex   uint32 = 0
		utxoAmount  int64  = 500000

		pinId           string = "c32453be098187bdb394932b4a5f34a94090a3ef7d8a71a2d6739c6ea9e211b1i0"
		pinUxtoTxId     string = "78dc5062dccd8e5595764568a192c3ba36cbfaa29cb11461598fe86de0ad8541"
		pinUxtoIndex    uint32 = 0
		pinUtxoOutValue int64  = 546

		mintPins = []*request.MintPinInfo{
			{
				PinId:           pinId,
				PinUtxoTxId:     pinUxtoTxId,
				PinUtxoIndex:    pinUxtoIndex,
				PinUtxoOutValue: pinUtxoOutValue,
				Address:         address,
				PkScript:        pkScript,
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
		networkFeeRate int64 = 400
		txRawByte      []byte
		commitTx       = wire.NewMsgTx(2)
	)
	//nested segwit address
	privateKeyBytes, err := hex.DecodeString(priKeyHex)
	if err != nil {
		t.Errorf("DecodeString() error = %v,", err)
		return
	}
	privateKey, _ := btcec.PrivKeyFromBytes(privateKeyBytes)
	nativeSegwitAddress, err := btcutil.NewAddressWitnessPubKeyHash(btcutil.Hash160(privateKey.PubKey().SerializeCompressed()), common.GetNetParams(net))
	if err != nil {
		t.Errorf("NewAddressWitnessPubKeyHash() error = %v,", err)
		return
	}

	pkScriptByte, err := txscript.PayToAddrScript(nativeSegwitAddress)
	if err != nil {
		t.Errorf("PayToAddrScript() error = %v,", err)
		return
	}
	redeemScript = hex.EncodeToString(pkScriptByte)

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
			PkScript:     pkScript,
			RedeemScript: redeemScript,
			Amount:       uint64(utxoAmount),
			SighashType:  txscript.SigHashAll,
			PriHex:       priKeyHex,
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

	for i, v := range mintPins {
		inSigner := &common.InputSign{
			UtxoType: common.Witness,
			Index:    i,
			//OutRaw:         "",
			PkScript:     v.PkScript,
			RedeemScript: redeemScript,
			Amount:       uint64(v.PinUtxoOutValue),
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
	   inscribe_test.go:699: resp = {"code":0,"message":"success","processingTime":14,"data":{"orderId":"c55048d1a863f9830f539c9785f1c8d17ec2b43ffa98880015b4e24b624acff7","commitTxId":"55f1559e28d5fe6437faff18e20b32fea85047fbbe4c61e17ea5ab7096429d3f","revealTxId":"5e726d77f43f65977194d71374e09f2b6b50576de1232e42f9a1229f6b067b92"}},
	*/
}

func TestMrc20MintPreForTaproot(t *testing.T) {
	var (
		net         = "testnet"
		priKeyHex   = "f6d6f6c04cfb465a87135f3ff0b100a510f70819ff15c0c2f6d9c8b287608d9d"
		address     = "tb1phwnwarkjfd7wmmwsfn9tw59eulx4wgnzhd6russw74hwh68rfy5qc55q0s"
		pkScript, _ = common.AddressToPkScript(net, address)

		url string = domain + "/api/v1/inscribe/mrc20/mint/pre"
		err error

		networkFeeRate int64  = 100
		tickerId       string = "c0170747548566ae27880dba7455b63853f31f00c4b1aa709b74e446e1c5c093i0"

		req = &request.Mrc20MintPreRequest{
			NetworkFeeRate: networkFeeRate,
			TickerId:       tickerId,
			MintPins: []*request.MintPinInfo{
				{
					PinId:           "5accb1bbce04f1cfa0a19f7174c571036efa1077572a4ff0d1fbfe8b5100b450i0",
					PinUtxoTxId:     "e0ffbb1084e1042083bcb182a308d2f81e2783b89744b4b4026fda3ab89d5479",
					PinUtxoIndex:    0,
					PinUtxoOutValue: 546,
					Address:         address,
					PkScript:        pkScript,
				},
			},
			OutAddress: address,
			OutValue:   546,
		}
	)

	resp, err := tool.PostUrl(url, req, addHeaders(priKeyHex))
	if err != nil {
		t.Fatalf("PostUrl() error = %v,", err)
	}
	t.Logf("resp = %v,", resp)
	/*
	   inscribe_test.go:740: resp = {"code":0,"message":"success","processingTime":6,"data":{"orderId":"0c90e56b29b2cc56db1f9260223f77ec7dadbb2e6b93516d587897d141bccf9a","totalFee":26546,"revealFee":26546,"revealAddress":"tb1pj29jy2ka6h5rzrnz045k55wln3upvmxkeaetuxe3zmy87ck4aeksml9r3h","serviceFee":0,"serviceAddress":"","revealPrePsbtRaw":"70736274ff0100b2020000000279549db83ada6f02b4b44497b883271ef8d208a382b1bc832004e18410bbffe00000000000ffffffff00000000000000000000000000000000000000000000000000000000000000000000000000ffffffff022202000000000000225120bba6ee8ed24b7cededd04ccab750b9e7cd572262bb743e420ef56eebe8e349282202000000000000225120bba6ee8ed24b7cededd04ccab750b9e7cd572262bb743e420ef56eebe8e34928000000000001012b2202000000000000225120bba6ee8ed24b7cededd04ccab750b9e7cd572262bb743e420ef56eebe8e34928010304010000000001012bb267000000000000225120928b222addd5e8310e627d696a51df9c78166cd6cf72be1b3116c87f62d5ee6d01030401000000000000","revealInputIndex":1,"extra":{}}},
	*/
}

func TestMrc20MintCommitForTaproot(t *testing.T) {
	var (
		net           = "testnet"
		priKeyHex     = "f6d6f6c04cfb465a87135f3ff0b100a510f70819ff15c0c2f6d9c8b287608d9d"
		address       = "tb1phwnwarkjfd7wmmwsfn9tw59eulx4wgnzhd6russw74hwh68rfy5qc55q0s"
		pkScript, _   = common.AddressToPkScript(net, address)
		changeAddress = address

		url string = domain + "/api/v1/inscribe/mrc20/mint/commit"
		err error

		orderId          string = "0c90e56b29b2cc56db1f9260223f77ec7dadbb2e6b93516d587897d141bccf9a"
		revealPrePsbtRaw string = "70736274ff0100b2020000000279549db83ada6f02b4b44497b883271ef8d208a382b1bc832004e18410bbffe00000000000ffffffff00000000000000000000000000000000000000000000000000000000000000000000000000ffffffff022202000000000000225120bba6ee8ed24b7cededd04ccab750b9e7cd572262bb743e420ef56eebe8e349282202000000000000225120bba6ee8ed24b7cededd04ccab750b9e7cd572262bb743e420ef56eebe8e34928000000000001012b2202000000000000225120bba6ee8ed24b7cededd04ccab750b9e7cd572262bb743e420ef56eebe8e34928010304010000000001012bb267000000000000225120928b222addd5e8310e627d696a51df9c78166cd6cf72be1b3116c87f62d5ee6d01030401000000000000"
		revealFee        int64  = 26546
		revealInputIndex int64  = 1
		revealAddress    string = "tb1pj29jy2ka6h5rzrnz045k55wln3upvmxkeaetuxe3zmy87ck4aeksml9r3h"
		serviceFee       int64  = 0

		req = &request.Mrc20MintCommitRequest{
			OrderId:          orderId,
			CommitTxRaw:      "",
			CommitTxOutIndex: 0,
			RevealPrePsbtRaw: "",
		}

		utxoOutTxId string = "7fc2aa0d5566397b1d45ce235736b622c5a33559ce62310234594e8c25074ccb"
		utxoIndex   uint32 = 0
		utxoAmount  int64  = 500000

		pinId           string = "5accb1bbce04f1cfa0a19f7174c571036efa1077572a4ff0d1fbfe8b5100b450i0"
		pinUxtoTxId     string = "e0ffbb1084e1042083bcb182a308d2f81e2783b89744b4b4026fda3ab89d5479"
		pinUxtoIndex    uint32 = 0
		pinUtxoOutValue int64  = 546

		mintPins = []*request.MintPinInfo{
			{
				PinId:           pinId,
				PinUtxoTxId:     pinUxtoTxId,
				PinUtxoIndex:    pinUxtoIndex,
				PinUtxoOutValue: pinUtxoOutValue,
				Address:         address,
				PkScript:        pkScript,
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
		networkFeeRate int64 = 100
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
			UtxoType: common.Taproot,
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

	for i, v := range mintPins {
		inSigner := &common.InputSign{
			UtxoType: common.Taproot,
			Index:    i,
			//OutRaw:         "",
			PkScript:     v.PkScript,
			RedeemScript: "",
			Amount:       uint64(v.PinUtxoOutValue),
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
	   inscribe_test.go:921: resp = {"code":0,"message":"success","processingTime":20,"data":{"orderId":"0c90e56b29b2cc56db1f9260223f77ec7dadbb2e6b93516d587897d141bccf9a","commitTxId":"2a287936ead98c2d60afda12c9369ccdae77b63a3ecf150675aeffe09a77ec34","revealTxId":"7a932f0213facf7f58a58e9812d1a3b40cbb7326e011b9e7290a9b2a60f5d564"}},
	*/
}
