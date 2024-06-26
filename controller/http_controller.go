package controller

import (
	"fmt"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"metaid-market-service/conf"
	"metaid-market-service/controller/auth"
	_ "metaid-market-service/docs"
	"net/http"
)

func Run() {
	//err := middleware.InitClient()
	//if err != nil {
	//	panic(err)
	//}
	//fmt.Println("Redis connect success for ip rate.")

	router := gin.Default()
	router.Use(Cors())
	router.Use(Logger())
	//router.Use(middleware.ResponseTime())

	//limiter := middleware.NewIPRateLimiter(100*time.Millisecond, 1000*1000*1000*1000)
	//router.Use(middleware.IPRateLimitMiddleware(limiter))

	v1 := router.Group("/api/v1")
	{
		v1.POST("/auth/test", auth.AuthSignMiddleware(), TestAuth)

		v1.POST("/market/order/push/batch", auth.AuthSignMiddleware(), BatchPushMarketOrder)
		v1.POST("/market/order/psbt/batch", auth.AuthSignMiddleware(), BatchFetchOrderPsbt)
		v1.POST("/market/order/take/batch", auth.AuthSignMiddleware(), BatchTakeMarketOrder)

		v1.POST("/market/order/push", auth.AuthSignMiddleware(), PushMarketOrder)
		v1.GET("/market/order/psbt", auth.AuthSignMiddleware(), FetchOrderPsbt)
		v1.POST("/market/order/take", auth.AuthSignMiddleware(), TakeMarketOrder)
		v1.POST("/market/order/cancel", auth.AuthSignMiddleware(), CancelMarketOrder)

		v1.GET("/market/orders", FetchMarketOrders)
		v1.GET("/market/order/detail", FetchMarketOneOrder)

		v1.GET("/market/address/assets", auth.AuthSignMiddleware(), FetchAddressAssetList)
		v1.GET("/market/asset/detail", FetchAssetDetail)

		v1.POST("/market/mrc20/order/push", auth.AuthSignMiddleware(), PushMarketMrc20Order)
		v1.GET("/market/mrc20/order/psbt", auth.AuthSignMiddleware(), FetchMrc20OrderPsbt)
		v1.POST("/market/mrc20/order/take", auth.AuthSignMiddleware(), TakeMarketMrc20Order)
		v1.POST("/market/mrc20/order/cancel", auth.AuthSignMiddleware(), CancelMarketMrc20Order)

		v1.GET("/market/mrc20/orders", FetchMarketMrc20Orders)
		v1.GET("/market/mrc20/order/detail", FetchMarketMrc20OneOrder)

		v1.POST("/inscribe/mrc20/mint/pre", auth.AuthSignMiddleware(), Mrc20MintPre)
		v1.POST("/inscribe/mrc20/mint/commit", auth.AuthSignMiddleware(), Mrc20MintCommit)
		v1.POST("/inscribe/mrc20/transfer/pre", auth.AuthSignMiddleware(), Mrc20TransferPre)
		v1.POST("/inscribe/mrc20/transfer/commit", auth.AuthSignMiddleware(), Mrc20TransferCommit)
		v1.POST("/inscribe/mrc20/deploy/commit", auth.AuthSignMiddleware(), Mrc20Deploy)
		v1.GET("/inscribe/mrc20/orders", FetchMrc20OpOrders)

		v1.GET("/common/mrc20/tick/info-list", FetchMrc20TickList)
		v1.GET("/common/mrc20/tick/info", FetchMrc20TickInfo)
		v1.GET("/common/mrc20/address/balance-list", FetchMrc20TickAddressBalances)
		v1.GET("/common/mrc20/address/utxo", FetchMrc20AddressUtxoList)
		v1.GET("/common/mrc20/address/shovel", FetchMrc20TickAddressShovels)

		v1.POST("/common/tx/broadcast", BroadcastTx)

		v1.POST("/admin/utxo/colddown", ColdDownDummyUtxo)
	}

	url := ginSwagger.URL("/swagger/doc.json")
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, url))

	_ = router.Run(fmt.Sprintf("0.0.0.0:%s", conf.Port))
}

func Cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		//origin := c.Request.Header.Get("Origin")
		//if origin != "" {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		c.Header("Access-Control-Allow-Headers", "Content-Type,AccessToken,X-CSRF-Token, Authorization,X-API-KEY,X-Signature,X-Public-Key")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Set("content-type", "application/json")
		//}
		if method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
		}
		c.Next()
	}
}

func Logger() gin.HandlerFunc {
	return func(context *gin.Context) {
		//context.JSON(http.StatusUnauthorized, gin.H{"status": "unauthorized"})
		//context.Abort()
		context.Next()
	}
}

func Handle(r *gin.Engine, httpMethods []string, relativePath string, handlers ...gin.HandlerFunc) gin.IRoutes {
	var routes gin.IRoutes
	for _, httpMethod := range httpMethods {
		routes = r.Handle(httpMethod, relativePath, handlers...)
	}
	return routes
}
