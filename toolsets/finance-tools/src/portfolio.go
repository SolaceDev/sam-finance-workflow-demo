package main

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	sdk "github.com/SolaceDev/solace-agent-mesh-go/pkg/samtoolsdk"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

type PortfolioSummaryParams struct{}

type PortfolioExecuteTradeParams struct {
	Ticker        string   `json:"ticker" desc:"The equity ticker symbol (e.g. NVDA)."`
	Action        string   `json:"action" desc:"Trade action: BUY, SELL, or HOLD."`
	Shares        int      `json:"shares" desc:"Number of shares to trade."`
	FillPrice     float64  `json:"fill_price" desc:"Execution fill price per share."`
	StopLoss      *float64 `json:"stop_loss,omitempty" desc:"Optional stop-loss price."`
	TakeProfit    *float64 `json:"take_profit,omitempty" desc:"Optional take-profit target price."`
	Conviction    *int     `json:"conviction,omitempty" desc:"Conviction rating 1-10."`
	BullThesis    *string  `json:"bull_thesis,omitempty" desc:"Summary of upside thesis."`
	BearThesis    *string  `json:"bear_thesis,omitempty" desc:"Summary of downside risks."`
	RiskNotes     *string  `json:"risk_notes,omitempty" desc:"Risk management audit notes."`
	CIOResolution *string  `json:"cio_resolution,omitempty" desc:"Lead CIO resolution and rationale."`
}

type PositionRecord struct {
	Symbol        string  `json:"symbol"`
	Shares        int     `json:"shares"`
	AvgCost       float64 `json:"avg_cost"`
	CurrentPrice  float64 `json:"current_price"`
	MarketValue   float64 `json:"market_value"`
	UnrealizedPnL float64 `json:"unrealized_pnl"`
}

func getDBPath() string {
	if p := os.Getenv("PORTFOLIO_DB_PATH"); p != "" {
		return p
	}
	return filepath.Join(os.TempDir(), "sam_portfolio.db")
}

func openDB() (*sql.DB, error) {
	dbPath := getDBPath()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db %q: %w", dbPath, err)
	}

	initSQL := `
	CREATE TABLE IF NOT EXISTS portfolio (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		cash_balance REAL NOT NULL,
		total_equity REAL NOT NULL,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS positions (
		symbol TEXT PRIMARY KEY,
		shares INTEGER NOT NULL,
		avg_cost REAL NOT NULL,
		current_price REAL NOT NULL,
		unrealized_pnl REAL NOT NULL,
		last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS trades (
		trade_id TEXT PRIMARY KEY,
		timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		symbol TEXT NOT NULL,
		action TEXT NOT NULL,
		shares INTEGER NOT NULL,
		fill_price REAL NOT NULL,
		total_amount REAL NOT NULL,
		stop_loss REAL,
		take_profit REAL,
		conviction_score INTEGER,
		bull_thesis TEXT,
		bear_thesis TEXT,
		risk_notes TEXT,
		cio_resolution TEXT
	);

	INSERT OR IGNORE INTO portfolio (id, cash_balance, total_equity) VALUES (1, 100000.0, 100000.0);
	`

	if _, err := db.Exec(initSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("init portfolio schema: %w", err)
	}

	return db, nil
}

func getPortfolioSummary(_ context.Context, _ PortfolioSummaryParams, _ *sdk.ToolContext) (*sdk.Result, error) {
	db, err := openDB()
	if err != nil {
		return sdk.Error(fmt.Sprintf("database error: %v", err)), nil
	}
	defer db.Close()

	var cashBalance, totalEquity float64
	row := db.QueryRow("SELECT cash_balance, total_equity FROM portfolio WHERE id = 1")
	if err := row.Scan(&cashBalance, &totalEquity); err != nil {
		return sdk.Error(fmt.Sprintf("query portfolio: %v", err)), nil
	}

	rows, err := db.Query("SELECT symbol, shares, avg_cost, current_price, unrealized_pnl FROM positions WHERE shares > 0")
	if err != nil {
		return sdk.Error(fmt.Sprintf("query positions: %v", err)), nil
	}
	defer rows.Close()

	var positions []PositionRecord
	var totalPositionValue float64
	for rows.Next() {
		var p PositionRecord
		if err := rows.Scan(&p.Symbol, &p.Shares, &p.AvgCost, &p.CurrentPrice, &p.UnrealizedPnL); err != nil {
			return sdk.Error(fmt.Sprintf("scan position: %v", err)), nil
		}
		p.MarketValue = float64(p.Shares) * p.CurrentPrice
		totalPositionValue += p.MarketValue
		positions = append(positions, p)
	}

	calculatedEquity := cashBalance + totalPositionValue
	if calculatedEquity != totalEquity {
		_, _ = db.Exec("UPDATE portfolio SET total_equity = ?, updated_at = CURRENT_TIMESTAMP WHERE id = 1", calculatedEquity)
		totalEquity = calculatedEquity
	}

	posStr := "None"
	if len(positions) > 0 {
		var parts []string
		for _, p := range positions {
			parts = append(parts, fmt.Sprintf("%s (%d shs @ $%.2f, Val: $%.2f, PnL: $%.2f)", p.Symbol, p.Shares, p.AvgCost, p.MarketValue, p.UnrealizedPnL))
		}
		posStr = strings.Join(parts, "; ")
	}

	msg := fmt.Sprintf(
		"Portfolio Ledger Summary (SQLite @ %s):\n"+
			"- Cash Balance: $%.2f\n"+
			"- Total Equity: $%.2f\n"+
			"- Positions: %s\n"+
			"- Open Position Count: %d",
		getDBPath(), cashBalance, totalEquity, posStr, len(positions),
	)

	return sdk.OK(
		msg,
		sdk.WithData(map[string]any{
			"db_path":        getDBPath(),
			"cash_balance":   cashBalance,
			"total_equity":   totalEquity,
			"positions":      positions,
			"position_count": len(positions),
		}),
	), nil
}

func executeTrade(_ context.Context, p PortfolioExecuteTradeParams, _ *sdk.ToolContext) (*sdk.Result, error) {
	ticker := strings.ToUpper(strings.TrimSpace(p.Ticker))
	if ticker == "" {
		return sdk.Error("ticker is required"), nil
	}
	action := strings.ToUpper(strings.TrimSpace(p.Action))
	if action != "BUY" && action != "SELL" && action != "HOLD" {
		return sdk.Error("action must be BUY, SELL, or HOLD"), nil
	}
	if (action == "BUY" || action == "SELL") && (p.Shares <= 0 || p.FillPrice <= 0) {
		return sdk.Error("shares and fill_price must be positive for BUY/SELL"), nil
	}

	db, err := openDB()
	if err != nil {
		return sdk.Error(fmt.Sprintf("database error: %v", err)), nil
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return sdk.Error(fmt.Sprintf("begin transaction: %v", err)), nil
	}
	defer tx.Rollback()

	var cashBalance, totalEquity float64
	row := tx.QueryRow("SELECT cash_balance, total_equity FROM portfolio WHERE id = 1")
	if err := row.Scan(&cashBalance, &totalEquity); err != nil {
		return sdk.Error(fmt.Sprintf("read portfolio: %v", err)), nil
	}

	totalAmount := float64(p.Shares) * p.FillPrice
	tradeID := "TX-" + uuid.New().String()[:8]

	switch action {
	case "BUY":
		if cashBalance < totalAmount {
			return sdk.Error(fmt.Sprintf("insufficient cash: have $%.2f, need $%.2f for %d shares of %s", cashBalance, totalAmount, p.Shares, ticker)), nil
		}
		cashBalance -= totalAmount

		var existingShares int
		var existingAvgCost float64
		pRow := tx.QueryRow("SELECT shares, avg_cost FROM positions WHERE symbol = ?", ticker)
		scanErr := pRow.Scan(&existingShares, &existingAvgCost)
		if scanErr == sql.ErrNoRows {
			_, err = tx.Exec(
				"INSERT INTO positions (symbol, shares, avg_cost, current_price, unrealized_pnl, last_updated) VALUES (?, ?, ?, ?, 0.0, CURRENT_TIMESTAMP)",
				ticker, p.Shares, p.FillPrice, p.FillPrice,
			)
		} else if scanErr == nil {
			newShares := existingShares + p.Shares
			newAvgCost := (float64(existingShares)*existingAvgCost + totalAmount) / float64(newShares)
			_, err = tx.Exec(
				"UPDATE positions SET shares = ?, avg_cost = ?, current_price = ?, unrealized_pnl = 0.0, last_updated = CURRENT_TIMESTAMP WHERE symbol = ?",
				newShares, newAvgCost, p.FillPrice, ticker,
			)
		} else {
			return sdk.Error(fmt.Sprintf("query existing position: %v", scanErr)), nil
		}
		if err != nil {
			return sdk.Error(fmt.Sprintf("update position: %v", err)), nil
		}

	case "SELL":
		var existingShares int
		var existingAvgCost float64
		pRow := tx.QueryRow("SELECT shares, avg_cost FROM positions WHERE symbol = ?", ticker)
		if err := pRow.Scan(&existingShares, &existingAvgCost); err != nil {
			return sdk.Error(fmt.Sprintf("cannot sell: no open position in %s", ticker)), nil
		}
		if existingShares < p.Shares {
			return sdk.Error(fmt.Sprintf("cannot sell %d shares of %s: only %d shares held", p.Shares, ticker, existingShares)), nil
		}

		cashBalance += totalAmount
		remainingShares := existingShares - p.Shares
		if remainingShares == 0 {
			_, err = tx.Exec("DELETE FROM positions WHERE symbol = ?", ticker)
		} else {
			_, err = tx.Exec(
				"UPDATE positions SET shares = ?, current_price = ?, unrealized_pnl = (?-avg_cost)*?, last_updated = CURRENT_TIMESTAMP WHERE symbol = ?",
				remainingShares, p.FillPrice, p.FillPrice, remainingShares, ticker,
			)
		}
		if err != nil {
			return sdk.Error(fmt.Sprintf("update position: %v", err)), nil
		}

	case "HOLD":
		totalAmount = 0
	}

	var posVal float64
	vRows, err := tx.Query("SELECT shares, current_price FROM positions")
	if err == nil {
		for vRows.Next() {
			var s int
			var cp float64
			if err := vRows.Scan(&s, &cp); err == nil {
				posVal += float64(s) * cp
			}
		}
		vRows.Close()
	}
	totalEquity = cashBalance + posVal

	_, err = tx.Exec(
		"UPDATE portfolio SET cash_balance = ?, total_equity = ?, updated_at = CURRENT_TIMESTAMP WHERE id = 1",
		cashBalance, totalEquity,
	)
	if err != nil {
		return sdk.Error(fmt.Sprintf("update portfolio totals: %v", err)), nil
	}

	var sl, tp sql.NullFloat64
	if p.StopLoss != nil {
		sl = sql.NullFloat64{Float64: *p.StopLoss, Valid: true}
	}
	if p.TakeProfit != nil {
		tp = sql.NullFloat64{Float64: *p.TakeProfit, Valid: true}
	}

	var conv sql.NullInt32
	if p.Conviction != nil {
		conv = sql.NullInt32{Int32: int32(*p.Conviction), Valid: true}
	}

	var bull, bear, risk, cio sql.NullString
	if p.BullThesis != nil {
		bull = sql.NullString{String: *p.BullThesis, Valid: true}
	}
	if p.BearThesis != nil {
		bear = sql.NullString{String: *p.BearThesis, Valid: true}
	}
	if p.RiskNotes != nil {
		risk = sql.NullString{String: *p.RiskNotes, Valid: true}
	}
	if p.CIOResolution != nil {
		cio = sql.NullString{String: *p.CIOResolution, Valid: true}
	}

	_, err = tx.Exec(
		`INSERT INTO trades (trade_id, symbol, action, shares, fill_price, total_amount, stop_loss, take_profit, conviction_score, bull_thesis, bear_thesis, risk_notes, cio_resolution)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tradeID, ticker, action, p.Shares, p.FillPrice, totalAmount, sl, tp, conv, bull, bear, risk, cio,
	)
	if err != nil {
		return sdk.Error(fmt.Sprintf("insert trade: %v", err)), nil
	}

	if err := tx.Commit(); err != nil {
		return sdk.Error(fmt.Sprintf("commit transaction: %v", err)), nil
	}

	msg := fmt.Sprintf(
		"Trade Executed Successfully [%s]:\n"+
			"- Order ID: %s\n"+
			"- Action: %s %d shares of %s @ $%.2f (Total: $%.2f)\n"+
			"- New Cash Balance: $%.2f\n"+
			"- Updated Total Equity: $%.2f\n"+
			"- Timestamp: %s",
		tradeID, tradeID, action, p.Shares, ticker, p.FillPrice, totalAmount,
		cashBalance, totalEquity, time.Now().UTC().Format(time.RFC3339),
	)

	return sdk.OK(
		msg,
		sdk.WithData(map[string]any{
			"trade_id":     tradeID,
			"symbol":       ticker,
			"action":       action,
			"shares":       p.Shares,
			"fill_price":   p.FillPrice,
			"total_amount": totalAmount,
			"cash_balance": cashBalance,
			"total_equity": totalEquity,
			"status":       "FILLED",
		}),
	), nil
}

type PortfolioAuditRiskParams struct {
	Ticker     string   `json:"ticker" jsonschema:"description=The equity ticker symbol (e.g. NVDA),required"`
	Action     string   `json:"action" jsonschema:"description=Trade action: BUY, SELL, or HOLD,required"`
	Shares     int      `json:"shares" jsonschema:"description=Proposed shares to trade,required"`
	FillPrice  float64  `json:"fill_price" jsonschema:"description=Limit entry price per share,required"`
	StopLoss   *float64 `json:"stop_loss,omitempty" jsonschema:"description=Stop-loss exit price"`
	TakeProfit *float64 `json:"take_profit,omitempty" jsonschema:"description=Take-profit target price"`
}

func auditRisk(_ context.Context, p PortfolioAuditRiskParams, _ *sdk.ToolContext) (*sdk.Result, error) {
	ticker := strings.ToUpper(strings.TrimSpace(p.Ticker))
	if ticker == "" {
		return sdk.Error("ticker is required"), nil
	}
	action := strings.ToUpper(strings.TrimSpace(p.Action))
	if action != "BUY" && action != "SELL" && action != "HOLD" {
		return sdk.Error("action must be BUY, SELL, or HOLD"), nil
	}

	db, err := openDB()
	if err != nil {
		return sdk.Error(fmt.Sprintf("database error: %v", err)), nil
	}
	defer db.Close()

	var cashBalance, totalEquity float64
	row := db.QueryRow("SELECT cash_balance, total_equity FROM portfolio WHERE id = 1")
	if err := row.Scan(&cashBalance, &totalEquity); err != nil {
		return sdk.Error(fmt.Sprintf("read portfolio: %v", err)), nil
	}

	// Read existing position in ticker
	var existingShares int
	pRow := db.QueryRow("SELECT shares FROM positions WHERE symbol = ?", ticker)
	_ = pRow.Scan(&existingShares)

	var verdict string
	var effectiveAction string
	var approvedShares int
	var riskNotes string
	var rrRatio float64

	switch action {
	case "HOLD":
		verdict = "APPROVED"
		effectiveAction = "HOLD"
		approvedShares = 0
		riskNotes = "HOLD proposal audited: no risk exposure change."

	case "SELL":
		if existingShares <= 0 {
			verdict = "REJECTED"
			effectiveAction = "HOLD"
			approvedShares = 0
			riskNotes = fmt.Sprintf("Rejected: cannot sell %s because no shares are currently held in portfolio.", ticker)
		} else if p.Shares > existingShares {
			verdict = "MODIFIED"
			effectiveAction = "SELL"
			approvedShares = existingShares
			riskNotes = fmt.Sprintf("Modified: proposed selling %d shares, but only %d shares are held. Downsized to full position size.", p.Shares, existingShares)
		} else {
			verdict = "APPROVED"
			effectiveAction = "SELL"
			approvedShares = p.Shares
			riskNotes = fmt.Sprintf("Approved: selling %d shares out of %d held.", p.Shares, existingShares)
		}

	case "BUY":
		// 1. Stop loss check
		if p.StopLoss == nil || *p.StopLoss <= 0 || *p.StopLoss >= p.FillPrice {
			verdict = "REJECTED"
			effectiveAction = "HOLD"
			approvedShares = 0
			riskNotes = fmt.Sprintf("Rejected: stop-loss discipline breached. Stop-loss must be defined and strictly below fill price ($%.2f).", p.FillPrice)
			break
		}

		// 2. Risk/Reward ratio check (minimum 2.0 : 1)
		risk := p.FillPrice - *p.StopLoss
		if p.TakeProfit == nil || *p.TakeProfit <= p.FillPrice {
			verdict = "REJECTED"
			effectiveAction = "HOLD"
			approvedShares = 0
			riskNotes = "Rejected: take-profit target must be defined and strictly above fill price."
			break
		}
		reward := *p.TakeProfit - p.FillPrice
		rrRatio = math.Round((reward/risk)*100) / 100
		if rrRatio < 2.0 {
			verdict = "REJECTED"
			effectiveAction = "HOLD"
			approvedShares = 0
			riskNotes = fmt.Sprintf("Rejected: unfavorable Risk/Reward ratio of %.2f : 1 (institutional minimum required is 2.00 : 1). Risk: $%.2f, Reward: $%.2f.", rrRatio, risk, reward)
			break
		}

		// 3. Single-Asset Concentration Cap (10% of total equity)
		singleAssetCap := totalEquity * 0.10
		currentPositionVal := float64(existingShares) * p.FillPrice
		maxIncrementalCap := singleAssetCap - currentPositionVal
		if maxIncrementalCap <= 0 {
			verdict = "REJECTED"
			effectiveAction = "HOLD"
			approvedShares = 0
			riskNotes = fmt.Sprintf("Rejected: single-asset concentration cap reached. Current position value is $%.2f (>= 10%% cap of $%.2f).", currentPositionVal, singleAssetCap)
			break
		}

		// 4. Cash liquidity limit
		maxAllowedBudget := math.Min(cashBalance, maxIncrementalCap)
		proposedCost := float64(p.Shares) * p.FillPrice

		if proposedCost <= maxAllowedBudget {
			verdict = "APPROVED"
			effectiveAction = "BUY"
			approvedShares = p.Shares
			riskNotes = fmt.Sprintf("Approved: trade satisfies 10%% equity cap ($%.2f / $%.2f limit), cash liquidity ($%.2f available), and R/R ratio (%.2f : 1).", proposedCost, singleAssetCap, cashBalance, rrRatio)
		} else {
			// Calculate max permitted shares
			maxPermittedShares := int(maxAllowedBudget / p.FillPrice)
			if maxPermittedShares > 0 {
				verdict = "MODIFIED"
				effectiveAction = "BUY"
				approvedShares = maxPermittedShares
				allocatedVal := float64(approvedShares) * p.FillPrice
				riskNotes = fmt.Sprintf("Modified: proposed allocation ($%.2f for %d shs) exceeded limits. Downsized to %d shares ($%.2f) to respect the 10%% equity cap ($%.2f) and available cash ($%.2f). R/R ratio: %.2f : 1.",
					proposedCost, p.Shares, approvedShares, allocatedVal, singleAssetCap, cashBalance, rrRatio)
			} else {
				verdict = "REJECTED"
				effectiveAction = "HOLD"
				approvedShares = 0
				riskNotes = fmt.Sprintf("Rejected: insufficient cash ($%.2f) or equity headroom ($%.2f) to purchase even 1 share @ $%.2f.", cashBalance, maxIncrementalCap, p.FillPrice)
			}
		}
	}

	stopLossVal := 0.0
	if p.StopLoss != nil {
		stopLossVal = *p.StopLoss
	}
	takeProfitVal := 0.0
	if p.TakeProfit != nil {
		takeProfitVal = *p.TakeProfit
	}
	totalAllocation := float64(approvedShares) * p.FillPrice

	msg := fmt.Sprintf(
		"Risk Officer Audit Verdict: %s\n"+
			"- Effective Action: %s\n"+
			"- Approved Shares: %d (Requested: %d)\n"+
			"- Execution Fill Price: $%.2f (Total Allocation: $%.2f)\n"+
			"- Risk/Reward Ratio: %.2f : 1\n"+
			"- 10%% Single-Asset Cap: $%.2f | Cash Balance: $%.2f | Total Equity: $%.2f\n"+
			"- Risk Notes: %s",
		verdict, effectiveAction, approvedShares, p.Shares, p.FillPrice, totalAllocation,
		rrRatio, totalEquity*0.10, cashBalance, totalEquity, riskNotes,
	)

	return sdk.OK(
		msg,
		sdk.WithData(map[string]any{
			"verdict":            verdict,
			"effective_action":   effectiveAction,
			"original_shares":    p.Shares,
			"approved_shares":    approvedShares,
			"fill_price":         p.FillPrice,
			"total_allocation":   totalAllocation,
			"stop_loss":          stopLossVal,
			"take_profit":        takeProfitVal,
			"risk_reward_ratio":  rrRatio,
			"single_asset_limit": totalEquity * 0.10,
			"cash_balance":       cashBalance,
			"total_equity":       totalEquity,
			"risk_notes":         riskNotes,
			"approved":           (verdict == "APPROVED" || verdict == "MODIFIED"),
		}),
	), nil
}
