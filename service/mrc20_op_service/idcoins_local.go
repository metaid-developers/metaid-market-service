package mrc20_op_service

import (
	"errors"
	"fmt"
	"metaid-market-service/common"
	"metaid-market-service/conf"
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
	"metaid-market-service/major"
	"metaid-market-service/models"
	"metaid-market-service/service/common_service"
	"metaid-market-service/service/man_service"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"github.com/spf13/viper"
)

const (
	idCoinsMaintenanceError = "ID-Coin issuance is being upgraded, please try again later"
	idCoinsUpgradeMessage   = "ID-Coin launch is being upgraded"
)

var errIdCoinsMaintenance = errors.New(idCoinsMaintenanceError)

type idCoinsTickCacheEntry struct {
	info        *man_service.Mrc20TickInfo
	refreshedAt time.Time
}

type idCoinsUserInfoCacheEntry struct {
	info      *respond.UserInfo
	expiresAt time.Time
}

type idCoinsFollowersCacheEntry struct {
	total     int64
	expiresAt time.Time
}

var (
	idCoinsTickCacheMu      sync.Mutex
	idCoinsTickCache        = make(map[string]idCoinsTickCacheEntry)
	idCoinsFollowersCacheMu sync.Mutex
	idCoinsFollowersCache   = make(map[string]idCoinsFollowersCacheEntry)
	idCoinsPinPopCacheMu    sync.Mutex
	idCoinsPinPopCache      = make(map[string]int64)
	idCoinsTxVerifyCacheMu  sync.Mutex
	idCoinsTxVerifyCache    = make(map[string]bool)
	idCoinsUserInfoCacheMu  sync.Mutex
	idCoinsUserInfoCache    = make(map[string]idCoinsUserInfoCacheEntry)
)

func idCoinsLegacyDB() string {
	db := strings.TrimSpace(viper.GetString("idcoins.legacy_db"))
	if db == "" {
		return "order_book_db"
	}
	return db
}

// fetchIdCoinTickInfoCached caches per-tick MAN tick info for ~60s. On a
// refresh failure the last good cached value is served instead of failing, so
// a transient MAN error never drops a coin. Nil data is never cached.
func fetchIdCoinTickInfoCached(tickId string) (*man_service.Mrc20TickInfo, error) {
	now := time.Now()
	idCoinsTickCacheMu.Lock()
	entry, cached := idCoinsTickCache[tickId]
	fresh := cached && now.Before(entry.refreshedAt.Add(60*time.Second))
	idCoinsTickCacheMu.Unlock()
	if fresh {
		return entry.info, nil
	}

	info, err := man_service.FetchMrc20TickInfo(tickId, "")
	if err != nil || info == nil {
		if cached {
			return entry.info, nil
		}
		if err == nil {
			err = errors.New("tick info not found")
		}
		return nil, err
	}
	idCoinsTickCacheMu.Lock()
	idCoinsTickCache[tickId] = idCoinsTickCacheEntry{info: info, refreshedAt: now}
	idCoinsTickCacheMu.Unlock()
	return info, nil
}

// fetchIdCoinUserInfoCached caches deployer user info: hits for 5 minutes,
// misses (nil) for 60 seconds.
func fetchIdCoinUserInfoCached(address string) *respond.UserInfo {
	if address == "" {
		return nil
	}
	now := time.Now()
	idCoinsUserInfoCacheMu.Lock()
	if entry, ok := idCoinsUserInfoCache[address]; ok && now.Before(entry.expiresAt) {
		info := entry.info
		idCoinsUserInfoCacheMu.Unlock()
		return info
	}
	idCoinsUserInfoCacheMu.Unlock()

	info := common.FetchMetaIDUserInfo(address)
	ttl := 5 * time.Minute
	if info == nil {
		ttl = 60 * time.Second
	}
	idCoinsUserInfoCacheMu.Lock()
	idCoinsUserInfoCache[address] = idCoinsUserInfoCacheEntry{info: info, expiresAt: now.Add(ttl)}
	idCoinsUserInfoCacheMu.Unlock()
	return info
}

func fetchIdCoinFollowersCountCached(metaId string) int64 {
	if metaId == "" {
		return 0
	}
	now := time.Now()
	idCoinsFollowersCacheMu.Lock()
	if entry, ok := idCoinsFollowersCache[metaId]; ok && now.Before(entry.expiresAt) {
		total := entry.total
		idCoinsFollowersCacheMu.Unlock()
		return total
	}
	idCoinsFollowersCacheMu.Unlock()

	total := int64(0)
	if resp, err := man_service.FetchFollowers(conf.Net, metaId, 0, 1000); err == nil && resp != nil {
		total = resp.Total
	}
	idCoinsFollowersCacheMu.Lock()
	idCoinsFollowersCache[metaId] = idCoinsFollowersCacheEntry{total: total, expiresAt: now.Add(60 * time.Second)}
	idCoinsFollowersCacheMu.Unlock()
	return total
}

func fetchIdCoinPinPopCached(pinId string) int64 {
	idCoinsPinPopCacheMu.Lock()
	if popLv, ok := idCoinsPinPopCache[pinId]; ok {
		idCoinsPinPopCacheMu.Unlock()
		return popLv
	}
	idCoinsPinPopCacheMu.Unlock()
	popLv := int64(0)
	if pinInfo, err := man_service.FetchPinInfo(conf.Net, pinId); err == nil && pinInfo != nil {
		popLv = int64(pinInfo.PopLv)
	}
	idCoinsPinPopCacheMu.Lock()
	idCoinsPinPopCache[pinId] = popLv
	idCoinsPinPopCacheMu.Unlock()
	return popLv
}

func fetchIdCoinTxVerifiedCached(txId string, index int64) bool {
	key := fmt.Sprintf("%s:%d", txId, index)
	idCoinsTxVerifyCacheMu.Lock()
	if verified, ok := idCoinsTxVerifyCache[key]; ok && verified {
		idCoinsTxVerifyCacheMu.Unlock()
		return true
	}
	idCoinsTxVerifyCacheMu.Unlock()
	verified := false
	txPointInfo, _, err := common_service.FetchTxPointInfo("", txId, index, 0, 100)
	if err == nil && len(txPointInfo) > 0 && txPointInfo[0].Verify {
		verified = true
	}
	if verified {
		idCoinsTxVerifyCacheMu.Lock()
		idCoinsTxVerifyCache[key] = true
		idCoinsTxVerifyCacheMu.Unlock()
	}
	return verified
}

func findIdCoinSnapshotByTickId(tickId string) *common.IdCoinSnapshotCoin {
	for _, coin := range common.IdCoinsSnapshot() {
		if coin.TickId == tickId {
			return &coin
		}
	}
	return nil
}

func findIdCoinSnapshotByTick(tick string) *common.IdCoinSnapshotCoin {
	for _, coin := range common.IdCoinsSnapshot() {
		if strings.EqualFold(coin.Tick, tick) {
			return &coin
		}
	}
	return nil
}

func findIdCoinSnapshotByIssuer(issuerAddress string) (*common.IdCoinSnapshotCoin, *man_service.Mrc20TickInfo, error) {
	type match struct {
		coin common.IdCoinSnapshotCoin
		info *man_service.Mrc20TickInfo
	}
	matches := make([]match, 0)
	for _, entry := range fetchIdCoinSnapshotEntries(false) {
		if entry.info.Address == issuerAddress {
			matches = append(matches, match{coin: entry.coin, info: entry.info})
		}
	}
	if len(matches) == 0 {
		return nil, nil, nil
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].info.DeployTime < matches[j].info.DeployTime })
	return &matches[0].coin, matches[0].info, nil
}

// findMergedIdCoinByIssuer is findIdCoinSnapshotByIssuer over the full coin
// set (snapshot first, then self-deployed).
func findMergedIdCoinByIssuer(issuerAddress string) (*common.IdCoinSnapshotCoin, *man_service.Mrc20TickInfo, error) {
	coin, info, err := findIdCoinSnapshotByIssuer(issuerAddress)
	if err != nil || coin != nil {
		return coin, info, err
	}
	type match struct {
		coin common.IdCoinSnapshotCoin
		info *man_service.Mrc20TickInfo
	}
	matches := make([]match, 0)
	deployed, derr := listSelfDeployedCoins()
	if derr != nil {
		return nil, nil, derr
	}
	for _, dc := range deployed {
		dinfo, derr := fetchIdCoinTickInfoCached(dc.TickId)
		if derr != nil || dinfo == nil {
			continue
		}
		if dinfo.Address == issuerAddress {
			matches = append(matches, match{coin: dc, info: dinfo})
		}
	}
	if len(matches) == 0 {
		return nil, nil, nil
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].info.DeployTime < matches[j].info.DeployTime })
	return &matches[0].coin, matches[0].info, nil
}

func idCoinStringFromInt(v int64) string {
	return strconv.FormatInt(v, 10)
}

func idCoinOptionalStringFromInt(v int64) string {
	if v == 0 {
		return ""
	}
	return strconv.FormatInt(v, 10)
}

func idCoinPayAmount(info *man_service.Mrc20TickInfo) int64 {
	if info == nil || info.PayCheck == nil {
		return 0
	}
	amount, _ := strconv.ParseInt(info.PayCheck.PayAmount, 10, 64)
	return amount
}

type idCoinDerivedValues struct {
	price       string
	pool        int64
	totalSupply string
	supply      string
	remaining   string
	mintable    bool
}

func deriveIdCoinValues(info *man_service.Mrc20TickInfo, pricePrecision int32) idCoinDerivedValues {
	ret := idCoinDerivedValues{price: "0", totalSupply: "0", supply: "0", remaining: "0"}
	if info == nil || info.AmtPerMint == "" || info.MintCount == 0 {
		return ret
	}
	totalMintedDe := decimal.New(info.TotalMinted, 0)
	amtPerMintDe, err := decimal.NewFromString(info.AmtPerMint)
	if err != nil || amtPerMintDe.IsZero() {
		return ret
	}
	mintCountDe := decimal.New(info.MintCount, 0)
	payPerPriceDe := decimal.New(idCoinPayAmount(info), 0)
	ret.price = payPerPriceDe.Div(amtPerMintDe).StringFixed(pricePrecision)
	mintedForSupply := totalMintedDe
	if totalMintedDe.GreaterThan(mintCountDe) {
		mintedForSupply = mintCountDe
	}
	ret.supply = mintedForSupply.Mul(amtPerMintDe).String()
	ret.pool = payPerPriceDe.Mul(mintedForSupply).IntPart()
	ret.totalSupply = mintCountDe.Mul(amtPerMintDe).String()
	remainingDe := mintCountDe.Sub(totalMintedDe).Mul(amtPerMintDe)
	ret.remaining = remainingDe.String()
	ret.mintable = remainingDe.GreaterThan(decimal.Zero)
	return ret
}

func buildIdCoinInfo(coin *common.IdCoinSnapshotCoin, info *man_service.Mrc20TickInfo, followerAddress string, includeFollowing bool) *respond.IdCoinsInfoResp {
	return buildIdCoinInfoMode(coin, info, followerAddress, includeFollowing, false)
}

// buildIdCoinInfoMode assembles the response. listMode mirrors the retired
// list endpoint: 8-decimal price, no followersCount lookup, empty ordersPrice.
func buildIdCoinInfoMode(coin *common.IdCoinSnapshotCoin, info *man_service.Mrc20TickInfo, followerAddress string, includeFollowing bool, listMode bool) *respond.IdCoinsInfoResp {
	pricePrecision := int32(2)
	if listMode {
		pricePrecision = 8
	}
	values := deriveIdCoinValues(info, pricePrecision)
	isFollowing := false
	if includeFollowing && followerAddress != "" && info != nil {
		followInfo, _ := man_service.FetchFollowInfo(conf.Net, info.MetaId, common.GetMetaIdByAddress(followerAddress))
		if followInfo != nil && followInfo.Status {
			isFollowing = true
		}
		pins, _ := man_service.FetchPinsByPath(conf.Net, "creator", followerAddress, "/follow", 0, 2000)
		if pins != nil {
			for _, pin := range pins.List {
				if pin != nil && pin.Number == -1 && pin.ContentSummary == info.MetaId {
					isFollowing = true
					break
				}
			}
		}
	}
	followersCount := int64(0)
	if !listMode {
		followersCount = fetchIdCoinFollowersCountCached(info.MetaId)
	}
	// P5: floor display — live burn-maker OrderTokenV2 bids on the coin's
	// FT/WBTC pair. "0"/0 when the coin has no bridged FT or no such bids.
	ordersPrice, ordersPool := idCoinFloorData(coin.TickId)
	resp := &respond.IdCoinsInfoResp{
		TickId:           coin.TickId,
		Tick:             coin.Tick,
		TokenName:        info.TokenName,
		Decimals:         info.Decimals,
		AmtPerMint:       info.AmtPerMint,
		FollowersLimit:   idCoinStringFromInt(info.MintCount),
		MintCount:        idCoinStringFromInt(info.MintCount),
		LiquidityPerMint: idCoinPayAmount(info),
		PremineCount:     idCoinOptionalStringFromInt(info.PremineCount),
		TotalMinted:      idCoinStringFromInt(info.TotalMinted),
		BlockHeight:      info.BeginHeight,
		MetaData:         info.Metadata,
		Type:             coin.TickId,
		Qual:             info.PinCheck,
		PinCheck:         info.PinCheck,
		PayCheck:         info.PayCheck,
		Mrc20Id:          coin.TickId,
		PinNumber:        info.PinNumber,
		Holders:          info.Holders,
		DeployerMetaId:   info.MetaId,
		DeployerAddress:  info.Address,
		DeployerUserInfo: fetchIdCoinUserInfoCached(info.Address),
		DeployTime:       info.DeployTime,
		Price:            values.price,
		PriceUsd:         "",
		Pool:             values.pool,
		TotalSupply:      values.totalSupply,
		Supply:           values.supply,
		Mintable:         values.mintable,
		Remaining:        values.remaining,
		IsFollowing:      isFollowing,
		FollowersCount:   followersCount,
		OrdersPrice:      ordersPrice,
		OrdersPool:       ordersPool,
	}
	return resp
}

func enrichIdCoinMarketInfo(resp *respond.IdCoinsInfoResp) {
	if resp == nil || major.GetSqlDB() == nil {
		return
	}
	marketInfo, _ := models.MarketMrc20InfoModelDao().GetOne(&models.MarketMrc20InfoModel{TickId: resp.Mrc20Id, Chain: "btc"})
	if marketInfo == nil {
		resp.MarketPrice = "0"
		resp.MarketPriceUsd = "0.00"
		resp.FloorPrice = "0"
		resp.FloorPriceUsd = "0.00"
		resp.Change24h = "+0.00%"
		resp.MarketCap = "0"
		resp.MarketCapUsd = "0.00"
		resp.TotalVolume = 0
		return
	}
	floorPrice := strconv.FormatFloat(marketInfo.FloorPrice, 'f', -1, 64)
	price := strconv.FormatFloat(marketInfo.LastPrice, 'f', -1, 64)
	marketCap := strconv.FormatInt(marketInfo.MarketCap, 10)
	change24HDe := decimal.New(marketInfo.Change24H, 0)
	change24h := change24HDe.Div(decimal.New(100, 0)).String() + "%"
	btcUsd := common_service.GetPriceForUsd("BTC")
	btcUsdDe, _ := decimal.NewFromString(btcUsd)
	satUsdDe := btcUsdDe.Div(decimal.New(100000000, 0))
	priceDe, _ := decimal.NewFromString(price)
	priceUsd := priceDe.Mul(satUsdDe).StringFixed(3)
	floorPriceDe, _ := decimal.NewFromString(floorPrice)
	floorPriceUsd := floorPriceDe.Mul(satUsdDe).StringFixed(3)
	marketCapDe, _ := decimal.NewFromString(marketCap)
	marketCapUsd := marketCapDe.Mul(satUsdDe).StringFixed(3)
	resp.MarketPrice = price
	resp.MarketPriceUsd = priceUsd
	resp.FloorPrice = floorPrice
	resp.FloorPriceUsd = floorPriceUsd
	resp.Change24h = change24h
	resp.MarketCap = marketCap
	resp.MarketCapUsd = marketCapUsd
	resp.TotalVolume = marketInfo.TotalVolume
}

func fetchIdCoinsInfoLocally(req *request.FetchOneIdCoinsRequest) (*respond.IdCoinsInfoResp, error) {
	if req.TickId == "" && req.Tick == "" && req.IssuerAddress == "" {
		return nil, errors.New("tickId and tick and issuerAddress are empty")
	}
	var coin *common.IdCoinSnapshotCoin
	var info *man_service.Mrc20TickInfo
	var err error
	if req.TickId != "" {
		coin = findMergedIdCoinByTickId(req.TickId)
		if coin != nil {
			info, err = fetchIdCoinTickInfoCached(coin.TickId)
		}
	} else if req.IssuerAddress != "" {
		coin, info, err = findMergedIdCoinByIssuer(req.IssuerAddress)
	} else {
		coin = findMergedIdCoinByTick(req.Tick)
		if coin != nil {
			info, err = fetchIdCoinTickInfoCached(coin.TickId)
		}
	}
	if err != nil {
		return nil, err
	}
	if coin == nil || info == nil {
		return nil, errors.New("no record")
	}
	resp := buildIdCoinInfo(coin, info, req.Address, true)
	enrichIdCoinMarketInfo(resp)
	return resp, nil
}

type idCoinListEntry struct {
	coin common.IdCoinSnapshotCoin
	info *man_service.Mrc20TickInfo
	resp *respond.IdCoinsInfoResp
}

// fetchMergedIdCoinEntries is fetchIdCoinSnapshotEntries over the full coin
// set: snapshot coins in snapshot order first (behaviour byte-identical when
// nothing self-deployed exists), self-deployed coins appended in deploy order.
func fetchMergedIdCoinEntries(buildResp bool) []*idCoinListEntry {
	entries := fetchIdCoinSnapshotEntries(buildResp)
	deployed, err := listSelfDeployedCoins()
	if err != nil || len(deployed) == 0 {
		return entries
	}
	type result struct {
		idx   int
		entry *idCoinListEntry
	}
	results := make(chan result, len(deployed))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for i, coin := range deployed {
		wg.Add(1)
		go func(idx int, coin common.IdCoinSnapshotCoin) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			info, err := fetchIdCoinTickInfoCached(coin.TickId)
			if err != nil || info == nil {
				return
			}
			entry := &idCoinListEntry{coin: coin, info: info}
			if buildResp {
				entry.resp = buildIdCoinInfoMode(&entry.coin, entry.info, "", false, true)
			}
			results <- result{idx: idx, entry: entry}
		}(i, coin)
	}
	wg.Wait()
	close(results)
	extra := make([]*idCoinListEntry, len(deployed))
	for r := range results {
		extra[r.idx] = r.entry
	}
	for _, entry := range extra {
		if entry != nil {
			entries = append(entries, entry)
		}
	}
	return entries
}

// fetchIdCoinSnapshotEntries fetches tick info for every snapshot coin with
// bounded parallelism (so the cold path is not 32 sequential network calls).
// When buildResp is true the full list-mode responses are assembled inside the
// same goroutines; simple-list passes false to skip user-info/followers work.
func fetchIdCoinSnapshotEntries(buildResp bool) []*idCoinListEntry {
	snapshot := common.IdCoinsSnapshot()
	entries := make([]*idCoinListEntry, len(snapshot))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for i, coin := range snapshot {
		wg.Add(1)
		go func(idx int, coin common.IdCoinSnapshotCoin) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			info, err := fetchIdCoinTickInfoCached(coin.TickId)
			if err != nil || info == nil {
				return
			}
			entry := &idCoinListEntry{coin: coin, info: info}
			if buildResp {
				entry.resp = buildIdCoinInfoMode(&entry.coin, entry.info, "", false, true)
			}
			entries[idx] = entry
		}(i, coin)
	}
	wg.Wait()
	out := make([]*idCoinListEntry, 0, len(entries))
	for _, entry := range entries {
		if entry != nil {
			out = append(out, entry)
		}
	}
	return out
}

func decimalOrZero(src string) decimal.Decimal {
	v, err := decimal.NewFromString(src)
	if err != nil {
		return decimal.Zero
	}
	return v
}

func fetchIdCoinsListLocally(req *request.FetchIdCoinsListRequest) (*respond.FetchIdCoinsListResp, error) {
	if req.Size <= 0 || req.Size >= 100 {
		req.Size = 100
	}
	entries := fetchMergedIdCoinEntries(true)
	if req.SearchTick != "" {
		keyword := strings.ToUpper(req.SearchTick)
		filtered := make([]*idCoinListEntry, 0, len(entries))
		for _, entry := range entries {
			if strings.Contains(strings.ToUpper(entry.coin.Tick), keyword) {
				filtered = append(filtered, entry)
			}
		}
		entries = filtered
	}
	// Entries arrive in snapshot order, which is the production ordering of
	// the retired service for the default timestamp/descending call. Explicit
	// supply/pool ordering sorts stable over that baseline; ascending reverses.
	ascending := req.SortType > 0
	switch strings.ToLower(req.OrderBy) {
	case "supply", "pool":
		sort.SliceStable(entries, func(i, j int) bool {
			if strings.ToLower(req.OrderBy) == "supply" {
				if ascending {
					return decimalOrZero(entries[i].resp.Supply).LessThan(decimalOrZero(entries[j].resp.Supply))
				}
				return decimalOrZero(entries[j].resp.Supply).LessThan(decimalOrZero(entries[i].resp.Supply))
			}
			if ascending {
				return entries[i].resp.Pool < entries[j].resp.Pool
			}
			return entries[j].resp.Pool < entries[i].resp.Pool
		})
	default:
		if ascending {
			for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}
	total := int64(len(entries))
	start := req.Cursor
	if start < 0 {
		start = 0
	}
	if start >= total {
		return &respond.FetchIdCoinsListResp{Total: total, List: []*respond.IdCoinsInfoResp{}}, nil
	}
	end := start + req.Size
	if req.Size <= 0 || end > total {
		end = total
	}
	list := make([]*respond.IdCoinsInfoResp, 0, end-start)
	for _, entry := range entries[start:end] {
		list = append(list, entry.resp)
	}
	return &respond.FetchIdCoinsListResp{Total: total, List: list}, nil
}

func fetchAddressIdCoinsInfoLocally(req *request.FetchAddressIdCoinInfoRequest) (*respond.IdCoinsInfoResp, error) {
	if req.IssuerAddress == "" {
		return nil, errors.New("issuerAddress is empty")
	}
	coin, info, err := findMergedIdCoinByIssuer(req.IssuerAddress)
	if err != nil {
		return nil, err
	}
	if coin == nil || info == nil {
		return nil, errors.New("no record")
	}
	resp := buildIdCoinInfo(coin, info, "", false)
	enrichIdCoinMarketInfo(resp)
	zeroProgress := float64(0)
	resp.Progress = &zeroProgress
	return resp, nil
}

func idCoinsSimpleInfoPage(entries []*idCoinListEntry, cursor, size int64) *respond.FetchIdCoinsSimpleInfoListResp {
	// The retired service ordered this list by deploy-order timestamp ascending;
	// the snapshot is frozen in descending order, so reversing reproduces it.
	ordered := make([]*idCoinListEntry, 0, len(entries))
	for i := len(entries) - 1; i >= 0; i-- {
		ordered = append(ordered, entries[i])
	}
	entries = ordered
	total := int64(len(entries))
	start := cursor
	if start < 0 {
		start = 0
	}
	if start >= total {
		return &respond.FetchIdCoinsSimpleInfoListResp{Total: total, List: []*respond.IdCoinsSimpleInfoResp{}}
	}
	end := start + size
	if size <= 0 || end > total {
		end = total
	}
	list := make([]*respond.IdCoinsSimpleInfoResp, 0, end-start)
	for _, entry := range entries[start:end] {
		list = append(list, &respond.IdCoinsSimpleInfoResp{
			TickId:          entry.coin.TickId,
			Tick:            entry.coin.Tick,
			TokenName:       entry.info.TokenName,
			DeployerMetaId:  entry.info.MetaId,
			DeployerAddress: entry.info.Address,
		})
	}
	return &respond.FetchIdCoinsSimpleInfoListResp{Total: total, List: list}
}

func fetchIdCoinsSimpleInfoListLocally(cursor, size int64) (*respond.FetchIdCoinsSimpleInfoListResp, error) {
	return idCoinsSimpleInfoPage(fetchMergedIdCoinEntries(false), cursor, size), nil
}
