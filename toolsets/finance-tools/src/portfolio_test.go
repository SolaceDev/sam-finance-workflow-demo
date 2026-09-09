package main

import (
	"context"
	"path/filepath"
	"testing"
)

func setupTestDB(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_portfolio.db")
	t.Setenv("PORTFOLIO_DB_PATH", dbPath)
}

func TestAuditRisk_Approved(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()

	// Initial portfolio is $100k cash, $100k equity. 10% cap = $10,000.
	// Buy 30 shares @ $200 = $6,000 (< $10k cap and < $100k cash).
	// Stop loss = $190 (Risk = $10), Take Profit = $230 (Reward = $30). R/R = 3.0 : 1.
	stopLoss := 190.0
	takeProfit := 230.0
	params := PortfolioAuditRiskParams{
		Ticker:     "NVDA",
		Action:     "BUY",
		Shares:     30,
		FillPrice:  200.0,
		StopLoss:   &stopLoss,
		TakeProfit: &takeProfit,
	}

	res, err := auditRisk(ctx, params, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != "success" {
		t.Fatalf("expected success, got %s: %s", res.Status, res.Message)
	}

	data := res.Data
	if data["verdict"] != "APPROVED" {
		t.Errorf("expected verdict APPROVED, got %v", data["verdict"])
	}
	if data["approved_shares"] != 30 {
		t.Errorf("expected 30 approved_shares, got %v", data["approved_shares"])
	}
	if data["effective_action"] != "BUY" {
		t.Errorf("expected effective_action BUY, got %v", data["effective_action"])
	}
}

func TestAuditRisk_Modified_Exceeds10PctCap(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()

	// Buy 100 shares @ $200 = $20,000 (> $10k cap).
	// Max allowed shares for $10,000 cap @ $200 = 50 shares.
	stopLoss := 190.0
	takeProfit := 230.0
	params := PortfolioAuditRiskParams{
		Ticker:     "NVDA",
		Action:     "BUY",
		Shares:     100,
		FillPrice:  200.0,
		StopLoss:   &stopLoss,
		TakeProfit: &takeProfit,
	}

	res, err := auditRisk(ctx, params, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data := res.Data
	if data["verdict"] != "MODIFIED" {
		t.Errorf("expected verdict MODIFIED, got %v", data["verdict"])
	}
	if data["approved_shares"] != 50 {
		t.Errorf("expected 50 approved_shares, got %v", data["approved_shares"])
	}
	if data["effective_action"] != "BUY" {
		t.Errorf("expected effective_action BUY, got %v", data["effective_action"])
	}
}

func TestAuditRisk_Rejected_PoorRiskReward(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()

	// Stop loss = $190 (Risk = $10), Take Profit = $205 (Reward = $5). R/R = 0.5 : 1 (< 2.0).
	stopLoss := 190.0
	takeProfit := 205.0
	params := PortfolioAuditRiskParams{
		Ticker:     "NVDA",
		Action:     "BUY",
		Shares:     20,
		FillPrice:  200.0,
		StopLoss:   &stopLoss,
		TakeProfit: &takeProfit,
	}

	res, err := auditRisk(ctx, params, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data := res.Data
	if data["verdict"] != "REJECTED" {
		t.Errorf("expected verdict REJECTED, got %v", data["verdict"])
	}
	if data["approved_shares"] != 0 {
		t.Errorf("expected 0 approved_shares, got %v", data["approved_shares"])
	}
	if data["effective_action"] != "HOLD" {
		t.Errorf("expected effective_action HOLD, got %v", data["effective_action"])
	}
}

func TestAuditRisk_Rejected_MissingStopLoss(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()

	params := PortfolioAuditRiskParams{
		Ticker:    "NVDA",
		Action:    "BUY",
		Shares:    20,
		FillPrice: 200.0,
	}

	res, err := auditRisk(ctx, params, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data := res.Data
	if data["verdict"] != "REJECTED" {
		t.Errorf("expected verdict REJECTED, got %v", data["verdict"])
	}
	if data["approved_shares"] != 0 {
		t.Errorf("expected 0 approved_shares, got %v", data["approved_shares"])
	}
}
