package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	sdk "github.com/SolaceDev/solace-agent-mesh-go/pkg/samtoolsdk"
)

type FundamentalsParams struct {
	Ticker string `json:"ticker" desc:"The equity ticker symbol (e.g. NVDA, AAPL)."`
}

type FundamentalAnalysis struct {
	Ticker                    string  `json:"ticker"`
	CompanyName               string  `json:"company_name"`
	QuarterlyRevenueGrowthYoY string  `json:"quarterly_revenue_growth_yoy"`
	GrossMargin               string  `json:"gross_margin"`
	OperatingMargin           string  `json:"operating_margin"`
	PERatio                   float64 `json:"pe_ratio"`
	ForwardPE                 float64 `json:"forward_pe"`
	EVToEBITDA                float64 `json:"ev_to_ebitda"`
	DebtToEquity              float64 `json:"debt_to_equity"`
	FreeCashFlow              string  `json:"free_cash_flow"`
	ValuationAssessment       string  `json:"valuation_assessment"`
	BalanceSheetHealth        string  `json:"balance_sheet_health"`
	MarginTrend               string  `json:"margin_trend"`
	FundamentalConviction     int     `json:"fundamental_conviction"`
	Summary                   string  `json:"summary"`
}

func getFundamentals(ctx context.Context, p FundamentalsParams, _ *sdk.ToolContext) (*sdk.Result, error) {
	ticker := strings.ToUpper(strings.TrimSpace(p.Ticker))
	if ticker == "" {
		return sdk.Error("ticker is required"), nil
	}

	fund := lookupFundamentals(ctx, ticker)

	msg := fmt.Sprintf(
		"Fundamental Analysis for %s (%s):\n"+
			"- Revenue Growth (YoY): %s\n"+
			"- Margins: Gross %s, Operating %s (%s)\n"+
			"- Valuation Multiples: Trailing P/E %.1f, Forward P/E %.1f, EV/EBITDA %.1f\n"+
			"- Debt to Equity: %.2f | Free Cash Flow: %s\n"+
			"- Balance Sheet: %s | Valuation Assessment: %s\n"+
			"- Conviction: %d/10\n"+
			"- Summary: %s",
		fund.Ticker, fund.CompanyName,
		fund.QuarterlyRevenueGrowthYoY,
		fund.GrossMargin, fund.OperatingMargin, fund.MarginTrend,
		fund.PERatio, fund.ForwardPE, fund.EVToEBITDA,
		fund.DebtToEquity, fund.FreeCashFlow,
		fund.BalanceSheetHealth, fund.ValuationAssessment,
		fund.FundamentalConviction, fund.Summary,
	)

	return sdk.OK(
		msg,
		sdk.WithData(map[string]any{
			"ticker":                       fund.Ticker,
			"company_name":                 fund.CompanyName,
			"quarterly_revenue_growth_yoy": fund.QuarterlyRevenueGrowthYoY,
			"gross_margin":                 fund.GrossMargin,
			"operating_margin":             fund.OperatingMargin,
			"pe_ratio":                     fund.PERatio,
			"forward_pe":                   fund.ForwardPE,
			"ev_to_ebitda":                 fund.EVToEBITDA,
			"debt_to_equity":               fund.DebtToEquity,
			"free_cash_flow":               fund.FreeCashFlow,
			"valuation_assessment":         fund.ValuationAssessment,
			"balance_sheet_health":         fund.BalanceSheetHealth,
			"margin_trend":                 fund.MarginTrend,
			"fundamental_conviction":       fund.FundamentalConviction,
			"summary":                      fund.Summary,
		}),
	), nil
}

func lookupFundamentals(ctx context.Context, ticker string) FundamentalAnalysis {
	switch ticker {
	case "NVDA":
		return FundamentalAnalysis{
			Ticker:                    "NVDA",
			CompanyName:               "NVIDIA Corporation",
			QuarterlyRevenueGrowthYoY: "+122%",
			GrossMargin:               "75.4%",
			OperatingMargin:           "62.1%",
			PERatio:                   54.8,
			ForwardPE:                 38.2,
			EVToEBITDA:                48.6,
			DebtToEquity:              0.15,
			FreeCashFlow:              "$14.9B",
			ValuationAssessment:       "Stretched",
			BalanceSheetHealth:        "Strong",
			MarginTrend:               "Expanding",
			FundamentalConviction:     8,
			Summary:                   "Dominant market position in AI acceleration hardware, hyper-growth datacenter revenue, and pristine balance sheet; valuation demands perfection.",
		}
	case "AAPL":
		return FundamentalAnalysis{
			Ticker:                    "AAPL",
			CompanyName:               "Apple Inc.",
			QuarterlyRevenueGrowthYoY: "+5.1%",
			GrossMargin:               "46.2%",
			OperatingMargin:           "31.4%",
			PERatio:                   33.5,
			ForwardPE:                 29.8,
			EVToEBITDA:                24.2,
			DebtToEquity:              1.45,
			FreeCashFlow:              "$28.1B",
			ValuationAssessment:       "Fair / Elevated",
			BalanceSheetHealth:        "Strong",
			MarginTrend:               "Stable",
			FundamentalConviction:     7,
			Summary:                   "Robust cash generation, fortress services ecosystem, steady buybacks, though top-line hardware growth is modest.",
		}
	case "MSFT":
		return FundamentalAnalysis{
			Ticker:                    "MSFT",
			CompanyName:               "Microsoft Corporation",
			QuarterlyRevenueGrowthYoY: "+15.3%",
			GrossMargin:               "69.8%",
			OperatingMargin:           "44.6%",
			PERatio:                   36.2,
			ForwardPE:                 31.0,
			EVToEBITDA:                23.4,
			DebtToEquity:              0.38,
			FreeCashFlow:              "$23.3B",
			ValuationAssessment:       "Fair",
			BalanceSheetHealth:        "Strong",
			MarginTrend:               "Stable",
			FundamentalConviction:     8,
			Summary:                   "Azure cloud momentum accelerating, enterprise copilot adoption expanding, solid operating cash flows.",
		}
	case "SHOP":
		return FundamentalAnalysis{
			Ticker:                    "SHOP",
			CompanyName:               "Shopify Inc.",
			QuarterlyRevenueGrowthYoY: "+21.4%",
			GrossMargin:               "51.1%",
			OperatingMargin:           "12.4%",
			PERatio:                   58.2,
			ForwardPE:                 44.5,
			EVToEBITDA:                32.1,
			DebtToEquity:              0.12,
			FreeCashFlow:              "$421M",
			ValuationAssessment:       "Premium / Growth Justified",
			BalanceSheetHealth:        "Strong",
			MarginTrend:               "Expanding post-logistics divestiture",
			FundamentalConviction:     8,
			Summary:                   "Free cash flow inflection following logistics exit, solid merchant solutions take-rate growth, and low balance sheet leverage.",
		}
	case "TSLA":
		return FundamentalAnalysis{
			Ticker:                    "TSLA",
			CompanyName:               "Tesla, Inc.",
			QuarterlyRevenueGrowthYoY: "+2.3%",
			GrossMargin:               "18.0%",
			OperatingMargin:           "6.3%",
			PERatio:                   62.0,
			ForwardPE:                 74.5,
			EVToEBITDA:                38.5,
			DebtToEquity:              0.10,
			FreeCashFlow:              "$1.3B",
			ValuationAssessment:       "Stretched",
			BalanceSheetHealth:        "Moderate",
			MarginTrend:               "Compressing",
			FundamentalConviction:     4,
			Summary:                   "Automotive price competition impacting gross margins; valuation increasingly reliant on autonomous FSD and energy storage options.",
		}
	default:
		name := fetchCompanyName(ctx, ticker)
		if name == "" {
			name = fmt.Sprintf("%s Corp", ticker)
		}
		return FundamentalAnalysis{
			Ticker:                    ticker,
			CompanyName:               name,
			QuarterlyRevenueGrowthYoY: "+8.0%",
			GrossMargin:               "45.0%",
			OperatingMargin:           "18.0%",
			PERatio:                   22.0,
			ForwardPE:                 19.0,
			EVToEBITDA:                14.0,
			DebtToEquity:              0.50,
			FreeCashFlow:              "$1.2B",
			ValuationAssessment:       "Fair",
			BalanceSheetHealth:        "Stable",
			MarginTrend:               "Stable",
			FundamentalConviction:     6,
			Summary:                   fmt.Sprintf("Standard baseline fundamentals for %s (%s) with balanced leverage and market-average multiples.", name, ticker),
		}
	}
}

func fetchCompanyName(ctx context.Context, ticker string) string {
	apiURL := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?interval=1d&range=1d", url.PathEscape(ticker))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	var parsed struct {
		Chart struct {
			Result []struct {
				Meta struct {
					LongName  string `json:"longName"`
					ShortName string `json:"shortName"`
				} `json:"meta"`
			} `json:"result"`
		} `json:"chart"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil && len(parsed.Chart.Result) > 0 {
		meta := parsed.Chart.Result[0].Meta
		if meta.LongName != "" {
			return meta.LongName
		}
		return meta.ShortName
	}
	return ""
}
