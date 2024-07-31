package grpc_service

import (
	"context"
	"errors"
	"fmt"
	"google.golang.org/grpc"
	"metaid-market-service/conf"
	"metaid-market-service/protobuf/mrc20_holders_service"
	"metaid-market-service/protobuf/mrc20_utxo_service"
	"time"
)

type Mrc20BaseConn struct {
	clientConn  *grpc.ClientConn
	callTimeOut time.Duration
}

func GetMrc20BaseConn() (*Mrc20BaseConn, error) {
	c := &Mrc20BaseConn{
		callTimeOut: 3 * time.Second,
	}
	connAddress := conf.GrpcAssetBaseAddress

	err := c.dial(connAddress)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Mrc20BaseConn) dial(addr string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.callTimeOut)
	defer cancel()

	var err error
	c.clientConn, err = grpc.DialContext(ctx, addr, grpc.WithInsecure())
	if err != nil {
		fmt.Printf("dial rpc error:%s", err.Error())
	}
	return nil
}

func (c *Mrc20BaseConn) FetchMrc20TickHolders(tickId string, cursor, limit int64) (*mrc20_holders_service.Mrc20TickHoldersResponse, error) {
	//t := tool.MakeTimestamp()
	client := mrc20_holders_service.NewMrc20HoldersServiceClient(c.clientConn)
	if client == nil {
		return nil, errors.New("grpc client connect err")
	}

	req := &mrc20_holders_service.Mrc20TickHoldersRequest{
		TickId: tickId,
		Offset: cursor,
		Limit:  limit,
	}
	resp, err := client.FetchMrc20TickHolders(context.Background(), req)
	if err != nil {
		fmt.Println(err)
	}

	//fmt.Printf("[GRPC] :%d\n", tool.MakeTimestamp()-t)
	defer c.clientConn.Close()
	return resp, err
}

func (c *Mrc20BaseConn) FetchMrc20AddressBalanceList(tickIds []string, address string, cursor, limit int64) (*mrc20_utxo_service.Mrc20BalanceListResponse, error) {
	//t := tool.MakeTimestamp()
	client := mrc20_utxo_service.NewMrc20UtxoServiceClient(c.clientConn)
	if client == nil {
		return nil, errors.New("grpc client connect err")
	}

	req := &mrc20_utxo_service.AddressMrc20BalanceRequest{
		Address: address,
		TickIds: tickIds,
		Offset:  cursor,
		Limit:   limit,
	}
	resp, err := client.FetchMrc20AddressBalanceList(context.Background(), req)
	if err != nil {
		fmt.Println(err)
	}

	//fmt.Printf("[GRPC] :%d\n", tool.MakeTimestamp()-t)
	defer c.clientConn.Close()
	return resp, err
}

func (c *Mrc20BaseConn) FetchMrc20AddressUtxoList(tickIds []string, address string, cursor, limit int64) (*mrc20_utxo_service.Mrc20UtxoResponse, error) {
	//t := tool.MakeTimestamp()
	client := mrc20_utxo_service.NewMrc20UtxoServiceClient(c.clientConn)
	if client == nil {
		return nil, errors.New("grpc client connect err")
	}

	req := &mrc20_utxo_service.AddressMrc20UtxoRequest{
		Address: address,
		TickIds: tickIds,
		Offset:  cursor,
		Limit:   limit,
	}
	resp, err := client.FetchMrc20AddressUtxoList(context.Background(), req)
	if err != nil {
		fmt.Println(err)
	}

	//fmt.Printf("[GRPC] :%d\n", tool.MakeTimestamp()-t)
	defer c.clientConn.Close()
	return resp, err
}

func (c *Mrc20BaseConn) FetchMrc20DeployList(tick string, completed, order, orderType string, offset, size int64) (*mrc20_utxo_service.Mrc20DeployListResponse, error) {
	//t := tool.MakeTimestamp()
	client := mrc20_utxo_service.NewMrc20UtxoServiceClient(c.clientConn)
	if client == nil {
		return nil, errors.New("grpc client connect err")
	}

	req := &mrc20_utxo_service.TickListRequest{
		Tick:      tick,
		Completed: completed,
		Order:     order,
		OrderType: orderType,
		Offset:    offset,
		Size:      size,
	}
	resp, err := client.FetchMrc20DeployList(context.Background(), req)
	if err != nil {
		fmt.Println(err)
	}

	//fmt.Printf("[GRPC] :%d\n", tool.MakeTimestamp()-t)
	defer c.clientConn.Close()
	return resp, err
}
