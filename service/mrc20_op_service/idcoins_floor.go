package mrc20_op_service

import (
	"encoding/json"
	"io"
	"metaid-market-service/service/orderbook_service"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"github.com/spf13/viper"
)

const (
	idCoinWbtcCodeHash = "c9cc7bbd1010b44873959a8b1a2bcedeb62302b7"
	idCoinWbtcTokenID  = "2ee1af1ae2d3605f08894c17358c99f1fc4c1325"
	idCoinBurnMaker20  = "0000000000000000000000000000000000000000"
)

type idCoinBridgeAsset struct {
	Network             string `json:"network"`
	OriginTokenId       string `json:"originTokenId"`
	TargetTokenId       string `json:"targetTokenId"`
	TargetTokenCodeHash string `json:"targetTokenCodeHash"`
	TargetTokenGenesis  string `json:"targetTokenGenesis"`
	TargetSymbol        string `json:"targetSymbol"`
	Decimals            int64  `json:"decimals"`
	IsEnableMint        bool   `json:"isEnableMint"`
}

type idCoinBridgeFT struct {
	codeHash string
	genesis  string
	decimals int64
}

var (
	idCoinBridgeCacheMu sync.Mutex
	idCoinBridgeCache   []idCoinBridgeAsset
	idCoinBridgeCacheAt time.Time
)

func idCoinBridgeAssetListURL() string {
	url := strings.TrimSpace(viper.GetString("idcoins.bridge_asset_list_url"))
	if url == "" {
		return "https://api.octopus.space/api-bridge-v3/assetList"
	}
	return url
}

// fetchBridgeAssetListCached fetches the octopus bridge asset list once per
// 5 minutes (spec floor); on a refresh failure the last good list is served.
func fetchBridgeAssetListCached() ([]idCoinBridgeAsset, error) {
	now := time.Now()
	idCoinBridgeCacheMu.Lock()
	if idCoinBridgeCache != nil && now.Before(idCoinBridgeCacheAt.Add(5*time.Minute)) {
		list := idCoinBridgeCache
		idCoinBridgeCacheMu.Unlock()
		return list, nil
	}
	idCoinBridgeCacheMu.Unlock()

	list, err := fetchBridgeAssetList(idCoinBridgeAssetListURL())
	if err != nil {
		idCoinBridgeCacheMu.Lock()
		cached := idCoinBridgeCache
		idCoinBridgeCacheMu.Unlock()
		if cached != nil {
			return cached, nil
		}
		return nil, err
	}
	idCoinBridgeCacheMu.Lock()
	idCoinBridgeCache = list
	idCoinBridgeCacheAt = now
	idCoinBridgeCacheMu.Unlock()
	return list, nil
}

func fetchBridgeAssetList(url string) ([]idCoinBridgeAsset, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Data struct {
			AssetList []idCoinBridgeAsset `json:"assetList"`
		} `json:"data"`
		AssetList []idCoinBridgeAsset `json:"assetList"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}
	if envelope.Data.AssetList != nil {
		return envelope.Data.AssetList, nil
	}
	if envelope.AssetList != nil {
		return envelope.AssetList, nil
	}
	return []idCoinBridgeAsset{}, nil
}

// resolveIdCoinBridgeFT finds the bridged MVC FT for a coin: an asset pair
// whose originTokenId equals the coin's MRC20 tickId.
func resolveIdCoinBridgeFT(tickId string) *idCoinBridgeFT {
	list, err := fetchBridgeAssetListCached()
	if err != nil {
		return nil
	}
	for _, asset := range list {
		if asset.OriginTokenId != tickId {
			continue
		}
		if asset.TargetTokenCodeHash == "" {
			continue
		}
		genesis := asset.TargetTokenGenesis
		if genesis == "" {
			genesis = asset.TargetTokenId
		}
		if genesis == "" {
			continue
		}
		return &idCoinBridgeFT{
			codeHash: strings.ToLower(asset.TargetTokenCodeHash),
			genesis:  strings.ToLower(genesis),
			decimals: asset.Decimals,
		}
	}
	return nil
}

// idCoinFloorData aggregates the live burn-maker (zero makerAddr) OrderTokenV2
// bids on the coin's FT/WBTC pair in the in-process orderbook: pool = total
// locked WBTC sats remaining, price = best (highest) bid in sats per whole
// coin, formatted fixed-8 like the legacy field. Zero values when the coin
// has no bridged FT or no such bids.
func idCoinFloorData(tickId string) (string, int64) {
	ft := resolveIdCoinBridgeFT(tickId)
	if ft == nil {
		return "0", 0
	}
	book := orderbookFloorBook()
	if book == nil {
		return "0", 0
	}
	// Canonicalize exactly like the book does (same quote-priority table), so
	// the pair key and the side our burn orders land on always match the
	// book's own coordinates regardless of the priority configuration.
	pairKey, _, _, _, _, canonicalSide := orderbook_service.CanonicalizeTokenPair(idCoinWbtcCodeHash, idCoinWbtcTokenID, ft.codeHash, ft.genesis)

	var (
		pool      int64 = 0
		bestPrice decimal.Decimal
		bestFound bool
	)
	for _, order := range book.ListLiveOrders("", pairKey, canonicalSide) {
		if !order.IsTokenOrder {
			continue
		}
		if order.MakerHash20 != idCoinBurnMaker20 {
			continue
		}
		if order.SoftLocked {
			continue
		}
		if !strings.EqualFold(order.SellTokenCodeHash, idCoinWbtcCodeHash) || !strings.EqualFold(order.SellTokenID, idCoinWbtcTokenID) {
			continue
		}
		if !strings.EqualFold(order.BuyTokenCodeHash, ft.codeHash) || !strings.EqualFold(order.BuyTokenID, ft.genesis) {
			continue
		}
		pool += int64(order.RemainAmount)
		if order.PriceDen != 0 {
			price := decimal.NewFromInt(int64(order.PriceNum)).Div(decimal.NewFromInt(int64(order.PriceDen)))
			if !bestFound || price.GreaterThan(bestPrice) {
				bestPrice = price
				bestFound = true
			}
		}
	}
	if !bestFound {
		return "0", 0
	}
	// canonical price is WBTC-sats per raw FT unit; scale to sats per whole coin
	perCoin := bestPrice.Mul(decimal.New(1, int32(ft.decimals)))
	return perCoin.StringFixed(8), pool
}

// orderbookFloorBook is a seam for tests: production uses the in-process
// orderbook syncer's book.
var orderbookFloorBook = func() *orderbook_service.OrderBook {
	syncer := orderbook_service.GetOrderbookSyncer()
	if syncer == nil {
		return nil
	}
	return syncer.Book()
}
