package common

import (
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
)

// Test vector from the keeper's public derivation sheet
// (idcoin-lp-public-derivation.json): account m/84'/0'/1000', child path =
// UTF-8 bytes of UPPER(tick) non-hardened.
func TestDeriveIdCoinLpVaultChildVectors(t *testing.T) {
	const xpub = "xpub6DDLXRDx6k34rNYcujmKp1xb92YSU4uAkEs1rhT6C3ytFbjU1YMaSt4gLQoB9cK9TTMRNrVVEZogsbPVxNQwui5TjKk7BMZrBwdBJdYktdg"
	vectors := map[string]string{
		"SUNNY":  "bc1qg0lt0nc4uja6fyhp8f0n2lfupd3hxld3vfeayf",
		"METAID": "bc1qlz9dfj0d3p8jl8870sdyhlnryhc9e7t37d3djd",
	}
	for tick, want := range vectors {
		pubHex, addr, err := DeriveIdCoinLpVaultChild(xpub, tick, &chaincfg.MainNetParams)
		if err != nil {
			t.Fatalf("%s: %v", tick, err)
		}
		if addr != want {
			t.Fatalf("%s: address = %s, want %s", tick, addr, want)
		}
		if len(pubHex) != 66 {
			t.Fatalf("%s: compressed pubkey hex length = %d", tick, len(pubHex))
		}
	}
	// lowercase input must derive identically (UPPER rule)
	_, addrLower, err := DeriveIdCoinLpVaultChild(xpub, "sunny", &chaincfg.MainNetParams)
	if err != nil || addrLower != vectors["SUNNY"] {
		t.Fatalf("lowercase derivation mismatch: %s %v", addrLower, err)
	}
	if _, _, err := DeriveIdCoinLpVaultChild("", "SUNNY", &chaincfg.MainNetParams); err == nil {
		t.Fatal("empty xpub must fail closed")
	}
	if _, _, err := DeriveIdCoinLpVaultChild(xpub, "", &chaincfg.MainNetParams); err == nil {
		t.Fatal("empty tick must fail")
	}
}
