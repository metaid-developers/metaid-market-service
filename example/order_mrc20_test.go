package example

import (
	"encoding/hex"
	"fmt"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/txscript"
	"log"
	"metaid-market-service/common"
	"metaid-market-service/controller/auth"
	"metaid-market-service/controller/request"
	"metaid-market-service/models"
	"metaid-market-service/tool"
	"testing"
)

func TestPushMarketMrc20Order(t *testing.T) {
	var (
		net       string = "testnet"
		priKeyHex        = "ddecb5d9064b202f6b3f4c154a3f09750ea7b5341134373cc0c679a10cb397bf"
		address          = "2NBB3b8PwTTMZRobry8AWkq99s5WBMEJ87D"

		txId     string = "5e726d77f43f65977194d71374e09f2b6b50576de1232e42f9a1229f6b067b92" //mrc20-utxo
		outIndex uint32 = 1

		tickId       string = "8e659899275b1d06db870fbee9b293bc73d25e063cc86860a6d52c1e11091e9bi0"
		outValue     uint64 = 546
		priceAmount  uint64 = 2000
		pkScript, _         = common.AddressToPkScript(net, address)
		redeemScript        = ""

		url string = domain + "/api/v1/market/mrc20/order/push"
		req        = &request.PushMrc20OrderReq{
			AssetType: models.AssetTypeMrc20,
			TickId:    tickId,
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
		Amount:  priceAmount,
	})

	psbtBuilder, err = common.CreatePsbtBuilder(common.GetNetParams(net), inputs, outputs)
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
	/*
	   order_mrc20_test.go:113: resp = {"code":0,"message":"success","processingTime":11,"data":{"orderId":"136519bb16d5158c54ff8c5bbb474ad42b34a5faf4f25cb6adcf199225963b91","assetType":"mrc20","tickId":"8e659899275b1d06db870fbee9b293bc73d25e063cc86860a6d52c1e11091e9bi0","orderState":1}},
	*/
}

func TestFetchMrc20OrderPsbt(t *testing.T) {
	var (
		//orderId         string = "136519bb16d5158c54ff8c5bbb474ad42b34a5faf4f25cb6adcf199225963b91"
		orderId         string = "acd99b9403e62ff36c331e2a35c79f2f7ee526093cee98ae9e7bfa8763025ebb"
		buyerPrivateKey string = "d4e34195c8a62d5773bfb2ce65056defe30c4cbc8ef4ac8a4074728e04ff5c34"
		buyerAddress    string = "tb1qe6r2e0d7kl92ul489g7r7kfm4hz66qndtr88ft"

		url   string = domain + "/api/v1/market/mrc20/order/psbt"
		query        = map[string]string{
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
	/*
	   order_mrc20_test.go:137: resp = {"code":0,"message":"success","processingTime":9,"data":{"orderId":"136519bb16d5158c54ff8c5bbb474ad42b34a5faf4f25cb6adcf199225963b91","utxoId":"5e726d77f43f65977194d71374e09f2b6b50576de1232e42f9a1229f6b067b92_1","outValue":546,"assetType":"mrc20","orderState":1,"sellerAddress":"2NBB3b8PwTTMZRobry8AWkq99s5WBMEJ87D","seller":null,"buyerAddress":"","buyer":null,"tickId":"8e659899275b1d06db870fbee9b293bc73d25e063cc86860a6d52c1e11091e9bi0","tick":"btc-mrc20","tokenName":"btc-mrc20","decimals":8,"chain":"pop","amount":1000,"amountStr":"1000","tokenPriceRate":2,"tokenPriceRateStr":"2","priceAmount":2000,"priceDecimal":8,"priceCoin":"BTC","fee":2000,"feeRate":1,"takePsbt":"70736274ff0100d902000000028cc4c9b303f7e841a487d26562a8af1352b285fa4f0435074a713ddb39a450540300000000ffffffff927b066b9f22a1f9422e23e16d57506b2b9fe07413d7947197653ff4776d725e0100000000ffffffff042202000000000000160014ce86acbdbeb7caae7ea72a3c3f593badc5ad026dd00700000000000017a914c4a7eb5ee40bac7082a08fdcc4e085311243e9b2875802000000000000160014d910fd9fa9e480b4061b322a7547130b11ee7f8bd0070000000000001600147d5a964c8d7c5b670955acf6f8e1432ed566b730000000000001011f5802000000000000160014d910fd9fa9e480b4061b322a7547130b11ee7f8b0103040100000000010120220200000000000017a914c4a7eb5ee40bac7082a08fdcc4e085311243e9b287010717160014c573ba87ac94bc1f906a99863b31ca468ed0581201086c02483045022100d09c42ef786cdc83a8b103a26482b45100121fa7c1a6ab16781ec7832300b61a02202fea1d5c0437884a346c314093df880eb4ab275649f7aea0bd1712512a17f2298321036d6729056f7072886d71bcf1728ba154bc3154fba0e0e56911e26dc291911e4a0000000000","blockHeight":0,"confirmationState":0,"dealTime":0,"txId":""}},
	*/
}

func TestTakeMarketMrc20Order(t *testing.T) {
	type buyerUtxo struct {
		TxId   string
		Index  uint32
		Amount uint64
	}
	var (
		net             string = "testnet"
		orderId         string = "136519bb16d5158c54ff8c5bbb474ad42b34a5faf4f25cb6adcf199225963b91"
		buyerPrivateKey string = "d4e34195c8a62d5773bfb2ce65056defe30c4cbc8ef4ac8a4074728e04ff5c34"
		buyerAddress    string = "tb1qe6r2e0d7kl92ul489g7r7kfm4hz66qndtr88ft"
		pkScript, _            = common.AddressToPkScript(net, buyerAddress)
		changeAddress   string = buyerAddress

		takePsbt       string = "70736274ff0100d902000000028cc4c9b303f7e841a487d26562a8af1352b285fa4f0435074a713ddb39a450540300000000ffffffff927b066b9f22a1f9422e23e16d57506b2b9fe07413d7947197653ff4776d725e0100000000ffffffff042202000000000000160014ce86acbdbeb7caae7ea72a3c3f593badc5ad026dd00700000000000017a914c4a7eb5ee40bac7082a08fdcc4e085311243e9b2875802000000000000160014d910fd9fa9e480b4061b322a7547130b11ee7f8bd0070000000000001600147d5a964c8d7c5b670955acf6f8e1432ed566b730000000000001011f5802000000000000160014d910fd9fa9e480b4061b322a7547130b11ee7f8b0103040100000000010120220200000000000017a914c4a7eb5ee40bac7082a08fdcc4e085311243e9b287010717160014c573ba87ac94bc1f906a99863b31ca468ed0581201086c02483045022100d09c42ef786cdc83a8b103a26482b45100121fa7c1a6ab16781ec7832300b61a02202fea1d5c0437884a346c314093df880eb4ab275649f7aea0bd1712512a17f2298321036d6729056f7072886d71bcf1728ba154bc3154fba0e0e56911e26dc291911e4a0000000000"
		networkFeeRate int64  = 1000

		utxos []buyerUtxo = []buyerUtxo{
			{
				TxId:   "36fd97feae4f41bac3d3f6dbf23705114058b71e2f8043960151156a79ab54f4",
				Index:  1,
				Amount: 4258100,
			},
		}

		url string = domain + "/api/v1/market/mrc20/order/take"
		req        = &request.TakeMrc20OrderReq{
			OrderId:        orderId,
			TakerPsbtRaw:   "",
			NetworkFeeRate: networkFeeRate,
		}

		outputs         []common.Output     = make([]common.Output, 0)
		inSigners       []*common.InputSign = make([]*common.InputSign, 0)
		takePsbtBuilder *common.PsbtBuilder
		err             error

		txSize     int64 = 0
		minerFee   int64 = 0
		utxoAmount int64 = 0
		outAmount  int64 = 0
	)
	takePsbtBuilder, err = common.NewPsbtBuilder(common.GetNetParams(net), takePsbt)
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
			PriHex:      buyerPrivateKey,
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
	/*
	   order_mrc20_test.go:268: resp = {"code":0,"message":"success","processingTime":15,"data":{"orderId":"136519bb16d5158c54ff8c5bbb474ad42b34a5faf4f25cb6adcf199225963b91","txId":"b773f5a409b6b408fbd3658e240b28a193d7656796b8047c41e9be176cb86a22","assetType":"mrc20","tickId":"8e659899275b1d06db870fbee9b293bc73d25e063cc86860a6d52c1e11091e9bi0","orderState":3}},
	*/
}

func TestCancelMarketMrc20Order(t *testing.T) {
	var (
		orderId string = "136519bb16d5158c54ff8c5bbb474ad42b34a5faf4f25cb6adcf199225963b91"
		url     string = domain + "/api/v1/market/mrc20/order/cancel"
		req            = &request.CancelOrderReq{
			OrderId: orderId,
		}
		priKeyHex string = "ddecb5d9064b202f6b3f4c154a3f09750ea7b5341134373cc0c679a10cb397bf"
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

func TestFetchMrc20OrderPsbtForTaproot(t *testing.T) {
	var (
		orderId         string = "acd99b9403e62ff36c331e2a35c79f2f7ee526093cee98ae9e7bfa8763025ebb"
		buyerPrivateKey string = "f6d6f6c04cfb465a87135f3ff0b100a510f70819ff15c0c2f6d9c8b287608d9d"
		buyerAddress    string = "tb1phwnwarkjfd7wmmwsfn9tw59eulx4wgnzhd6russw74hwh68rfy5qc55q0s"

		url   string = domain + "/api/v1/market/mrc20/order/psbt"
		query        = map[string]string{
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
	/*
	   order_mrc20_test.go:328: resp = {"code":0,"message":"success","processingTime":9,"data":{"orderId":"acd99b9403e62ff36c331e2a35c79f2f7ee526093cee98ae9e7bfa8763025ebb","utxoId":"dff13dee1a00d4bda8f9ce634441304ff24d9505f420dce1f138b95e654504bb_1","outValue":546,"assetType":"mrc20","orderState":1,"sellerMetaId":"","sellerAddress":"n4FSvStqcX5VL1dJd7XgYsiaAzaFtDmQ64","seller":null,"buyerMetaId":"","buyerAddress":"","buyer":null,"tickId":"44a1600fd6bbea372aa68c01d1920d374f86ea76d1d65d72b5888c60006975cci0","tick":"MTT3","tokenName":"Hey","decimals":8,"chain":"btc","amount":10,"amountStr":"10","tokenPriceRate":100,"tokenPriceRateStr":"100","priceAmount":1000,"priceDecimal":8,"priceCoin":"BTC","fee":2000,"feeRate":1,"takePsbt":"70736274ff0100e70200000002f4e8141d697e294b429cb0f4fcd189181e89bd5b5b49be6e0b694bc9d8e61ef60300000000ffffffffbb0445655eb938f1e1dc20f405954df24f30414463cef9a8bdd4001aee3df1df0100000000ffffffff042202000000000000225120bba6ee8ed24b7cededd04ccab750b9e7cd572262bb743e420ef56eebe8e34928e8030000000000001976a914f95c22783107cb1fb41510c85b0dfb14cc1f0c5688ac5802000000000000160014d910fd9fa9e480b4061b322a7547130b11ee7f8bd0070000000000001600147d5a964c8d7c5b670955acf6f8e1432ed566b730000000000001011f5802000000000000160014d910fd9fa9e480b4061b322a7547130b11ee7f8b01030401000000000100fd0a010200000002e158b5b362c35982898e5f1260fe56abe1aa562839afb1fcae959d102460da1d010000006a47304402205b813bb28a37ecff7c114da109fff7a6e35765277a6b59b79c359a1e571c0fca02200c9db7ea739ef6a8b124ec9cc082bf77e2f0dd049ef2028c057711ec9a71f6cd01210340835e7d0001840073fbd7b138f923399e5566cf84ff710ac5b664fe8b5e1be2fffffffffcb06c35d1d30d585b846f56f61442a18bf04bfed2ec7a2cedf753a2fc278f5d0000000000ffffffff0222020000000000001976a914f95c22783107cb1fb41510c85b0dfb14cc1f0c5688ac22020000000000001976a914f95c22783107cb1fb41510c85b0dfb14cc1f0c5688ac0000000001076a4730440220266ba1ebfe1782e0898b0c91358c747e05912c65547c76e07d4bd00fdf49f714022049a35ddcb90c33ef7a37363633111e2c0120ac81ff8f74065f01d020b388a52d83210340835e7d0001840073fbd7b138f923399e5566cf84ff710ac5b664fe8b5e1be20000000000","blockHeight":0,"confirmationState":0,"dealTime":0,"txId":""}},
	*/
}

func TestTakeMarketMrc20OrderForTaproot(t *testing.T) {
	type buyerUtxo struct {
		TxId   string
		Index  uint32
		Amount uint64
	}
	var (
		net             string = "testnet"
		orderId         string = "acd99b9403e62ff36c331e2a35c79f2f7ee526093cee98ae9e7bfa8763025ebb"
		buyerPrivateKey string = "f6d6f6c04cfb465a87135f3ff0b100a510f70819ff15c0c2f6d9c8b287608d9d"
		buyerAddress    string = "tb1phwnwarkjfd7wmmwsfn9tw59eulx4wgnzhd6russw74hwh68rfy5qc55q0s"
		pkScript, _            = common.AddressToPkScript(net, buyerAddress)
		changeAddress   string = buyerAddress

		takePsbt       string = "70736274ff0100e70200000002f4e8141d697e294b429cb0f4fcd189181e89bd5b5b49be6e0b694bc9d8e61ef60300000000ffffffffbb0445655eb938f1e1dc20f405954df24f30414463cef9a8bdd4001aee3df1df0100000000ffffffff042202000000000000225120bba6ee8ed24b7cededd04ccab750b9e7cd572262bb743e420ef56eebe8e34928e8030000000000001976a914f95c22783107cb1fb41510c85b0dfb14cc1f0c5688ac5802000000000000160014d910fd9fa9e480b4061b322a7547130b11ee7f8bd0070000000000001600147d5a964c8d7c5b670955acf6f8e1432ed566b730000000000001011f5802000000000000160014d910fd9fa9e480b4061b322a7547130b11ee7f8b01030401000000000100fd0a010200000002e158b5b362c35982898e5f1260fe56abe1aa562839afb1fcae959d102460da1d010000006a47304402205b813bb28a37ecff7c114da109fff7a6e35765277a6b59b79c359a1e571c0fca02200c9db7ea739ef6a8b124ec9cc082bf77e2f0dd049ef2028c057711ec9a71f6cd01210340835e7d0001840073fbd7b138f923399e5566cf84ff710ac5b664fe8b5e1be2fffffffffcb06c35d1d30d585b846f56f61442a18bf04bfed2ec7a2cedf753a2fc278f5d0000000000ffffffff0222020000000000001976a914f95c22783107cb1fb41510c85b0dfb14cc1f0c5688ac22020000000000001976a914f95c22783107cb1fb41510c85b0dfb14cc1f0c5688ac0000000001076a4730440220266ba1ebfe1782e0898b0c91358c747e05912c65547c76e07d4bd00fdf49f714022049a35ddcb90c33ef7a37363633111e2c0120ac81ff8f74065f01d020b388a52d83210340835e7d0001840073fbd7b138f923399e5566cf84ff710ac5b664fe8b5e1be20000000000"
		networkFeeRate int64  = 1000

		utxos []buyerUtxo = []buyerUtxo{
			{
				TxId:   "e0e20b960127940dedf15ffac182b5bfab4badd663cfb249c7678f08e81cdbb2",
				Index:  1,
				Amount: 304400,
			},
		}

		url string = domain + "/api/v1/market/mrc20/order/take"
		req        = &request.TakeMrc20OrderReq{
			OrderId:        orderId,
			TakerPsbtRaw:   "",
			NetworkFeeRate: networkFeeRate,
		}

		outputs         []common.Output     = make([]common.Output, 0)
		inSigners       []*common.InputSign = make([]*common.InputSign, 0)
		takePsbtBuilder *common.PsbtBuilder
		err             error

		txSize     int64 = 0
		minerFee   int64 = 0
		utxoAmount int64 = 0
		outAmount  int64 = 0
	)
	takePsbtBuilder, err = common.NewPsbtBuilder(common.GetNetParams(net), takePsbt)
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
			UtxoType:    common.Taproot,
			Index:       d + i,
			PkScript:    pkScript,
			Amount:      utxo.Amount,
			SighashType: txscript.SigHashAll,
			PriHex:      buyerPrivateKey,
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
	/*
	   order_mrc20_test.go:459: resp = {"code":0,"message":"success","processingTime":21,"data":{"orderId":"acd99b9403e62ff36c331e2a35c79f2f7ee526093cee98ae9e7bfa8763025ebb","txId":"b8ffd23c7ffe3164c5cff50377f2071d34d3c71b545950b1ba3f4b486fa7e326","assetType":"mrc20","tickId":"44a1600fd6bbea372aa68c01d1920d374f86ea76d1d65d72b5888c60006975cci0","orderState":3}},
	*/
}
