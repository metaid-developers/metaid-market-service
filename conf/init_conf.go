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
	OwnDomain            = ""
	WalletDomain         = ""
	OrdersExchangeDomain = ""
	OrdersExchangeKey    = ""

	RpcUrl      = ""
	RpcUsername = ""
	RpcPassword = ""

	ZmqRawTxUrl = ""

	RedisEndpoint     = ""
	RedisPassword     = ""
	RedisDbUtxo   int = 1

	PlatformPrivateKeyDummyAsk, PlatformAddressDummyAsk     = "", ""
	PlatformPrivateKeyReceiveFee, PlatformAddressReceiveFee = "", ""
	PlatformPrivateKeySignMsg, PlatformPublicKeySignMsg     = "", ""

	PopExtractCount int = 0
)

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
	OwnDomain = viper.GetString("own.domain")
	WalletDomain = viper.GetString("wallet_node.domain")
	OrdersExchangeDomain = viper.GetString("orders_exchange.domain")
	OrdersExchangeKey = viper.GetString("orders_exchange.key")

	RedisEndpoint, RedisPassword = viper.GetString("redis.endpoint"), viper.GetString("redis.password")
	RedisDbUtxo = viper.GetInt("redis.db_utxo")

	RpcUrl = viper.GetString("rpc.url")
	RpcUsername = viper.GetString("rpc.username")
	RpcPassword = viper.GetString("rpc.password")

	ZmqRawTxUrl = viper.GetString("zmq.btc.rawtx_url")

	PlatformPrivateKeyDummyAsk, PlatformAddressDummyAsk = viper.GetString("platform.dummy.private_key"), viper.GetString("platform.dummy.address")
	PlatformPrivateKeyReceiveFee, PlatformAddressReceiveFee = viper.GetString("platform.receive_fee.private_key"), viper.GetString("platform.receive_fee.address")
	PlatformPrivateKeySignMsg, PlatformPublicKeySignMsg = viper.GetString("platform.sign_msg.private_key"), viper.GetString("platform.sign_msg.public_key")

	PopExtractCount = viper.GetInt("pop.extract_count")
}
