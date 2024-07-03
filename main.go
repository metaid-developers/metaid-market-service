package main

import (
	"flag"
	"fmt"
	"metaid-market-service/conf"
	"metaid-market-service/controller"
	"metaid-market-service/major"
	"metaid-market-service/redis"
	"metaid-market-service/service/task"
)

var ENV string

func init() {
	flag.StringVar(&ENV, "env", "testnet", "EnvironmentEnum")
}

func InitEnv() {
	flag.Parse()
	if ENV == "loc" {
		conf.SystemEnvironmentEnum = conf.ExampleEnvironmentEnum
	} else if ENV == "mainnet" {
		conf.SystemEnvironmentEnum = conf.MainnetEnvironmentEnum
	} else if ENV == "testnet" {
		conf.SystemEnvironmentEnum = conf.TestnetEnvironmentEnum

	}
	fmt.Println(fmt.Sprintf("%s%v", "Env : ", ENV))
}

func InitAll() {
	conf.InitConfig()
	major.InitSqlConfig()
	redis.InitRedisManager()
	logName := "metaid-market-service"
	major.InitLogger(logName)
}

func run() {
	var (
		endRunning = make(chan bool, 1)
	)
	<-endRunning
}

// @title MetaID-Market API Service
// @version 1.0
// @description  MetaID-Market API Service
// @termsOfService
// @contact.name API Support
// @schemes https
// @BasePath /api-market-testnet
func main() {
	InitEnv()
	InitAll()
	//fix.FixMarketInfo()
	task.JobForCheckValidOrders()
	go task.RunJob()
	controller.Run()
}
