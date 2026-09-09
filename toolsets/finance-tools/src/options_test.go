package main

import (
	"context"
	"strings"
	"testing"
)

func TestGetOptionsSentiment_LiveNVDA(t *testing.T) {
	ctx := context.Background()
	res, err := getOptionsSentiment(ctx, OptionsSentimentParams{Ticker: "NVDA"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data := res.Data
	if data == nil {
		t.Fatalf("expected res.Data not to be nil")
	}

	ticker, _ := data["ticker"].(string)
	if ticker != "NVDA" {
		t.Errorf("expected ticker 'NVDA', got %q", ticker)
	}

	iv30, ok := data["iv30"].(float64)
	if !ok || iv30 <= 0 {
		t.Errorf("expected iv30 > 0, got %v (%T)", data["iv30"], data["iv30"])
	}

	pcVolRatio, ok := data["put_call_volume_ratio"].(float64)
	if !ok || pcVolRatio <= 0 {
		t.Errorf("expected put_call_volume_ratio > 0, got %v", data["put_call_volume_ratio"])
	}

	tilt, _ := data["sentiment_tilt"].(string)
	if !strings.Contains(tilt, "BULLISH") && !strings.Contains(tilt, "BEARISH") && tilt != "NEUTRAL" {
		t.Errorf("unexpected sentiment_tilt: %q", tilt)
	}

	regime, _ := data["volatility_regime"].(string)
	if regime == "" {
		t.Errorf("expected non-empty volatility_regime")
	}
}

func TestGetOptionsSentiment_EmptyTicker(t *testing.T) {
	ctx := context.Background()
	res, err := getOptionsSentiment(ctx, OptionsSentimentParams{Ticker: ""}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != "error" {
		t.Errorf("expected Status='error' for empty ticker, got %q", res.Status)
	}
}
