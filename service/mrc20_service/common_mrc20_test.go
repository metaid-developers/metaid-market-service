package mrc20_service

import (
	"encoding/json"
	"testing"
)

func TestMakeTransferPayloadPrecisionAndValidation(t *testing.T) {
	inputs := []*TransferMrc20{
		{Mrc20Amount: "0.05"},
		{Mrc20Amount: "0.10"},
	}
	outs := []*Mrc20OutInfo{
		{Amount: "0.05", Address: "addr1"},
		{Amount: "0.10", Address: "addr2"},
	}
	payload, err := MakeTransferPayload("tick-1", inputs, outs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var items []Mrc20DataItem
	if err := json.Unmarshal([]byte(payload), &items); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(items) != 2 || items[0].Amount != "0.05" || items[1].Amount != "0.10" {
		t.Fatalf("unexpected payload items: %+v", items)
	}

	// Output exceeding input should fail
	if _, err := MakeTransferPayload("tick-1", inputs, []*Mrc20OutInfo{{Amount: "0.16"}}); err == nil {
		t.Fatal("expected error when output exceeds input amount")
	}

	// Non-positive or invalid output should fail
	if _, err := MakeTransferPayload("tick-1", inputs, []*Mrc20OutInfo{{Amount: "0"}}); err == nil {
		t.Fatal("expected error for zero output amount")
	}
	if _, err := MakeTransferPayload("tick-1", inputs, []*Mrc20OutInfo{{Amount: "-0.01"}}); err == nil {
		t.Fatal("expected error for negative output amount")
	}
}
