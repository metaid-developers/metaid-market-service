package grpc_service

import (
	"context"
	"errors"
	"fmt"
	"metaid-market-service/conf"
	"metaid-market-service/protobuf/btc_service"
	"time"

	"google.golang.org/grpc"
)

type BtcBaseConn struct {
	clientConn  *grpc.ClientConn
	callTimeOut time.Duration
}

func GetBtcBaseConn() (*BtcBaseConn, error) {
	c := &BtcBaseConn{
		callTimeOut: 3 * time.Second,
	}
	connAddress := conf.GrpcAssetBaseAddress

	err := c.dial(connAddress)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *BtcBaseConn) dial(addr string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.callTimeOut)
	defer cancel()

	var err error
	c.clientConn, err = grpc.DialContext(ctx, addr, grpc.WithInsecure())
	if err != nil {
		fmt.Printf("dial rpc error:%s", err.Error())
	}
	return nil
}

func (c *BtcBaseConn) FetchTxRaw(txId string) (*btc_service.FetchTxRawResp, error) {
	//t := tool.MakeTimestamp()
	client := btc_service.NewBtcServiceClient(c.clientConn)
	if client == nil {
		return nil, errors.New("grpc client connect err")
	}

	req := &btc_service.FetchTxRawReq{
		TxId: txId,
	}
	resp, err := client.FetchTxRaw(context.Background(), req)
	if err != nil {
		fmt.Println(err)
	}

	//fmt.Printf("[GRPC] :%d\n", tool.MakeTimestamp()-t)
	defer c.clientConn.Close()
	return resp, err
}

func (c *BtcBaseConn) CheckUtxoInfo(outpoints []string) (*btc_service.CheckUtxoInfoResp, error) {
	//t := tool.MakeTimestamp()
	client := btc_service.NewBtcServiceClient(c.clientConn)
	if client == nil {
		return nil, errors.New("grpc client connect err")
	}

	req := &btc_service.CheckUtxoInfoReq{
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

func (c *BtcBaseConn) BroadcastTx(rawTx string) (*btc_service.BtcBroadcastTxResp, error) {
	//t := tool.MakeTimestamp()
	client := btc_service.NewBtcServiceClient(c.clientConn)
	if client == nil {
		return nil, errors.New("grpc client connect err")
	}

	req := &btc_service.BtcBroadcastTxReq{
		RawTx: rawTx,
	}
	resp, err := client.BtcBroadcastTx(context.Background(), req)
	if err != nil {
		fmt.Println(err)
	}

	//fmt.Printf("[GRPC] :%d\n", tool.MakeTimestamp()-t)
	defer c.clientConn.Close()
	return resp, err
}

func (c *BtcBaseConn) FetchBtcUtxo(address, order, unconfirmed string) (*btc_service.FetchBtcUtxoResp, error) {
	//t := tool.MakeTimestamp()
	client := btc_service.NewBtcServiceClient(c.clientConn)
	if client == nil {
		return nil, errors.New("grpc client connect err")
	}

	req := &btc_service.FetchBtcUtxoReq{
		Address:     address,
		Order:       order,
		Unconfirmed: unconfirmed,
	}
	resp, err := client.FetchBtcUtxo(context.Background(), req)
	if err != nil {
		fmt.Println(err)
	}

	//fmt.Printf("[GRPC] :%d\n", tool.MakeTimestamp()-t)
	defer c.clientConn.Close()
	return resp, err
}

func (c *BtcBaseConn) FetchBtcBalance(address string) (*btc_service.FetchBtcBalanceResp, error) {
	//t := tool.MakeTimestamp()
	client := btc_service.NewBtcServiceClient(c.clientConn)
	if client == nil {
		return nil, errors.New("grpc client connect err")
	}

	req := &btc_service.FetchBtcBalanceReq{
		Address: address,
	}
	resp, err := client.FetchBtcBalance(context.Background(), req)
	if err != nil {
		fmt.Println(err)
	}

	//fmt.Printf("[GRPC] :%d\n", tool.MakeTimestamp()-t)
	defer c.clientConn.Close()
	return resp, err
}

// address service
func (c *BtcBaseConn) GetBtcAddressBalance(req *btc_service.BtcAddressBalanceReq) (*btc_service.BtcAddressBalanceResp, error) {
	client := btc_service.NewBtcServiceClient(c.clientConn)
	if client == nil {
		return nil, errors.New("grpc client connect err")
	}

	resp, err := client.GetBtcAddressBalance(context.Background(), req)
	if err != nil {
		fmt.Println(err)
	}

	defer c.clientConn.Close()
	return resp, err
}

func (c *BtcBaseConn) GetBtcAddressUtxo(req *btc_service.BtcAddressUtxoReq) (*btc_service.BtcAddressUtxoResp, error) {
	client := btc_service.NewBtcServiceClient(c.clientConn)
	if client == nil {
		return nil, errors.New("grpc client connect err")
	}

	resp, err := client.GetBtcAddressUtxo(context.Background(), req)
	if err != nil {
		fmt.Println(err)
	}

	defer c.clientConn.Close()
	return resp, err
}

func (c *BtcBaseConn) GetBtcAddressTx(req *btc_service.BtcAddressTxReq) (*btc_service.BtcAddressTxResp, error) {
	client := btc_service.NewBtcServiceClient(c.clientConn)
	if client == nil {
		return nil, errors.New("grpc client connect err")
	}

	resp, err := client.GetBtcAddressTx(context.Background(), req)
	if err != nil {
		fmt.Println(err)
	}

	defer c.clientConn.Close()
	return resp, err
}

func (c *BtcBaseConn) GetBtcAddressTxCount(req *btc_service.BtcAddressTxCountReq) (*btc_service.BtcAddressTxCountResp, error) {
	client := btc_service.NewBtcServiceClient(c.clientConn)
	if client == nil {
		return nil, errors.New("grpc client connect err")
	}

	resp, err := client.GetBtcAddressTxCount(context.Background(), req)
	if err != nil {
		fmt.Println(err)
	}

	defer c.clientConn.Close()
	return resp, err
}

// tx service
func (c *BtcBaseConn) GetBtcTxRaw(req *btc_service.BtcTxRawReq) (*btc_service.BtcTxRawResp, error) {
	client := btc_service.NewBtcServiceClient(c.clientConn)
	if client == nil {
		return nil, errors.New("grpc client connect err")
	}

	resp, err := client.GetBtcTxRaw(context.Background(), req)
	if err != nil {
		fmt.Println(err)
	}

	defer c.clientConn.Close()
	return resp, err
}

// fee service
func (c *BtcBaseConn) GetBtcFeeRate(req *btc_service.BtcFeeRateReq) (*btc_service.BtcFeeRateResp, error) {
	client := btc_service.NewBtcServiceClient(c.clientConn)
	if client == nil {
		return nil, errors.New("grpc client connect err")
	}

	resp, err := client.GetBtcFeeRate(context.Background(), req)
	if err != nil {
		fmt.Println(err)
	}

	defer c.clientConn.Close()
	return resp, err
}
