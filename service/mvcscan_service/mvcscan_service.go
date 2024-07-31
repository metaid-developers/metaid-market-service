package mvcscan_service

import (
	"errors"
	"fmt"
	"metaid-market-service/conf"
	"metaid-market-service/tool"
)

var (
	errReq = fmt.Errorf("request error")
)

const (
	CodeSuccess = 0
)

func FetchMetaContractFtSummaryInfo(codeHash, genesis string, headers map[string]string) (*MetaContractFtSummaryResp, error) {
	var (
		url    string
		result string
		resp   *MvcScanResp
		data   *MetaContractFtSummaryResp
		err    error
		query  map[string]string = map[string]string{
			"codehash": codeHash,
			"genesis":  genesis,
		}
	)

	url = fmt.Sprintf("%s/browser/v1/contract/ft/summaries", conf.MvcscanDamain)
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
		return nil, errors.New(fmt.Sprintf("Msg:%v", resp.Data))
	}

	if err = tool.JsonToAny(resp.Data, &data); err != nil {
		return nil, errors.New(fmt.Sprintf("Get request err:%s", err))
	}

	return data, nil
}
