package order_service

import (
	"errors"
	"testing"

	"github.com/shopspring/decimal"
)

func TestParseMrc20DisplayAmount(t *testing.T) {
	cases := []struct {
		name       string
		raw        string
		decimals   int64
		wantAmount int64
		wantStr    string
		wantErr    bool
	}{
		{name: "fractional 0.05 with 8 decimals", raw: "0.05", decimals: 8, wantAmount: 0, wantStr: "0.05", wantErr: false},
		{name: "integer and fraction", raw: "123.45", decimals: 2, wantAmount: 123, wantStr: "123.45", wantErr: false},
		{name: "trailing zero beyond decimals allowed if value fits", raw: "0.050000000", decimals: 8, wantAmount: 0, wantStr: "0.05", wantErr: false},
		{name: "whitespace trimmed", raw: "  10.25 ", decimals: 8, wantAmount: 10, wantStr: "10.25", wantErr: false},
		{name: "exceeds decimals", raw: "0.000000001", decimals: 8, wantErr: true},
		{name: "zero amount", raw: "0", decimals: 8, wantErr: true},
		{name: "negative amount", raw: "-1.5", decimals: 8, wantErr: true},
		{name: "empty amount", raw: "", decimals: 8, wantErr: true},
		{name: "invalid string", raw: "abc", decimals: 8, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			amount, dec, err := ParseMrc20DisplayAmount(tc.raw, tc.decimals)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q (decimals=%d), got amount=%d dec=%s", tc.raw, tc.decimals, amount, dec.String())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.raw, err)
			}
			if amount != tc.wantAmount {
				t.Fatalf("expected integer amount %d, got %d", tc.wantAmount, amount)
			}
			if dec.String() != tc.wantStr {
				t.Fatalf("expected decimal %s, got %s", tc.wantStr, dec.String())
			}
		})
	}
}

func TestFormatTokenPriceRate(t *testing.T) {
	if got := FormatTokenPriceRate(decimal.NewFromInt(2000).Div(decimal.RequireFromString("0.05"))); got != "40000.000000" {
		t.Fatalf("expected 40000.000000, got %s", got)
	}
	if got := FormatTokenPriceRate(decimal.NewFromInt(2000).Div(decimal.NewFromInt(1000000000))); got != "0.000002" {
		t.Fatalf("expected 0.000002, got %s", got)
	}
	// Very small prices < 1e-6 must NOT collapse to 0.000000
	if got := FormatTokenPriceRate(decimal.NewFromInt(2000).Div(decimal.NewFromInt(10000000000))); got != "0.0000002" {
		t.Fatalf("expected 0.0000002, got %s", got)
	}
	if got := FormatTokenPriceRate(decimal.NewFromInt(2000).Div(decimal.NewFromInt(10000000000000))); got != "0.0000000002" {
		t.Fatalf("expected 0.0000000002, got %s", got)
	}
	if got := FormatTokenPriceRate(decimal.Zero); got != "0.000000" {
		t.Fatalf("expected 0.000000 for zero rate, got %s", got)
	}
}

func TestFormatChange24H(t *testing.T) {
	if got := FormatChange24H(0); got != "+0.00%" {
		t.Fatalf("expected +0.00%%, got %s", got)
	}
	if got := FormatChange24H(1234); got != "+12.34%" {
		t.Fatalf("expected +12.34%%, got %s", got)
	}
	if got := FormatChange24H(-1234); got != "-12.34%" {
		t.Fatalf("expected -12.34%%, got %s", got)
	}
}

func TestBroadcastRejectionClassification(t *testing.T) {
	dustErr := errors.New("rpc error: code = Unknown desc = [-26]dust")
	if !IsDustBroadcastRejection(dustErr) {
		t.Fatal("expected [-26]dust to be classified as dust rejection")
	}
	if !IsDeterministicBroadcastRejection(dustErr) {
		t.Fatal("expected [-26]dust to be classified as deterministic rejection")
	}

	deterministicErrs := []error{
		errors.New("bad-txns-inputs-missingorspent"),
		errors.New("txn-mempool-conflict"),
		errors.New("Missing inputs"),
		errors.New("min relay fee not met"),
		errors.New("insufficient fee"),
	}
	for _, err := range deterministicErrs {
		if !IsDeterministicBroadcastRejection(err) {
			t.Fatalf("expected %q to be deterministic rejection", err)
		}
	}

	nonDeterministicErrs := []error{
		nil,
		errors.New("txn-already-known"),
		errors.New("Transaction already in block chain"),
		errors.New("context deadline exceeded"),
		errors.New("connection refused"),
	}
	for _, err := range nonDeterministicErrs {
		if IsDeterministicBroadcastRejection(err) {
			t.Fatalf("did not expect %v to be deterministic rejection", err)
		}
	}
}
