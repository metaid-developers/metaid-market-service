package orders_exchange_service

import (
	"errors"
	"fmt"
	"metaid-market-service/conf"
	"metaid-market-service/tool"
)

const (
	CodeSuccess = 0
)

var (
	errReq = fmt.Errorf("Request error")
)

func addHeaderKey(headers map[string]string) map[string]string {
	if headers == nil {
		headers = make(map[string]string)
	}
	headers["X-Meta-Id-Market-Key"] = conf.OrdersExchangeKey
	return headers
}

func BuildIdCoinsPre(req *BuildIdCoinsPreRequest, headers map[string]string) (*BuildIdCoinsPreResp, error) {
	var (
		url    string
		result string
		resp   *Message
		data   *BuildIdCoinsPreResp
		err    error
	)
	headers = addHeaderKey(headers)

	url = fmt.Sprintf("%s/id-coins/deploy/pre", conf.OrdersExchangeDomain)
	//fmt.Println(url)
	result, err = tool.PostUrl(url, req, headers)
	if err != nil {
		return nil, errReq
	}

	//fmt.Println(result)
	if err = tool.JsonToObject(result, &resp); err != nil {
		return nil, errors.New(fmt.Sprintf("Get request err:%s", err))
	}

	if resp.Code != CodeSuccess {
		return nil, errors.New(fmt.Sprintf("Msg:%v", resp.Message))
	}

	if err = tool.JsonToAny(resp.Data, &data); err != nil {
		return nil, errors.New(fmt.Sprintf("Get request err:%s", err))
	}

	return data, nil
}

func BuildIdCoinsCommit(req *BuildIdCoinsCommitRequest, headers map[string]string) (*BuildIdCoinsCommitResp, error) {
	var (
		url    string
		result string
		resp   *Message
		data   *BuildIdCoinsCommitResp
		err    error
	)
	headers = addHeaderKey(headers)

	url = fmt.Sprintf("%s/id-coins/deploy/commit", conf.OrdersExchangeDomain)
	//fmt.Println(url)
	result, err = tool.PostUrl(url, req, headers)
	if err != nil {
		return nil, errReq
	}

	//fmt.Println(result)
	if err = tool.JsonToObject(result, &resp); err != nil {
		return nil, errors.New(fmt.Sprintf("Get request err:%s", err))
	}

	if resp.Code != CodeSuccess {
		return nil, errors.New(fmt.Sprintf("Msg:%v", resp.Message))
	}

	if err = tool.JsonToAny(resp.Data, &data); err != nil {
		return nil, errors.New(fmt.Sprintf("Get request err:%s", err))
	}

	return data, nil
}

func IdCoinsMintPre(req *IdCoinsMintPreRequest, headers map[string]string) (*IdCoinsMintPreResp, error) {
	var (
		url    string
		result string
		resp   *Message
		data   *IdCoinsMintPreResp
		err    error
	)
	headers = addHeaderKey(headers)

	url = fmt.Sprintf("%s/id-coins/mint/pre", conf.OrdersExchangeDomain)
	//fmt.Println(url)
	result, err = tool.PostUrl(url, req, headers)
	if err != nil {
		return nil, errReq
	}

	//fmt.Println(result)
	if err = tool.JsonToObject(result, &resp); err != nil {
		return nil, errors.New(fmt.Sprintf("Get request err:%s", err))
	}

	if resp.Code != CodeSuccess {
		return nil, errors.New(fmt.Sprintf("Msg:%v", resp.Message))
	}

	if err = tool.JsonToAny(resp.Data, &data); err != nil {
		return nil, errors.New(fmt.Sprintf("Get request err:%s", err))
	}

	return data, nil
}

func IdCoinsMintCommit(req *IdCoinsMintCommitRequest, headers map[string]string) (*IdCoinsMintCommitResp, error) {
	var (
		url    string
		result string
		resp   *Message
		data   *IdCoinsMintCommitResp
		err    error
	)
	headers = addHeaderKey(headers)

	url = fmt.Sprintf("%s/id-coins/mint/commit", conf.OrdersExchangeDomain)
	//fmt.Println(url)
	result, err = tool.PostUrl(url, req, headers)
	if err != nil {
		return nil, errReq
	}

	//fmt.Println(result)
	if err = tool.JsonToObject(result, &resp); err != nil {
		return nil, errors.New(fmt.Sprintf("Get request err:%s", err))
	}

	if resp.Code != CodeSuccess {
		return nil, errors.New(fmt.Sprintf("Msg:%v", resp.Message))
	}

	if err = tool.JsonToAny(resp.Data, &data); err != nil {
		return nil, errors.New(fmt.Sprintf("Get request err:%s", err))
	}

	return data, nil
}
