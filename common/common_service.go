package common

import (
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/shopspring/decimal"
	"metaid-market-service/conf"
	"metaid-market-service/service/orders_exchange_service"
	"metaid-market-service/service/own_service"
	"metaid-market-service/tool"
	"strings"
)

func GetNetParams(net string) *chaincfg.Params {
	var (
		netParams *chaincfg.Params = &chaincfg.MainNetParams
	)
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

func GetPlatformKeyAndAddressForDummyAsk() (string, string) {
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
func AddressToPkScript(net, address string) (string, error) {
	netParams := GetNetParams(net)
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

func CheckTaprootAddressType(net, address string) (bool, error) {
	return false, nil
	netParams := GetNetParams(net)
	addr, err := btcutil.DecodeAddress(address, netParams)
	if err != nil {
		return false, err
	}
	if _, ok := addr.(*btcutil.AddressTaproot); ok {
		return true, nil
	}
	return false, nil
}

func CheckLegacyAddressType(net, address string) (bool, error) {
	netParams := GetNetParams(net)
	addr, err := btcutil.DecodeAddress(address, netParams)
	if err != nil {
		return false, err
	}
	if _, ok := addr.(*btcutil.AddressPubKeyHash); ok {
		return true, nil
	}
	return false, nil
}

func PkScriptToAddress(net, pkScript string) (string, error) {
	pkScriptByte, err := hex.DecodeString(pkScript)
	if err != nil {
		return "", err
	}
	_, addrs, _, err := txscript.ExtractPkScriptAddrs(pkScriptByte, GetNetParams(net))
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

func GetUtxoInfo(net, txId string, txIndex int64) *own_service.OwnUtxoInfo {
	var (
		infoMap   map[string]*own_service.OwnUtxoInfo
		outPoints []string = []string{
			fmt.Sprintf("%s:%d", txId, txIndex),
		}
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
}

func BroadcastTx(txRaw string) (string, error) {
	var (
		txId string = ""
		err  error
	)
	txId, err = own_service.BroadcastTransaction(conf.Net, txRaw)
	if err != nil {
		return "", err
	}
	return txId, nil
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

func FetchTxHex(txId string) (string, error) {
	return own_service.GetTxRaw(txId)
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

func GetPlatformMrc20TradeServiceFee(tradeAmount int64) (int64, int64, string, string) {
	var (
		serviceFee       int64  = 0
		serviceFeeAmount int64  = GetPlatformServiceFeeConfigData().Mrc20TradeFee
		serviceFeeRate   int64  = GetPlatformServiceFeeConfigData().Mrc20TradeFeeRate
		serviceFeeMin    int64  = GetPlatformServiceFeeConfigData().Mrc20TradeFeeMin
		serviceAddress   string = GetPlatformServiceFeeConfigData().ServiceAddress
		feeRateStr       string = "0"
	)
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

func GetPlatformPinTradeServiceFee(tradeAmount int64) (int64, int64, string, string) {
	var (
		serviceFee       int64  = 0
		serviceFeeAmount int64  = GetPlatformServiceFeeConfigData().PinTradeFee
		serviceFeeRate   int64  = GetPlatformServiceFeeConfigData().PinTradeFeeRate
		serviceFeeMin    int64  = GetPlatformServiceFeeConfigData().PinTradeFeeMin
		serviceAddress   string = GetPlatformServiceFeeConfigData().ServiceAddress
		feeRateStr       string = "0"
	)
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
		idCoins, _ := orders_exchange_service.FetchOneIdCoinsInfo(&orders_exchange_service.FetchOneIdCoinsRequest{
			TickId:        "",
			Tick:          tick,
			IssuerAddress: "",
		}, nil)
		if idCoins == nil {
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
