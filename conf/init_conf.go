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

	ManDomain            = ""
	ManBaseDomain        = ""
	OwnDomain            = ""
	WalletDomain         = ""
	OrdersExchangeDomain = ""
	OrdersExchangeKey    = ""
	TicketFansDomain     = ""
	MvcscanDamain        = ""
	MempoolSpace         = ""

	ZmqRawTxUrl = ""

	RedisEndpoint     = ""
	RedisPassword     = ""
	RedisDbUtxo   int = 1

	PlatformPrivateKeyDummyAsk, PlatformAddressDummyAsk     = "", ""
	PlatformPrivateKeyReceiveFee, PlatformAddressReceiveFee = "", ""
	PlatformPrivateKeySignMsg, PlatformPublicKeySignMsg     = "", ""

	PlatformServiceFeeConfigData *ServiceFeeConfig = &ServiceFeeConfig{}

	IdCoinsSignPublicKey string = ""
	IdCoinsSignTimestamp int64  = 0

	MetaCoinMrc20Id                          = ""
	MetaCoinCodehash, MetaCoinGenesis string = "", ""

	PopExtractCount int = 0

	GrpcAssetBaseAddress string = ""

	AutoBridgeRuleConfigData *AutoBridgeRuleConfig = &AutoBridgeRuleConfig{}

	Host string = ""
)

type ServiceFeeConfig struct {
	ServiceAddress    string `json:"service_address"`
	PinTradeFee       int64  `json:"pin_trade_fee"`
	PinTradeFeeRate   int64  `json:"pin_trade_fee_rate"`
	PinTradeFeeMin    int64  `json:"pin_trade_fee_min"`
	Mrc20TradeFee     int64  `json:"mrc20_trade_fee"`
	Mrc20TradeFeeRate int64  `json:"mrc20_trade_fee_rate"`
	Mrc20TradeFeeMin  int64  `json:"mrc20_trade_fee_min"`
	DeployFee         int64  `json:"deploy_fee"`
	MintFee           int64  `json:"mint_fee"`
	TransferFee       int64  `json:"transfer_fee"`
	MetaIdInscribeFee int64  `json:"meta_id_inscribe_fee"`
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
	OrdersExchangeDomain = viper.GetString("orders_exchange.domain")
	OrdersExchangeKey = viper.GetString("orders_exchange.key")
	TicketFansDomain = viper.GetString("ticket_fans.domain")
	MvcscanDamain = viper.GetString("mvcscan.domain")
	MempoolSpace = viper.GetString("mempool_space.domain")

	RedisEndpoint, RedisPassword = viper.GetString("redis.endpoint"), viper.GetString("redis.password")
	RedisDbUtxo = viper.GetInt("redis.db_utxo")

	ZmqRawTxUrl = viper.GetString("zmq.btc.rawtx_url")

	PlatformPrivateKeyDummyAsk, PlatformAddressDummyAsk = viper.GetString("platform.dummy.private_key"), viper.GetString("platform.dummy.address")
	PlatformPrivateKeyReceiveFee, PlatformAddressReceiveFee = viper.GetString("platform.receive_fee.private_key"), viper.GetString("platform.receive_fee.address")
	PlatformPrivateKeySignMsg, PlatformPublicKeySignMsg = viper.GetString("platform.sign_msg.private_key"), viper.GetString("platform.sign_msg.public_key")

	IdCoinsSignPublicKey = viper.GetString("idcoins.public_key")
	IdCoinsSignTimestamp = viper.GetInt64("idcoins.timestamp")

	MetaCoinMrc20Id = viper.GetString("meta_coin.mrc20id")
	MetaCoinCodehash, MetaCoinGenesis = viper.GetString("meta_coin.codehash"), viper.GetString("meta_coin.genesis")

	PopExtractCount = viper.GetInt("pop.extract_count")

	GrpcAssetBaseAddress = viper.GetString("grpc.asset_base.address")

	PlatformServiceFeeConfigData = &ServiceFeeConfig{
		ServiceAddress:    viper.GetString("platform.service_fee.service_address"),
		PinTradeFee:       viper.GetInt64("platform.service_fee.pin_trade_fee"),
		PinTradeFeeRate:   viper.GetInt64("platform.service_fee.pin_trade_fee_rate"),
		PinTradeFeeMin:    viper.GetInt64("platform.service_fee.pin_trade_fee_min"),
		Mrc20TradeFee:     viper.GetInt64("platform.service_fee.mrc20_trade_fee"),
		Mrc20TradeFeeRate: viper.GetInt64("platform.service_fee.mrc20_trade_fee_rate"),
		Mrc20TradeFeeMin:  viper.GetInt64("platform.service_fee.mrc20_trade_fee_min"),
		DeployFee:         viper.GetInt64("platform.service_fee.deploy_fee"),
		MintFee:           viper.GetInt64("platform.service_fee.mint_fee"),
		TransferFee:       viper.GetInt64("platform.service_fee.transfer_fee"),
	}

	AutoBridgeRuleConfigData = &AutoBridgeRuleConfig{
		OrderCountLimit:    viper.GetInt64("auto_bridge_rule.order_count_limit"),
		MarketCapUsdtLimit: viper.GetInt64("auto_bridge_rule.market_cap_usdt_limit"),
	}

	Host = viper.GetString("host")
}
