package wallet_service

import (
	"errors"
	"fmt"
	"metaid-market-service/conf"
	"metaid-market-service/tool"
)

var (
	errReq = errors.New("request error")
)

func FetchPoolKey(host, tick, protocol string, headers map[string]string) (*PoolKey, error) {
	var (
		url    string
		result string
		resp   *WalletMessage
		data   *PoolKey
		//data   *Brc20PoolKey
		err   error
		query map[string]string = map[string]string{
			"tick":     tick,
			"protocol": protocol,
		}
	)
	if host == "" {
		host = conf.WalletDomain
	}

	url = fmt.Sprintf("%s/getBrc20PoolKey", host)
	result, err = tool.GetUrl(url, query, headers)
	if err != nil {
		fmt.Printf("err:%s\n", err)
		return nil, errReq
	}

	//fmt.Println(result)
	if err = tool.JsonToObject(result, &resp); err != nil {
		return nil, errors.New(fmt.Sprintf("Get request err:%s", err))
	}
	if err = tool.JsonToAny(resp.Data, &data); err != nil {
		return nil, errors.New(fmt.Sprintf("Get request err:%s", err))
	}

	return data, nil
}
