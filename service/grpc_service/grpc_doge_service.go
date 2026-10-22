package grpc_service

import (
	"context"
	"errors"
	"fmt"
	"metaid-market-service/conf"
	"metaid-market-service/protobuf/doge_service"
	"time"

	"google.golang.org/grpc"
)

type DogeBaseConn struct {
	clientConn  *grpc.ClientConn
	callTimeOut time.Duration
}

func GetDogeBaseConn() (*DogeBaseConn, error) {
	c := &DogeBaseConn{
		callTimeOut: 3 * time.Second,
	}
	connAddress := conf.GrpcAssetBaseAddress

	err := c.dial(connAddress)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *DogeBaseConn) dial(addr string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.callTimeOut)
	defer cancel()

	var err error
	c.clientConn, err = grpc.DialContext(ctx, addr, grpc.WithInsecure())
	if err != nil {
		fmt.Printf("dial rpc error:%s", err.Error())
	}
	return nil
}

func (c *DogeBaseConn) CheckUtxoInfo(outpoints []string) (*doge_service.DogeCheckUtxoInfoResp, error) {
	//t := tool.MakeTimestamp()
	client := doge_service.NewDogeServiceClient(c.clientConn)
	if client == nil {
		return nil, errors.New("grpc client connect err")
	}

	req := &doge_service.DogeCheckUtxoInfoReq{
		OutPoints: outpoints,
	}
	resp, err := client.CheckUtxoInfo(context.Background(), req)
	if err != nil {
		fmt.Println(err)
	}

	//fmt.Printf("[GRPC] :%d\n", tool.MakeTimestamp()-t)
	defer c.clientConn.Close()
	return resp, err
}

func (c *DogeBaseConn) BroadcastTx(hex, source string) (*doge_service.DogeBroadcastTxResp, error) {
	//t := tool.MakeTimestamp()
	client := doge_service.NewDogeServiceClient(c.clientConn)
	if client == nil {
		return nil, errors.New("grpc client connect err")
	}

	req := &doge_service.DogeBroadcastTxReq{
		Hex:    hex,
		Source: source,
	}
	resp, err := client.BroadcastTx(context.Background(), req)
	if err != nil {
		fmt.Println(err)
	}

	//fmt.Printf("[GRPC] :%d\n", tool.MakeTimestamp()-t)
	defer c.clientConn.Close()
	return resp, err
}

// address service
func (c *DogeBaseConn) GetDogeAddressBalance(req *doge_service.DogeAddressBalanceReq) (*doge_service.DogeAddressBalanceResp, error) {
	client := doge_service.NewDogeServiceClient(c.clientConn)
	if client == nil {
		return nil, errors.New("grpc client connect err")
	}

	resp, err := client.GetAddressBalance(context.Background(), req)
	if err != nil {
		fmt.Println(err)
	}

	defer c.clientConn.Close()
	return resp, err
}

func (c *DogeBaseConn) GetDogeAddressUtxo(req *doge_service.DogeAddressUtxoReq) (*doge_service.DogeAddressUtxoResp, error) {
	client := doge_service.NewDogeServiceClient(c.clientConn)
	if client == nil {
		return nil, errors.New("grpc client connect err")
	}

	resp, err := client.GetAddressUtxo(context.Background(), req)
	if err != nil {
		fmt.Println(err)
	}

	defer c.clientConn.Close()
	return resp, err
}

func (c *DogeBaseConn) GetDogeAddressTx(req *doge_service.DogeAddressTxReq) (*doge_service.DogeAddressTxResp, error) {
	client := doge_service.NewDogeServiceClient(c.clientConn)
	if client == nil {
		return nil, errors.New("grpc client connect err")
	}

	resp, err := client.GetAddressTx(context.Background(), req)
	if err != nil {
		fmt.Println(err)
	}

	defer c.clientConn.Close()
	return resp, err
}

func (c *DogeBaseConn) GetDogeAddressTxCount(req *doge_service.DogeAddressTxCountReq) (*doge_service.DogeAddressTxCountResp, error) {
	client := doge_service.NewDogeServiceClient(c.clientConn)
	if client == nil {
		return nil, errors.New("grpc client connect err")
	}

	resp, err := client.GetAddressTxCount(context.Background(), req)
	if err != nil {
		fmt.Println(err)
	}

	defer c.clientConn.Close()
	return resp, err
}

// tx service
func (c *DogeBaseConn) GetDogeTxRaw(req *doge_service.DogeTxRawReq) (*doge_service.DogeTxRawResp, error) {
	client := doge_service.NewDogeServiceClient(c.clientConn)
	if client == nil {
		return nil, errors.New("grpc client connect err")
	}

	resp, err := client.GetTxRaw(context.Background(), req)
	if err != nil {
		fmt.Println(err)
	}

	defer c.clientConn.Close()
	return resp, err
}

// fee service
func (c *DogeBaseConn) GetDogeFeeRate(req *doge_service.DogeFeeRateReq) (*doge_service.DogeFeeRateResp, error) {
	client := doge_service.NewDogeServiceClient(c.clientConn)
	if client == nil {
		return nil, errors.New("grpc client connect err")
	}

	resp, err := client.GetFeeRate(context.Background(), req)
	if err != nil {
		fmt.Println(err)
	}

	defer c.clientConn.Close()
	return resp, err
}

// tx info service
func (c *DogeBaseConn) GetTxInfo(req *doge_service.DogeTxInfoReq) (*doge_service.DogeTxInfoResp, error) {
	client := doge_service.NewDogeServiceClient(c.clientConn)
	if client == nil {
		return nil, errors.New("grpc client connect err")
	}

	resp, err := client.GetTxInfo(context.Background(), req)
	if err != nil {
		fmt.Println(err)
	}

	defer c.clientConn.Close()
	return resp, err
}
