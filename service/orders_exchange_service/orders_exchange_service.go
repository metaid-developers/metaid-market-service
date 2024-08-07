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

func FetchIdCoinsOpOrders(req *FetchIdCoinsOpOrdersRequest, headers map[string]string) (*FetchIdCoinsOpOrdersResp, error) {
	var (
		url    string
		result string
		resp   *Message
		data   *FetchIdCoinsOpOrdersResp
		err    error
		query  map[string]string = map[string]string{
			"opOrderType":  req.OpOrderType,
			"address":      req.Address,
			"tickId":       req.TickId,
			"cursor":       fmt.Sprintf("%d", req.Cursor),
			"size":         fmt.Sprintf("%d", req.Size),
			"confirmation": fmt.Sprintf("%d", req.Confirmation),
		}
	)
	headers = addHeaderKey(headers)

	url = fmt.Sprintf("%s/id-coins/inscribe/orders", conf.OrdersExchangeDomain)
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

func FetchIdCoinsAddressMintOrder(req *FetchIdCoinsMintOrderRequest, headers map[string]string) (*FetchOneIdCoinsMintOrderResp, error) {
	var (
		url    string
		result string
		resp   *Message
		data   *FetchOneIdCoinsMintOrderResp
		err    error
		query  map[string]string = map[string]string{
			"address": req.Address,
			"tickId":  req.TickId,
		}
	)
	headers = addHeaderKey(headers)

	url = fmt.Sprintf("%s/id-coins/address/mint/order", conf.OrdersExchangeDomain)
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

func FetchIdCoinsList(req *FetchIdCoinsListRequest, headers map[string]string) (*FetchIdCoinsListResp, error) {
	var (
		url    string
		result string
		resp   *Message
		data   *FetchIdCoinsListResp
		err    error
		query  map[string]string = map[string]string{
			"address":         req.Address,
			"cursor":          fmt.Sprintf("%d", req.Cursor),
			"size":            fmt.Sprintf("%d", req.Size),
			"orderBy":         req.OrderBy,
			"sortType":        fmt.Sprintf("%d", req.SortType),
			"followerAddress": req.FollowerAddress,
			"searchTick":      req.SearchTick,
		}
	)
	headers = addHeaderKey(headers)

	url = fmt.Sprintf("%s/id-coins/coins-list", conf.OrdersExchangeDomain)
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

func FetchOneIdCoinsInfo(req *FetchOneIdCoinsRequest, headers map[string]string) (*IdCoinsInfoResp, error) {
	var (
		url    string
		result string
		resp   *Message
		data   *IdCoinsInfoResp
		err    error
		query  map[string]string = map[string]string{
			"tickId":        req.TickId,
			"tick":          req.Tick,
			"issuerAddress": req.IssuerAddress,
			"address":       req.Address,
		}
	)
	headers = addHeaderKey(headers)

	url = fmt.Sprintf("%s/id-coins/coins-info", conf.OrdersExchangeDomain)
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

func FetchIdCoinsTickIds(headers map[string]string) (*FetchIdCoinsTickIdsResp, error) {
	var (
		url    string
		result string
		resp   *Message
		data   *FetchIdCoinsTickIdsResp
		err    error
		query  map[string]string = map[string]string{}
	)
	headers = addHeaderKey(headers)

	url = fmt.Sprintf("%s/id-coins/tick-ids", conf.OrdersExchangeDomain)
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

func FetchIdCoinsDeployCheckInfo(req *FetchIdCoinsDeployCheckRequest, headers map[string]string) (*FetchIdCoinsDeployCheckResp, error) {
	var (
		url    string
		result string
		resp   *Message
		data   *FetchIdCoinsDeployCheckResp
		err    error
		query  map[string]string = map[string]string{
			"address": req.Address,
		}
	)
	headers = addHeaderKey(headers)

	url = fmt.Sprintf("%s/id-coins/deploy/check/info", conf.OrdersExchangeDomain)
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

func RefundIdCoinsInvalidLpPre(req *RefundIdCoinsValidPreRequest, headers map[string]string) (*RefundIdCoinsValidPreResp, error) {
	var (
		url    string
		result string
		resp   *Message
		data   *RefundIdCoinsValidPreResp
		err    error
	)
	headers = addHeaderKey(headers)

	url = fmt.Sprintf("%s/id-coins/invalid/refund/pre", conf.OrdersExchangeDomain)
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

func RefundIdCoinsInvalidLpCommit(req *RefundIdCoinsValidCommitRequest, headers map[string]string) (*RefundIdCoinsValidCommitResp, error) {
	var (
		url    string
		result string
		resp   *Message
		data   *RefundIdCoinsValidCommitResp
		err    error
	)
	headers = addHeaderKey(headers)

	url = fmt.Sprintf("%s/id-coins/invalid/refund/commit", conf.OrdersExchangeDomain)
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

func BookOrderTakeMintBidPreview(req *BookTakeMintBidPreviewReq, headers map[string]string) (*BookMintOrderTakePreviewResp, error) {
	var (
		url    string
		result string
		resp   *Message
		data   *BookMintOrderTakePreviewResp
		err    error
	)
	headers = addHeaderKey(headers)

	url = fmt.Sprintf("%s/book/bid-mint-take/preview", conf.OrdersExchangeDomain)
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

func BookOrderTakeMintBidPre(req *BookTakeMintBidPreReq, headers map[string]string) (*BookMintOrderTakePreResp, error) {
	var (
		url    string
		result string
		resp   *Message
		data   *BookMintOrderTakePreResp
		err    error
	)
	headers = addHeaderKey(headers)

	url = fmt.Sprintf("%s/book/bid-mint-take/pre", conf.OrdersExchangeDomain)
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

func BookOrderTakeMintBidCommit(req *BookTakeMintBidCommitReq, headers map[string]string) (*BookMintOrderTakeCommitResp, error) {
	var (
		url    string
		result string
		resp   *Message
		data   *BookMintOrderTakeCommitResp
		err    error
	)
	headers = addHeaderKey(headers)

	url = fmt.Sprintf("%s/book/bid-mint-take/commit", conf.OrdersExchangeDomain)
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
