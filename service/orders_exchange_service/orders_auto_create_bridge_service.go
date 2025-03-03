package orders_exchange_service

import (
	"errors"
	"fmt"
	"metaid-market-service/conf"
	"metaid-market-service/tool"
)

func AddBridgeBuild(req *AdminAddAutoBridgeReq, headers map[string]string) (*AdminAddAutoBridgeResp, error) {
	var (
		url    string
		result string
		resp   *Message
		data   *AdminAddAutoBridgeResp
		err    error
	)
	headers = addHeaderKey(headers)

	url = fmt.Sprintf("%s/admin/auto-bridge/pool/add", conf.OrdersExchangeDomain)
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

func GetBridgeBuildInfo(req *AdminAutoBridgeInfoReq, headers map[string]string) (*AdminAutoBridgeInfoResp, error) {
	var (
		url    string
		result string
		resp   *Message
		data   *AdminAutoBridgeInfoResp
		err    error
		query  map[string]string = map[string]string{
			"mrc20Id": req.Mrc20Id,
		}
	)
	headers = addHeaderKey(headers)

	url = fmt.Sprintf("%s/admin/auto-bridge/pool/info", conf.OrdersExchangeDomain)
	//fmt.Println(url)
	result, err = tool.GetUrl(url, query, headers)
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
