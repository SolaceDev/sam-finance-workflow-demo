package main

import (
	"context"
	"strings"
	"testing"

	sdk "github.com/SolaceDev/solace-agent-mesh-go/pkg/samtoolsdk"
)

func TestSearchPredictionMarkets_ArtifactCreationAndCuration(t *testing.T) {
	ctx := context.Background()
	query := "Amazon"
	params := PredictionSearchParams{
		Query: &query,
	}

	res, err := searchPredictionMarkets(ctx, params, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Status != "success" {
		t.Fatalf("expected status 'success', got %q: %s", res.Status, res.Message)
	}

	// 1. Verify that raw API response was packaged as an artifact DataObject
	if len(res.DataObjects) != 1 {
		t.Fatalf("expected exactly 1 DataObject (raw artifact), got %d", len(res.DataObjects))
	}

	artifact := res.DataObjects[0]
	if artifact.Name != "polymarket_raw_search_amazon.json" {
		t.Errorf("expected artifact name 'polymarket_raw_search_amazon.json', got %q", artifact.Name)
	}

	if artifact.MIMEType != "application/json" {
		t.Errorf("expected MIMEType 'application/json', got %q", artifact.MIMEType)
	}

	if artifact.Disposition != sdk.DispositionArtifact {
		t.Errorf("expected Disposition 'artifact', got %q", artifact.Disposition)
	}

	if len(artifact.Content) == 0 {
		t.Errorf("expected non-empty artifact content")
	}

	// 2. Verify curated data returned to LLM
	data := res.Data
	if data == nil {
		t.Fatalf("expected non-nil res.Data")
	}

	rawArtifactFile, ok := data["raw_artifact_file"].(string)
	if !ok || rawArtifactFile != artifact.Name {
		t.Errorf("expected data.raw_artifact_file to match artifact name %q, got %v", artifact.Name, data["raw_artifact_file"])
	}

	topMarkets, ok := data["top_markets"].([]MarketContract)
	if !ok {
		t.Fatalf("expected top_markets to be []MarketContract, got %T", data["top_markets"])
	}

	if len(topMarkets) == 0 {
		t.Errorf("expected at least 1 curated active market for Amazon")
	}

	if len(topMarkets) > 5 {
		t.Errorf("expected curated markets count <= 5, got %d", len(topMarkets))
	}

	// 3. Verify clean message for LLM
	if !strings.Contains(res.Message, "Polymarket Crowd Prediction Summary") {
		t.Errorf("expected message to contain summary header, got %q", res.Message)
	}
}

func TestSearchPredictionMarkets_EmptyQueryAndTicker(t *testing.T) {
	ctx := context.Background()
	params := PredictionSearchParams{}

	res, err := searchPredictionMarkets(ctx, params, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Status != "error" {
		t.Errorf("expected status 'error' for empty search params, got %q", res.Status)
	}
}
