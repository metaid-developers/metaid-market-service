package common

import (
	"testing"

	"github.com/btcsuite/btcd/wire"
)

func TestCheckLegacyAddressType(t *testing.T) {
	var (
		chain string = "doge"
		net   string = "livenet"
		//address string = ""
		address string = "bc1q0hzgkuls62egjpqxxhzs3cky7auyz596252p78"
	)
	res, err := CheckLegacyAddressType(net, chain, address)
	if err != nil {
		t.Errorf("err:%s", err.Error())
	}
	t.Logf("Res:%t", res)
}

func TestCheckPublicKeyAddress(t *testing.T) {
	var (
		chain     string = "doge"
		net       string = "livenet"
		address   string = "DDDnSASEoZ89rPy3HvrShWnujYWEABGhUB"
		publicKey string = "03ae69554ddf303fa8d83063933b509c01f8cbd548d678a88a305a1c5aa947f996"
	)
	res, err := CheckPublicKeyAddress(GetNetParams(net, chain), publicKey, address)
	if err != nil {
		t.Errorf("err:%s", err.Error())
	}
	t.Logf("Res:%t", res)
}

func TestAddressToPkScript(t *testing.T) {
	var (
		chain   string = ""
		net     string = "livenet"
		address string = "1111111111111111111114oLvT2"
	)
	res, err := AddressToPkScript(net, chain, address)
	if err != nil {
		t.Errorf("err:%s", err.Error())
	}
	t.Logf("Res:%s", res)
}

func TestPkScriptToAddress(t *testing.T) {
	var (
		chain string = "doge"
		// net   string = "testnet"
		net      string = "livenet"
		pkScript string = "76a914000000000000000000000000000000000000000088ac"
	)
	res, err := PkScriptToAddress(net, chain, pkScript)
	if err != nil {
		t.Errorf("err:%s", err.Error())
	}
	t.Logf("Res:%s", res)
}

func TestCheckAddressClass(t *testing.T) {
	var (
		chain   string = ""
		net     string = "testnet"
		address string = "mnVinwawKEtqyFJk3BHqmaEGkKNGrxYRh9"
		//address string = "tb1qe6r2e0d7kl92ul489g7r7kfm4hz66qndtr88ft"
		//address string = "tb1ppkvfwnw67q4w8pt86l7wr3jkngsyymqucrn6vxak7zpntawm6n6qwz929l"
		//address string = "2N1T8zd8iNoejkgbxPEq3bKdMSA9PMJ4woe"
	)
	addressClass, err := CheckAddressClass(GetNetParams(net, chain), address)
	if err != nil {
		t.Errorf("err:%s", err.Error())
	}
	t.Logf("AddressClass:%s", addressClass)
}

func TestLenByte(t *testing.T) {
	var (
		byteData           []byte = make([]byte, 32)
		emptySegwitWitenss        = wire.TxWitness{make([]byte, 71), make([]byte, 33)}
	)
	t.Logf("Len:%d", len(byteData))
	t.Logf("Len:%d", emptySegwitWitenss.SerializeSize())
}
