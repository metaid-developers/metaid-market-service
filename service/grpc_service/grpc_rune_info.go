package grpc_service

import (
	"context"
	"errors"
	"fmt"
	"google.golang.org/grpc"
	"metaid-market-service/conf"
	"metaid-market-service/protobuf/etching_info_service"
	"metaid-market-service/protobuf/rune_utxo_service"

	"time"
)

type RuneBaseConn struct {
	clientConn  *grpc.ClientConn
	callTimeOut time.Duration
}

func GetRuneBaseConn() (*RuneBaseConn, error) {
	c := &RuneBaseConn{
		callTimeOut: 3 * time.Second,
	}
	connAddress := conf.GrpcAssetBaseAddress

	err := c.dial(connAddress)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *RuneBaseConn) dial(addr string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.callTimeOut)
	defer cancel()

	var err error
	c.clientConn, err = grpc.DialContext(ctx, addr, grpc.WithInsecure())
	if err != nil {
		fmt.Printf("dial rpc error:%s", err.Error())
	}
	return nil
}

func (c *RuneBaseConn) FetchEtchingInfo(runeId string) (*etching_info_service.EtchingInfoResponse, error) {
	//t := tool.MakeTimestamp()
	client := etching_info_service.NewEtchingServiceClient(c.clientConn)
	if client == nil {
		return nil, errors.New("grpc client connect err")
	}

	req := &etching_info_service.EtchingInfoRequest{
		RuneId: runeId,
	}
	resp, err := client.FetchEtchingInfo(context.Background(), req)
	if err != nil {
		fmt.Println(err)
	}

	//fmt.Printf("[GRPC] :%d\n", tool.MakeTimestamp()-t)
	defer c.clientConn.Close()
	return resp, err
}

func (c *RuneBaseConn) SearchEtchingList(spacedRuneLike, sort, complete string, offset, limit int64) (*etching_info_service.EtchingInfoListResponse, error) {
	//t := tool.MakeTimestamp()
	client := etching_info_service.NewEtchingServiceClient(c.clientConn)
	if client == nil {
		return nil, errors.New("grpc client connect err")
	}

	req := &etching_info_service.SearchEtchingListRequest{
		SpacedRune: spacedRuneLike,
		Sort:       sort,
		Complete:   complete,
		Offset:     offset,
		Limit:      limit,
	}
	resp, err := client.SearchEtchingList(context.Background(), req)
	if err != nil {
		fmt.Println(err)
	}

	//fmt.Printf("[GRPC] :%d\n", tool.MakeTimestamp()-t)
	defer c.clientConn.Close()
	return resp, err
}

func (c *RuneBaseConn) FetchAddressRuneBalanceList(address string, offset, limit int64) (*rune_utxo_service.RuneBalanceInfoListResponse, error) {
	//t := tool.MakeTimestamp()
	client := rune_utxo_service.NewRuneUtxoServiceClient(c.clientConn)
	if client == nil {
		return nil, errors.New("grpc client connect err")
	}

	req := &rune_utxo_service.AddressRuneBalanceInfoListRequest{
		Address: address,
		Offset:  offset,
		Limit:   limit,
	}
	resp, err := client.FetchAddressRuneBalanceList(context.Background(), req)
	if err != nil {
		fmt.Println(err)
	}

	//fmt.Printf("[GRPC] :%d\n", tool.MakeTimestamp()-t)
	defer c.clientConn.Close()
	return resp, err
}

func (c *RuneBaseConn) FetchAddressRuneBalanceInfo(address, runeId string) (*rune_utxo_service.RuneBalanceInfoResponse, error) {
	//t := tool.MakeTimestamp()
	client := rune_utxo_service.NewRuneUtxoServiceClient(c.clientConn)
	if client == nil {
		return nil, errors.New("grpc client connect err")
	}

	req := &rune_utxo_service.AddressRuneBalanceInfoRequest{
		Address: address,
		RuneId:  runeId,
	}
	resp, err := client.FetchAddressRuneBalanceInfo(context.Background(), req)
	if err != nil {
		fmt.Println(err)
	}

	//fmt.Printf("[GRPC] :%d\n", tool.MakeTimestamp()-t)
	defer c.clientConn.Close()
	return resp, err
}

func (c *RuneBaseConn) FetchRuneUtxoInfo(txId string, index, offset, limit int64) (*rune_utxo_service.RuneUtxoInfoListResponse, error) {
	//t := tool.MakeTimestamp()
	client := rune_utxo_service.NewRuneUtxoServiceClient(c.clientConn)
	if client == nil {
		return nil, errors.New("grpc client connect err")
	}

	req := &rune_utxo_service.RuneUtxoRequest{
		TxId:   txId,
		Index:  index,
		Offset: offset,
		Limit:  limit,
	}
	resp, err := client.FetchRuneUtxoInfo(context.Background(), req)
	if err != nil {
		fmt.Println(err)
	}

	//fmt.Printf("[GRPC] :%d\n", tool.MakeTimestamp()-t)
	defer c.clientConn.Close()
	return resp, err
}

func (c *RuneBaseConn) FetchAddressRuneUtxoList(address, runeId string, offset, limit int64) (*rune_utxo_service.RuneUtxoListResponse, error) {
	//t := tool.MakeTimestamp()
	client := rune_utxo_service.NewRuneUtxoServiceClient(c.clientConn)
	if client == nil {
		return nil, errors.New("grpc client connect err")
	}

	req := &rune_utxo_service.AddressRuneUtxoRequest{
		Address: address,
		RuneId:  runeId,
		Offset:  offset,
		Limit:   limit,
	}
	resp, err := client.FetchAddressRuneUtxoList(context.Background(), req)
	if err != nil {
		fmt.Println(err)
	}

	//fmt.Printf("[GRPC] :%d\n", tool.MakeTimestamp()-t)
	defer c.clientConn.Close()
	return resp, err
}

func (c *RuneBaseConn) FetchRuneUtxoCheck(txId string, index int64) (*rune_utxo_service.RuneUtxoCheckResponse, error) {
	//t := tool.MakeTimestamp()
	client := rune_utxo_service.NewRuneUtxoServiceClient(c.clientConn)
	if client == nil {
		return nil, errors.New("grpc client connect err")
	}

	req := &rune_utxo_service.RuneUtxoCheckRequest{
		TxId: txId,
		Vout: index,
	}
	resp, err := client.FetchRuneUtxoCheck(context.Background(), req)
	if err != nil {
		fmt.Println(err)
	}

	//fmt.Printf("[GRPC] :%d\n", tool.MakeTimestamp()-t)
	defer c.clientConn.Close()
	return resp, err
}
