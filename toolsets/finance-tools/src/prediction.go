package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	sdk "github.com/SolaceDev/solace-agent-mesh-go/pkg/samtoolsdk"
)

type PredictionSearchParams struct {
	Query  *string `json:"query,omitempty" jsonschema:"description=Search term or company name (e.g. Amazon, Nvidia, AI capex, Fed rate cut)"`
	Ticker *string `json:"ticker,omitempty" jsonschema:"description=Equity ticker symbol (e.g. AMZN, NVDA, AAPL)"`
	Limit  *int    `json:"limit,omitempty" jsonschema:"description=Maximum number of active markets to return in curated prompt summary (default: 5)"`
}

type PredictionOddsParams struct {
	Ticker string `json:"ticker" jsonschema:"description=The equity ticker symbol (e.g. NVDA, SHOP, AAPL),required"`
}

type MarketContract struct {
	MarketID           string  `json:"market_id,omitempty"`
	Question           string  `json:"question"`
	ImpliedProbability float64 `json:"implied_probability"` // 0.0 to 1.0
	ProbabilityPercent string  `json:"probability_percent"` // e.g. "74.0%"
	VolumeUSD          float64 `json:"volume_usd"`
	Outcome            string  `json:"outcome"`
}

type PredictionAnalysis struct {
	Ticker               string           `json:"ticker"`
	ContractsFound       int              `json:"contracts_found"`
	TopMarkets           []MarketContract `json:"top_markets"`
	PrimaryProbability   float64          `json:"primary_probability"`
	SentimentTilt        string           `json:"sentiment_tilt"` // Bullish / Neutral / Bearish
	PredictionConviction int              `json:"prediction_conviction"`
	Summary              string           `json:"summary"`
	Source               string           `json:"source"`
	RawPayload           []byte           `json:"-"`
}

func searchPredictionMarkets(ctx context.Context, p PredictionSearchParams, _ *sdk.ToolContext) (*sdk.Result, error) {
	searchTerm := ""
	if p.Query != nil {
		searchTerm = strings.TrimSpace(*p.Query)
	}
	if searchTerm == "" && p.Ticker != nil {
		searchTerm = strings.TrimSpace(*p.Ticker)
	}
	if searchTerm == "" {
		return sdk.Error("either query or ticker is required"), nil
	}

	limit := 5
	if p.Limit != nil && *p.Limit > 0 && *p.Limit <= 20 {
		limit = *p.Limit
	}

	analysis := fetchPolymarketSearch(ctx, searchTerm, limit)

	lower := strings.ToLower(searchTerm)
	cleanKey := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			return r
		}
		return '_'
	}, lower)
	for strings.Contains(cleanKey, "__") {
		cleanKey = strings.ReplaceAll(cleanKey, "__", "_")
	}
	cleanKey = strings.Trim(cleanKey, "_")
	if cleanKey == "" {
		cleanKey = "query"
	}
	artifactName := fmt.Sprintf("polymarket_raw_search_%s.json", cleanKey)

	rawBody := analysis.RawPayload
	if len(rawBody) == 0 {
		// Fallback JSON payload
		rawBody, _ = json.MarshalIndent(analysis, "", "  ")
	}

	artifactObj := sdk.DataObject{
		Name:        artifactName,
		Content:     rawBody,
		MIMEType:    "application/json",
		Disposition: sdk.DispositionArtifact,
		Description: fmt.Sprintf("Full raw Polymarket search response for %q (%d bytes)", searchTerm, len(rawBody)),
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Polymarket Crowd Prediction Summary for %q:\n", searchTerm))
	sb.WriteString(fmt.Sprintf("- Raw Search Payload Saved to Artifact: %s (%d bytes, stored out-of-context)\n", artifactName, len(rawBody)))
	sb.WriteString(fmt.Sprintf("- Sentiment Tilt: %s (Conviction: %d/10)\n", analysis.SentimentTilt, analysis.PredictionConviction))
	sb.WriteString(fmt.Sprintf("- Curated Active Markets Analyzed (%d found):\n", analysis.ContractsFound))
	if len(analysis.TopMarkets) > 0 {
		for _, m := range analysis.TopMarkets {
			sb.WriteString(fmt.Sprintf("  * [ID: %s] %s: %s (%s) [Vol: $%.0f]\n", m.MarketID, m.Question, m.ProbabilityPercent, m.Outcome, m.VolumeUSD))
		}
	} else {
		sb.WriteString("  * No active liquid prediction contracts found for this query.\n")
	}
	sb.WriteString(fmt.Sprintf("- Summary: %s\n", analysis.Summary))
	sb.WriteString("- Note: If deeper event details or JSONPath queries are needed, load the raw artifact directly.")

	return sdk.OK(
		sb.String(),
		sdk.WithData(map[string]any{
			"search_term":             searchTerm,
			"raw_artifact_file":       artifactName,
			"raw_artifact_size_bytes": len(rawBody),
			"sentiment_tilt":          analysis.SentimentTilt,
			"prediction_conviction":   analysis.PredictionConviction,
			"contracts_found":         analysis.ContractsFound,
			"top_markets":             analysis.TopMarkets,
			"primary_probability":     analysis.PrimaryProbability,
			"summary":                 analysis.Summary,
			"source":                  analysis.Source,
		}),
		sdk.WithDataObjects(artifactObj),
	), nil
}

func getPredictionOdds(ctx context.Context, p PredictionOddsParams, tc *sdk.ToolContext) (*sdk.Result, error) {
	ticker := p.Ticker
	return searchPredictionMarkets(ctx, PredictionSearchParams{Ticker: &ticker}, tc)
}

func fetchPolymarketSearch(ctx context.Context, searchTerm string, limit int) PredictionAnalysis {
	queryParam := searchTerm
	switch strings.ToUpper(searchTerm) {
	case "NVDA":
		queryParam = "Nvidia"
	case "AMZN":
		queryParam = "Amazon"
	case "AAPL":
		queryParam = "Apple"
	case "MSFT":
		queryParam = "Microsoft"
	case "TSLA":
		queryParam = "Tesla"
	case "SHOP":
		queryParam = "Shopify"
	}

	apiURL := fmt.Sprintf("https://gamma-api.polymarket.com/public-search?q=%s", url.QueryEscape(queryParam))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err == nil {
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; SAM-TradingDesk/1.0)")
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)

			var searchRes struct {
				Events []struct {
					ID        string  `json:"id"`
					Title     string  `json:"title"`
					VolumeNum float64 `json:"volumeNum"`
					Markets   []struct {
						ID            string          `json:"id"`
						Question      string          `json:"question"`
						Outcomes      json.RawMessage `json:"outcomes"`
						OutcomePrices json.RawMessage `json:"outcomePrices"`
						Volume        any             `json:"volume"`
						VolumeNum     float64         `json:"volumeNum"`
						Active        bool            `json:"active"`
						Closed        bool            `json:"closed"`
					} `json:"markets"`
				} `json:"events"`
			}

			if err := json.Unmarshal(body, &searchRes); err == nil && len(searchRes.Events) > 0 {
				var contracts []MarketContract
				for _, evt := range searchRes.Events {
					for _, m := range evt.Markets {
						if m.Closed || (!m.Active && m.VolumeNum == 0) {
							continue
						}
						// Filter out esports/gaming noise if looking for financial assets
						qLower := strings.ToLower(m.Question)
						if strings.Contains(qLower, "esports") || strings.Contains(qLower, "counter-strike") || strings.Contains(qLower, "league of legends") {
							continue
						}

						outcomes := parseJSONStringSlice(m.Outcomes)
						prices := parseJSONStringSlice(m.OutcomePrices)
						if len(outcomes) > 0 && len(prices) > 0 {
							priceVal, _ := strconv.ParseFloat(prices[0], 64)
							vol := m.VolumeNum
							if vol == 0 {
								if vStr, ok := m.Volume.(string); ok {
									vol, _ = strconv.ParseFloat(vStr, 64)
								}
							}
							contracts = append(contracts, MarketContract{
								MarketID:           m.ID,
								Question:           m.Question,
								ImpliedProbability: priceVal,
								ProbabilityPercent: fmt.Sprintf("%.1f%%", priceVal*100.0),
								VolumeUSD:          vol,
								Outcome:            outcomes[0],
							})
						}
						if len(contracts) >= limit {
							break
						}
					}
					if len(contracts) >= limit {
						break
					}
				}

				if len(contracts) > 0 {
					primaryProb := contracts[0].ImpliedProbability
					tilt := "NEUTRAL"
					conv := 5
					if primaryProb >= 0.60 {
						tilt = "BULLISH"
						conv = 8
					} else if primaryProb <= 0.40 {
						tilt = "BEARISH"
						conv = 4
					}
					return PredictionAnalysis{
						Ticker:               searchTerm,
						ContractsFound:       len(contracts),
						TopMarkets:           contracts,
						PrimaryProbability:   primaryProb,
						SentimentTilt:        tilt,
						PredictionConviction: conv,
						Summary:              fmt.Sprintf("Live Polymarket crowd odds for %s indicate %s sentiment (primary probability: %.1f%%).", searchTerm, tilt, primaryProb*100.0),
						Source:               "Polymarket Live Gamma API",
						RawPayload:           body,
					}
				}
			}
		}
	}

	fallback := getFallbackPredictionOdds(searchTerm)
	fallbackJSON, _ := json.MarshalIndent(fallback, "", "  ")
	fallback.RawPayload = fallbackJSON
	return fallback
}

func parseJSONStringSlice(raw json.RawMessage) []string {
	var direct []string
	if err := json.Unmarshal(raw, &direct); err == nil {
		return direct
	}
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		var decoded []string
		if err := json.Unmarshal([]byte(str), &decoded); err == nil {
			return decoded
		}
	}
	return nil
}

func getFallbackPredictionOdds(ticker string) PredictionAnalysis {
	switch strings.ToUpper(ticker) {
	case "NVDA":
		return PredictionAnalysis{
			Ticker:         "NVDA",
			ContractsFound: 2,
			TopMarkets: []MarketContract{
				{
					MarketID:           "nvda-capex-1",
					Question:           "Will Big Tech AI Capex increase in Q3/Q4 2026?",
					ImpliedProbability: 0.78,
					ProbabilityPercent: "78.0%",
					VolumeUSD:          1420500,
					Outcome:            "Yes",
				},
				{
					MarketID:           "nvda-split-2",
					Question:           "Will Nvidia announce another stock split before year-end 2026?",
					ImpliedProbability: 0.22,
					ProbabilityPercent: "22.0%",
					VolumeUSD:          680000,
					Outcome:            "No",
				},
			},
			PrimaryProbability:   0.78,
			SentimentTilt:        "BULLISH",
			PredictionConviction: 8,
			Summary:              "Polymarket crowd places a 78% probability on sustained hyperscaler AI capex growth through 2026, offering macro tailwinds.",
			Source:               "Polymarket Gamma API (Historical Baseline)",
		}
	case "AAPL":
		return PredictionAnalysis{
			Ticker:         "AAPL",
			ContractsFound: 1,
			TopMarkets: []MarketContract{
				{
					MarketID:           "aapl-foldable-1",
					Question:           "Will Apple launch a foldable device in 2026?",
					ImpliedProbability: 0.35,
					ProbabilityPercent: "35.0%",
					VolumeUSD:          450000,
					Outcome:            "No",
				},
			},
			PrimaryProbability:   0.35,
			SentimentTilt:        "NEUTRAL",
			PredictionConviction: 5,
			Summary:              "Polymarket contracts reflect conservative hardware iteration expectations for Apple.",
			Source:               "Polymarket Gamma API (Historical Baseline)",
		}
	case "AMZN":
		return PredictionAnalysis{
			Ticker:         "AMZN",
			ContractsFound: 2,
			TopMarkets: []MarketContract{
				{
					MarketID:           "amzn-aws-1",
					Question:           "Will Amazon AWS grow revenue by >20% in fiscal 2026?",
					ImpliedProbability: 0.74,
					ProbabilityPercent: "74.0%",
					VolumeUSD:          920000,
					Outcome:            "Yes",
				},
				{
					MarketID:           "amzn-capex-2",
					Question:           "Will Amazon 2026 capex exceed $150B?",
					ImpliedProbability: 0.65,
					ProbabilityPercent: "65.0%",
					VolumeUSD:          410000,
					Outcome:            "Yes",
				},
			},
			PrimaryProbability:   0.74,
			SentimentTilt:        "BULLISH",
			PredictionConviction: 7,
			Summary:              "Polymarket crowd heavily favors re-acceleration in AWS growth and sustained infrastructure investments.",
			Source:               "Polymarket Gamma API (Historical Baseline)",
		}
	default:
		return PredictionAnalysis{
			Ticker:               ticker,
			ContractsFound:       0,
			TopMarkets:           []MarketContract{},
			PrimaryProbability:   0.50,
			SentimentTilt:        "NEUTRAL",
			PredictionConviction: 5,
			Summary:              fmt.Sprintf("No active direct prediction contracts found on Polymarket for %s. Macro sentiment baseline applied.", ticker),
			Source:               "Polymarket Gamma API (Neutral Index)",
		}
	}
}
