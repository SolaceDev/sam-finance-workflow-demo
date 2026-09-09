package main

import (
	sdk "github.com/SolaceDev/solace-agent-mesh-go/pkg/samtoolsdk"
)

func main() {
	sdk.Run(
		sdk.NewTool(
			"market_get_technicals",
			"Calculates technical momentum indicators (20/50/200 SMA, RSI 14, MACD, support/resistance) for a given equity ticker.",
			getMarketTechnicals,
		),
		sdk.NewTool(
			"fundamentals_get_financials",
			"Retrieves fundamental corporate health metrics (YoY revenue growth, margins, P/E, forward P/E, balance sheet health) for a given equity ticker.",
			getFundamentals,
		),
		sdk.NewTool(
			"prediction_get_odds",
			"Retrieves crowd-implied probability and market sentiment from Polymarket prediction markets for a given equity ticker.",
			getPredictionOdds,
		),
		sdk.NewTool(
			"prediction_search_markets",
			"Searches live Polymarket prediction markets for a query or ticker. Automatically saves the full uncurated raw response as an artifact and returns a compact curated summary to LLM context.",
			searchPredictionMarkets,
		),
		sdk.NewTool(
			"options_get_sentiment",
			"Retrieves institutional options market sentiment, 30-day implied volatility (IV30), Put/Call volume and open interest ratios from CBOE for a given equity ticker.",
			getOptionsSentiment,
		),
		sdk.NewTool(
			"portfolio_get_summary",
			"Inspects the current paper portfolio cash balance, total equity, and open positions in SQLite.",
			getPortfolioSummary,
		),
		sdk.NewTool(
			"portfolio_audit_risk",
			"Audits a proposed trade order against the 10% single-asset equity cap, cash liquidity, and 2.0 Risk/Reward ratio, returning an APPROVED, MODIFIED (downsized), or REJECTED verdict.",
			auditRisk,
		),
		sdk.NewTool(
			"portfolio_execute_trade",
			"Executes a BUY, SELL, or HOLD trade order against the SQLite paper portfolio ledger, updating positions and cash balance.",
			executeTrade,
		),
	)
}
