package grpc_service

import (
	"context"
	"errors"
	"fmt"
	"metaid-market-service/conf"
	"metaid-market-service/protobuf/common_service"
	"time"

	"google.golang.org/grpc"
)

type AssetBaseConn struct {
	clientConn  *grpc.ClientConn
	callTimeOut time.Duration
}

func GetAssetBaseConn() (*AssetBaseConn, error) {
	c := &AssetBaseConn{
		callTimeOut: 3 * time.Second,
	}
	connAddress := conf.GrpcAssetBaseAddress

	err := c.dial(connAddress)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *AssetBaseConn) dial(addr string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.callTimeOut)
	defer cancel()

	var err error
	c.clientConn, err = grpc.DialContext(ctx, addr, grpc.WithInsecure())
	if err != nil {
		fmt.Printf("dial rpc error:%s", err.Error())
		return err
	}
	return nil
}

func (c *AssetBaseConn) FetchCommonCoinPrice(coin string) (*common_service.CommonCoinPriceResp, error) {
	//t := tool.MakeTimestamp()
	client := common_service.NewCommonServiceClient(c.clientConn)
	if client == nil {
		return nil, errors.New("grpc client connect err")
	}

	req := &common_service.CommonCoinPriceReq{
		Coin: coin,
	}
	resp, err := client.FetchCommonCoinPrice(context.Background(), req)
	if err != nil {
		fmt.Println(err)
	}

	//fmt.Printf("[GRPC] :%d\n", tool.MakeTimestamp()-t)
	defer c.clientConn.Close()
	return resp, err
}
