package main

import (
	"context"
	"strings"
	"testing"
)

func TestGetFundamentals_LiveCompanyNameForCLS(t *testing.T) {
	ctx := context.Background()
	res, err := getFundamentals(ctx, FundamentalsParams{Ticker: "CLS"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data := res.Data
	if data == nil {
		t.Fatalf("expected res.Data not to be nil")
	}

	name, _ := data["company_name"].(string)
	if !strings.Contains(strings.ToLower(name), "celestica") {
		t.Errorf("expected company_name to contain 'Celestica', got %q", name)
	}
}
