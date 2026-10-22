package common_service

import (
	"fmt"
	"metaid-market-service/common"
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
	"metaid-market-service/tool"
	"sort"
	"strings"
	"sync"

	"github.com/shopspring/decimal"
)

var (
	idCoinsListCache      []*respond.Mrc20TickInfo
	idCoinsListCacheTime  int64
	idCoinsListCacheMutex sync.Mutex
)

// FetchIdCoinsTickList serves the market "ID Coins" tab from the fixed id-coin
// snapshot instead of the retired orders.exchange coins-list API. The full list
// is assembled concurrently and cached briefly; search/sort/pagination are
// applied per request.
func FetchIdCoinsTickList(req *request.FetchMrc20TickListReq) (*respond.Mrc20TickListResp, error) {
	tickIds := common.IdCoinsTickIdList()

	idCoinsListCacheMutex.Lock()
	cached := idCoinsListCache
	cacheTime := idCoinsListCacheTime
	idCoinsListCacheMutex.Unlock()
	now := tool.MakeTimestamp()
	if cached == nil || now-cacheTime >= 1000*60 {
		fresh := make([]*respond.Mrc20TickInfo, len(tickIds))
		var wg sync.WaitGroup
		sem := make(chan struct{}, 8)
		for i, tickId := range tickIds {
			wg.Add(1)
			go func(idx int, id string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				info, err := FetchMrc20TickInfo(&request.FetchMrc20TickInfoReq{TickId: id})
				if err != nil {
					fmt.Printf("[IdCoins]fetch tick info err, tickId:%s, err:%v\n", id, err)
					return
				}
				fresh[idx] = info
			}(i, tickId)
		}
		wg.Wait()
		list := make([]*respond.Mrc20TickInfo, 0, len(tickIds))
		for _, info := range fresh {
			if info != nil {
				list = append(list, info)
			}
		}
		cached = list
		idCoinsListCacheMutex.Lock()
		idCoinsListCache = cached
		idCoinsListCacheTime = now
		idCoinsListCacheMutex.Unlock()
	}

	list := cached
	if req.SearchTick != "" {
		keyword := strings.ToUpper(req.SearchTick)
		filtered := make([]*respond.Mrc20TickInfo, 0, len(list))
		for _, v := range list {
			if strings.Contains(strings.ToUpper(v.Tick), keyword) {
				filtered = append(filtered, v)
			}
		}
		list = filtered
	}

	asc := req.SortType > 0
	sort.SliceStable(list, func(i, j int) bool {
		less := idCoinsLess(list[i], list[j], req.OrderBy)
		if asc {
			return less
		}
		return idCoinsLess(list[j], list[i], req.OrderBy)
	})

	total := int64(len(list))
	start := req.Cursor
	if start < 0 {
		start = 0
	}
	if start >= total {
		return &respond.Mrc20TickListResp{Total: total, List: []*respond.Mrc20TickInfo{}}, nil
	}
	end := start + req.Size
	if req.Size <= 0 || end > total {
		end = total
	}
	return &respond.Mrc20TickListResp{Total: total, List: list[start:end]}, nil
}

func idCoinsLess(a, b *respond.Mrc20TickInfo, orderBy string) bool {
	switch strings.ToLower(orderBy) {
	case "tick":
		return strings.ToUpper(a.Tick) < strings.ToUpper(b.Tick)
	case "holders":
		return a.Holders < b.Holders
	case "txcount":
		return a.TxCount < b.TxCount
	case "supply":
		return idCoinsDecimal(a.Supply).LessThan(idCoinsDecimal(b.Supply))
	case "totalsupply":
		return idCoinsDecimal(a.TotalSupply).LessThan(idCoinsDecimal(b.TotalSupply))
	case "marketcap":
		return idCoinsDecimal(a.MarketCap).LessThan(idCoinsDecimal(b.MarketCap))
	case "price", "lastprice":
		return idCoinsDecimal(a.Price).LessThan(idCoinsDecimal(b.Price))
	default: // timestamp / deployTime / deploytime
		return a.DeployTime < b.DeployTime
	}
}

func idCoinsDecimal(src string) decimal.Decimal {
	de, err := decimal.NewFromString(src)
	if err != nil {
		return decimal.Zero
	}
	return de
}
