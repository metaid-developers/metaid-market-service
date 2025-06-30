package man_service

import (
	"fmt"
	"metaid-market-service/conf"
	"metaid-market-service/tool"
)

const (
	ManCodeSuccess = 1
)

// Fetch wallet pins
func FetchPins(net, addressType, address string, cursor, size int64, cnt bool) (*PinInfoList, error) {
	var (
		url    string
		result string
		resp   *ManResp
		data   *PinInfoList
		err    error
	)
	query := map[string]string{
		"cursor": fmt.Sprintf("%d", cursor),
		"size":   fmt.Sprintf("%d", size),
		"cnt":    fmt.Sprintf("%t", cnt),
	}
	url = fmt.Sprintf("%s/api/address/pin/list/%s/%s", conf.ManDomain, addressType, address)
	//if net == "testnet" {
	//	url = fmt.Sprintf("%s/api/address/pin/list/%s/%s", conf.ManTestDomain, addressType, address)
	//} else if net == "regtest" {
	//	url = fmt.Sprintf("%s/api/address/pin/list/%s/%s", conf.ManRegTestDomain, addressType, address)
	//}
	//creator/owner

	result, err = tool.GetUrl(url, query, nil)
	if err != nil {
		return nil, err
	}
	//fmt.Printf("result:%s\n", result)
	if err = tool.JsonToObject(result, &resp); err != nil {
		return nil, fmt.Errorf("get request err:%s", err.Error())
	}
	if resp.Code != ManCodeSuccess {
		return nil, fmt.Errorf("msg:%s", resp.Message)
	}

	if resp.Data == nil {
		return data, nil
	}
	if err = tool.JsonToAny(resp.Data, &data); err != nil {
		return nil, fmt.Errorf("get request err:%s", err.Error())
	}
	return data, nil
}

// Fetch wallet pin info
func FetchPinInfo(net, pinId string) (*PinInfo, error) {
	var (
		url    string
		result string
		resp   *ManResp
		data   *PinInfo
		err    error
	)
	query := map[string]string{}
	url = fmt.Sprintf("%s/api/pin/%s", conf.ManDomain, pinId)
	//if net == "testnet" {
	//	url = fmt.Sprintf("%s/api/pin/%s", conf.ManTestDomain, pinId)
	//} else if net == "regtest" {
	//	url = fmt.Sprintf("%s/api/pin/%s", conf.ManRegTestDomain, pinId)
	//
	//}

	result, err = tool.GetUrl(url, query, nil)
	if err != nil {
		return nil, err
	}
	if err = tool.JsonToObject(result, &resp); err != nil {
		return nil, fmt.Errorf("get request err:%s", err.Error())
	}
	if resp.Code != ManCodeSuccess {
		return nil, fmt.Errorf("msg:%s", resp.Message)
	}

	if err = tool.JsonToAny(resp.Data, &data); err != nil {
		return nil, fmt.Errorf("get request err:%s", err.Error())
	}
	return data, nil
}

func FetchMetaIDUserInfoInfo(net, address string) (*MetaIDUserInfo, error) {
	var (
		url    string
		result string
		resp   *ManResp
		data   *MetaIDUserInfo
		err    error
	)
	query := map[string]string{}
	//url = fmt.Sprintf("%s/api/info/address/%s", conf.ManDomain, address)
	url = fmt.Sprintf("%s/api/info/address/%s", conf.ManBaseDomain, address)
	//if net == "testnet" {
	//	url = fmt.Sprintf("%s/api/pin/%s", conf.ManTestDomain, pinId)
	//} else if net == "regtest" {
	//	url = fmt.Sprintf("%s/api/pin/%s", conf.ManRegTestDomain, pinId)
	//
	//}

	result, err = tool.GetUrl(url, query, nil)
	if err != nil {
		return nil, err
	}
	if err = tool.JsonToObject(result, &resp); err != nil {
		return nil, fmt.Errorf("get request err:%s", err.Error())
	}
	if resp.Code != ManCodeSuccess {
		return nil, fmt.Errorf("msg:%s", resp.Message)
	}

	if err = tool.JsonToAny(resp.Data, &data); err != nil {
		return nil, fmt.Errorf("get request err:%s", err.Error())
	}
	return data, nil
}

func FetchPinUtxoTotalValue(net, addresss string) (*PinUtxoTotalValue, error) {
	var (
		url    string
		result string
		resp   *ManResp
		data   *PinUtxoTotalValue
		err    error
	)
	query := map[string]string{}
	url = fmt.Sprintf("%s/api/address/pin/utxo/count/%s", conf.ManDomain, addresss)
	//if net == "testnet" {
	//	url = fmt.Sprintf("%s/api/address/pin/utxo/count/%s", conf.ManTestDomain, addresss)
	//} else if net == "regtest" {
	//	url = fmt.Sprintf("%s/api/address/pin/utxo/count/%s", conf.ManRegTestDomain, addresss)
	//
	//}

	result, err = tool.GetUrl(url, query, nil)
	if err != nil {
		return nil, err
	}
	if err = tool.JsonToObject(result, &resp); err != nil {
		return nil, fmt.Errorf("get request err:%s", err.Error())
	}
	if resp.Code != ManCodeSuccess {
		return nil, fmt.Errorf("msg:%s", resp.Message)
	}

	if err = tool.JsonToAny(resp.Data, &data); err != nil {
		return nil, fmt.Errorf("get request err:%s", err.Error())
	}
	return data, nil
}

// Fetch wallet pin info
func FetchUtxoPinInfo(net, txId string, txIndex int64) (*PinInfo, error) {
	var (
		url    string
		result string
		resp   *ManResp
		data   *PinInfo
		err    error
	)
	query := map[string]string{}
	url = fmt.Sprintf("%s/api/pin/ByOutput/%s:%d", conf.ManDomain, txId, txIndex)
	//if net == "testnet" {
	//	url = fmt.Sprintf("%s/api/pin/%s", conf.ManTestDomain, pinId)
	//} else if net == "regtest" {
	//	url = fmt.Sprintf("%s/api/pin/%s", conf.ManRegTestDomain, pinId)
	//
	//}
	fmt.Printf("url:%s\n", url)

	result, err = tool.GetUrl(url, query, nil)
	if err != nil {
		return nil, err
	}
	if err = tool.JsonToObject(result, &resp); err != nil {
		return nil, fmt.Errorf("get request err:%s", err.Error())
	}
	if resp.Code != ManCodeSuccess {
		return nil, fmt.Errorf("msg:%s", resp.Message)
	}

	if err = tool.JsonToAny(resp.Data, &data); err != nil {
		return nil, fmt.Errorf("get request err:%s", err.Error())
	}
	return data, nil
}
