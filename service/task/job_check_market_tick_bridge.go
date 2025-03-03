package task

import "metaid-market-service/service/auto_service"

func JobForCheckMarketTickInfo() {
	auto_service.CheckMarketTickInfo()

	//sync auto bridge info
	auto_service.SyncAutoBridgeInfo()
}
