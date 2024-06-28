package man_service

import (
	"fmt"
	"metaid-market-service/conf"
	"metaid-market-service/tool"
)

var (
	reqErr = fmt.Errorf("request error")
)

func FetchMrc20AddressBalanceList(address string, cursor, size int64) (*Mrc20BalanceResp, error) {
	var (
		url    string
		result string
		resp   *ManResp
		data   *Mrc20BalanceResp
		err    error
	)
	query := map[string]string{
		"cursor": fmt.Sprintf("%d", cursor),
		"size":   fmt.Sprintf("%d", size),
	}
	url = fmt.Sprintf("%s/api/mrc20/address/balance/%s", conf.ManDomain, address)
	//if net == "testnet" {
	//	url = fmt.Sprintf("%s/api/mrc20/address/balance/%s", conf.ManTestDomain, address)
	//} else if net == "regtest" {
	//	url = fmt.Sprintf("%s/api/mrc20/address/balance/%s", conf.ManRegTestDomain, address)
	//
	//}

	result, err = tool.GetUrl(url, query, nil)
	if err != nil {
		return nil, reqErr
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

func FetchMrc20AddressUtxoList(address, tickId string, cursor, size int64) (*Mrc20UtxoResp, error) {
	var (
		url    string
		result string
		resp   *ManResp
		data   *Mrc20UtxoResp
		err    error
	)
	query := map[string]string{
		"cursor":  fmt.Sprintf("%d", cursor),
		"size":    fmt.Sprintf("%d", size),
		"tickId":  tickId,
		"address": address,
		"status":  fmt.Sprintf("%d", 0),
		"verify":  fmt.Sprintf("%t", true),
	}
	url = fmt.Sprintf("%s/api/mrc20/tick/address", conf.ManDomain)
	//if net == "testnet" {
	//	url = fmt.Sprintf("%s/api/mrc20/tick/address", conf.ManTestDomain)
	//} else if net == "regtest" {
	//	url = fmt.Sprintf("%s/api/mrc20/tick/address", conf.ManRegTestDomain)
	//}

	//fmt.Printf("url:%s\n", url)
	result, err = tool.GetUrl(url, query, nil)
	if err != nil {
		return nil, reqErr
	}
	//fmt.Printf("result:%s\n", result)
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

func FetchMrc20TickInfo(tickId string) (*Mrc20TickInfo, error) {
	var (
		url    string
		result string
		resp   *ManResp
		data   *Mrc20TickInfo
		err    error
	)
	query := map[string]string{}
	url = fmt.Sprintf("%s/api/mrc20/tick/info/%s", conf.ManDomain, tickId)
	//if net == "testnet" {
	//	url = fmt.Sprintf("%s/api/mrc20/tick/info/%s", conf.ManTestDomain, tickId)
	//} else if net == "regtest" {
	//	url = fmt.Sprintf("%s/api/mrc20/tick/info/%s", conf.ManRegTestDomain, tickId)
	//
	//}

	//fmt.Printf("url:%s\n", url)
	result, err = tool.GetUrl(url, query, nil)
	if err != nil {
		return nil, reqErr
	}
	//fmt.Printf("result:%s\n", result)
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

func FetchMrc20AddressShovelList(address, tickId string, cursor, size int64) (*Mrc20ShovelResp, error) {
	var (
		url    string
		result string
		resp   *ManResp
		data   *Mrc20ShovelResp
		err    error
	)
	query := map[string]string{
		"cursor":  fmt.Sprintf("%d", cursor),
		"size":    fmt.Sprintf("%d", size),
		"tickId":  tickId,
		"address": address,
	}
	url = fmt.Sprintf("%s/api/mrc20/address/shovel/list", conf.ManDomain)
	//if net == "testnet" {
	//	url = fmt.Sprintf("%s/api/mrc20/tick/address", conf.ManTestDomain)
	//} else if net == "regtest" {
	//	url = fmt.Sprintf("%s/api/mrc20/tick/address", conf.ManRegTestDomain)
	//}

	//fmt.Printf("url:%s\n", url)
	result, err = tool.GetUrl(url, query, nil)
	if err != nil {
		return nil, reqErr
	}
	//fmt.Printf("result:%s\n", result)
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

func FetchMrc20txPointList(txId string, index, cursor, size int64) (*Mrc20UtxoResp, error) {
	var (
		url    string
		result string
		resp   *ManResp
		data   *Mrc20UtxoResp
		err    error
	)
	query := map[string]string{
		"cursor": fmt.Sprintf("%d", cursor),
		"size":   fmt.Sprintf("%d", size),
		"txId":   txId,
		"index":  fmt.Sprintf("%d", index),
	}
	url = fmt.Sprintf("%s/api/mrc20/tx/history", conf.ManDomain)
	//if net == "testnet" {
	//	url = fmt.Sprintf("%s/api/mrc20/tick/address", conf.ManTestDomain)
	//} else if net == "regtest" {
	//	url = fmt.Sprintf("%s/api/mrc20/tick/address", conf.ManRegTestDomain)
	//}

	//fmt.Printf("url:%s\n", url)
	result, err = tool.GetUrl(url, query, nil)
	if err != nil {
		return nil, reqErr
	}
	//fmt.Printf("result:%s\n", result)
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

func FetchMrc20TickList(cursor, size int64, completed bool, order, sortType string) (*Mrc20TickListResp, error) {
	var (
		url    string
		result string
		resp   *ManResp
		data   *Mrc20TickListResp
		err    error
	)
	query := map[string]string{
		"cursor":    fmt.Sprintf("%d", cursor),
		"size":      fmt.Sprintf("%d", size),
		"completed": fmt.Sprintf("%t", completed),
		"order":     order,
		"orderType": sortType,
	}
	url = fmt.Sprintf("%s/api/mrc20/tick/all", conf.ManDomain)
	//if net == "testnet" {
	//	url = fmt.Sprintf("%s/api/mrc20/tick/address", conf.ManTestDomain)
	//} else if net == "regtest" {
	//	url = fmt.Sprintf("%s/api/mrc20/tick/address", conf.ManRegTestDomain)
	//}

	//fmt.Printf("url:%s\n", url)
	result, err = tool.GetUrl(url, query, nil)
	if err != nil {
		return nil, reqErr
	}
	//fmt.Printf("result:%s\n", result)
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

func FetchMrc20TickUsedShovelList(tickId string, cursor, size int64) (*Mrc20TickUsedShovelResp, error) {
	var (
		url    string
		result string
		resp   *ManResp
		data   *Mrc20TickUsedShovelResp
		err    error
	)
	query := map[string]string{
		"cursor": fmt.Sprintf("%d", cursor),
		"size":   fmt.Sprintf("%d", size),
		"tickId": tickId,
	}
	url = fmt.Sprintf("%s/api/mrc20/shovel/used", conf.ManDomain)
	//if net == "testnet" {
	//	url = fmt.Sprintf("%s/api/mrc20/tick/address", conf.ManTestDomain)
	//} else if net == "regtest" {
	//	url = fmt.Sprintf("%s/api/mrc20/tick/address", conf.ManRegTestDomain)
	//}

	//fmt.Printf("url:%s\n", url)
	result, err = tool.GetUrl(url, query, nil)
	if err != nil {
		return nil, reqErr
	}
	//fmt.Printf("result:%s\n", result)
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
