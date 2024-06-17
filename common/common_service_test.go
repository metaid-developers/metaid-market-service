package common

import "testing"

func TestCheckLegacyAddressType(t *testing.T) {
	var (
		net string = "testnet"
		//address string = ""
		address string = "mwKUTvJF43BqGqANeVdrtpRwd2zxNFvnWQ"
	)
	res, err := CheckLegacyAddressType(net, address)
	if err != nil {
		t.Errorf("err:%s", err.Error())
	}
	t.Logf("Res:%t", res)
}

func TestAddressToPkScript(t *testing.T) {
	var (
		net     string = "livenet"
		address string = "1111111111111111111114oLvT2"
	)
	res, err := AddressToPkScript(net, address)
	if err != nil {
		t.Errorf("err:%s", err.Error())
	}
	t.Logf("Res:%s", res)
}

func TestPkScriptToAddress(t *testing.T) {
	var (
		net string = "testnet"
		//net      string = "livenet"
		pkScript string = "76a914000000000000000000000000000000000000000088ac"
	)
	res, err := PkScriptToAddress(net, pkScript)
	if err != nil {
		t.Errorf("err:%s", err.Error())
	}
	t.Logf("Res:%s", res)
}
