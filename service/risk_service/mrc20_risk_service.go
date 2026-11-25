package risk_service

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"metaid-market-service/common"
	"metaid-market-service/conf"
	"metaid-market-service/major"
	"metaid-market-service/models"
	"metaid-market-service/tool"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/btcsuite/btcd/btcutil/psbt"
	"github.com/btcsuite/btcd/wire"
	"gorm.io/gorm"
)

var ErrBlockedStolenAsset = errors.New("Asset or address is blocked due to theft/security risk. ")

const (
	// Incident cluster minimum block height (~977028 was the first phishing drain tx).
	// Confirmed transactions strictly below minTaintWalkBlockHeight occurred before the
	// phishing mnemonic leak: unless explicitly listed in blackTxIds/blackUtxos (or currently
	// held by a blacklisted wallet), pre-incident inputs must never taint counterparties.
	minTaintWalkBlockHeight int64 = 977025
	maxTaintWalkDepth       int   = 15
	maxTaintVisitedTxs      int   = 40
	maxBatchExchangeOutputs int   = 20
	maxMultiPartyInputAddrs int   = 10
	cacheRefreshInterval          = 15 * time.Second
	maxCachedCleanOutpoints       = 20000
	maxCachedChainTxs             = 10000
	maxCachedTrades               = 20000
	negativeTradeCacheTTL         = 30 * time.Second

	mainnetPlatformDummyAddr = "bc1q0hzgkuls62egjpqxxhzs3cky7auyz596252p78"
	mainnetPlatformFeeAddr   = "bc1pf4gj2uyjfytzzx3h06m977tvw6pl6hgy6kmgyle2757pcrrukr8s2srvy5"
	victim1SelfRescueAddr1   = "13TzdVkgh8WxpKwc6P5S2oRfW3U5LqE1xS"
	victim1SelfRescueAddr2   = "1AkEdzTXgDUckLb19yoGspBTMwxjvxHKzY"
)

type ChainTxInput struct {
	TxId    string
	Vout    uint32
	Address string
	Value   int64
}

type ChainTxOutput struct {
	Address string
	Value   int64
}

type ChainTx struct {
	TxId        string
	Confirmed   bool
	BlockHeight int64
	Inputs      []ChainTxInput
	Outputs     []ChainTxOutput
}

type CompletedMarketTrade struct {
	TxId          string
	SellerAddress string
	BuyerAddress  string
}

type TxFetcherFunc func(chain, txId string) (*ChainTx, error)
type MarketTradeLookupFunc func(txId string) (*CompletedMarketTrade, error)
type PersistBlacklistFunc func(entry *models.MarketBlacklistModel) error

type TaintHint struct {
	FromAddress string
	PrevTxId    string
	PrevVout    uint32
}

type RiskEngine struct {
	mu sync.RWMutex

	blackAddrs map[string]string // normalized addr -> reason
	blackTxIds map[string]string // lowercase txid -> reason
	blackUtxos map[string]string // lowercase txid_vout -> reason

	whiteAddrs map[string]bool
	whiteTxIds map[string]bool
	whiteUtxos map[string]bool

	cleanMu        sync.RWMutex
	cleanOutpoints map[string]bool

	txCacheMu sync.RWMutex
	txCache   map[string]*ChainTx // confirmed ChainTx cache

	tradeCacheMu sync.RWMutex
	tradeCache   map[string]cachedTradeEntry

	txFetcher   TxFetcherFunc
	tradeLookup MarketTradeLookupFunc
	persister   PersistBlacklistFunc
}

type cachedTradeEntry struct {
	trade     *CompletedMarketTrade
	expiresAt time.Time
}

var (
	defaultEngineOnce sync.Once
	defaultEngine     *RiskEngine
	mempoolHTTPClient = &http.Client{Timeout: 5 * time.Second}
	sweepRunning      int32
)

func NewRiskEngine(fetcher TxFetcherFunc, tradeLookup MarketTradeLookupFunc, persister PersistBlacklistFunc) *RiskEngine {
	return &RiskEngine{
		blackAddrs:     make(map[string]string),
		blackTxIds:     make(map[string]string),
		blackUtxos:     make(map[string]string),
		whiteAddrs:     make(map[string]bool),
		whiteTxIds:     make(map[string]bool),
		whiteUtxos:     make(map[string]bool),
		cleanOutpoints: make(map[string]bool),
		txCache:        make(map[string]*ChainTx),
		tradeCache:     make(map[string]cachedTradeEntry),
		txFetcher:      fetcher,
		tradeLookup:    tradeLookup,
		persister:      persister,
	}
}

func DefaultEngine() *RiskEngine {
	defaultEngineOnce.Do(func() {
		defaultEngine = NewRiskEngine(defaultFetchChainTx, defaultLookupCompletedTrade, defaultPersistBlacklist)
		defaultEngine.LoadSeedEntriesInMemory()
	})
	return defaultEngine
}

func NormalizeAddress(addr string) string {
	a := strings.TrimSpace(addr)
	low := strings.ToLower(a)
	if strings.HasPrefix(low, "bc1") || strings.HasPrefix(low, "tb1") || strings.HasPrefix(low, "bcrt1") {
		return low
	}
	return a
}

func NormalizeTxId(txId string) string {
	return strings.ToLower(strings.TrimSpace(txId))
}

func NormalizeUtxoId(utxoId string) string {
	s := strings.TrimSpace(utxoId)
	s = strings.ReplaceAll(s, ":", "_")
	idx := strings.LastIndex(s, "_")
	if idx <= 0 || idx >= len(s)-1 {
		return strings.ToLower(s)
	}
	return fmt.Sprintf("%s_%s", strings.ToLower(strings.TrimSpace(s[:idx])), strings.TrimSpace(s[idx+1:]))
}

func ParseUtxoId(utxoId string) (string, int64, bool) {
	norm := NormalizeUtxoId(utxoId)
	idx := strings.LastIndex(norm, "_")
	if idx <= 0 || idx >= len(norm)-1 {
		return "", 0, false
	}
	txId := norm[:idx]
	vout, err := strconv.ParseInt(norm[idx+1:], 10, 64)
	if err != nil || vout < 0 {
		return "", 0, false
	}
	return txId, vout, true
}

func DefaultSeedEntries() []*models.MarketBlacklistModel {
	entries := []*models.MarketBlacklistModel{
		// Victim #1 verified clean self-rescue wallets & rescue UTXOs (explicitly protected)
		{EntryType: models.BlacklistTypeWhitelistAddress, EntryValue: "13TzdVkgh8WxpKwc6P5S2oRfW3U5LqE1xS", Source: "seed", Reason: "Victim #1 verified clean rescue wallet"},
		{EntryType: models.BlacklistTypeWhitelistAddress, EntryValue: "1AkEdzTXgDUckLb19yoGspBTMwxjvxHKzY", Source: "seed", Reason: "Victim #1 verified clean rescue wallet"},
		{EntryType: models.BlacklistTypeWhitelistTxId, EntryValue: "ac30da2b8dc59d2a19d9bd83b64761d74a17e76a4a9573f38e99291731b76cb8", Source: "seed", Reason: "Victim #1 self-rescue 999 METAID to 13Tzd..."},
		{EntryType: models.BlacklistTypeWhitelistUtxo, EntryValue: "ac30da2b8dc59d2a19d9bd83b64761d74a17e76a4a9573f38e99291731b76cb8_1", Source: "seed", Reason: "Victim #1 self-rescue 999 METAID to 13Tzd..."},
		{EntryType: models.BlacklistTypeWhitelistTxId, EntryValue: "815aae66564f3349f2a80e740b596a39988f793259fd3c48554079bfcc7cf6ca", Source: "seed", Reason: "Victim #1 self-rescue 1000 METAID to 1AkEdz..."},
		{EntryType: models.BlacklistTypeWhitelistUtxo, EntryValue: "815aae66564f3349f2a80e740b596a39988f793259fd3c48554079bfcc7cf6ca_1", Source: "seed", Reason: "Victim #1 self-rescue 1000 METAID to 1AkEdz..."},
		{EntryType: models.BlacklistTypeWhitelistTxId, EntryValue: "d9e87f9db4095439449414fd99c20413bfa5a1bb39079ee9e28b0f71c60f6f05", Source: "seed", Reason: "Victim #1 self-rescue to 13Tzd"},
		{EntryType: models.BlacklistTypeWhitelistTxId, EntryValue: "475c6e9feac80d95450a9bc1fd0d1d7f71c6d5d0649d6825639fad9a809471ce", Source: "seed", Reason: "Victim #1 self-rescue to 13Tzd"},
		{EntryType: models.BlacklistTypeWhitelistTxId, EntryValue: "5239b833aef8bcf390d0b1c65bc06b4ce13197ac8772ff3e9240dd7313b60418", Source: "seed", Reason: "Victim #1 self-rescue to 13Tzd"},
		{EntryType: models.BlacklistTypeWhitelistTxId, EntryValue: "6f986959c7a6f9fdc11466f8f65b595e0cc3879f43c685c6a6e7891bb4171349", Source: "seed", Reason: "Victim #1 self-rescue to 13Tzd"},
		{EntryType: models.BlacklistTypeWhitelistTxId, EntryValue: "ac30da2bd35169f43fcbf4e8bb8c8f2cf115d87c87683b5a0e4c283d352bbbcd", Source: "seed", Reason: "Victim #1 self-rescue to 13Tzd"},
		{EntryType: models.BlacklistTypeWhitelistTxId, EntryValue: "815aae66958dad97ae3f835e637ce4064685e10cea78df33c25dcb8792728554", Source: "seed", Reason: "Victim #1 self-rescue to 1AkEdz"},
		{EntryType: models.BlacklistTypeWhitelistTxId, EntryValue: "0b1ca0c3edec6b37255fd4d473df7e707d2d200ce497626e84b37fb8f59dc95a", Source: "seed", Reason: "Victim #1 self-rescue to 1AkEdz"},
		{EntryType: models.BlacklistTypeWhitelistTxId, EntryValue: "b490c5fc5f16534ae84f98dd324562e0873ed56221ff0eb281d8266fd117a187", Source: "seed", Reason: "Victim #1 self-rescue to 1AkEdz"},
		// Protect MRC20 cross-chain bridge custody address & clean reserve outputs while keeping stolen UTXOs blocked
		{EntryType: models.BlacklistTypeWhitelistAddress, EntryValue: "1Fw9tW2p6hossLoXXbsX8vjYrCBJyCFLCX", Source: "seed", Reason: "MRC20 bridge custody address (Mr.P) protected from address-level poisoning"},
		{EntryType: models.BlacklistTypeWhitelistUtxo, EntryValue: "214104a1dcd6ba8ac8ca1ccb4510f97ea72f82856824b976860068b6871fca18_0", Source: "seed", Reason: "MRC20 bridge custody 1,000 METAID reserve change"},
		{EntryType: models.BlacklistTypeWhitelistUtxo, EntryValue: "214104a1dcd6ba8ac8ca1ccb4510f97ea72f82856824b976860068b6871fca18_2", Source: "seed", Reason: "MRC20 bridge custody BTC change"},

		// Central phishing attacker BTC gas & proceeds hubs
		{EntryType: models.BlacklistTypeAddress, EntryValue: "bc1pe2vr873pqlufgeh2vlr4yf04z92ndvkjhnqmt8p7jp9wuevpuzjs8z2evg", Source: "seed", Reason: "Fake Metalet phishing attacker central BTC gas & proceeds hub #1"},
		{EntryType: models.BlacklistTypeAddress, EntryValue: "bc1pky6zpsx8gpv257kr3w43rvl2dre0j7phndxlf5muetzcyane8cyq2cys66", Source: "seed", Reason: "Fake Metalet phishing attacker rotated BTC consolidation hub #2"},
		{EntryType: models.BlacklistTypeTxId, EntryValue: "d80f539534df27a078bc290472dd0590df79fa06502c0afdd5ad42d37c5a1847", Source: "seed", Reason: "Attacker hub #1 initial funding tx"},
		{EntryType: models.BlacklistTypeTxId, EntryValue: "694b87ca038d21c57b36313006d0d739a6baac26862b9359b2266c2c7937f15f", Source: "seed", Reason: "Attacker hub #1 to hub #2 BTC sweep tx"},
		{EntryType: models.BlacklistTypeTxId, EntryValue: "a62d633c31ad9ad0de4e26ffe1f5d71f7b6ac78db7dc3fcc9b92e9a165bab2fb", Source: "seed", Reason: "Mule 158QRkSz... sale proceeds BTC sweep to hub #2"},
		{EntryType: models.BlacklistTypeTxId, EntryValue: "19a6676f92c1b900bc59e2299040c774cb219de9717869b34fae311219819579", Source: "seed", Reason: "Mule 15S5e9Yk... sale proceeds BTC sweep to hub #2"},
		{EntryType: models.BlacklistTypeTxId, EntryValue: "1ef5460150b9e2522b9fa1143714f684735ad9a1e5d63b3a7c8e61a697fae346", Source: "seed", Reason: "Attacker hub #2 funding gas to mule 1Atxucrhg..."},
		{EntryType: models.BlacklistTypeTxId, EntryValue: "e94b41498f3caef8fa07acc81bf17857bb18aa744f898ffb9a66355529f13d50", Source: "seed", Reason: "Thief mule 1GtPDnwu... transferring 8000 stolen METAID to 1Atxucrhg..."},
		{EntryType: models.BlacklistTypeUtxo, EntryValue: "7510793d8c9ed3e088dfdf9ee8681a0ab6f76b0bd6d8bd40dad5442b4ef9bd64_1", Source: "seed", Reason: "Thief mule 198FAh9... depositing 3000 stolen METAID into bridge 1Fw9tW2p..."},
		{EntryType: models.BlacklistTypeUtxo, EntryValue: "214104a1dcd6ba8ac8ca1ccb4510f97ea72f82856824b976860068b6871fca18_1", Source: "seed", Reason: "Bridge-laundered 3,000 METAID at 1AVyhwDZ..."},

		// Compromised victim addresses (mnemonics leaked to phishing attacker)
		{EntryType: models.BlacklistTypeAddress, EntryValue: "1DrLUVDKyaQSrSK16SixNbuHGNnWnZSiCm", Source: "seed", Reason: "Compromised phishing victim #1 wallet (mnemonic leaked)"},
		{EntryType: models.BlacklistTypeAddress, EntryValue: "1HRuj7jEZeUikQATyQTfm9WUUDzB1FWZo9", Source: "seed", Reason: "Compromised phishing victim #2 wallet (mnemonic leaked)"},
		{EntryType: models.BlacklistTypeAddress, EntryValue: "1CuCuXbgjpov1t54yB2YanN4yCYtGTS7Bq", Source: "seed", Reason: "Compromised phishing victim #3 wallet (mnemonic leaked)"},

		// Attacker primary receivers, splitting/listing mules, and rotated Sybil wallets
		{EntryType: models.BlacklistTypeAddress, EntryValue: "1JeiAHnCcqtYiZr1EPYogeKh67fkJCFmUA", Source: "seed", Reason: "Phishing thief primary receiver from 1DrLUV..."},
		{EntryType: models.BlacklistTypeAddress, EntryValue: "1GtPDnwuWkULR36euBCYJTTGQuicVhsre4", Source: "seed", Reason: "Phishing thief mule address (10,000 METAID split)"},
		{EntryType: models.BlacklistTypeAddress, EntryValue: "1PCzk7TqRTnrDbVjZkRqsRfAT6EPVj5N74", Source: "seed", Reason: "Phishing thief mule address (2,000 METAID split)"},
		{EntryType: models.BlacklistTypeAddress, EntryValue: "158QRkSznSUxC8GY6KF8iGJfmnuAwdkkrM", Source: "seed", Reason: "Phishing thief mule address (2,000 METAID split)"},
		{EntryType: models.BlacklistTypeAddress, EntryValue: "15S5e9Yka5ReiqYnvjYWgrw8WhMznYTQPu", Source: "seed", Reason: "Phishing thief mule address (500 METAID split)"},
		{EntryType: models.BlacklistTypeAddress, EntryValue: "1N5qTr8jD4nDPWBD8nLMXwVVK4rCQRCBJ1", Source: "seed", Reason: "Phishing thief mule address (3,000 METAID split)"},
		{EntryType: models.BlacklistTypeAddress, EntryValue: "17vGB948xbkSvsEerJPXQaSsQAA3xJqgqC", Source: "seed", Reason: "Phishing thief pre-funded mule address"},
		{EntryType: models.BlacklistTypeAddress, EntryValue: "1627uHcbfr7b6UfKj2eJBmftjtzrq74cd4", Source: "seed", Reason: "Phishing thief pre-funded mule address"},
		{EntryType: models.BlacklistTypeAddress, EntryValue: "1MTEVpcigyzKU9AhKQvfqNbSUHiZggALZ7", Source: "seed", Reason: "Phishing thief mule address (2,000 METAID + Victim #2 BTC sweep)"},
		{EntryType: models.BlacklistTypeAddress, EntryValue: "1Gx23Z2EByLmHVSdUXhgN2fDNxcHckK6vG", Source: "seed", Reason: "Phishing thief mule address (5,000 METAID + 100,000 SHABI from 1HRuj7jE...)"},
		{EntryType: models.BlacklistTypeAddress, EntryValue: "198FAh9JKiKDg27xjbvzpVrDRASbrNoAbS", Source: "seed", Reason: "Phishing thief mule address (3,000 METAID from 1HRuj7jE...)"},
		{EntryType: models.BlacklistTypeAddress, EntryValue: "1AtxucrhgHetPGwNiZudJeTUZmns8G7pQS", Source: "seed", Reason: "Phishing attacker rotated mule address funded by hub #2"},
		{EntryType: models.BlacklistTypeAddress, EntryValue: "1DrcaSDzmmuqkNTFj2UuYcfjmJ1BXKSTAo", Source: "seed", Reason: "Phishing cluster rotated mule wallet (blk 977060 from 1Gx23Z2E)"},
		{EntryType: models.BlacklistTypeAddress, EntryValue: "1AVyhwDZGe3FVUKkxzXG4Nm3TWmc541Wza", Source: "seed", Reason: "Phishing cluster bridge-laundered exit wallet (blk 977065 3,000 METAID)"},
		{EntryType: models.BlacklistTypeAddress, EntryValue: "1K1xQjTAw8BmBt8n9KS6FLKoe2i31V8unb", Source: "seed", Reason: "Phishing cluster rotated mule wallet (blk 977060 from 158QRkSz)"},
		{EntryType: models.BlacklistTypeAddress, EntryValue: "1CjSkrT5zEowwGtckmyqsqLE68GJZ8Ziu1", Source: "seed", Reason: "Phishing cluster rotated mule wallet (blk 977060 from 1PCzk7Tq)"},
		{EntryType: models.BlacklistTypeAddress, EntryValue: "18CaqsRn3DvPMPssfF7g1ERL1S7Rp2DNsG", Source: "seed", Reason: "Phishing cluster rotated mule wallet (blk 977060 from 17vGB948)"},

		// Root theft, split & rotated Sybil transactions
		{EntryType: models.BlacklistTypeTxId, EntryValue: "85837206dd1f7cfac725782839249ef181856c50164a7cc1f80e79e881a570c0", Source: "seed", Reason: "Stolen 1,000 METAID from 1DrLUV..."},
		{EntryType: models.BlacklistTypeTxId, EntryValue: "533173fbcbdc1fc9800a301a992faf54d27a3950928be2c02e4bf719add6548f", Source: "seed", Reason: "Stolen 16,160 METAID from 1DrLUV..."},
		{EntryType: models.BlacklistTypeTxId, EntryValue: "5b2b3400077dd0967441578a85a279ad4191cac7c04bb41cfa1c62173a456afe", Source: "seed", Reason: "Stolen 7,000.51556315 METAID from 1DrLUV..."},
		{EntryType: models.BlacklistTypeTxId, EntryValue: "b6994a0b4c520ed9ecea489cd462d92387108ab042bf2aeb6424c0943e033718", Source: "seed", Reason: "Stolen 48,000 MC from 1DrLUV..."},
		{EntryType: models.BlacklistTypeUtxo, EntryValue: "b6994a0b4c520ed9ecea489cd462d92387108ab042bf2aeb6424c0943e033718_1", Source: "seed", Reason: "Stolen 48,000 MC UTXO in 1JeiAHn..."},
		{EntryType: models.BlacklistTypeTxId, EntryValue: "b4c372a4e03a4154e1ef07dc8fcc1948c2d29e60b450573f7dcb3410b06c837a", Source: "seed", Reason: "Stolen 2,356.08731668 METASO from 1DrLUV..."},
		{EntryType: models.BlacklistTypeUtxo, EntryValue: "b4c372a4e03a4154e1ef07dc8fcc1948c2d29e60b450573f7dcb3410b06c837a_1", Source: "seed", Reason: "Stolen 2,356.08731668 METASO UTXO in 1JeiAHn..."},
		{EntryType: models.BlacklistTypeTxId, EntryValue: "74b07fbf41e22303a12d227c8a5d06cbb7af95ab4cc191a498df34245137d3f0", Source: "seed", Reason: "Stolen 100,000 SHABI from 1DrLUV..."},
		{EntryType: models.BlacklistTypeUtxo, EntryValue: "74b07fbf41e22303a12d227c8a5d06cbb7af95ab4cc191a498df34245137d3f0_1", Source: "seed", Reason: "Stolen 100,000 SHABI UTXO in 1JeiAHn..."},
		{EntryType: models.BlacklistTypeTxId, EntryValue: "016aeee719503a92b726a8c44283c9a09ef2393d0a9ad0edb54bfe2572d62dd7", Source: "seed", Reason: "Stolen 201,365.20400224 METALD from 1DrLUV..."},
		{EntryType: models.BlacklistTypeUtxo, EntryValue: "016aeee719503a92b726a8c44283c9a09ef2393d0a9ad0edb54bfe2572d62dd7_1", Source: "seed", Reason: "Stolen 201,365.20400224 METALD UTXO in 1JeiAHn..."},
		{EntryType: models.BlacklistTypeTxId, EntryValue: "9e0be7941ef66c31c55a3497b8489c3b3fa5e2d5a421f0c41520a24b3789c61e", Source: "seed", Reason: "Stolen 5,000 METAID + 100,000 SHABI from 1HRuj7jE..."},
		{EntryType: models.BlacklistTypeTxId, EntryValue: "3aa4e91ecc516b518cf24cc4b196380a02cd2050d2a8b1a09e69cb5c45ac1a67", Source: "seed", Reason: "Stolen 3,000 METAID from 1HRuj7jE..."},
		{EntryType: models.BlacklistTypeTxId, EntryValue: "87ad901483044f596d4916b71370253a60e2a7bc2e0e3d058607e7b31a5f42ee", Source: "seed", Reason: "Stolen 2,000 METAID from 1CuCuXbg..."},
		{EntryType: models.BlacklistTypeTxId, EntryValue: "dd51318238798de987a77b433c3234769267079388496c598c4d94f3977c2604", Source: "seed", Reason: "Thief split 10,000 METAID to 1GtPDnwu..."},
		{EntryType: models.BlacklistTypeTxId, EntryValue: "1dde43a6be924b22f5f6737ba9c0bfbc20bd595cc1b2651be34824f5eb29df88", Source: "seed", Reason: "Thief split 3,000 METAID to 1N5qTr8j..."},
		{EntryType: models.BlacklistTypeTxId, EntryValue: "4fc0f532c20a3c6b532fa2728b4c9a3f885a16a9f06ba9209ca323438df07aa1", Source: "seed", Reason: "Thief split 2,000 METAID to 1PCzk7Tq..."},
		{EntryType: models.BlacklistTypeTxId, EntryValue: "d6ad3ac090b9028bf0860d1af9ab43a5a85f701eb56ab5b92049e9915d975c7f", Source: "seed", Reason: "Thief split 2,000 METAID to 158QRkSz..."},
		{EntryType: models.BlacklistTypeTxId, EntryValue: "3d603a6b74e94ecf1eae7b9718d2ff76e8216b36bee1dd4ae2152c013d2a97b9", Source: "seed", Reason: "Thief split 500 METAID to 15S5e9Yk..."},
		{EntryType: models.BlacklistTypeTxId, EntryValue: "76c1c529cc451d6c5d2aa56531a337481dc1858692e956049450f7a46a769813", Source: "seed", Reason: "Thief split 600 METAID to 17vGB948..."},
		{EntryType: models.BlacklistTypeTxId, EntryValue: "7c6ef3f1c02cfe85fdbc727727493a1337e8d677c95ad9cf9a5de862b0c71f74", Source: "seed", Reason: "1Gx23Z2E -> 1DrcaSDz commit tx"},
		{EntryType: models.BlacklistTypeTxId, EntryValue: "9cb38847f3e8e34c1f3ebb3cbe6d40bb0d100a1b6da8929673599be579c10b4b", Source: "seed", Reason: "1Gx23Z2E -> 1DrcaSDz reveal tx"},
		{EntryType: models.BlacklistTypeTxId, EntryValue: "7ecb8cfcb630353789652924dab6b4af31ca058f6f440f55a05cc05bcb211857", Source: "seed", Reason: "1Gx23Z2E -> 1DrcaSDz gas tx"},
		{EntryType: models.BlacklistTypeTxId, EntryValue: "7df8c143f0db3c748b261598a4a9cbe9f9702340c5527b4c1c3f1db677a4c280", Source: "seed", Reason: "158QRkSz -> 1K1xQjTA MRC20 transfer tx"},
		{EntryType: models.BlacklistTypeTxId, EntryValue: "64bfdf1fd1ddccc631287660518965bede02b4b067f99998c844a5ee617528b7", Source: "seed", Reason: "158QRkSz -> 1K1xQjTA BTC gas tx"},
		{EntryType: models.BlacklistTypeTxId, EntryValue: "1220babe1e882640e2e82bce5baf0e7d9f37c96621528341f83e1563935de0e9", Source: "seed", Reason: "1PCzk7Tq -> 1CjSkrT5 MRC20 transfer tx"},
		{EntryType: models.BlacklistTypeTxId, EntryValue: "fa8b080c76123a9012b0f719b6abc6ab814ce221686b44a4253d80db1cbcdc3e", Source: "seed", Reason: "1PCzk7Tq -> 1CjSkrT5 BTC sweep tx"},
		{EntryType: models.BlacklistTypeTxId, EntryValue: "2487e23fddfda07608049f6ef07705134d40b994ff4b7dda4acde6f6036b5f4b", Source: "seed", Reason: "17vGB948 -> 18CaqsRn MRC20 transfer tx"},
		{EntryType: models.BlacklistTypeTxId, EntryValue: "c36f5d6a814d53b74fa7a0c01174598d9a764905e3ab0ca9ec4154db71e17b63", Source: "seed", Reason: "17vGB948 -> 18CaqsRn BTC sweep tx"},
	}
	for _, e := range entries {
		e.EntryValue = normalizeByType(e.EntryType, e.EntryValue)
		e.State = models.STATE_EXIST
	}
	return entries
}

func normalizeByType(entryType, value string) string {
	switch entryType {
	case models.BlacklistTypeAddress, models.BlacklistTypeWhitelistAddress:
		return NormalizeAddress(value)
	case models.BlacklistTypeTxId, models.BlacklistTypeWhitelistTxId:
		return NormalizeTxId(value)
	case models.BlacklistTypeUtxo, models.BlacklistTypeWhitelistUtxo:
		return NormalizeUtxoId(value)
	default:
		return strings.TrimSpace(value)
	}
}

func (e *RiskEngine) LoadSeedEntriesInMemory() {
	e.ApplyEntries(DefaultSeedEntries())
}

func (e *RiskEngine) ApplyEntries(list []*models.MarketBlacklistModel) {
	newBlackAddrs := make(map[string]string)
	newBlackTxIds := make(map[string]string)
	newBlackUtxos := make(map[string]string)
	newWhiteAddrs := make(map[string]bool)
	newWhiteTxIds := make(map[string]bool)
	newWhiteUtxos := make(map[string]bool)

	for _, item := range list {
		if item == nil || item.State != models.STATE_EXIST {
			continue
		}
		val := normalizeByType(item.EntryType, item.EntryValue)
		if val == "" {
			continue
		}
		switch item.EntryType {
		case models.BlacklistTypeAddress:
			newBlackAddrs[val] = item.Reason
		case models.BlacklistTypeTxId:
			newBlackTxIds[val] = item.Reason
		case models.BlacklistTypeUtxo:
			newBlackUtxos[val] = item.Reason
		case models.BlacklistTypeWhitelistAddress:
			newWhiteAddrs[val] = true
		case models.BlacklistTypeWhitelistTxId:
			newWhiteTxIds[val] = true
		case models.BlacklistTypeWhitelistUtxo:
			newWhiteUtxos[val] = true
		}
	}

	e.mu.Lock()
	changed := !mapsEqualString(e.blackAddrs, newBlackAddrs) ||
		!mapsEqualString(e.blackTxIds, newBlackTxIds) ||
		!mapsEqualString(e.blackUtxos, newBlackUtxos) ||
		!mapsEqualBool(e.whiteAddrs, newWhiteAddrs) ||
		!mapsEqualBool(e.whiteTxIds, newWhiteTxIds) ||
		!mapsEqualBool(e.whiteUtxos, newWhiteUtxos)
	e.blackAddrs = newBlackAddrs
	e.blackTxIds = newBlackTxIds
	e.blackUtxos = newBlackUtxos
	e.whiteAddrs = newWhiteAddrs
	e.whiteTxIds = newWhiteTxIds
	e.whiteUtxos = newWhiteUtxos
	e.mu.Unlock()

	if changed {
		e.clearCleanCache()
	}
}

func mapsEqualString(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

func mapsEqualBool(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

func (e *RiskEngine) clearCleanCache() {
	e.cleanMu.Lock()
	e.cleanOutpoints = make(map[string]bool)
	e.cleanMu.Unlock()
}

// isPlatformSystemAddress identifies Market dummy-UTXO pool and fee addresses.
// Because the 600-sat dummy UTXO is recycled across every unrelated Market trade,
// platform system addresses must both be immune from blacklisting and cut ancestor traversal.
func (e *RiskEngine) isPlatformSystemAddress(addr string) bool {
	norm := NormalizeAddress(addr)
	if norm == "" {
		return false
	}
	if norm == mainnetPlatformDummyAddr ||
		norm == mainnetPlatformFeeAddr ||
		norm == NormalizeAddress(conf.PlatformAddressDummyAsk) ||
		norm == NormalizeAddress(conf.PlatformDogeAddressDummyAsk) ||
		norm == NormalizeAddress(conf.PlatformAddressReceiveFee) ||
		norm == NormalizeAddress(conf.PlatformDogeAddressReceiveFee) ||
		(conf.PlatformServiceFeeConfigData != nil &&
			(norm == NormalizeAddress(conf.PlatformServiceFeeConfigData.ServiceAddress) ||
				norm == NormalizeAddress(conf.PlatformServiceFeeConfigData.DogeServiceAddress))) {
		return true
	}
	return false
}

func isSelfRescueSafeAddress(addr string) bool {
	norm := NormalizeAddress(addr)
	return norm == victim1SelfRescueAddr1 || norm == victim1SelfRescueAddr2
}

func (e *RiskEngine) isProtectedAddress(addr string) bool {
	norm := NormalizeAddress(addr)
	if norm == "" {
		return false
	}
	if e.isPlatformSystemAddress(norm) || isSelfRescueSafeAddress(norm) {
		return true
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.whiteAddrs[norm]
}

func (e *RiskEngine) IsAddressBlacklisted(addr string) (bool, string) {
	norm := NormalizeAddress(addr)
	if norm == "" || e.isProtectedAddress(norm) {
		return false, ""
	}
	e.mu.RLock()
	reason, ok := e.blackAddrs[norm]
	e.mu.RUnlock()
	return ok, reason
}

func (e *RiskEngine) isWhitelistedOutpoint(txId, utxoId string) bool {
	normTx := NormalizeTxId(txId)
	normUtxo := NormalizeUtxoId(utxoId)
	e.mu.RLock()
	defer e.mu.RUnlock()
	return (normTx != "" && e.whiteTxIds[normTx]) || (normUtxo != "" && e.whiteUtxos[normUtxo])
}

func (e *RiskEngine) isDirectBlacklistedOutpoint(txId, utxoId string) (bool, string, string) {
	normTx := NormalizeTxId(txId)
	normUtxo := NormalizeUtxoId(utxoId)
	if e.isWhitelistedOutpoint(normTx, normUtxo) {
		return false, "", ""
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	if normUtxo != "" {
		if r, ok := e.blackUtxos[normUtxo]; ok {
			return true, normUtxo, r
		}
	}
	if normTx != "" {
		if r, ok := e.blackTxIds[normTx]; ok {
			return true, normTx, r
		}
	}
	return false, "", ""
}

func (e *RiskEngine) IsOrderBlockedFast(sellerAddress, utxoId string) bool {
	if blocked, _ := e.IsAddressBlacklisted(sellerAddress); blocked {
		return true
	}
	txId, _, ok := ParseUtxoId(utxoId)
	if !ok {
		return false
	}
	blocked, _, _ := e.isDirectBlacklistedOutpoint(txId, utxoId)
	return blocked
}

func (e *RiskEngine) recordAutoTaint(entryType, value, parentValue, reason string) {
	normVal := normalizeByType(entryType, value)
	if normVal == "" {
		return
	}
	if entryType == models.BlacklistTypeAddress && e.isProtectedAddress(normVal) {
		return
	}
	if (entryType == models.BlacklistTypeTxId || entryType == models.BlacklistTypeUtxo) && e.isWhitelistedOutpoint(normVal, normVal) {
		return
	}

	alreadyExists := false
	e.mu.Lock()
	switch entryType {
	case models.BlacklistTypeAddress:
		if _, ok := e.blackAddrs[normVal]; ok {
			alreadyExists = true
		} else {
			e.blackAddrs[normVal] = reason
		}
	case models.BlacklistTypeTxId:
		if _, ok := e.blackTxIds[normVal]; ok {
			alreadyExists = true
		} else {
			e.blackTxIds[normVal] = reason
		}
	case models.BlacklistTypeUtxo:
		if _, ok := e.blackUtxos[normVal]; ok {
			alreadyExists = true
		} else {
			e.blackUtxos[normVal] = reason
		}
	}
	e.mu.Unlock()

	if !alreadyExists {
		e.clearCleanCache()
		if e.persister != nil {
			now := tool.MakeTimestamp()
			_ = e.persister(&models.MarketBlacklistModel{
				EntryType:   entryType,
				EntryValue:  normVal,
				Source:      "auto_taint",
				ParentValue: parentValue,
				Reason:      reason,
				CreateTime:  now,
				UpdateTime:  now,
				State:       models.STATE_EXIST,
			})
		}
	}
}

// isExemptCompletedTradeOutput checks whether (txId, vout, outputAddr) is a buyer/platform
// output of a legitimate completed market order (orderState = 3).
// Buyer outputs (e.g. vout=0 MRC20, vout=4 buyer BTC change) and platform outputs are exempt,
// whereas seller BTC proceeds (outputAddr == order.SellerAddress) remain tainted if the seller was tainted.
func (e *RiskEngine) lookupTradeCached(txId string) (*CompletedMarketTrade, error) {
	if e.tradeLookup == nil || txId == "" {
		return nil, nil
	}
	normTx := NormalizeTxId(txId)
	now := time.Now()
	e.tradeCacheMu.RLock()
	entry, ok := e.tradeCache[normTx]
	e.tradeCacheMu.RUnlock()
	if ok && (entry.expiresAt.IsZero() || now.Before(entry.expiresAt)) {
		return entry.trade, nil
	}

	trade, err := e.tradeLookup(normTx)
	if err != nil {
		return nil, err
	}

	e.tradeCacheMu.Lock()
	if len(e.tradeCache) >= maxCachedTrades {
		e.tradeCache = make(map[string]cachedTradeEntry)
	}
	if trade != nil {
		e.tradeCache[normTx] = cachedTradeEntry{trade: trade}
	} else {
		e.tradeCache[normTx] = cachedTradeEntry{trade: nil, expiresAt: now.Add(negativeTradeCacheTTL)}
	}
	e.tradeCacheMu.Unlock()
	return trade, nil
}

func (e *RiskEngine) isExemptCompletedTradeOutput(txId string, vout int64, outputAddr string) bool {
	if e.tradeLookup == nil || txId == "" {
		return false
	}
	trade, err := e.lookupTradeCached(txId)
	if err != nil || trade == nil {
		return false
	}
	normSeller := NormalizeAddress(trade.SellerAddress)
	normBuyer := NormalizeAddress(trade.BuyerAddress)
	normOut := NormalizeAddress(outputAddr)

	// If the buyer themselves is a blacklisted thief address (wash-trading with own mule), never exempt!
	if normBuyer != "" {
		if buyerBlocked, _ := e.IsAddressBlacklisted(normBuyer); buyerBlocked {
			return false
		}
	}

	if normOut == "" && vout >= 0 {
		if tx := e.getChainTx("btc", txId); tx != nil && int(vout) < len(tx.Outputs) {
			normOut = NormalizeAddress(tx.Outputs[vout].Address)
		}
	}
	if normOut != "" {
		return normOut != normSeller
	}
	// If outputAddr is still unresolved, vout 0 in metaid.market trades is the buyer's asset output.
	return vout == 0
}

func (e *RiskEngine) getChainTx(chain, txId string) *ChainTx {
	normTx := NormalizeTxId(txId)
	if normTx == "" || e.txFetcher == nil {
		return nil
	}
	e.txCacheMu.RLock()
	cached, ok := e.txCache[normTx]
	e.txCacheMu.RUnlock()
	if ok {
		return cached
	}
	tx, err := e.txFetcher(chain, normTx)
	if err != nil || tx == nil {
		return nil
	}
	if tx.Confirmed {
		e.txCacheMu.Lock()
		if len(e.txCache) >= maxCachedChainTxs {
			e.txCache = make(map[string]*ChainTx)
		}
		e.txCache[normTx] = tx
		e.txCacheMu.Unlock()
	}
	return tx
}

// CheckUtxoAndAddress evaluates an address + UTXO + optional PSBT/indexer hints,
// walking the BTC transaction ancestry up to maxTaintWalkDepth hops and automatically
// propagating taint to newly discovered thief transactions/UTXOs/mule addresses.
func (e *RiskEngine) CheckUtxoAndAddress(chain, address, utxoId string, outValue int64, hints []TaintHint) (bool, string) {
	normAddr := NormalizeAddress(address)
	if blocked, reason := e.IsAddressBlacklisted(normAddr); blocked {
		if txId, _, ok := ParseUtxoId(utxoId); ok && !e.isWhitelistedOutpoint(txId, utxoId) && !e.isExemptCompletedTradeOutput(txId, 0, normAddr) {
			normUtxo := NormalizeUtxoId(utxoId)
			e.recordAutoTaint(models.BlacklistTypeUtxo, normUtxo, normAddr, "listed/transferred by blacklisted address "+normAddr)
		}
		return true, fmt.Sprintf("blacklisted address %s (%s)", normAddr, reason)
	}

	normUtxo := ""
	rootTxId := ""
	var rootVout int64 = -1
	if utxoId != "" {
		normUtxo = NormalizeUtxoId(utxoId)
		if txId, vout, ok := ParseUtxoId(normUtxo); ok {
			rootTxId = txId
			rootVout = vout
		}
	}

	if rootTxId != "" {
		if e.isWhitelistedOutpoint(rootTxId, normUtxo) {
			return false, ""
		}
		if e.isExemptCompletedTradeOutput(rootTxId, rootVout, normAddr) {
			return false, ""
		}
		if blocked, matchKey, reason := e.isDirectBlacklistedOutpoint(rootTxId, normUtxo); blocked {
			e.propagateHitToCurrent(normAddr, rootTxId, normUtxo, outValue, matchKey, 0)
			return true, fmt.Sprintf("blacklisted outpoint %s (%s)", matchKey, reason)
		}
	}

	// Check immediate hints (e.g., Mrc20Info.FromAddress, PSBT NonWitnessUtxo inputs, commitTx inputs)
	for _, h := range hints {
		if h.FromAddress != "" {
			if blocked, reason := e.IsAddressBlacklisted(h.FromAddress); blocked {
				if normAddr != "" && (e.isPlatformSystemAddress(normAddr) || isSelfRescueSafeAddress(normAddr)) {
					continue
				}
				if rootTxId != "" {
					if e.isExemptCompletedTradeOutput(rootTxId, rootVout, normAddr) {
						return false, ""
					}
					// If rootTxId was confirmed strictly before the phishing incident, FromAddress
					// sent the asset before any mnemonic leak occurred; do not taint pre-incident holders.
					if rTx := e.getChainTx("btc", rootTxId); rTx != nil && rTx.Confirmed && rTx.BlockHeight > 0 && rTx.BlockHeight < minTaintWalkBlockHeight {
						continue
					}
				}
				e.propagateHitToCurrent(normAddr, rootTxId, normUtxo, outValue, NormalizeAddress(h.FromAddress), 1)
				return true, fmt.Sprintf("tainted by sender %s (%s)", h.FromAddress, reason)
			}
		}
		if h.PrevTxId != "" {
			prevTx := NormalizeTxId(h.PrevTxId)
			prevUtxo := fmt.Sprintf("%s_%d", prevTx, h.PrevVout)
			if e.isWhitelistedOutpoint(prevTx, prevUtxo) || e.isExemptCompletedTradeOutput(prevTx, int64(h.PrevVout), "") {
				continue
			}
			if blocked, matchKey, reason := e.isDirectBlacklistedOutpoint(prevTx, prevUtxo); blocked {
				e.propagateHitToCurrent(normAddr, rootTxId, normUtxo, outValue, matchKey, 1)
				return true, fmt.Sprintf("tainted by parent %s (%s)", matchKey, reason)
			}
		}
	}

	if strings.ToLower(chain) != "" && strings.ToLower(chain) != "btc" {
		return false, ""
	}
	if rootTxId == "" && len(hints) == 0 {
		return false, ""
	}

	if normUtxo != "" {
		e.cleanMu.RLock()
		isClean := e.cleanOutpoints[normUtxo]
		e.cleanMu.RUnlock()
		if isClean {
			return false, ""
		}
	}

	type queueItem struct {
		txId  string
		vout  int64
		depth int
	}

	queue := make([]queueItem, 0, 16)
	visitedOutpoints := make(map[string]bool)
	visitedTxs := make(map[string]bool)

	if rootTxId != "" {
		key := fmt.Sprintf("%s_%d", rootTxId, rootVout)
		visitedOutpoints[key] = true
		queue = append(queue, queueItem{txId: rootTxId, vout: rootVout, depth: 0})
	}
	for _, h := range hints {
		if h.PrevTxId != "" {
			pTx := NormalizeTxId(h.PrevTxId)
			key := fmt.Sprintf("%s_%d", pTx, h.PrevVout)
			if !visitedOutpoints[key] {
				visitedOutpoints[key] = true
				queue = append(queue, queueItem{txId: pTx, vout: int64(h.PrevVout), depth: 1})
			}
		}
	}

	rootConfirmed := false

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		curUtxo := fmt.Sprintf("%s_%d", cur.txId, cur.vout)
		if e.isWhitelistedOutpoint(cur.txId, curUtxo) {
			continue
		}

		tx := e.getChainTx("btc", cur.txId)
		outAddr := ""
		if tx != nil && cur.vout >= 0 && int(cur.vout) < len(tx.Outputs) {
			outAddr = tx.Outputs[cur.vout].Address
		}
		if cur.depth == 0 && tx != nil && tx.Confirmed {
			rootConfirmed = true
		}

		if outAddr != "" && (e.isPlatformSystemAddress(outAddr) || isSelfRescueSafeAddress(outAddr)) {
			if blocked, _, _ := e.isDirectBlacklistedOutpoint(cur.txId, curUtxo); !blocked {
				continue
			}
		}

		if e.isExemptCompletedTradeOutput(cur.txId, cur.vout, outAddr) {
			continue
		}

		if blocked, matchKey, reason := e.isDirectBlacklistedOutpoint(cur.txId, curUtxo); blocked {
			e.propagateHitToCurrent(normAddr, rootTxId, normUtxo, outValue, matchKey, cur.depth)
			return true, fmt.Sprintf("tainted by ancestor %s at depth %d (%s)", matchKey, cur.depth, reason)
		}
		if outAddr != "" {
			if blocked, reason := e.IsAddressBlacklisted(outAddr); blocked {
				e.propagateHitToCurrent(normAddr, rootTxId, normUtxo, outValue, NormalizeAddress(outAddr), cur.depth)
				return true, fmt.Sprintf("tainted by ancestor address %s at depth %d (%s)", outAddr, cur.depth, reason)
			}
		}

		if tx == nil {
			continue
		}
		visitedTxs[cur.txId] = true

		// Pre-incident confirmed block cutoff MUST run BEFORE inspecting tx.Inputs:
		// any transaction confirmed strictly before minTaintWalkBlockHeight (977025) happened
		// prior to the phishing mnemonic leak, so historical counterparties in its inputs are clean.
		if tx.Confirmed && tx.BlockHeight > 0 && tx.BlockHeight < minTaintWalkBlockHeight {
			continue
		}

		// Inspect all inputs of post-incident cur.txId before deciding whether to expand further
		exemptInputs := make([]bool, len(tx.Inputs))
		distinctInputAddrs := make(map[string]struct{}, len(tx.Inputs))
		for i, in := range tx.Inputs {
			inTx := NormalizeTxId(in.TxId)
			inUtxo := fmt.Sprintf("%s_%d", inTx, in.Vout)
			if e.isWhitelistedOutpoint(inTx, inUtxo) {
				exemptInputs[i] = true
				continue
			}
			if in.Address != "" && (e.isPlatformSystemAddress(in.Address) || isSelfRescueSafeAddress(in.Address)) {
				if blocked, _, _ := e.isDirectBlacklistedOutpoint(inTx, inUtxo); !blocked {
					exemptInputs[i] = true
					continue
				}
			}
			if e.isExemptCompletedTradeOutput(inTx, int64(in.Vout), in.Address) {
				exemptInputs[i] = true
				continue
			}
			if blocked, matchKey, reason := e.isDirectBlacklistedOutpoint(inTx, inUtxo); blocked {
				e.propagateHitToCurrent(normAddr, rootTxId, normUtxo, outValue, matchKey, cur.depth+1)
				return true, fmt.Sprintf("tainted by ancestor input %s at depth %d (%s)", matchKey, cur.depth+1, reason)
			}
			if in.Address != "" {
				normInAddr := NormalizeAddress(in.Address)
				if blocked, reason := e.IsAddressBlacklisted(normInAddr); blocked {
					e.propagateHitToCurrent(normAddr, rootTxId, normUtxo, outValue, normInAddr, cur.depth+1)
					return true, fmt.Sprintf("tainted by ancestor input address %s at depth %d (%s)", in.Address, cur.depth+1, reason)
				}
				distinctInputAddrs[normInAddr] = struct{}{}
			}
		}

		// Do not expand across large exchange batch withdrawals or high-fan-in multi-party exchange deposit sweeps
		if len(tx.Outputs) > maxBatchExchangeOutputs || len(distinctInputAddrs) >= maxMultiPartyInputAddrs {
			continue
		}
		if cur.depth >= maxTaintWalkDepth || len(visitedTxs) >= maxTaintVisitedTxs {
			continue
		}

		for i, in := range tx.Inputs {
			if exemptInputs[i] || in.TxId == "" {
				continue
			}
			inTx := NormalizeTxId(in.TxId)
			inKey := fmt.Sprintf("%s_%d", inTx, in.Vout)
			if visitedOutpoints[inKey] {
				continue
			}
			visitedOutpoints[inKey] = true
			queue = append(queue, queueItem{
				txId:  inTx,
				vout:  int64(in.Vout),
				depth: cur.depth + 1,
			})
		}
	}

	if rootConfirmed && normUtxo != "" {
		e.cleanMu.Lock()
		if len(e.cleanOutpoints) >= maxCachedCleanOutpoints {
			e.cleanOutpoints = make(map[string]bool)
		}
		e.cleanOutpoints[normUtxo] = true
		e.cleanMu.Unlock()
	}

	return false, ""
}

// propagateHitToCurrent persists outpoint-exact UTXO taint (`BlacklistTypeUtxo`) only.
// It intentionally NEVER auto-blacklists `BlacklistTypeAddress` or whole multi-output `BlacklistTypeTxId`,
// preventing dusting griefing or multi-party/exchange transactions from freezing innocent users' clean assets.
func (e *RiskEngine) propagateHitToCurrent(normAddr, rootTxId, normUtxo string, outValue int64, parentMatch string, depth int) {
	reason := fmt.Sprintf("auto-tainted from %s (depth=%d)", parentMatch, depth)
	if normUtxo != "" {
		e.recordAutoTaint(models.BlacklistTypeUtxo, normUtxo, parentMatch, reason)
	}
}

// --- Production Data Providers & Lifecycle ---

type mempoolTxResp struct {
	TxId   string `json:"txid"`
	Status struct {
		Confirmed   bool  `json:"confirmed"`
		BlockHeight int64 `json:"block_height"`
	} `json:"status"`
	Vin []struct {
		TxId    string `json:"txid"`
		Vout    uint32 `json:"vout"`
		Prevout *struct {
			Address string `json:"scriptpubkey_address"`
			Value   int64  `json:"value"`
		} `json:"prevout"`
	} `json:"vin"`
	Vout []struct {
		Address string `json:"scriptpubkey_address"`
		Value   int64  `json:"value"`
	} `json:"vout"`
}

func defaultFetchChainTx(chain, txId string) (*ChainTx, error) {
	if strings.ToLower(chain) != "" && strings.ToLower(chain) != "btc" {
		return nil, errors.New("unsupported chain for taint walk")
	}
	domain := strings.TrimRight(conf.MempoolSpace, "/")
	if domain == "" {
		domain = "https://mempool.space"
	}
	reqURL := fmt.Sprintf("%s/api/tx/%s", domain, txId)
	resp, err := mempoolHTTPClient.Get(reqURL)
	if err == nil && resp != nil {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		_ = resp.Body.Close()
		if readErr == nil && resp.StatusCode == http.StatusOK && len(body) > 0 {
			var parsed mempoolTxResp
			if jsonErr := json.Unmarshal(body, &parsed); jsonErr == nil && parsed.TxId != "" {
				res := &ChainTx{
					TxId:        NormalizeTxId(parsed.TxId),
					Confirmed:   parsed.Status.Confirmed,
					BlockHeight: parsed.Status.BlockHeight,
					Inputs:      make([]ChainTxInput, 0, len(parsed.Vin)),
					Outputs:     make([]ChainTxOutput, 0, len(parsed.Vout)),
				}
				for _, v := range parsed.Vin {
					in := ChainTxInput{
						TxId: NormalizeTxId(v.TxId),
						Vout: v.Vout,
					}
					if v.Prevout != nil {
						in.Address = NormalizeAddress(v.Prevout.Address)
						in.Value = v.Prevout.Value
					}
					res.Inputs = append(res.Inputs, in)
				}
				for _, o := range parsed.Vout {
					res.Outputs = append(res.Outputs, ChainTxOutput{
						Address: NormalizeAddress(o.Address),
						Value:   o.Value,
					})
				}
				return res, nil
			}
		}
	}

	// Fallback to local btc-base gRPC raw tx + CheckUtxoInfo if mempool.space is unavailable
	rawHex, hexErr := common.FetchTxHex("btc", txId)
	if hexErr != nil || rawHex == "" {
		return nil, errors.New("failed to fetch btc tx from mempool and local rpc")
	}
	rawBytes, decErr := hex.DecodeString(strings.TrimSpace(rawHex))
	if decErr != nil {
		return nil, decErr
	}
	msgTx := wire.NewMsgTx(2)
	if desErr := msgTx.Deserialize(bytes.NewReader(rawBytes)); desErr != nil {
		return nil, desErr
	}
	netStr := conf.Net
	if netStr == "" {
		netStr = "mainnet"
	}
	res := &ChainTx{
		TxId:    NormalizeTxId(msgTx.TxHash().String()),
		Inputs:  make([]ChainTxInput, 0, len(msgTx.TxIn)),
		Outputs: make([]ChainTxOutput, 0, len(msgTx.TxOut)),
	}
	for _, in := range msgTx.TxIn {
		prevTx := NormalizeTxId(in.PreviousOutPoint.Hash.String())
		prevVout := in.PreviousOutPoint.Index
		inItem := ChainTxInput{
			TxId: prevTx,
			Vout: prevVout,
		}
		if uInfo := common.GetUtxoInfo(netStr, "btc", prevTx, int64(prevVout)); uInfo != nil && uInfo.IsExist {
			inItem.Address = NormalizeAddress(uInfo.Address)
			inItem.Value = int64(uInfo.Value)
		}
		res.Inputs = append(res.Inputs, inItem)
	}
	for _, out := range msgTx.TxOut {
		addr, _ := common.PkScriptToAddress(netStr, "btc", hex.EncodeToString(out.PkScript))
		res.Outputs = append(res.Outputs, ChainTxOutput{
			Address: NormalizeAddress(addr),
			Value:   out.Value,
		})
	}
	if len(res.Outputs) > 0 {
		if selfInfo := common.GetUtxoInfo(netStr, "btc", res.TxId, 0); selfInfo != nil && selfInfo.IsExist && selfInfo.Height > 0 {
			res.Confirmed = true
			res.BlockHeight = selfInfo.Height
		}
	}
	return res, nil
}

func defaultLookupCompletedTrade(txId string) (*CompletedMarketTrade, error) {
	if major.GetSqlDB() == nil || txId == "" {
		return nil, nil
	}
	mrcOrder, err := models.MarketMrc20OrderModelDao().GetOne(&models.MarketMrc20OrderModel{
		TxId:       txId,
		OrderState: models.OrderStateFinish,
	})
	if err == nil && mrcOrder != nil && mrcOrder.OrderId != "" {
		return &CompletedMarketTrade{
			TxId:          txId,
			SellerAddress: mrcOrder.SellerAddress,
			BuyerAddress:  mrcOrder.BuyerAddress,
		}, nil
	}
	pinOrder, err := models.MarketOrderModelDao().GetOne(&models.MarketOrderModel{
		TxId:       txId,
		OrderState: models.OrderStateFinish,
	})
	if err == nil && pinOrder != nil && pinOrder.OrderId != "" {
		return &CompletedMarketTrade{
			TxId:          txId,
			SellerAddress: pinOrder.SellerAddress,
			BuyerAddress:  pinOrder.BuyerAddress,
		}, nil
	}
	return nil, nil
}

func defaultPersistBlacklist(entry *models.MarketBlacklistModel) error {
	if major.GetSqlDB() == nil || entry == nil {
		return nil
	}
	return models.MarketBlacklistModelDao().Upsert(entry)
}

func ensureRiskIndexes() {
	db := major.GetSqlDB()
	if db == nil {
		return
	}
	type indexSpec struct {
		table   string
		name    string
		columns string
	}
	specs := []indexSpec{
		{table: "tb_market_mrc20_order", name: "idx_txid_state", columns: "txId(64), orderState"},
		{table: "tb_market_mrc20_order", name: "idx_utxo_seller_state", columns: "utxoId(80), sellerAddress(64), orderState"},
		{table: "tb_market_order", name: "idx_txid_state", columns: "txId(64), orderState"},
		{table: "tb_market_order", name: "idx_utxo_seller_state", columns: "utxoId(80), sellerAddress(64), orderState"},
		{table: "tb_mrc20_transfer_order", name: "idx_txid", columns: "txId(64)"},
	}
	for _, s := range specs {
		var cnt int64
		err := db.Raw(
			"SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?",
			s.table, s.name,
		).Scan(&cnt).Error
		if err != nil || cnt > 0 {
			continue
		}
		ddl := fmt.Sprintf("CREATE INDEX %s ON %s (%s)", s.name, s.table, s.columns)
		if err := db.Exec(ddl).Error; err != nil {
			fmt.Printf("[RISK] Ensure index %s.%s error: %v\n", s.table, s.name, err)
		} else {
			fmt.Printf("[RISK] Created index %s.%s\n", s.table, s.name)
		}
	}
}

// InitRiskService creates tb_market_blacklist, seeds initial entries (INSERT IGNORE),
// loads active entries into memory, sweeps existing tainted orders, and starts periodic refresh.
func InitRiskService() {
	eng := DefaultEngine()
	if major.GetSqlDB() == nil {
		return
	}
	ensureRiskIndexes()
	if err := models.MarketBlacklistModelDao().EnsureTable(); err != nil {
		fmt.Printf("[RISK] EnsureTable error: %v\n", err)
		return
	}
	for _, seed := range DefaultSeedEntries() {
		if err := models.MarketBlacklistModelDao().SeedIgnore(seed); err != nil {
			fmt.Printf("[RISK] SeedIgnore (%s:%s) error: %v\n", seed.EntryType, seed.EntryValue, err)
		}
	}
	if err := ReloadBlacklistFromDB(); err != nil {
		fmt.Printf("[RISK] ReloadBlacklistFromDB error: %v\n", err)
	}
	go func() {
		canceledMrc20, canceledPin := SweepAllActiveOrders()
		fmt.Printf("[RISK] Initialized blacklist risk engine; swept active orders: mrc20=%d, pin=%d\n", len(canceledMrc20), len(canceledPin))
	}()

	go func() {
		ticker := time.NewTicker(cacheRefreshInterval)
		defer ticker.Stop()
		for range ticker.C {
			_ = ReloadBlacklistFromDB()
		}
	}()
	_ = eng
}

func ReloadBlacklistFromDB() error {
	if major.GetSqlDB() == nil {
		return nil
	}
	list, err := models.MarketBlacklistModelDao().GetAllActive()
	if err != nil {
		return err
	}
	DefaultEngine().ApplyEntries(list)
	return nil
}

// ExtractPsbtHints pulls parent outpoints embedded in PSBT NonWitnessUtxo inputs
// (e.g. for unconfirmed Legacy AskTypePreTransfer orders) and local Mrc20TransferOrder records.
func ExtractPsbtHints(psbtBuilder *common.PsbtBuilder, preTxId string) []TaintHint {
	hints := make([]TaintHint, 0, 4)
	if psbtBuilder != nil && psbtBuilder.PsbtUpdater != nil && psbtBuilder.PsbtUpdater.Upsbt != nil {
		for _, pIn := range psbtBuilder.PsbtUpdater.Upsbt.Inputs {
			if pIn.NonWitnessUtxo != nil {
				for _, txIn := range pIn.NonWitnessUtxo.TxIn {
					hints = append(hints, TaintHint{
						PrevTxId: txIn.PreviousOutPoint.Hash.String(),
						PrevVout: txIn.PreviousOutPoint.Index,
					})
				}
			}
		}
	}
	if preTxId != "" && major.GetSqlDB() != nil {
		if trOrder, err := models.Mrc20TransferOrderModelDao().GetOne(&models.Mrc20TransferOrderModel{TxId: preTxId}); err == nil && trOrder != nil {
			if trOrder.Address != "" {
				hints = append(hints, TaintHint{FromAddress: trOrder.Address})
			}
			if trOrder.CommitTxId != "" {
				hints = append(hints, TaintHint{PrevTxId: trOrder.CommitTxId, PrevVout: 0})
			}
		}
	}
	return hints
}

func CheckMrc20OrderListing(chain, sellerAddress, utxoId string, outValue int64, hints []TaintHint) error {
	blocked, reason := DefaultEngine().CheckUtxoAndAddress(chain, sellerAddress, utxoId, outValue, hints)
	if blocked {
		fmt.Printf("[RISK] Blocked MRC20 order listing/take: seller=%s utxo=%s reason=%s\n", sellerAddress, utxoId, reason)
		return ErrBlockedStolenAsset
	}
	return nil
}

func CheckAssetOrderListing(chain, sellerAddress, utxoId string, outValue int64, hints []TaintHint) error {
	blocked, reason := DefaultEngine().CheckUtxoAndAddress(chain, sellerAddress, utxoId, outValue, hints)
	if blocked {
		fmt.Printf("[RISK] Blocked PIN order listing/take: seller=%s utxo=%s reason=%s\n", sellerAddress, utxoId, reason)
		return ErrBlockedStolenAsset
	}
	return nil
}

func CheckTransferPre(chain, changeAddress string, inputAddrs []string, inputUtxos []string, outAddrs []string) error {
	eng := DefaultEngine()
	if blocked, reason := eng.IsAddressBlacklisted(changeAddress); blocked {
		fmt.Printf("[RISK] Blocked Mrc20TransferPre changeAddress=%s reason=%s\n", changeAddress, reason)
		return ErrBlockedStolenAsset
	}
	for _, outAddr := range outAddrs {
		if blocked, reason := eng.IsAddressBlacklisted(outAddr); blocked {
			fmt.Printf("[RISK] Blocked Mrc20TransferPre recipient=%s reason=%s\n", outAddr, reason)
			return ErrBlockedStolenAsset
		}
	}
	for i, utxoId := range inputUtxos {
		inAddr := changeAddress
		if i < len(inputAddrs) && inputAddrs[i] != "" {
			inAddr = inputAddrs[i]
		}
		if blocked, reason := eng.CheckUtxoAndAddress(chain, inAddr, utxoId, 546, nil); blocked {
			fmt.Printf("[RISK] Blocked Mrc20TransferPre input utxo=%s addr=%s reason=%s\n", utxoId, inAddr, reason)
			return ErrBlockedStolenAsset
		}
	}
	return nil
}

func CheckTransferCommit(chain, orderWalletAddr, commitSignerAddr string, commitTx *wire.MsgTx) error {
	eng := DefaultEngine()
	for _, a := range []string{orderWalletAddr, commitSignerAddr} {
		if blocked, reason := eng.IsAddressBlacklisted(a); blocked {
			fmt.Printf("[RISK] Blocked Mrc20TransferCommit addr=%s reason=%s\n", a, reason)
			return ErrBlockedStolenAsset
		}
	}
	if commitTx != nil {
		for _, txIn := range commitTx.TxIn {
			utxoId := fmt.Sprintf("%s_%d", txIn.PreviousOutPoint.Hash.String(), txIn.PreviousOutPoint.Index)
			if blocked, reason := eng.CheckUtxoAndAddress(chain, commitSignerAddr, utxoId, 0, nil); blocked {
				fmt.Printf("[RISK] Blocked Mrc20TransferCommit gas input=%s signer=%s reason=%s\n", utxoId, commitSignerAddr, reason)
				return ErrBlockedStolenAsset
			}
		}
	}
	return nil
}

func IsOrderBlockedFast(sellerAddress, utxoId string) bool {
	return DefaultEngine().IsOrderBlockedFast(sellerAddress, utxoId)
}

func isBtcChain(chain string) bool {
	c := strings.ToLower(strings.TrimSpace(chain))
	return c == "" || c == "btc"
}

func CheckAddress(chain, address string) error {
	if !isBtcChain(chain) {
		return nil
	}
	if blocked, reason := DefaultEngine().IsAddressBlacklisted(address); blocked {
		fmt.Printf("[RISK] Blocked address=%s reason=%s\n", address, reason)
		return ErrBlockedStolenAsset
	}
	return nil
}

func CheckUtxo(chain, address, utxoId string, outValue int64) error {
	if !isBtcChain(chain) {
		return nil
	}
	if blocked, reason := DefaultEngine().CheckUtxoAndAddress(chain, address, utxoId, outValue, nil); blocked {
		fmt.Printf("[RISK] Blocked utxo=%s addr=%s reason=%s\n", utxoId, address, reason)
		return ErrBlockedStolenAsset
	}
	return nil
}

func (e *RiskEngine) IsUtxoTainted(chain, utxoId string, outValue int64, blockHeight int64) bool {
	if !isBtcChain(chain) {
		return false
	}
	normUtxo := NormalizeUtxoId(utxoId)
	txId, _, ok := ParseUtxoId(normUtxo)
	if !ok {
		return false
	}
	if e.isWhitelistedOutpoint(txId, normUtxo) {
		return false
	}
	if blockHeight > 0 && blockHeight < minTaintWalkBlockHeight {
		blocked, _, _ := e.isDirectBlacklistedOutpoint(txId, normUtxo)
		return blocked
	}
	blocked, _ := e.CheckUtxoAndAddress("btc", "", normUtxo, outValue, nil)
	return blocked
}

func IsUtxoTainted(chain, utxoId string, outValue int64, blockHeight int64) bool {
	return DefaultEngine().IsUtxoTainted(chain, utxoId, outValue, blockHeight)
}

// IsUtxoTaintedFast performs an O(1) in-memory whitelist/blacklist check without external HTTP
// ancestry traversal. Used on high-QPS read paths (e.g. /address/utxo) while write paths
// still run full multi-hop ancestry verification via CheckUtxoAndAddress.
func (e *RiskEngine) IsUtxoTaintedFast(chain, utxoId string, blockHeight int64) bool {
	if !isBtcChain(chain) {
		return false
	}
	normUtxo := NormalizeUtxoId(utxoId)
	txId, _, ok := ParseUtxoId(normUtxo)
	if !ok {
		return false
	}
	if e.isWhitelistedOutpoint(txId, normUtxo) {
		return false
	}
	blocked, _, _ := e.isDirectBlacklistedOutpoint(txId, normUtxo)
	return blocked
}

func IsUtxoTaintedFast(chain, utxoId string, blockHeight int64) bool {
	return DefaultEngine().IsUtxoTaintedFast(chain, utxoId, blockHeight)
}

func ExtractInputHintsAt(psbtBuilder *common.PsbtBuilder, idx int) []TaintHint {
	if psbtBuilder == nil || psbtBuilder.PsbtUpdater == nil || psbtBuilder.PsbtUpdater.Upsbt == nil {
		return nil
	}
	inputs := psbtBuilder.PsbtUpdater.Upsbt.Inputs
	if idx < 0 || idx >= len(inputs) || inputs[idx].NonWitnessUtxo == nil {
		return nil
	}
	nwu := inputs[idx].NonWitnessUtxo
	hints := make([]TaintHint, 0, len(nwu.TxIn))
	for _, txIn := range nwu.TxIn {
		hints = append(hints, TaintHint{
			PrevTxId: txIn.PreviousOutPoint.Hash.String(),
			PrevVout: txIn.PreviousOutPoint.Index,
		})
	}
	return hints
}

// CheckBuyerTakePsbt inspects the buyer address and every buyer-supplied BTC funding input
// (inputs[firstBuyerInputIdx:]) so stolen BTC or proceeds from selling stolen tokens cannot
// be used to buy clean MRC20/PIN/NFT assets on MetaID.Market, even from fresh Sybil wallets.
func (e *RiskEngine) CheckBuyerTakePsbt(chain, buyerAddress string, psbtBuilder *common.PsbtBuilder, firstBuyerInputIdx int) (bool, string) {
	if !isBtcChain(chain) {
		return false, ""
	}
	if blocked, reason := e.IsAddressBlacklisted(buyerAddress); blocked {
		return true, fmt.Sprintf("blacklisted buyer address %s (%s)", buyerAddress, reason)
	}
	if psbtBuilder == nil {
		return false, ""
	}
	txIns := psbtBuilder.GetInputs()
	if firstBuyerInputIdx < 0 {
		firstBuyerInputIdx = 0
	}
	for i := firstBuyerInputIdx; i < len(txIns); i++ {
		in := txIns[i]
		prevTx := NormalizeTxId(in.PreviousOutPoint.Hash.String())
		if prevTx == "" || prevTx == "0000000000000000000000000000000000000000000000000000000000000000" {
			continue
		}
		utxoId := fmt.Sprintf("%s_%d", prevTx, in.PreviousOutPoint.Index)
		hints := ExtractInputHintsAt(psbtBuilder, i)
		if blocked, reason := e.CheckUtxoAndAddress("btc", "", utxoId, 0, hints); blocked {
			return true, fmt.Sprintf("tainted buyer BTC input %s (%s)", utxoId, reason)
		}
	}
	return false, ""
}

func CheckBuyerTakePsbt(chain, buyerAddress string, psbtBuilder *common.PsbtBuilder, firstBuyerInputIdx int) error {
	if blocked, reason := DefaultEngine().CheckBuyerTakePsbt(chain, buyerAddress, psbtBuilder, firstBuyerInputIdx); blocked {
		fmt.Printf("[RISK] Blocked Market buyer take: buyer=%s reason=%s\n", buyerAddress, reason)
		return ErrBlockedStolenAsset
	}
	return nil
}

// CheckRawBtcTx inspects every input and recipient output of a raw BTC transaction
// before broadcast/commit so stolen BTC, sale-proceeds BTC, or stolen tokens cannot
// be broadcast through Market's /common/tx/broadcast or ID-Coins commit endpoints.
func (e *RiskEngine) CheckRawBtcTx(chain, rawTxHex string) (bool, string) {
	if !isBtcChain(chain) {
		return false, ""
	}
	trimmed := strings.TrimSpace(rawTxHex)
	if trimmed == "" {
		return false, ""
	}
	rawBytes, err := hex.DecodeString(trimmed)
	if err != nil {
		return false, ""
	}
	msgTx := wire.NewMsgTx(wire.TxVersion)
	if err := msgTx.Deserialize(bytes.NewReader(rawBytes)); err != nil {
		msgTx = wire.NewMsgTx(wire.TxVersion)
		if err2 := msgTx.DeserializeNoWitness(bytes.NewReader(rawBytes)); err2 != nil {
			return false, ""
		}
	}
	candidateHash := NormalizeTxId(msgTx.TxHash().String())
	if blocked, matchKey, reason := e.isDirectBlacklistedOutpoint(candidateHash, ""); blocked {
		return true, fmt.Sprintf("blacklisted txid %s (%s)", matchKey, reason)
	}
	for _, txIn := range msgTx.TxIn {
		prevTx := NormalizeTxId(txIn.PreviousOutPoint.Hash.String())
		if prevTx == "" || prevTx == "0000000000000000000000000000000000000000000000000000000000000000" {
			continue
		}
		utxoId := fmt.Sprintf("%s_%d", prevTx, txIn.PreviousOutPoint.Index)
		if blocked, reason := e.CheckUtxoAndAddress("btc", "", utxoId, 0, nil); blocked {
			return true, fmt.Sprintf("tainted tx input %s (%s)", utxoId, reason)
		}
	}
	netStr := conf.Net
	if netStr == "" {
		netStr = "mainnet"
	}
	for _, txOut := range msgTx.TxOut {
		if len(txOut.PkScript) == 0 {
			continue
		}
		if outAddr, addrErr := common.PkScriptToAddress(netStr, "btc", hex.EncodeToString(txOut.PkScript)); addrErr == nil && outAddr != "" {
			if blocked, reason := e.IsAddressBlacklisted(outAddr); blocked {
				return true, fmt.Sprintf("blacklisted recipient address %s (%s)", outAddr, reason)
			}
		}
	}
	return false, ""
}

func CheckRawBtcTx(chain, rawTxHex string) error {
	if blocked, reason := DefaultEngine().CheckRawBtcTx(chain, rawTxHex); blocked {
		fmt.Printf("[RISK] Blocked raw BTC tx broadcast/commit: reason=%s\n", reason)
		return ErrBlockedStolenAsset
	}
	return nil
}

func (e *RiskEngine) CheckPsbtInputs(chain, signerAddress, psbtRaw string) (bool, string) {
	if !isBtcChain(chain) {
		return false, ""
	}
	if signerAddress != "" {
		if blocked, reason := e.IsAddressBlacklisted(signerAddress); blocked {
			return true, fmt.Sprintf("blacklisted address %s (%s)", signerAddress, reason)
		}
	}
	trimmed := strings.TrimSpace(psbtRaw)
	if trimmed == "" {
		return false, ""
	}
	var pkt *psbt.Packet
	if rawBytes, err := hex.DecodeString(trimmed); err == nil {
		pkt, _ = psbt.NewFromRawBytes(bytes.NewReader(rawBytes), false)
	}
	if pkt == nil {
		pkt, _ = psbt.NewFromRawBytes(strings.NewReader(trimmed), true)
	}
	if pkt == nil || pkt.UnsignedTx == nil {
		return false, ""
	}
	for i, txIn := range pkt.UnsignedTx.TxIn {
		prevTx := NormalizeTxId(txIn.PreviousOutPoint.Hash.String())
		if prevTx == "" || prevTx == "0000000000000000000000000000000000000000000000000000000000000000" {
			continue
		}
		var hints []TaintHint
		if i < len(pkt.Inputs) && pkt.Inputs[i].NonWitnessUtxo != nil {
			for _, pTxIn := range pkt.Inputs[i].NonWitnessUtxo.TxIn {
				hints = append(hints, TaintHint{
					PrevTxId: pTxIn.PreviousOutPoint.Hash.String(),
					PrevVout: pTxIn.PreviousOutPoint.Index,
				})
			}
		}
		utxoId := fmt.Sprintf("%s_%d", prevTx, txIn.PreviousOutPoint.Index)
		if blocked, reason := e.CheckUtxoAndAddress("btc", "", utxoId, 0, hints); blocked {
			return true, fmt.Sprintf("tainted PSBT input %s (%s)", utxoId, reason)
		}
	}
	return false, ""
}

func CheckPsbtInputs(chain, signerAddress, psbtRaw string) error {
	if blocked, reason := DefaultEngine().CheckPsbtInputs(chain, signerAddress, psbtRaw); blocked {
		fmt.Printf("[RISK] Blocked PSBT commit: signer=%s reason=%s\n", signerAddress, reason)
		return ErrBlockedStolenAsset
	}
	return nil
}

// CancelBlockedMrc20Order cancels a tainted active MRC20 order and recalculates floorPrice
// without clobbering Supply or other market statistics.
func CancelBlockedMrc20Order(entity *models.MarketMrc20OrderModel, reason string) error {
	if entity == nil || major.GetSqlDB() == nil {
		return nil
	}
	db := major.GetSqlDB()
	now := tool.MakeTimestamp()
	if len(reason) > 240 {
		reason = reason[:240]
	}
	res := db.Model(&models.MarketMrc20OrderModel{}).
		Where("id = ? AND orderState = ?", entity.Id, models.OrderStateCreate).
		Updates(map[string]interface{}{
			"orderState": models.OrderStateCancel,
			"reason":     reason,
			"version":    gorm.Expr("version + 1"),
			"updateTime": now,
		})
	if res.Error != nil {
		return res.Error
	}
	entity.OrderState = models.OrderStateCancel
	entity.Reason = reason
	return RefreshMrc20FloorPrice(entity.TickId, entity.Chain)
}

func CancelBlockedPinOrder(entity *models.MarketOrderModel) error {
	if entity == nil || major.GetSqlDB() == nil {
		return nil
	}
	db := major.GetSqlDB()
	now := tool.MakeTimestamp()
	res := db.Model(&models.MarketOrderModel{}).
		Where("id = ? AND orderState = ?", entity.Id, models.OrderStateCreate).
		Updates(map[string]interface{}{
			"orderState": models.OrderStateCancel,
			"version":    gorm.Expr("version + 1"),
			"updateTime": now,
		})
	if res.Error != nil {
		return res.Error
	}
	entity.OrderState = models.OrderStateCancel
	return nil
}

func RefreshMrc20FloorPrice(tickId, chain string) error {
	if major.GetSqlDB() == nil || tickId == "" {
		return nil
	}
	if chain == "" {
		chain = "btc"
	}
	db := major.GetSqlDB()
	var orders []models.MarketMrc20OrderModel
	if err := db.Where("tickId = ? AND chain = ? AND orderState = ?", tickId, chain, models.OrderStateCreate).
		Order("tokenPriceRate asc").
		Find(&orders).Error; err != nil {
		return err
	}
	floorPrice := float64(0)
	for _, o := range orders {
		if IsOrderBlockedFast(o.SellerAddress, o.UtxoId) {
			continue
		}
		floorPrice = o.TokenPriceRate
		break
	}
	now := tool.MakeTimestamp()
	return db.Model(&models.MarketMrc20InfoModel{}).
		Where("tickId = ? AND chain = ?", tickId, chain).
		Updates(map[string]interface{}{
			"floorPrice": floorPrice,
			"version":    gorm.Expr("version + 1"),
			"updateTime": now,
		}).Error
}

// SweepAllActiveOrders scans all OrderStateCreate MRC20 and PIN orders (paginated),
// cancels any order whose seller address or UTXO lineage is tainted, and refreshes floorPrice.
func SweepAllActiveOrders() ([]string, []string) {
	if major.GetSqlDB() == nil {
		return nil, nil
	}
	if !atomic.CompareAndSwapInt32(&sweepRunning, 0, 1) {
		return nil, nil
	}
	defer atomic.StoreInt32(&sweepRunning, 0)
	db := major.GetSqlDB()
	canceledMrc20 := make([]string, 0)
	affectedTicks := make(map[string]string)

	var activeMrc20 []*models.MarketMrc20OrderModel
	if err := db.Where("orderState = ?", models.OrderStateCreate).Order("id asc").Find(&activeMrc20).Error; err == nil {
		for _, entity := range activeMrc20 {
			var psbtHints []TaintHint
			if entity.MakerPsbt != "" {
				if pb, pbErr := common.NewPsbtBuilder(common.GetNetParams(conf.Net, entity.Chain), entity.MakerPsbt); pbErr == nil {
					txId, _, _ := ParseUtxoId(entity.UtxoId)
					psbtHints = ExtractPsbtHints(pb, txId)
				}
			}
			if blocked, reason := DefaultEngine().CheckUtxoAndAddress(entity.Chain, entity.SellerAddress, entity.UtxoId, entity.OutValue, psbtHints); blocked {
				_ = CancelBlockedMrc20Order(entity, "Blocked stolen asset: "+reason)
				canceledMrc20 = append(canceledMrc20, entity.OrderId)
				affectedTicks[entity.TickId] = entity.Chain
			}
		}
	}
	// Always ensure METAID floorPrice is refreshed as well
	affectedTicks["8ce590918a3d493631ab9d9e3bbc89e322f3dd08354af87f97c07218818b78f4i0"] = "btc"
	for tickId, chain := range affectedTicks {
		_ = RefreshMrc20FloorPrice(tickId, chain)
	}

	canceledPin := make([]string, 0)
	var activePin []*models.MarketOrderModel
	if err := db.Where("orderState = ?", models.OrderStateCreate).Order("id asc").Find(&activePin).Error; err == nil {
		for _, entity := range activePin {
			if blocked, _ := DefaultEngine().CheckUtxoAndAddress(entity.Chain, entity.SellerAddress, entity.UtxoId, entity.OutValue, nil); blocked {
				_ = CancelBlockedPinOrder(entity)
				canceledPin = append(canceledPin, entity.OrderId)
			}
		}
	}

	return canceledMrc20, canceledPin
}
