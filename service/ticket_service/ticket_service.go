package ticket_service

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

func FetchClubTicketPriceInfo(req *FetchClubTicketPriceInfoRequest, headers map[string]string) (*FetchClubTicketPriceInfoResp, error) {
	var (
		url    string
		result string
		resp   *Message
		data   *FetchClubTicketPriceInfoResp
		err    error
		query  map[string]string = map[string]string{
			"tick":     req.Tick,
			"ticketId": req.TicketId,
		}
	)

	url = fmt.Sprintf("%s/api/v1/ticket/club/price/info", conf.TicketFansDomain)
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
