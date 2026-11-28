package mempool_space_service

import (
	"errors"
	"fmt"
	"metaid-market-service/tool"
)

// Get Fee recommended
func GetFeeRecommended() (*FeeRecommended, error) {
	result, status, err := Get("/api/v1/fees/recommended", Foreground)
	if err != nil {
		return nil, err
	}
	if status != 200 {
		return nil, fmt.Errorf("mempool.space fees status %d", status)
	}
	var data *FeeRecommended
	if err = tool.JsonToObject(result, &data); err != nil {
		return nil, errors.New(fmt.Sprintf("Get request err:%s", err))
	}
	return data, nil
}
