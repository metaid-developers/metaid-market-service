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
	"metaid-market-service/conf"
	"metaid-market-service/service/own_service"
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
