package example

import (
	"encoding/hex"
	"fmt"
	"log"
	"metaid-market-service/common"
	"metaid-market-service/controller/auth"
	"metaid-market-service/controller/request"
	"metaid-market-service/models"
	"metaid-market-service/tool"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/txscript"
)

func TestPushMarketOrder(t *testing.T) {
	var (
		net       string = "testnet"
		chain     string = ""
		priKeyHex string = "f0f4d09a90ff279d25036b11da569690e60bcc5cbab0474eeb271f3623f0f483"
		address   string = "2Mw4oj2jNTboEauHBhNyDLJwtGSNWu5rybM"

		txId     string = "40b028882478db9b0cff1862d1c05458317be231dbfeaa5094f17da2a49e10a7" // pin text
		outIndex uint32 = 0
		assetId  string = "5fd9df151bb152dd091280593edb605f77427ca1f3b589a18e62d49407884f4di0"
		outValue uint64 = 546
		//sellPriceAmount uint64 = 30000
		sellPriceAmount uint64 = 2000
		pkScript, _            = common.AddressToPkScript(net, chain, address)
		redeemScript           = ""

		url string = domain + "/api/v1/market/order/push"
		req        = &request.PushOrderReq{
			Chain:     chain,
			AssetType: models.AssetTypePins,
			AssetId:   assetId,
			Address:   address,
			PsbtRaw:   "",
		}

		inputs      []common.Input      = make([]common.Input, 0)
		outputs     []common.Output     = make([]common.Output, 0)
		inSginers   []*common.InputSign = make([]*common.InputSign, 0)
		psbtBuilder *common.PsbtBuilder
		err         error
	)
	fmt.Printf("pkScript: %s\n", pkScript)

	inputs = append(inputs, common.Input{
		OutTxId:  txId,
		OutIndex: outIndex,
	})
	outputs = append(outputs, common.Output{
		Address: address,
		Amount:  sellPriceAmount,
	})

	psbtBuilder, err = common.CreatePsbtBuilder(common.GetNetParams(net, chain), inputs, outputs)
	if err != nil {
		t.Errorf("CreatePsbtBuilder() error = %v,", err)
		return
	}

	privateKeyBytes, err := hex.DecodeString(priKeyHex)
	if err != nil {
		t.Errorf("DecodeString() error = %v,", err)
		return
	}
	privateKey, _ := btcec.PrivKeyFromBytes(privateKeyBytes)
	nativeSegwitAddress, err := btcutil.NewAddressWitnessPubKeyHash(btcutil.Hash160(privateKey.PubKey().SerializeCompressed()), common.GetNetParams(net, chain))
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

	inSginers = append(inSginers, &common.InputSign{
		UtxoType:     common.Witness,
		Index:        0,
		PkScript:     pkScript,
		RedeemScript: redeemScript,
		Amount:       outValue,
		SighashType:  txscript.SigHashSingle | txscript.SigHashAnyOneCanPay,
		//SighashType: txscript.SigHashDefault,
		PriHex: priKeyHex,
	})

	err = psbtBuilder.UpdateAndSignInput(inSginers)
	//err = psbtBuilder.UpdateAndSignInputNoFinalize(inSginers)
	if err != nil {
		t.Errorf("UpdateAndSignInput() error = %v,", err)
		return
	}
	req.PsbtRaw, err = psbtBuilder.ToString()
	if err != nil {
		t.Errorf("ToString() error = %v,", err)
		return
	}
	fmt.Printf("req.PsbtRaw: %s\n", req.PsbtRaw)
	//return

	resp, err := tool.PostUrl(url, req, addHeaders(priKeyHex))
	if err != nil {
		t.Errorf("PostUrl() error = %v,", err)
		return
	}
	t.Logf("resp = %v,", resp)
	//orderId: 7818c13ce41b6f16184f3a84fe7269cbf7e128c1f299188f00e7613587328677
	//orderId: da1aa85d5d328c745f60deb2ee41503f24f0109fe1609f94703eec7fb4d7a087
	//orderId: 6de267ab59fca7d4c71dce14a8a499b97faad4897759dc0b919e9b5591f0d610
}

func TestPushMarketOrderByLegacy(t *testing.T) {
	var (
		net string = "livenet"

		// btc
		// chain           string = "btc"
		// priKeyHex       string = "766b9d706b2f97ae066dd1cdf994181fe8712f51d9493b54b03fab9759ab5893"
		// address         string = "1LdFYY1CLsmGRJwTXKsod4NscAFn3iUbCA"
		// txId            string = "a6142806cced5e24a972da0fd0250ae17567470906a6ef90bd665fc614b3026e" // pin image
		// outIndex        uint32 = 0
		// preTxHex        string = "020000000001029c3f6b957d0f5456cc4248709300593ace32ab5687758566ed2ff467559282c00100000000ffffffff9c3f6b957d0f5456cc4248709300593ace32ab5687758566ed2ff467559282c00500000000ffffffff0222020000000000001976a914d7458146ad85e527d87f8020845ee8118312000b88ac80023e00000000001600148344941fb503504a174b9a15e6015e8fcf50ea500248304502210089c422464be2fa663dcf8503311076d1bef319ee212b7d2d5c74f6343ab5436e02200c8739812a3e1f390f8a1a2051e45bd42b5eb628ec07ae571bdbc9d873ceff0d0121030896a4be190299bf2d95d896386d8dc29ec315e2c49c0e600164d1f3b01c749502483045022100ba5c56ad7a40658d33fe5f52589c5fdbb6fd0f13959fe9df30805db02159acc9022027c66bf7d8e96899721976537b69bcc4bff99afbca22829bccf56b49087df9720121030896a4be190299bf2d95d896386d8dc29ec315e2c49c0e600164d1f3b01c749500000000"
		// assetId         string = "fa387e936bd347b1f22a3d5f9989ae3b5d1a7726da00a4c5462a624387467014i0"
		// outValue        uint64 = 546
		// sellPriceAmount uint64 = 2000
		//fakeAddress     string = "1111111111111111111114oLvT2"

		// doge
		chain           string = "doge"
		priKeyHex       string = "766b9d706b2f97ae066dd1cdf994181fe8712f51d9493b54b03fab9759ab5893"
		address         string = "DQmM5nwqeHfYxK84FusNApYUVHz5Kc9RYc"
		txId            string = "adf719b9c8d2051bf21d340e6777cfd27b344d63c2cc3d033adeb29669bf4ba0" // pin image
		outIndex        uint32 = 0
		preTxHex        string = "0200000002297529b858362c0229aaa71287dbb932fc4b6fd5064fbec9966effaf1b07a522010000006b483045022100f6c83d64e977cac5b9969de07a7c19cdee803337b2c605d7bb48645a0132447002201d3f54f527cd63693a930477fa50e3fc473fa8fc8cf58451877ab161bbb591b20121028645862c2ddc0224fe3b28bc861a5b383babbaf82573904ea1a92fbba751ef6affffffff1b7434e34cdd5cc70177c3f3bb9a4cae9483c51c0333302d0ab14b4f1ea9a48c030000006b4830450221009882be7607c4ddb4167bb9499e05f4bac0018c356366f9147920c1d9d1b0256d02203ed524288896933642abc5fc7c0db14b9963e023567d6b927f7dafd6a26985ca0121028645862c2ddc0224fe3b28bc861a5b383babbaf82573904ea1a92fbba751ef6affffffff02200b2000000000001976a914d7458146ad85e527d87f8020845ee8118312000b88acee62f002000000001976a914541a9019cb940c3bb519d41818b1b04ba4832bad88ac00000000"
		assetId         string = "adf719b9c8d2051bf21d340e6777cfd27b344d63c2cc3d033adeb29669bf4ba0i0"
		outValue        uint64 = 2100000
		sellPriceAmount uint64 = 3000000
		fakeAddress     string = "D596YFweJQuHY1BbjazZYmAbt8jJPbKehC"
		//orderId: 6b17f9a0c6080ea72a11b54d232cfb0961b3a9299f976909aa8d76a3ca1e1f52

		pkScript, _ = common.AddressToPkScript(net, chain, address)

		url string = domain + "/api/v1/market/order/push"
		req        = &request.PushOrderReq{
			Chain:     chain,
			AssetType: models.AssetTypePins,
			AssetId:   assetId,
			Address:   address,
			PsbtRaw:   "",
		}

		inputs      []common.Input      = make([]common.Input, 0)
		outputs     []common.Output     = make([]common.Output, 0)
		inSginers   []*common.InputSign = make([]*common.InputSign, 0)
		psbtBuilder *common.PsbtBuilder
		err         error

		fakeTxId     string = "0000000000000000000000000000000000000000000000000000000000000000"
		fakeOutIndex uint32 = 0
		fakeInput           = common.Input{
			OutTxId:  fakeTxId,
			OutIndex: fakeOutIndex,
		}
		fakeOutput = common.Output{
			Address: fakeAddress,
			Amount:  0,
		}
		fakePkScript, _                     = common.AddressToPkScript(net, chain, fakeOutput.Address)
		fakeInSginers   []*common.InputSign = make([]*common.InputSign, 0)
	)
	fmt.Printf("pkScript: %s\n", pkScript)

	//add fake dummy
	inputs = append(inputs, fakeInput)
	inputs = append(inputs, fakeInput)

	inputs = append(inputs, common.Input{
		OutTxId:  txId,
		OutIndex: outIndex,
	})

	outputs = append(outputs, fakeOutput)
	outputs = append(outputs, fakeOutput)
	outputs = append(outputs, common.Output{
		Address: address,
		Amount:  sellPriceAmount,
	})

	psbtBuilder, err = common.CreatePsbtBuilder(common.GetNetParams(net, chain), inputs, outputs)
	if err != nil {
		t.Errorf("CreatePsbtBuilder() error = %v,", err)
		return
	}

	fakeInSginers = append(fakeInSginers, &common.InputSign{
		UtxoType:    common.Witness,
		Index:       0,
		PkScript:    fakePkScript,
		Amount:      0,
		SighashType: txscript.SigHashDefault,
		//SighashType: txscript.SigHashAll,
		PriHex: "",
		OutRaw: "",
	})
	fakeInSginers = append(fakeInSginers, &common.InputSign{
		UtxoType:    common.Witness,
		Index:       1,
		PkScript:    fakePkScript,
		Amount:      0,
		SighashType: txscript.SigHashDefault,
		//SighashType: txscript.SigHashAll,
		PriHex: "",
		OutRaw: "",
	})

	err = psbtBuilder.UpdateAndAddInputWitness(fakeInSginers)
	if err != nil {
		t.Errorf("UpdateAndAddInputWitness() error = %v,", err)
		return
	}

	inSginers = append(inSginers, &common.InputSign{
		UtxoType: common.NonWitness,
		Index:    2,
		//PkScript:    pkScript,
		Amount:      outValue,
		SighashType: txscript.SigHashSingle | txscript.SigHashAnyOneCanPay,
		PriHex:      priKeyHex,
		OutRaw:      preTxHex,
	})

	//err = psbtBuilder.UpdateAndSignInput(inSginers)
	err = psbtBuilder.UpdateAndSignInputNoFinalize(inSginers)
	if err != nil {
		t.Errorf("UpdateAndSignInput() error = %v,", err)
		return
	}
	req.PsbtRaw, err = psbtBuilder.ToString()
	if err != nil {
		t.Errorf("ToString() error = %v,", err)
		return
	}
	fmt.Printf("req.PsbtRaw: %s\n", req.PsbtRaw)
	//return

	resp, err := tool.PostUrl(url, req, addHeaders(priKeyHex))
	if err != nil {
		t.Errorf("PostUrl() error = %v,", err)
		return
	}
	t.Logf("resp = %v,", resp)
	//orderId: 7818c13ce41b6f16184f3a84fe7269cbf7e128c1f299188f00e7613587328677
	//orderId: 6b17f9a0c6080ea72a11b54d232cfb0961b3a9299f976909aa8d76a3ca1e1f52
}

func TestPushMarketOrderByNestSegwit(t *testing.T) {
	var (
		net       string = "testnet"
		chain     string = ""
		priKeyHex string = "f0f4d09a90ff279d25036b11da569690e60bcc5cbab0474eeb271f3623f0f483"
		address   string = "2Mw4oj2jNTboEauHBhNyDLJwtGSNWu5rybM"
		txId      string = "40b028882478db9b0cff1862d1c05458317be231dbfeaa5094f17da2a49e10a7" // pin text
		outIndex  uint32 = 0
		//preTxHex  string = ""

		assetId  string = "5fd9df151bb152dd091280593edb605f77427ca1f3b589a18e62d49407884f4di0"
		outValue uint64 = 546
		//sellPriceAmount uint64 = 30000
		sellPriceAmount uint64 = 2000
		pkScript, _            = common.AddressToPkScript(net, chain, address)
		redeemScript    string = ""

		url string = domain + "/api/v1/market/order/push"
		req        = &request.PushOrderReq{
			Chain:     chain,
			AssetType: models.AssetTypePins,
			AssetId:   assetId,
			Address:   address,
			PsbtRaw:   "",
		}

		inputs      []common.Input      = make([]common.Input, 0)
		outputs     []common.Output     = make([]common.Output, 0)
		inSginers   []*common.InputSign = make([]*common.InputSign, 0)
		psbtBuilder *common.PsbtBuilder
		err         error

		fakeTxId     string = "0000000000000000000000000000000000000000000000000000000000000000"
		fakeOutIndex uint32 = 0
		fakeInput           = common.Input{
			OutTxId:  fakeTxId,
			OutIndex: fakeOutIndex,
		}
		fakeOutput = common.Output{
			Address: "mfWxJ45yp2SFn7UciZyNpvDKrzbhyfKrY8",
			Amount:  0,
		}
		fakePkScript, _                     = common.AddressToPkScript(net, chain, fakeOutput.Address)
		fakeInSginers   []*common.InputSign = make([]*common.InputSign, 0)
	)
	fmt.Printf("pkScript: %s\n", pkScript)

	//add fake dummy
	inputs = append(inputs, fakeInput)
	inputs = append(inputs, fakeInput)

	inputs = append(inputs, common.Input{
		OutTxId:  txId,
		OutIndex: outIndex,
	})

	outputs = append(outputs, fakeOutput)
	outputs = append(outputs, fakeOutput)
	outputs = append(outputs, common.Output{
		Address: address,
		Amount:  sellPriceAmount,
	})

	psbtBuilder, err = common.CreatePsbtBuilder(common.GetNetParams(net, chain), inputs, outputs)
	if err != nil {
		t.Errorf("CreatePsbtBuilder() error = %v,", err)
		return
	}

	fakeInSginers = append(fakeInSginers, &common.InputSign{
		UtxoType:    common.Witness,
		Index:       0,
		PkScript:    fakePkScript,
		Amount:      0,
		SighashType: txscript.SigHashDefault,
		//SighashType: txscript.SigHashAll,
		PriHex: "",
		OutRaw: "",
	})
	fakeInSginers = append(fakeInSginers, &common.InputSign{
		UtxoType:    common.Witness,
		Index:       1,
		PkScript:    fakePkScript,
		Amount:      0,
		SighashType: txscript.SigHashDefault,
		//SighashType: txscript.SigHashAll,
		PriHex: "",
		OutRaw: "",
	})

	err = psbtBuilder.UpdateAndAddInputWitness(fakeInSginers)
	if err != nil {
		t.Errorf("UpdateAndAddInputWitness() error = %v,", err)
		return
	}

	privateKeyBytes, err := hex.DecodeString(priKeyHex)
	if err != nil {
		t.Errorf("DecodeString() error = %v,", err)
		return
	}
	privateKey, _ := btcec.PrivKeyFromBytes(privateKeyBytes)
	nativeSegwitAddress, err := btcutil.NewAddressWitnessPubKeyHash(btcutil.Hash160(privateKey.PubKey().SerializeCompressed()), common.GetNetParams(net, chain))
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

	inSginers = append(inSginers, &common.InputSign{
		UtxoType:     common.Witness,
		Index:        2,
		PkScript:     pkScript,
		RedeemScript: redeemScript,
		Amount:       outValue,
		SighashType:  txscript.SigHashSingle | txscript.SigHashAnyOneCanPay,
		PriHex:       priKeyHex,
		//OutRaw:      preTxHex,
	})

	//err = psbtBuilder.UpdateAndSignInput(inSginers)
	err = psbtBuilder.UpdateAndSignInputNoFinalize(inSginers)
	if err != nil {
		t.Errorf("UpdateAndSignInput() error = %v,", err)
		return
	}
	req.PsbtRaw, err = psbtBuilder.ToString()
	if err != nil {
		t.Errorf("ToString() error = %v,", err)
		return
	}
	fmt.Printf("req.PsbtRaw: %s\n", req.PsbtRaw)
	//return

	resp, err := tool.PostUrl(url, req, addHeaders(priKeyHex))
	if err != nil {
		t.Errorf("PostUrl() error = %v,", err)
		return
	}
	t.Logf("resp = %v,", resp)
	//orderId: 7818c13ce41b6f16184f3a84fe7269cbf7e128c1f299188f00e7613587328677
	//orderId: da1aa85d5d328c745f60deb2ee41503f24f0109fe1609f94703eec7fb4d7a087
}

func TestFetchOrderPsbt(t *testing.T) {
	var (
		// btc
		// orderId string = "6de267ab59fca7d4c71dce14a8a499b97faad4897759dc0b919e9b5591f0d610"
		// buyerPrivateKey string = "990cb917c774e0120db8f1f8dce7d59ea7ed2fbf7f9bf0d033990fc52c1edb25"
		// buyerAddress    string = "1FZ4vXoU2dQ2owkxBP48szvgcjAzq6kCT4"

		// doge
		chain           string = "doge"
		orderId         string = "6b17f9a0c6080ea72a11b54d232cfb0961b3a9299f976909aa8d76a3ca1e1f52"
		buyerPrivateKey string = "990cb917c774e0120db8f1f8dce7d59ea7ed2fbf7f9bf0d033990fc52c1edb25"
		buyerAddress    string = "DKhATnk7L3JKLwwYuy3hRm6HVruJ8k5MGS"

		url   string = domain + "/api/v1/market/order/psbt"
		query        = map[string]string{
			"chain":        chain,
			"orderId":      orderId,
			"buyerAddress": buyerAddress,
		}
	)

	resp, err := tool.GetUrl(url, query, addHeaders(buyerPrivateKey))
	if err != nil {
		t.Errorf("GetUrl() error = %v,", err)
		return
	}
	t.Logf("resp = %v,", resp)
	//takePsbt: 70736274ff0100fd5801020000000483fca8664acebcf3ee8ca99e7c99f1cbe415f41177a2fe2f88ebc1fb57259f830000000000ffffffff83fca8664acebcf3ee8ca99e7c99f1cbe415f41177a2fe2f88ebc1fb57259f830100000000ffffffffa04bbf6996b2de3a033dccc2634d347bd2cf77670e341df21b05d2c8b919f7ad0000000000ffffffff1b7434e34cdd5cc70177c3f3bb9a4cae9483c51c0333302d0ab14b4f1ea9a48c0200000000ffffffff0580841e00000000001976a9147dc48b73f0d2b289040635c508e2c4f7784150ba88ac200b2000000000001976a9149fa258b1d06f08584c469486e86d708689b312cd88acc0c62d00000000001976a914d7458146ad85e527d87f8020845ee8118312000b88ac40420f00000000001976a9147dc48b73f0d2b289040635c508e2c4f7784150ba88ac40420f00000000001976a9147dc48b73f0d2b289040635c508e2c4f7784150ba88ac00000000000100fd6a01020000000145303889866a2fb1e41707c6db8abb05ef08f2c4f59677729472994e12a3020c000000006b483045022100b6ce677093f9ab59414ad97c4ead814ac26c669c565af3692c5d7c5ad61d0b220220237ed09ca907438d1b4f71fc0648aee7e359f6e54becb3fba04ea44b6a3bef43012102da6011dcf627140ec3f00f8cd4e2ce36f17077b2ef8bd5c371bbb7f454fe2c56ffffffff0640420f00000000001976a9147dc48b73f0d2b289040635c508e2c4f7784150ba88ac40420f00000000001976a9147dc48b73f0d2b289040635c508e2c4f7784150ba88ac40420f00000000001976a9147dc48b73f0d2b289040635c508e2c4f7784150ba88ac40420f00000000001976a9147dc48b73f0d2b289040635c508e2c4f7784150ba88ac40420f00000000001976a9147dc48b73f0d2b289040635c508e2c4f7784150ba88ace01a2d00000000001976a914541a9019cb940c3bb519d41818b1b04ba4832bad88ac0000000001012200000000000000001976a914000000000000000000000000000000000000000088ac01030401000000000100fd6a01020000000145303889866a2fb1e41707c6db8abb05ef08f2c4f59677729472994e12a3020c000000006b483045022100b6ce677093f9ab59414ad97c4ead814ac26c669c565af3692c5d7c5ad61d0b220220237ed09ca907438d1b4f71fc0648aee7e359f6e54becb3fba04ea44b6a3bef43012102da6011dcf627140ec3f00f8cd4e2ce36f17077b2ef8bd5c371bbb7f454fe2c56ffffffff0640420f00000000001976a9147dc48b73f0d2b289040635c508e2c4f7784150ba88ac40420f00000000001976a9147dc48b73f0d2b289040635c508e2c4f7784150ba88ac40420f00000000001976a9147dc48b73f0d2b289040635c508e2c4f7784150ba88ac40420f00000000001976a9147dc48b73f0d2b289040635c508e2c4f7784150ba88ac40420f00000000001976a9147dc48b73f0d2b289040635c508e2c4f7784150ba88ace01a2d00000000001976a914541a9019cb940c3bb519d41818b1b04ba4832bad88ac0000000001012200000000000000001976a914000000000000000000000000000000000000000088ac01030401000000000100fd76010200000002297529b858362c0229aaa71287dbb932fc4b6fd5064fbec9966effaf1b07a522010000006b483045022100f6c83d64e977cac5b9969de07a7c19cdee803337b2c605d7bb48645a0132447002201d3f54f527cd63693a930477fa50e3fc473fa8fc8cf58451877ab161bbb591b20121028645862c2ddc0224fe3b28bc861a5b383babbaf82573904ea1a92fbba751ef6affffffff1b7434e34cdd5cc70177c3f3bb9a4cae9483c51c0333302d0ab14b4f1ea9a48c030000006b4830450221009882be7607c4ddb4167bb9499e05f4bac0018c356366f9147920c1d9d1b0256d02203ed524288896933642abc5fc7c0db14b9963e023567d6b927f7dafd6a26985ca0121028645862c2ddc0224fe3b28bc861a5b383babbaf82573904ea1a92fbba751ef6affffffff02200b2000000000001976a914d7458146ad85e527d87f8020845ee8118312000b88acee62f002000000001976a914541a9019cb940c3bb519d41818b1b04ba4832bad88ac0000000022020388ed50b94b7e2288f0448f3a66db4282bb792a155888dd088ebd80888f476dc44730440220618ba12c5c7e68f4051d5126092dc0edad10f3a98d06e5aeb9157431e12ac3880220307d29366d84daf45537e218b73c9929019eff06933a29811462474434e6085283010304830000000001012280841e00000000001976a9147dc48b73f0d2b289040635c508e2c4f7784150ba88ac01030401000000000000000000
}

func TestTakeMarketOrder(t *testing.T) {
	type buyerUtxo struct {
		TxId     string
		Index    uint32
		Amount   uint64
		UtxoType common.UtxoType
		PreTxHex string
	}
	var (
		net string = "livenet"

		// btc
		// chain           string = "btc"
		// orderId         string = "6de267ab59fca7d4c71dce14a8a499b97faad4897759dc0b919e9b5591f0d610"
		// buyerPrivateKey string = "990cb917c774e0120db8f1f8dce7d59ea7ed2fbf7f9bf0d033990fc52c1edb25"
		// buyerAddress    string = "1FZ4vXoU2dQ2owkxBP48szvgcjAzq6kCT4"
		// takePsbt        string = "70736274ff0100fd690102000000048201135d7e9ced51bf6d0871c9091389631f855039cc96b0a67f877ed2e6e6790300000000ffffffff8201135d7e9ced51bf6d0871c9091389631f855039cc96b0a67f877ed2e6e6790400000000ffffffffa7109ea4a27df19450aafedb31e27b315854c0d16218ff0c9bdb78248828b0400000000000ffffffff8201135d7e9ced51bf6d0871c9091389631f855039cc96b0a67f877ed2e6e6790000000000ffffffff06b004000000000000160014d910fd9fa9e480b4061b322a7547130b11ee7f8b22020000000000001600149fa258b1d06f08584c469486e86d708689b312cdd00700000000000017a91429e816fccc05e73675666bc940753df27dc30d44875802000000000000160014d910fd9fa9e480b4061b322a7547130b11ee7f8b5802000000000000160014d910fd9fa9e480b4061b322a7547130b11ee7f8bd0070000000000001600147d5a964c8d7c5b670955acf6f8e1432ed566b730000000000001011f5802000000000000160014d910fd9fa9e480b4061b322a7547130b11ee7f8b010304010000000001011f5802000000000000160014d910fd9fa9e480b4061b322a7547130b11ee7f8b0103040100000000010120220200000000000017a91429e816fccc05e73675666bc940753df27dc30d4487010717160014639cde9772c1dd0ade831eadf7353f57e0f3875d01086b0247304402206f88be3f977b3db76ee862934b9df3290f50933112ed356f90b039b509c2199402203e4ee216e3964ca963fa56e50be35025d697a2bb8627467516158416e6e1c1088321027586afcd7a2cca8cfee999d81aae12bb9fe1a893425589f131db1fbdcd36a3110001011fb004000000000000160014d910fd9fa9e480b4061b322a7547130b11ee7f8b0103040100000000000000000000"

		// doge
		chain           string = "doge"
		orderId         string = "6b17f9a0c6080ea72a11b54d232cfb0961b3a9299f976909aa8d76a3ca1e1f52"
		buyerPrivateKey string = "990cb917c774e0120db8f1f8dce7d59ea7ed2fbf7f9bf0d033990fc52c1edb25"
		buyerAddress    string = "DKhATnk7L3JKLwwYuy3hRm6HVruJ8k5MGS"
		takePsbt        string = "70736274ff0100fd5801020000000483fca8664acebcf3ee8ca99e7c99f1cbe415f41177a2fe2f88ebc1fb57259f830000000000ffffffff83fca8664acebcf3ee8ca99e7c99f1cbe415f41177a2fe2f88ebc1fb57259f830100000000ffffffffa04bbf6996b2de3a033dccc2634d347bd2cf77670e341df21b05d2c8b919f7ad0000000000ffffffff1b7434e34cdd5cc70177c3f3bb9a4cae9483c51c0333302d0ab14b4f1ea9a48c0200000000ffffffff0580841e00000000001976a9147dc48b73f0d2b289040635c508e2c4f7784150ba88ac200b2000000000001976a9149fa258b1d06f08584c469486e86d708689b312cd88acc0c62d00000000001976a914d7458146ad85e527d87f8020845ee8118312000b88ac40420f00000000001976a9147dc48b73f0d2b289040635c508e2c4f7784150ba88ac40420f00000000001976a9147dc48b73f0d2b289040635c508e2c4f7784150ba88ac00000000000100fd6a01020000000145303889866a2fb1e41707c6db8abb05ef08f2c4f59677729472994e12a3020c000000006b483045022100b6ce677093f9ab59414ad97c4ead814ac26c669c565af3692c5d7c5ad61d0b220220237ed09ca907438d1b4f71fc0648aee7e359f6e54becb3fba04ea44b6a3bef43012102da6011dcf627140ec3f00f8cd4e2ce36f17077b2ef8bd5c371bbb7f454fe2c56ffffffff0640420f00000000001976a9147dc48b73f0d2b289040635c508e2c4f7784150ba88ac40420f00000000001976a9147dc48b73f0d2b289040635c508e2c4f7784150ba88ac40420f00000000001976a9147dc48b73f0d2b289040635c508e2c4f7784150ba88ac40420f00000000001976a9147dc48b73f0d2b289040635c508e2c4f7784150ba88ac40420f00000000001976a9147dc48b73f0d2b289040635c508e2c4f7784150ba88ace01a2d00000000001976a914541a9019cb940c3bb519d41818b1b04ba4832bad88ac0000000001012200000000000000001976a914000000000000000000000000000000000000000088ac01030401000000000100fd6a01020000000145303889866a2fb1e41707c6db8abb05ef08f2c4f59677729472994e12a3020c000000006b483045022100b6ce677093f9ab59414ad97c4ead814ac26c669c565af3692c5d7c5ad61d0b220220237ed09ca907438d1b4f71fc0648aee7e359f6e54becb3fba04ea44b6a3bef43012102da6011dcf627140ec3f00f8cd4e2ce36f17077b2ef8bd5c371bbb7f454fe2c56ffffffff0640420f00000000001976a9147dc48b73f0d2b289040635c508e2c4f7784150ba88ac40420f00000000001976a9147dc48b73f0d2b289040635c508e2c4f7784150ba88ac40420f00000000001976a9147dc48b73f0d2b289040635c508e2c4f7784150ba88ac40420f00000000001976a9147dc48b73f0d2b289040635c508e2c4f7784150ba88ac40420f00000000001976a9147dc48b73f0d2b289040635c508e2c4f7784150ba88ace01a2d00000000001976a914541a9019cb940c3bb519d41818b1b04ba4832bad88ac0000000001012200000000000000001976a914000000000000000000000000000000000000000088ac01030401000000000100fd76010200000002297529b858362c0229aaa71287dbb932fc4b6fd5064fbec9966effaf1b07a522010000006b483045022100f6c83d64e977cac5b9969de07a7c19cdee803337b2c605d7bb48645a0132447002201d3f54f527cd63693a930477fa50e3fc473fa8fc8cf58451877ab161bbb591b20121028645862c2ddc0224fe3b28bc861a5b383babbaf82573904ea1a92fbba751ef6affffffff1b7434e34cdd5cc70177c3f3bb9a4cae9483c51c0333302d0ab14b4f1ea9a48c030000006b4830450221009882be7607c4ddb4167bb9499e05f4bac0018c356366f9147920c1d9d1b0256d02203ed524288896933642abc5fc7c0db14b9963e023567d6b927f7dafd6a26985ca0121028645862c2ddc0224fe3b28bc861a5b383babbaf82573904ea1a92fbba751ef6affffffff02200b2000000000001976a914d7458146ad85e527d87f8020845ee8118312000b88acee62f002000000001976a914541a9019cb940c3bb519d41818b1b04ba4832bad88ac0000000022020388ed50b94b7e2288f0448f3a66db4282bb792a155888dd088ebd80888f476dc44730440220618ba12c5c7e68f4051d5126092dc0edad10f3a98d06e5aeb9157431e12ac3880220307d29366d84daf45537e218b73c9929019eff06933a29811462474434e6085283010304830000000001012280841e00000000001976a9147dc48b73f0d2b289040635c508e2c4f7784150ba88ac01030401000000000000000000"

		pkScript, _ = common.AddressToPkScript(net, chain, buyerAddress)

		networkFeeRate int64 = 5000

		utxos []buyerUtxo = []buyerUtxo{
			{
				TxId:     "c19038208aac79e88d1387f86ec71f4636d8d6e6eb02f0c7dd4539db2fc9c670",
				Index:    0,
				Amount:   20000000,
				UtxoType: common.NonWitness,
				PreTxHex: "0200000001a04bbf6996b2de3a033dccc2634d347bd2cf77670e341df21b05d2c8b919f7ad010000006b483045022100dee110d11ba952359c131b93d6fe39ff16e2d99d2642caa733c50a77ba5517c202204660d814346ce407e6b370b4f30075fc941d39a47458303e78bd433f1ee225d10121028645862c2ddc0224fe3b28bc861a5b383babbaf82573904ea1a92fbba751ef6affffffff02002d3101000000001976a9149fa258b1d06f08584c469486e86d708689b312cd88acdef7ad01000000001976a914541a9019cb940c3bb519d41818b1b04ba4832bad88ac00000000",
			},
		}

		url string = domain + "/api/v1/market/order/take"
		req        = &request.TakeOrderReq{
			OrderId:        orderId,
			TakerPsbtRaw:   "",
			NetworkFeeRate: networkFeeRate,
		}

		outputs         []common.Output     = make([]common.Output, 0)
		inSigners       []*common.InputSign = make([]*common.InputSign, 0)
		takePsbtBuilder *common.PsbtBuilder
		err             error
	)
	takePsbtBuilder, err = common.NewPsbtBuilder(common.GetNetParams(net, chain), takePsbt)
	if err != nil {
		t.Errorf("NewPsbtBuilder() error = %v,", err)
		return
	}
	d := len(takePsbtBuilder.GetInputs())

	for i, utxo := range utxos {
		input := common.Input{
			OutTxId:  utxo.TxId,
			OutIndex: utxo.Index,
		}
		inSigner := &common.InputSign{
			UtxoType:    utxo.UtxoType,
			Index:       d + i,
			PkScript:    pkScript,
			Amount:      utxo.Amount,
			SighashType: txscript.SigHashAll,
			PriHex:      buyerPrivateKey,
			OutRaw:      utxo.PreTxHex,
		}
		err = takePsbtBuilder.AddInputOnly(input)
		if err != nil {
			t.Errorf("AddInputOnly() error = %v,", err)
			return
		}
		fmt.Printf("index: %d\n", inSigner.Index)
		inSigners = append(inSigners, inSigner)
	}

	err = takePsbtBuilder.UpdateAndAddInputWitness(inSigners)
	if err != nil {
		t.Errorf("UpdateAndAddInputWitness() error = %v,", err)
		return
	}

	if len(outputs) > 0 {
		//err = takePsbtBuilder.AddOutput(outputs)
		//if err != nil {
		//	t.Errorf("AddOutputOnly() error = %v,", err)
		//	return
		//}
	}

	//err = takePsbtBuilder.UpdateAndSignInput(inSigners)
	err = takePsbtBuilder.UpdateAndSignInputNoFinalize(inSigners)
	if err != nil {
		t.Errorf("UpdateAndSignInput() error = %v,", err)
		return
	}

	req.TakerPsbtRaw, err = takePsbtBuilder.ToString()
	if err != nil {
		t.Errorf("ToString() error = %v,", err)
		return
	}

	resp, err := tool.PostUrl(url, req, addHeaders(buyerPrivateKey))
	if err != nil {
		t.Errorf("PostUrl() error = %v,", err)
		return
	}
	t.Logf("resp = %v,", resp)
	//1aa8b3f358fcd4931c1a59d0c1eab6476909b92f02d2c215471cbdd03bb910da
	//doge 37ce799276eac42e117baed3fcd2f7c7d0c6c0b089410af038fa770965f64286
}

func TestCancelMarketOrder(t *testing.T) {
	var (
		orderId string = "6de267ab59fca7d4c71dce14a8a499b97faad4897759dc0b919e9b5591f0d610"
		url     string = domain + "/api/v1/market/order/cancel"
		req            = &request.CancelOrderReq{
			OrderId: orderId,
		}
		//priKeyHex string = "766b9d706b2f97ae066dd1cdf994181fe8712f51d9493b54b03fab9759ab5893"
		priKeyHex string = "f0f4d09a90ff279d25036b11da569690e60bcc5cbab0474eeb271f3623f0f483"
		headers          = map[string]string{
			"X-Signature":  "",
			"X-Public-Key": "",
		}
	)
	privateKeyBytes, err := hex.DecodeString(priKeyHex)
	if err != nil {
		log.Fatal(err)
	}
	privateKey, _ := btcec.PrivKeyFromBytes(privateKeyBytes)
	publicKey := hex.EncodeToString(privateKey.PubKey().SerializeCompressed())

	headers["X-Public-Key"] = publicKey

	signature, err := auth.SignTextMessage("metaid.market", priKeyHex)
	if err != nil {
		log.Fatal(err)
	}
	headers["X-Signature"] = signature
	resp, err := tool.PostUrl(url, req, headers)
	if err != nil {
		t.Errorf("PostUrl() error = %v,", err)
		return
	}
	t.Logf("resp = %v,", resp)
}

func TestLegacySigSingle(t *testing.T) {
	var (
		net       string = "testnet"
		chain     string = ""
		priKeyHex string = "b7b93c5c9374405b68613cd37be719dfa5b78f630da0950675b79c6c64fcab3a"
		address   string = "miGitBv1eh5c5UTppu3jHKAkdRfDbrV9jN"
		txId      string = "f034a43bf648e9f39c16fa79148c80669e400b7905f84008ec1995c1cc28e6db" // pin image
		outIndex  uint32 = 0
		preTxHex  string = "020000000001015339b764d7f4be5d88fb23c84a123f868cfc370ac07fce175665e532e6ef74dc0000000000ffffffff02204e0000000000001976a9141e375bca78593e75e3b2ad537a43d73299ec371588aca0553c00000000001600148344941fb503504a174b9a15e6015e8fcf50ea500247304402206cc78aafbd80e7396f2d343e96813a7fced06548f2a3a80628a6fd77dd065882022024c14eeb27ad06223a3661e199059aa505dc3c1a7b50b62934e53945541b68a70121030896a4be190299bf2d95d896386d8dc29ec315e2c49c0e600164d1f3b01c749500000000"

		outValue uint64 = 20000
		//sellPriceAmount uint64 = 30000
		sellPriceAmount uint64 = 1000
		pkScript, _            = common.AddressToPkScript(net, chain, address)

		inputs      []common.Input      = make([]common.Input, 0)
		outputs     []common.Output     = make([]common.Output, 0)
		inSginers   []*common.InputSign = make([]*common.InputSign, 0)
		psbtBuilder *common.PsbtBuilder
		err         error
	)
	fmt.Printf("pkScript: %s\n", pkScript)

	inputs = append(inputs, common.Input{
		OutTxId:  txId,
		OutIndex: outIndex,
	})
	outputs = append(outputs, common.Output{
		Address: address,
		Amount:  sellPriceAmount,
	})

	psbtBuilder, err = common.CreatePsbtBuilder(common.GetNetParams(net, chain), inputs, outputs)
	if err != nil {
		t.Errorf("CreatePsbtBuilder() error = %v,", err)
		return
	}

	inSginers = append(inSginers, &common.InputSign{
		UtxoType: common.NonWitness,
		Index:    0,
		//PkScript:    pkScript,
		Amount:      outValue,
		SighashType: txscript.SigHashSingle | txscript.SigHashAnyOneCanPay,
		//SighashType: txscript.SigHashDefault,
		PriHex: priKeyHex,
		OutRaw: preTxHex,
	})

	err = psbtBuilder.UpdateAndSignInput(inSginers)
	//err = psbtBuilder.UpdateAndSignInputNoFinalize(inSginers)
	if err != nil {
		t.Errorf("UpdateAndSignInput() error = %v,", err)
		return
	}
	psbtRaw, err := psbtBuilder.ToString()
	if err != nil {
		t.Errorf("ToString() error = %v,", err)
		return
	}
	fmt.Printf("req.PsbtRaw: %s\n", psbtRaw)

	txRaw, err := psbtBuilder.ExtractPsbtTransaction()
	if err != nil {
		t.Errorf("ExtractPsbtTransaction() error = %v,", err)
		return
	}
	fmt.Printf("txRaw: %s\n", txRaw)
}
