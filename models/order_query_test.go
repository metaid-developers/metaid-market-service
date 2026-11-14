package models

import (
	"reflect"
	"strings"
	"testing"
)

func TestMarketOrderFilterUsesBoundParameters(t *testing.T) {
	address := "x' OR 1=1 --"
	path := "/protocols/%' OR 1=1 --"
	filter, args := buildMarketOrderFilter(&MarketOrderModel{OrderState: OrderStateFinish}, address, path, []string{"a', 'b"})
	if strings.Contains(filter, address) || strings.Contains(filter, path) {
		t.Fatalf("user input was interpolated into SQL: %s", filter)
	}
	wantArgs := []interface{}{address, address, path + "%", "a', 'b"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("unexpected args: %#v", args)
	}
	if filter != "(sellerAddress = ? OR buyerAddress = ?) AND assetPath LIKE ? AND assetId NOT IN (?)" {
		t.Fatalf("unexpected filter: %s", filter)
	}
}

func TestMrc20AddressFilterUsesBoundParameters(t *testing.T) {
	address := "x' OR 1=1 --"
	filter, args := buildMrc20AddressFilter(&MarketMrc20OrderModel{OrderState: OrderStateFinish}, address)
	if filter != "(sellerAddress = ? OR buyerAddress = ?)" {
		t.Fatalf("unexpected filter: %s", filter)
	}
	if len(args) != 2 || args[0] != address || args[1] != address {
		t.Fatalf("unexpected args: %#v", args)
	}
}
