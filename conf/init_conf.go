package conf

import (
	"fmt"

	"github.com/spf13/viper"
)

var (
	Port string = ""

	Net string = ""

	RdsDsn          string = ""
	RdsMaxOpenConns int    = 0
	RdsMaxIgleConns int    = 0

	ManDomain           = ""
	ManBaseDomain       = ""
	OwnDomain           = ""
	WalletDomain        = ""
	IdCoinServiceDomain = ""
	IdCoinServiceKey    = ""
	TicketFansDomain    = ""
	MvcscanDamain       = ""
	MempoolSpace        = ""

	ZmqRawTxUrl = ""

	RedisEndpoint     = ""
	RedisPassword     = ""
	RedisDbUtxo   int = 1

	PlatformPrivateKeyDummyAsk, PlatformAddressDummyAsk, PlatformDogeAddressDummyAsk       = "", "", ""
	PlatformPrivateKeyReceiveFee, PlatformAddressReceiveFee, PlatformDogeAddressReceiveFee = "", "", ""
	PlatformPrivateKeySignMsg, PlatformPublicKeySignMsg                                    = "", ""

	PlatformServiceFeeConfigData *ServiceFeeConfig = &ServiceFeeConfig{}

	IdCoinsSignPublicKey string = ""
	IdCoinsSignTimestamp int64  = 0

	MetaCoinMrc20Id                          = ""
	MetaCoinCodehash, MetaCoinGenesis string = "", ""

	PopExtractCount int = 0

	GrpcAssetBaseAddress string = ""

	AutoBridgeRuleConfigData *AutoBridgeRuleConfig = &AutoBridgeRuleConfig{}

	Host string = ""

	AdminAPIKey string = ""

	SwapServiceUrl string = "http://127.0.0.1:18080"

	MinOrderPriceAmountBtc  int64 = 2000
	MinOrderPriceAmountDoge int64 = 1000000

	OrderbookConfigData *OrderbookConfig = DefaultOrderbookConfig()
)

type OrderbookPresetPairConfig struct {
	PairKey            string `json:"pair_key" mapstructure:"pair_key"`
	TokenCodeHash      string `json:"token_codehash" mapstructure:"token_codehash"`
	TokenID            string `json:"token_id" mapstructure:"token_id"`
	Symbol             string `json:"symbol" mapstructure:"symbol"`
	Decimals           int    `json:"decimals" mapstructure:"decimals"`
	QuoteTokenCodeHash string `json:"quote_token_codehash,omitempty" mapstructure:"quote_token_codehash"`
	QuoteTokenID       string `json:"quote_token_id,omitempty" mapstructure:"quote_token_id"`
	QuoteSymbol        string `json:"quote_symbol" mapstructure:"quote_symbol"`
	QuoteDecimals      int    `json:"quote_decimals" mapstructure:"quote_decimals"`
}

type OrderbookConfig struct {
	Enabled        bool                          `json:"enabled"`
	ServiceUrl     string                        `json:"service_url"`
	HigunUrl       string                        `json:"higun_url"`
	AskCodeHash    string                        `json:"ask_codehash"`
	BidCodeHash    string                        `json:"bid_codehash"`
	TokenCodeHash  string                        `json:"token_codehash"`
	QuoteTokens    []string                      `json:"quote_tokens"`
	PresetPairs    []OrderbookPresetPairConfig   `json:"preset_pairs"`
	FeeBps         int                           `json:"fee_bps"`
	FeeAddress     string                        `json:"fee_address"`
	PollIntervalMs int                           `json:"poll_interval_ms"`
	SoftLockTTLSec int                           `json:"soft_lock_ttl_sec"`
}

func DefaultOrderbookPresetPairs() []OrderbookPresetPairConfig {
	const (
		wbtcCH   = "c9cc7bbd1010b44873959a8b1a2bcedeb62302b7"
		wbtcID   = "2ee1af1ae2d3605f08894c17358c99f1fc4c1325"
		metaidCH = "c9cc7bbd1010b44873959a8b1a2bcedeb62302b7"
		metaidID = "28a481ecdae9499a599a9457ae70b5cf234ef417"
		mcCH     = "a2421f1e90c6048c36745edd44fad682e8644693"
		mcID     = "07e4c5a9f866164108de005be81d40ccbd2e964c"
	)
	return []OrderbookPresetPairConfig{
		{
			PairKey:       wbtcCH + ":" + wbtcID,
			TokenCodeHash: wbtcCH,
			TokenID:       wbtcID,
			Symbol:        "WBTC",
			Decimals:      8,
			QuoteSymbol:   "SPACE",
			QuoteDecimals: 8,
		},
		{
			PairKey:            metaidCH + ":" + metaidID + "/" + wbtcCH + ":" + wbtcID,
			TokenCodeHash:      metaidCH,
			TokenID:            metaidID,
			Symbol:             "METAID",
			Decimals:           8,
			QuoteTokenCodeHash: wbtcCH,
			QuoteTokenID:       wbtcID,
			QuoteSymbol:        "WBTC",
			QuoteDecimals:      8,
		},
		{
			PairKey:            mcCH + ":" + mcID + "/" + wbtcCH + ":" + wbtcID,
			TokenCodeHash:      mcCH,
			TokenID:            mcID,
			Symbol:             "MC",
			Decimals:           8,
			QuoteTokenCodeHash: wbtcCH,
			QuoteTokenID:       wbtcID,
			QuoteSymbol:        "WBTC",
			QuoteDecimals:      8,
		},
		{
			PairKey:       metaidCH + ":" + metaidID,
			TokenCodeHash: metaidCH,
			TokenID:       metaidID,
			Symbol:        "METAID",
			Decimals:      8,
			QuoteSymbol:   "SPACE",
			QuoteDecimals: 8,
		},
		{
			PairKey:       mcCH + ":" + mcID,
			TokenCodeHash: mcCH,
			TokenID:       mcID,
			Symbol:        "MC",
			Decimals:      8,
			QuoteSymbol:   "SPACE",
			QuoteDecimals: 8,
		},
	}
}

func DefaultOrderbookConfig() *OrderbookConfig {
	return &OrderbookConfig{
		Enabled:       false,
		ServiceUrl:    "http://127.0.0.1:18082",
		HigunUrl:      "http://127.0.0.1:8080",
		AskCodeHash:   "2704a8071b898b895b6d7bac3158959cea17c6d7",
		BidCodeHash:   "fdf3bc81875c241e4bb58c54fe3272ad6bbccebc",
		TokenCodeHash: "e4082bffecd647405c2a9b3d5f23d7abb2f6018e",
		QuoteTokens: []string{
			"c9cc7bbd1010b44873959a8b1a2bcedeb62302b7:2ee1af1ae2d3605f08894c17358c99f1fc4c1325",
		},
		PresetPairs:    DefaultOrderbookPresetPairs(),
		FeeBps:         25,
		FeeAddress:     "",
		PollIntervalMs: 300,
		SoftLockTTLSec: 25,
	}
}

type ServiceFeeConfig struct {
	ServiceAddress       string `json:"service_address"`
	DogeServiceAddress   string `json:"doge_service_address"`
	PinTradeFee          int64  `json:"pin_trade_fee"`
	PinTradeFeeRate      int64  `json:"pin_trade_fee_rate"`
	PinTradeFeeMin       int64  `json:"pin_trade_fee_min"`
	PinTradeFeeMinDoge   int64  `json:"pin_trade_fee_min_doge"`
	Mrc20TradeFee        int64  `json:"mrc20_trade_fee"`
	Mrc20TradeFeeRate    int64  `json:"mrc20_trade_fee_rate"`
	Mrc20TradeFeeMin     int64  `json:"mrc20_trade_fee_min"`
	Mrc20TradeFeeMinDoge int64  `json:"mrc20_trade_fee_min_doge"`
	DeployFee            int64  `json:"deploy_fee"`
	MintFee              int64  `json:"mint_fee"`
	TransferFee          int64  `json:"transfer_fee"`
	MetaIdInscribeFee    int64  `json:"meta_id_inscribe_fee"`
}

type AutoBridgeRuleConfig struct {
	OrderCountLimit    int64 `json:"order_count_limit"`
	MarketCapUsdtLimit int64 `json:"market_cap_usdt_limit"`
}

func InitConfig() {
	viper.SetConfigFile(GetYaml())
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}
	Port = viper.GetString("port")

	Net = viper.GetString("net")

	RdsDsn = viper.GetString("rds.dsn")
	RdsMaxOpenConns = viper.GetInt("rds.max_open_conns")
	RdsMaxIgleConns = viper.GetInt("rds.max_igle_conns")

	ManDomain = viper.GetString("man.domain")
	ManBaseDomain = viper.GetString("man.base")
	OwnDomain = viper.GetString("own.domain")
	WalletDomain = viper.GetString("wallet_node.domain")
	IdCoinServiceDomain = viper.GetString("orders_exchange.domain")
	IdCoinServiceKey = viper.GetString("orders_exchange.key")
	TicketFansDomain = viper.GetString("ticket_fans.domain")
	MvcscanDamain = viper.GetString("mvcscan.domain")
	MempoolSpace = viper.GetString("mempool_space.domain")

	RedisEndpoint, RedisPassword = viper.GetString("redis.endpoint"), viper.GetString("redis.password")
	RedisDbUtxo = viper.GetInt("redis.db_utxo")

	ZmqRawTxUrl = viper.GetString("zmq.btc.rawtx_url")

	PlatformPrivateKeyDummyAsk, PlatformAddressDummyAsk, PlatformDogeAddressDummyAsk = viper.GetString("platform.dummy.private_key"), viper.GetString("platform.dummy.address"), viper.GetString("platform.dummy.doge_address")
	PlatformPrivateKeyReceiveFee, PlatformAddressReceiveFee, PlatformDogeAddressReceiveFee = viper.GetString("platform.receive_fee.private_key"), viper.GetString("platform.receive_fee.address"), viper.GetString("platform.receive_fee.doge_address")
	PlatformPrivateKeySignMsg, PlatformPublicKeySignMsg = viper.GetString("platform.sign_msg.private_key"), viper.GetString("platform.sign_msg.public_key")

	IdCoinsSignPublicKey = viper.GetString("idcoins.public_key")
	IdCoinsSignTimestamp = viper.GetInt64("idcoins.timestamp")

	MetaCoinMrc20Id = viper.GetString("meta_coin.mrc20id")
	MetaCoinCodehash, MetaCoinGenesis = viper.GetString("meta_coin.codehash"), viper.GetString("meta_coin.genesis")

	PopExtractCount = viper.GetInt("pop.extract_count")

	GrpcAssetBaseAddress = viper.GetString("grpc.asset_base.address")

	PlatformServiceFeeConfigData = &ServiceFeeConfig{
		ServiceAddress:       viper.GetString("platform.service_fee.service_address"),
		PinTradeFee:          viper.GetInt64("platform.service_fee.pin_trade_fee"),
		PinTradeFeeRate:      viper.GetInt64("platform.service_fee.pin_trade_fee_rate"),
		PinTradeFeeMin:       viper.GetInt64("platform.service_fee.pin_trade_fee_min"),
		PinTradeFeeMinDoge:   viper.GetInt64("platform.service_fee.pin_trade_fee_min_doge"),
		Mrc20TradeFee:        viper.GetInt64("platform.service_fee.mrc20_trade_fee"),
		Mrc20TradeFeeRate:    viper.GetInt64("platform.service_fee.mrc20_trade_fee_rate"),
		Mrc20TradeFeeMin:     viper.GetInt64("platform.service_fee.mrc20_trade_fee_min"),
		Mrc20TradeFeeMinDoge: viper.GetInt64("platform.service_fee.mrc20_trade_fee_min_doge"),
		DeployFee:            viper.GetInt64("platform.service_fee.deploy_fee"),
		MintFee:              viper.GetInt64("platform.service_fee.mint_fee"),
		TransferFee:          viper.GetInt64("platform.service_fee.transfer_fee"),
	}

	AutoBridgeRuleConfigData = &AutoBridgeRuleConfig{
		OrderCountLimit:    viper.GetInt64("auto_bridge_rule.order_count_limit"),
		MarketCapUsdtLimit: viper.GetInt64("auto_bridge_rule.market_cap_usdt_limit"),
	}

	Host = viper.GetString("host")
	AdminAPIKey = viper.GetString("admin.api_key")
	if v := viper.GetString("swap.service_url"); v != "" {
		SwapServiceUrl = v
	}
	if v := viper.GetInt64("order.min_price_amount_btc"); v > 0 {
		MinOrderPriceAmountBtc = v
	}
	if v := viper.GetInt64("order.min_price_amount_doge"); v > 0 {
		MinOrderPriceAmountDoge = v
	}

	obCfg := DefaultOrderbookConfig()
	if viper.IsSet("orderbook.enabled") {
		obCfg.Enabled = viper.GetBool("orderbook.enabled")
	}
	if v := viper.GetString("orderbook.service_url"); v != "" {
		obCfg.ServiceUrl = v
	}
	if v := viper.GetString("orderbook.higun_url"); v != "" {
		obCfg.HigunUrl = v
	}
	if v := viper.GetString("orderbook.ask_codehash"); v != "" {
		obCfg.AskCodeHash = v
	}
	if v := viper.GetString("orderbook.bid_codehash"); v != "" {
		obCfg.BidCodeHash = v
	}
	if v := viper.GetString("orderbook.token_codehash"); v != "" {
		obCfg.TokenCodeHash = v
	}
	if vs := viper.GetStringSlice("orderbook.quote_tokens"); len(vs) > 0 {
		obCfg.QuoteTokens = vs
	}
	if viper.IsSet("orderbook.preset_pairs") {
		var customPresets []OrderbookPresetPairConfig
		if err := viper.UnmarshalKey("orderbook.preset_pairs", &customPresets); err == nil && len(customPresets) > 0 {
			obCfg.PresetPairs = customPresets
		}
	}
	if viper.IsSet("orderbook.fee_bps") {
		obCfg.FeeBps = viper.GetInt("orderbook.fee_bps")
	}
	if v := viper.GetString("orderbook.fee_address"); v != "" {
		obCfg.FeeAddress = v
	}
	if v := viper.GetInt("orderbook.poll_interval_ms"); v > 0 {
		obCfg.PollIntervalMs = v
	}
	if v := viper.GetInt("orderbook.soft_lock_ttl_sec"); v > 0 {
		obCfg.SoftLockTTLSec = v
	}
	OrderbookConfigData = obCfg
}
