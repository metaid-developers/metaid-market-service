package own_service

import (
	"errors"
	"fmt"
	"metaid-market-service/conf"
	"metaid-market-service/tool"
	"strings"
)

const (
	OwnCodeSuccess     = 2000
	OwnStatusSuccessV3 = "1"
)

var (
	errReq = errors.New("own-service request err")
)

func CheckUtxoInfo(net string, outPoints []string) (map[string]*OwnUtxoInfo, error) {
	var (
		url    string
		result string
		resp   *OwnResp
		data   map[string]*OwnUtxoInfo
		err    error
		req    map[string]interface{} = map[string]interface{}{
			"outPoints": outPoints,
		}
	)

	url = fmt.Sprintf("%s/tx/btc-utxo/check", conf.OwnDomain)
	//fmt.Println(url)
	result, err = tool.PostUrl(url, req, nil)
	if err != nil {
		return nil, errReq
	}

	//fmt.Println(result)
	if err = tool.JsonToObject(result, &resp); err != nil {
		return nil, errors.New(fmt.Sprintf("Get request err:%s", err))
	}

	if resp.Code != OwnCodeSuccess {
		fmt.Printf("[CheckUtxoInfo]Msg:%v\n", resp.Data)
		return nil, errors.New(fmt.Sprintf("Msg:%v", resp.Data))
	}

	if err = tool.JsonToAny(resp.Data, &data); err != nil {
		return nil, errors.New(fmt.Sprintf("Get request err:%s", err))
	}

	return data, nil
}

// broadcast Transaction
func BroadcastTransaction(net, signedhex string) (string, error) {
	var (
		url    string
		result string
		data   *OwnResp
		err    error
	)
	query := map[string]string{
		"rawTx": signedhex,
	}
	url = fmt.Sprintf("%s/btc/broadcastTx", conf.OwnDomain)
	result, err = tool.PostUrl(url, query, nil)
	if err != nil {
		return "", errReq
	}
	fmt.Printf("[OWN]BroadcastTransaction result:%s\n", result)
	if err = tool.JsonToObject(result, &data); err != nil {
		return "", fmt.Errorf("get request err:%s", err.Error())
	}

	if data.Code != OwnCodeSuccess {
		return "", fmt.Errorf("%s", data.Msg)
	}

	//go unisat_service.BroadcastTx(net, signedhex)

	return strings.Trim(data.Msg, "\""), nil
}

// GetTxInfo
func GetTxInfo(net, txId string) (*CheckTxInfo, error) {
	var (
		url    string
		result string
		resp   *OwnResp
		data   *CheckTxInfo
		err    error
	)
	query := map[string]string{
		"txId": txId,
	}
	url = fmt.Sprintf("%s/tx/detail", conf.OwnDomain)
	result, err = tool.GetUrl(url, query, nil)
	if err != nil {
		return nil, errReq
	}
	if err = tool.JsonToObject(result, &resp); err != nil {
		return nil, fmt.Errorf("get request err:%s", err.Error())
	}

	if resp.Code != OwnCodeSuccess {
		return nil, fmt.Errorf("%s", resp.Msg)
	}

	if err = tool.JsonToAny(resp.Data, &data); err != nil {
		return nil, fmt.Errorf("get request err:%s", err.Error())
	}
	return data, nil
}

// GetTxInfo
func GetTxRaw(txId string) (string, error) {
	var (
		url    string
		result string
		resp   *OwnResp
		data   string
		err    error
	)
	query := map[string]string{
		"txId": txId,
	}
	url = fmt.Sprintf("%s/btc/getRawTx", conf.OwnDomain)
	result, err = tool.PostUrl(url, query, nil)
	if err != nil {
		return "", errReq
	}
	if err = tool.JsonToObject(result, &resp); err != nil {
		return "", fmt.Errorf("get request err:%s", err.Error())
	}

	if resp.Code != OwnCodeSuccess {
		return "", fmt.Errorf("%s", resp.Msg)
	}
	data = resp.Msg
	data = strings.Trim(data, "\"")
	return data, nil
}
