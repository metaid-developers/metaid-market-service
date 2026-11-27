package common_service

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestFormatUsdAmountAndChange24H(t *testing.T) {
	if got := formatUsdAmount(decimal.RequireFromString("12.3456")); got != "12.346" {
		t.Fatalf("expected 12.346, got %s", got)
	}
	if got := formatUsdAmount(decimal.RequireFromString("0.00123")); got != "0.001" {
		t.Fatalf("expected 0.001, got %s", got)
	}
	if got := formatUsdAmount(decimal.RequireFromString("0.00005")); got != "0.00005" {
		t.Fatalf("expected 0.00005, got %s", got)
	}
	if got := formatUsdAmount(decimal.Zero); got != "0.00" {
		t.Fatalf("expected 0.00, got %s", got)
	}

	if got := formatMarketChange24H(0); got != "+0.00%" {
		t.Fatalf("expected +0.00%%, got %s", got)
	}
	if got := formatMarketChange24H(525); got != "+5.25%" {
		t.Fatalf("expected +5.25%%, got %s", got)
	}
	if got := formatMarketChange24H(-525); got != "-5.25%" {
		t.Fatalf("expected -5.25%%, got %s", got)
	}
}
