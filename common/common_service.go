package common

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"metaid-market-service/conf"
	"metaid-market-service/protobuf/btc_service"
	"metaid-market-service/protobuf/doge_service"
	"metaid-market-service/service/grpc_service"
	"metaid-market-service/service/own_service"
	"metaid-market-service/tool"
	"strings"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/shopspring/decimal"
)

// DogeMainNetParams defines the network parameters for the main Dogecoin network.
var DogeMainNetParams = chaincfg.Params{
	Name:        "mainnet",
	Net:         wire.BitcoinNet(0xc0c0c0c0), // Dogecoin MainNet magic
	DefaultPort: "22556",
	DNSSeeds: []chaincfg.DNSSeed{
		{Host: "seed.dogecoin.com", HasFiltering: true},
		{Host: "seed.multidoge.org", HasFiltering: true},
	},
	GenesisHash:      newHashFromStr("1a91e3dace36e2be3bf030a65679fe821aa1d6ef92e7c9902eb318182c355691"),
	PowLimit:         newBigIntFromHex("00000fffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
	CoinbaseMaturity: 240,
	PubKeyHashAddrID: 0x1e,   // starts with D
	ScriptHashAddrID: 0x16,   // starts with 9
	PrivateKeyID:     0x9e,   // starts with Q
	Bech32HRPSegwit:  "doge", // Not widely used but defined
	HDCoinType:       3,
}

// DogeTestNetParams defines the network parameters for the test Dogecoin network.
var DogeTestNetParams = chaincfg.Params{
	Name:        "testnet",
	Net:         wire.BitcoinNet(0xfcc1b7dc), // Dogecoin TestNet magic
	DefaultPort: "44556",
	DNSSeeds: []chaincfg.DNSSeed{
		{Host: "testseed.jrn.me.uk", HasFiltering: true},
	},
	GenesisHash:      newHashFromStr("bb0a78264637406b6360aad926284d544d7049f45189db5664f3c4d07350559e"),
	PowLimit:         newBigIntFromHex("00000fffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
	CoinbaseMaturity: 240,
	PubKeyHashAddrID: 0x71, // starts with n
	ScriptHashAddrID: 0xc4, // starts with 2
	PrivateKeyID:     0xf1, // starts with 9 or c
	Bech32HRPSegwit:  "tdoge",
	HDCoinType:       1,
}

// DogeRegTestParams defines the network parameters for the regression test Dogecoin network.
var DogeRegTestParams = chaincfg.Params{
	Name:             "regtest",
	Net:              wire.BitcoinNet(0xfabfb5da), // Dogecoin RegTest magic
	DefaultPort:      "18444",
	DNSSeeds:         []chaincfg.DNSSeed{},
	GenesisHash:      newHashFromStr("3d2160a3b5dc4a9d62e7404bb5aa85b0183cd8db1d244508f6003d23713e8819"),
	PowLimit:         newBigIntFromHex("7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
	CoinbaseMaturity: 150,
	PubKeyHashAddrID: 0x6f, // starts with m or n
	ScriptHashAddrID: 0xc4, // starts with 2
	PrivateKeyID:     0xef,
	Bech32HRPSegwit:  "rdoge",
	HDCoinType:       1,
}

func newHashFromStr(str string) *chainhash.Hash {
	hash, _ := chainhash.NewHashFromStr(str)
	return hash
}

func newBigIntFromHex(str string) *big.Int {
	i, _ := new(big.Int).SetString(str, 16)
	return i
}

// var DogeMainNetParams = chaincfg.Params{
// 	Name:        "dogemain",
// 	Net:         0xc0c0c0c0, // Dogecoin Mainnet Magic Bytes
// 	DefaultPort: "22556",
// 	DNSSeeds: []chaincfg.DNSSeed{
// 		{Host: "seed.dogecoin.com", HasFiltering: true},
// 		{Host: "multidoge.org", HasFiltering: true},
// 	},
// 	GenesisHash:      nil, // 你可以填入实际的 Doge 创世区块哈希，如果需要的话
// 	PowLimit:         nil,
// 	CoinbaseMaturity: 240,

// 	// 地址前缀关键配置
// 	PubKeyHashAddrID: 0x1e, // 30 (十进制), Base58 开头是 'D'
// 	ScriptHashAddrID: 0x16, // 22 (十进制), Base58 开头是 '9' 或 'A'
// 	PrivateKeyID:     0x9e, // 158 (十进制), WIF 开头是 '6' 或 'Q'

// 	// BIP32 HD 钱包前缀 (跟 Bitcoin 一样)
// 	HDPrivateKeyID: [4]byte{0x04, 0x88, 0xad, 0xe4}, // xprv
// 	HDPublicKeyID:  [4]byte{0x04, 0x88, 0xb2, 0x1e}, // xpub

// 	// Bech32 前缀 (Dogecoin 虽然没有广泛使用 SegWit，但代码里可能需要占位)
// 	Bech32HRPSegwit: "doge",
// }

// var DogeTestNetParams = chaincfg.Params{
// 	Name:        "dogetest",
// 	Net:         0xc0c0c0c0, // Dogecoin Testnet Magic Bytes
// 	DefaultPort: "22556",
// 	DNSSeeds: []chaincfg.DNSSeed{
// 		{Host: "seed.dogecoin.com", HasFiltering: true},
// 		{Host: "multidoge.org", HasFiltering: true},
// 	},
// }

func GetNetParams(net, chain string) *chaincfg.Params {
	var (
		netParams *chaincfg.Params = &chaincfg.MainNetParams
	)
	if chain == "doge" {
		netParams = &DogeMainNetParams
		if net == "testnet" {
			netParams = &DogeTestNetParams
		}
		return netParams
	}
	switch strings.ToLower(net) {
	case "mainnet", "livenet":
		netParams = &chaincfg.MainNetParams
		break
	case "signet":
		netParams = &chaincfg.SigNetParams
		break
	case "testnet":
		netParams = &chaincfg.TestNet3Params
		break
	}
	return netParams
}

func GetPlatformKeyAndAddressForDummyAsk(chain string) (string, string) {
	if chain == "doge" {
		return conf.PlatformPrivateKeyDummyAsk, conf.PlatformDogeAddressDummyAsk
	}
	return conf.PlatformPrivateKeyDummyAsk, conf.PlatformAddressDummyAsk
}

func GetPlatformKeyAndAddressForReceiveFee(net string) (string, string) {
	return conf.PlatformPrivateKeyReceiveFee, conf.PlatformAddressReceiveFee
}

func GetPlatformKeyForSignMsg() (string, string) {
	return conf.PlatformPrivateKeySignMsg, conf.PlatformPublicKeySignMsg
}

func GetPlatformServiceFeeConfigData() *conf.ServiceFeeConfig {
	return conf.PlatformServiceFeeConfigData
}

// address to pkScript
func AddressToPkScript(net, chain, address string) (string, error) {
	netParams := GetNetParams(net, chain)
	addr, err := btcutil.DecodeAddress(address, netParams)
	if err != nil {
		return "", err
	}
	pkScriptByte, err := txscript.PayToAddrScript(addr)
	if err != nil {
		return "", err
	}
	pkScript := hex.EncodeToString(pkScriptByte)
	return pkScript, nil
}

func CheckTaprootAddressType(net, chain, address string) (bool, error) {
	return false, nil
	netParams := GetNetParams(net, chain)
	addr, err := btcutil.DecodeAddress(address, netParams)
	if err != nil {
		return false, err
	}
	if _, ok := addr.(*btcutil.AddressTaproot); ok {
		return true, nil
	}
	return false, nil
}

func CheckLegacyAddressType(net, chain, address string) (bool, error) {
	netParams := GetNetParams(net, chain)
	addr, err := btcutil.DecodeAddress(address, netParams)
	if err != nil {
		return false, err
	}
	if _, ok := addr.(*btcutil.AddressPubKeyHash); ok {
		return true, nil
	}
	return false, nil
}

func PkScriptToAddress(net, chain, pkScript string) (string, error) {
	pkScriptByte, err := hex.DecodeString(pkScript)
	if err != nil {
		return "", err
	}
	_, addrs, _, err := txscript.ExtractPkScriptAddrs(pkScriptByte, GetNetParams(net, chain))
	if err != nil {
		return "", errors.New("Extract address from pkScript. ")
	}
	if len(addrs) == 0 {
		return "", errors.New("Extract address from pkScript. ")
	}
	address := addrs[0].EncodeAddress()
	return address, nil
}

func CheckAddressClass(net *chaincfg.Params, address string) (txscript.ScriptClass, error) {
	addr, err := btcutil.DecodeAddress(address, net)
	if err != nil {
		return txscript.NonStandardTy, err
	}
	pkScriptByte, err := txscript.PayToAddrScript(addr)
	if err != nil {
		return txscript.NonStandardTy, err
	}
	scriptClass, _, _, err := txscript.ExtractPkScriptAddrs(pkScriptByte, net)
	if err != nil {
		return txscript.NonStandardTy, err
	}
	return scriptClass, nil
}

func GetUtxoInfo(net, chain, txId string, txIndex int64) *own_service.OwnUtxoInfo {
	outPoints := []string{
		fmt.Sprintf("%s:%d", txId, txIndex),
	}

	if chain == "doge" {
		client, err := grpc_service.GetDogeBaseConn()
		if err != nil {
			return nil
		}
		grpcResp, err := client.CheckUtxoInfo(outPoints)
		if err != nil {
			return nil
		}
		if grpcResp == nil || grpcResp.GetUtxoInfos() == nil {
			return nil
		}
		grpcUtxoInfos := grpcResp.GetUtxoInfos()
		if len(grpcUtxoInfos) == 0 {
			return nil
		}
		outPointKey := fmt.Sprintf("%s:%d", txId, txIndex)
		grpcUtxoInfo, ok := grpcUtxoInfos[outPointKey]
		if !ok {
			return nil
		}
		// 转换 btc_service.UtxoInfo 到 own_service.OwnUtxoInfo
		ownUtxoInfo := &own_service.OwnUtxoInfo{
			IsExist:     grpcUtxoInfo.GetIsExist(),
			TxConfirm:   grpcUtxoInfo.GetTxConfirm(),
			SpendStatus: grpcUtxoInfo.GetSpendStatus(),
			Height:      grpcUtxoInfo.GetHeight(),
			Date:        grpcUtxoInfo.GetDate(),
			Value:       grpcUtxoInfo.GetValue(),
			Where:       grpcUtxoInfo.GetWhere(),
			Address:     grpcUtxoInfo.GetAddress(),
		}

		if grpcUtxoInfo.GetSpendInfo() != nil {
			ownUtxoInfo.SpendInfo = struct {
				SpendTx string `json:"spendTx"`
				Height  int64  `json:"height"`
				Date    int64  `json:"date"`
				Where   string `json:"where"`
			}{
				SpendTx: grpcUtxoInfo.GetSpendInfo().GetSpendTx(),
				Height:  grpcUtxoInfo.GetSpendInfo().GetHeight(),
				Date:    grpcUtxoInfo.GetSpendInfo().GetDate(),
				Where:   grpcUtxoInfo.GetSpendInfo().GetWhere(),
			}
		}
		return ownUtxoInfo
	} else {
		// 使用 grpc 的 CheckUtxoInfo 替换 own_service.CheckUtxoInfo
		client, err := grpc_service.GetBtcBaseConn()
		if err != nil {
			return nil
		}
		grpcResp, err := client.CheckUtxoInfo(outPoints)
		if err != nil {
			return nil
		}
		if grpcResp == nil || grpcResp.GetUtxoInfos() == nil {
			return nil
		}

		// 转换 grpc 响应到 own_service.OwnUtxoInfo
		grpcUtxoInfos := grpcResp.GetUtxoInfos()
		if len(grpcUtxoInfos) == 0 {
			return nil
		}
		outPointKey := fmt.Sprintf("%s:%d", txId, txIndex)
		grpcUtxoInfo, ok := grpcUtxoInfos[outPointKey]
		if !ok {
			return nil
		}
		// 转换 btc_service.UtxoInfo 到 own_service.OwnUtxoInfo
		ownUtxoInfo := &own_service.OwnUtxoInfo{
			IsExist:     grpcUtxoInfo.GetIsExist(),
			TxConfirm:   grpcUtxoInfo.GetTxConfirm(),
			SpendStatus: grpcUtxoInfo.GetSpendStatus(),
			Height:      grpcUtxoInfo.GetHeight(),
			Date:        grpcUtxoInfo.GetDate(),
			Value:       grpcUtxoInfo.GetValue(),
			Where:       grpcUtxoInfo.GetWhere(),
			Address:     grpcUtxoInfo.GetAddress(),
		}

		if grpcUtxoInfo.GetSpendInfo() != nil {
			ownUtxoInfo.SpendInfo = struct {
				SpendTx string `json:"spendTx"`
				Height  int64  `json:"height"`
				Date    int64  `json:"date"`
				Where   string `json:"where"`
			}{
				SpendTx: grpcUtxoInfo.GetSpendInfo().GetSpendTx(),
				Height:  grpcUtxoInfo.GetSpendInfo().GetHeight(),
				Date:    grpcUtxoInfo.GetSpendInfo().GetDate(),
				Where:   grpcUtxoInfo.GetSpendInfo().GetWhere(),
			}
		}
		return ownUtxoInfo
	}

	/* 旧的 own_service.CheckUtxoInfo 实现（已注释）
	var (
		infoMap map[string]*own_service.OwnUtxoInfo
	)
	infoMap, err := own_service.CheckUtxoInfo(net, outPoints)
	if err != nil {
		return nil
	}
	if len(infoMap) == 0 {
		return nil
	}
	if _, ok := infoMap[fmt.Sprintf("%s:%d", txId, txIndex)]; !ok {
		return nil
	}
	return infoMap[fmt.Sprintf("%s:%d", txId, txIndex)]
	*/
}

func BroadcastTx(chain, txRaw string) (string, error) {
	var (
		txId string = ""
	)

	if chain == "doge" {
		// 使用 Doge gRPC 服务广播交易
		client, err := grpc_service.GetDogeBaseConn()
		if err != nil {
			return "", err
		}
		resp, err := client.BroadcastTx(txRaw, conf.Net)
		if err != nil {
			return "", err
		}
		if resp == nil {
			return "", errors.New("doge broadcast tx response is nil")
		}
		txId = resp.GetTxId()
		if txId == "" {
			return "", errors.New(resp.GetMessage())
		}
		return txId, nil
	} else {
		// 使用 BTC gRPC 服务广播交易
		client, err := grpc_service.GetBtcBaseConn()
		if err != nil {
			return "", err
		}
		resp, err := client.BroadcastTx(txRaw)
		if err != nil {
			return "", err
		}
		if resp == nil {
			return "", errors.New("btc broadcast tx response is nil")
		}
		txId = resp.GetTxId()
		if txId == "" {
			return "", errors.New("btc broadcast tx failed")
		}
		return txId, nil
	}
}

func CheckPublicKeyAddress(netParams *chaincfg.Params, publicKeyStr, checkAddress string) (bool, error) {
	if publicKeyStr == "" {
		return true, nil
	}
	publicKeyByte, err := hex.DecodeString(publicKeyStr)
	if err != nil {
		return false, err
	}

	publicKey, err := btcec.ParsePubKey(publicKeyByte)
	if err != nil {
		return false, err
	}

	legacyAddress, err := btcutil.NewAddressPubKey(publicKeyByte, netParams)
	if err != nil {
		return false, err
	}
	if legacyAddress.EncodeAddress() == checkAddress {
		return true, nil
	}
	fmt.Println("legacyAddress:", legacyAddress.EncodeAddress())

	nativeSegwitAddress, err := btcutil.NewAddressWitnessPubKeyHash(btcutil.Hash160(publicKey.SerializeCompressed()), netParams)
	if err != nil {
		return false, err
	}
	if nativeSegwitAddress.EncodeAddress() == checkAddress {
		return true, nil
	}

	taprootAddress, err := btcutil.NewAddressTaproot(schnorr.SerializePubKey(txscript.ComputeTaprootKeyNoScript(publicKey)), netParams)
	if err != nil {
		return false, err
	}
	if taprootAddress.EncodeAddress() == checkAddress {
		return true, nil
	}

	pkScriptByte, err := txscript.PayToAddrScript(nativeSegwitAddress)
	if err != nil {
		return false, err
	}
	nestSegwitAddress, err := btcutil.NewAddressScriptHash(pkScriptByte, netParams)
	if err != nil {
		return false, err
	}
	if nestSegwitAddress.EncodeAddress() == checkAddress {
		return true, nil
	}

	return false, nil
}

func FetchTxHex(chain, txId string) (string, error) {
	var (
		txHex string = ""
	)

	if chain == "doge" {
		// 使用 Doge gRPC 服务获取交易原始数据
		client, err := grpc_service.GetDogeBaseConn()
		if err != nil {
			return "", err
		}
		req := &doge_service.DogeTxRawReq{
			TxId:   txId,
			Source: conf.Net,
		}
		resp, err := client.GetDogeTxRaw(req)
		if err != nil {
			return "", err
		}
		if resp == nil {
			return "", errors.New("doge fetch tx hex response is nil")
		}
		txHex = resp.GetTxRaw()
		if txHex == "" {
			return "", errors.New("doge fetch tx hex failed")
		}
		return txHex, nil
	} else {
		// 使用 BTC gRPC 服务获取交易原始数据；FetchTxRaw 的上游当前返回脏
		// JSON（"invalid character 'p'"故障源），改用健康的 GetBtcTxRaw，
		// 失败时回退 mempool.space
		if client, err := grpc_service.GetBtcBaseConn(); err == nil {
			resp, rpcErr := client.GetBtcTxRaw(&btc_service.BtcTxRawReq{TxId: txId})
			if rpcErr == nil && resp != nil && resp.GetTxRaw() != "" {
				return resp.GetTxRaw(), nil
			}
		}
		hexStr, _, hexErr := tool.GetUrlAndCode(fmt.Sprintf("%s/api/tx/%s/hex", conf.MempoolSpace, txId), nil, nil)
		if hexErr == nil && hexStr != "" && !strings.HasPrefix(hexStr, "{") {
			return strings.TrimSpace(hexStr), nil
		}
		return "", errors.New("btc fetch tx hex failed")
	}
}

func GetSegwitAddressFromPublicKey(netParams *chaincfg.Params, publicKeyHex string) (string, error) {
	publicKeyBytes, err := hex.DecodeString(publicKeyHex)
	if err != nil {
		return "", err
	}
	publicKey, err := btcec.ParsePubKey(publicKeyBytes)
	if err != nil {
		return "", err
	}
	nativeSegwitAddress, err := btcutil.NewAddressWitnessPubKeyHash(btcutil.Hash160(publicKey.SerializeCompressed()), netParams)
	if err != nil {
		return "", err
	}
	return nativeSegwitAddress.EncodeAddress(), nil
}

func GetPlatformMrc20TradeServiceFee(chain string, tradeAmount int64) (int64, int64, string, string) {
	var (
		serviceFee       int64  = 0
		serviceFeeAmount int64  = GetPlatformServiceFeeConfigData().Mrc20TradeFee
		serviceFeeRate   int64  = GetPlatformServiceFeeConfigData().Mrc20TradeFeeRate
		serviceFeeMin    int64  = GetPlatformServiceFeeConfigData().Mrc20TradeFeeMin
		serviceAddress   string = GetPlatformServiceFeeConfigData().ServiceAddress
		feeRateStr       string = "0"
	)
	if chain == "doge" {
		serviceFeeMin = GetPlatformServiceFeeConfigData().Mrc20TradeFeeMinDoge
		serviceAddress = GetPlatformServiceFeeConfigData().DogeServiceAddress
	}
	if serviceFeeRate > 0 {
		feeRateDe := decimal.New(int64(serviceFeeRate), -4)
		feeRateStr = feeRateDe.Mul(decimal.New(100, 0)).StringFixed(2)
		serviceFee = decimal.New(int64(tradeAmount), 0).Mul(feeRateDe).IntPart()
		if serviceFee < serviceFeeMin {
			serviceFee = 0
		}
	} else if serviceFeeAmount > 0 {
		serviceFee = serviceFeeAmount
	}
	return serviceFee, serviceFeeRate, feeRateStr, serviceAddress
}

func GetPlatformPinTradeServiceFee(chain string, tradeAmount int64) (int64, int64, string, string) {
	var (
		serviceFee       int64  = 0
		serviceFeeAmount int64  = GetPlatformServiceFeeConfigData().PinTradeFee
		serviceFeeRate   int64  = GetPlatformServiceFeeConfigData().PinTradeFeeRate
		serviceFeeMin    int64  = GetPlatformServiceFeeConfigData().PinTradeFeeMin
		serviceAddress   string = GetPlatformServiceFeeConfigData().ServiceAddress
		feeRateStr       string = "0"
	)
	if chain == "doge" {
		serviceFeeMin = GetPlatformServiceFeeConfigData().PinTradeFeeMinDoge
		serviceAddress = GetPlatformServiceFeeConfigData().DogeServiceAddress
	}

	if serviceFeeRate > 0 {
		feeRateDe := decimal.New(int64(serviceFeeRate), -4)
		feeRateStr = feeRateDe.Mul(decimal.New(100, 0)).StringFixed(2)
		serviceFee = decimal.New(int64(tradeAmount), 0).Mul(feeRateDe).IntPart()
		if serviceFee < serviceFeeMin {
			serviceFee = 0
		}
	} else if serviceFeeAmount > 0 {
		serviceFee = serviceFeeAmount
	}
	return serviceFee, serviceFeeRate, feeRateStr, serviceAddress
}

type MetaDataInfo struct {
	TickSign string `json:"tickSign"`
}

func CheckIdCoins(tick, metaData string, deployTime int64) string {
	var (
		metaDataInfo  *MetaDataInfo
		err           error
		tickSign      string = ""
		signPublic    string = conf.IdCoinsSignPublicKey
		signTimestamp int64  = conf.IdCoinsSignTimestamp
		verify        bool   = false
	)
	err = tool.JsonToObject(metaData, &metaDataInfo)
	if err != nil {
		return ""
	}
	tickSign = metaDataInfo.TickSign
	if tickSign == "" {
		return ""
	}
	if deployTime <= 0 {
		return ""
	}
	if signTimestamp > 0 && deployTime <= signTimestamp {
		if !IsIdCoinsTick(tick) {
			return ""
		}
		return "id-coins"
	}

	verify, err = tool.VerifyTextSign(strings.ToUpper(tick), tickSign, signPublic)
	if err != nil {
		return ""
	}
	if !verify {
		return ""
	}

	return "id-coins"
}
