package mempool_space_service

import (
	"errors"
	"fmt"
	"metaid-market-service/conf"
	"metaid-market-service/tool"
)

// Get Fee recommended
func GetFeeRecommended() (*FeeRecommended, error) {
	var (
		url     string
		result  string
		data    *FeeRecommended
		err     error
		query   map[string]string = map[string]string{}
		headers map[string]string = map[string]string{}
	)
	url = fmt.Sprintf("%s/api/v1/fees/recommended", conf.MempoolSpace)

	result, _, err = tool.GetUrlAndCode(url, query, headers)
	if err != nil {
		fmt.Printf("Get request err:%s\n", err)
		return nil, err
	}
	fmt.Printf("GetFeeRecommended result:%s\n", result)
	if err = tool.JsonToObject(result, &data); err != nil {
		return nil, errors.New(fmt.Sprintf("Get request err:%s", err))
	}

	return data, nil
}
