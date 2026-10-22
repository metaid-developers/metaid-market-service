package common

import "testing"

func TestIdCoinsStaticList(t *testing.T) {
	if len(IdCoinsTickIdList()) != 32 {
		t.Fatalf("expected 32 id-coins, got %d", len(IdCoinsTickIdList()))
	}
	if !IsIdCoinsTickId("8ce590918a3d493631ab9d9e3bbc89e322f3dd08354af87f97c07218818b78f4i0") {
		t.Error("METAID tickId should be in the id-coins list")
	}
	if !IsIdCoinsTick("metaid") {
		t.Error("tick matching should be case-insensitive")
	}
	if !IsIdCoinsTick("MetaWeb") {
		t.Error("MetaWeb should match with its original casing")
	}
	if IsIdCoinsTickId("nonexistenti0") {
		t.Error("unknown tickId must not match")
	}
	if IsIdCoinsTick("nonexistent") {
		t.Error("unknown tick must not match")
	}
}
