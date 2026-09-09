package main

import (
	"context"
	"testing"
)

func TestGetMarketTechnicals_LivePriceForCLS(t *testing.T) {
	ctx := context.Background()
	res, err := getMarketTechnicals(ctx, MarketTechnicalsParams{Ticker: "CLS"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data := res.Data
	if data == nil {
		t.Fatalf("expected res.Data not to be nil")
	}

	price, ok := data["current_price"].(float64)
	if !ok {
		t.Fatalf("expected current_price to be float64, got %T (%v)", data["current_price"], data["current_price"])
	}

	// CLS trades > $200 (currently ~$344); the static fallback default was $100.00.
	if price < 200.0 {
		t.Errorf("expected live price for CLS > 200.0, got %v (indicates static $100 fallback)", price)
	}

	sma20, _ := data["sma_20"].(float64)
	if sma20 < 200.0 {
		t.Errorf("expected live SMA20 for CLS > 200.0, got %v", sma20)
	}
}
