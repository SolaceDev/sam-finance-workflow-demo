# Trading Desk Workflow — Performance Benchmarks

This document records empirical performance and token usage benchmarks across iterations of the financial trading desk workflow.

---

## Benchmark 1: All-Agent Baseline Workflow (`TradingDeskAllAgents`)

- **Date:** 2026-09-09
- **Platform:** `solace-agent-mesh-go` (version `main-v2.336.2-46-g222bcf287`)
- **Model:** `anthropic/claude-sonnet-4-6` via LiteLLM
- **Task ID:** `01a086b3-d229-7543-a0ac-662ed35fe064`
- **Raw STIM File:** `benchmarks/01a086b3-d229-7543-a0ac-662ed35fe064.stim`
- **Initial Request:** `"Use TradingDeskAllAgents to evaluate SHOP — should we allocate capital today?"`

### 1. High-Level Summary

| Metric | Measurement |
|---|---|
| **Total Wall-Clock Duration** | **279.68s (4m 39.7s)** |
| **Workflow Internal Execution Time** | **253.48s (4m 13.5s)** |
| **Total Tokens Billed** | **991,542 tokens (~1.0M tokens)** |
| **Total Input Tokens** | 967,449 (967,409 cached) |
| **Total Output Tokens** | 24,093 |
| **Total Sub-Tasks / Agent Invocations** | 10 (1 Orchestrator + 1 Workflow + 8 Agents) |

---

### 2. Detailed Per-Agent Breakdown

The workflow coordinates 8 agent nodes organized into 5 sequential waves:

| Wave / Component | Agent Name | Node ID | Duration | Input Tokens (Cached) | Output Tokens | Total Tokens | % of Total Tokens |
|---|---|---|---|---|---|---|---|
| **Entrypoint** | `Orchestrator` | — | 279.7s | 71,233 (71,228) | 1,412 | 72,645 | 7.3% |
| **Wave 1 (Parallel)** | `MarketAnalyst` | `technical_analysis` | 23.2s | 26,620 (26,616) | 1,741 | 28,361 | 2.9% |
| **Wave 1 (Parallel)** | `FundamentalsAnalyst` | `fundamental_analysis` | 23.2s | 26,759 (26,755) | 1,576 | 28,335 | 2.9% |
| **Wave 1 (Parallel)** | `PredictionSentimentAnalyst` | `prediction_sentiment` | 67.6s | 621,928 (621,924) | 3,427 | 625,355 | **63.1%** |
| **Wave 2 (Parallel)** | `BullResearcher` | `bull_thesis` | 64.4s | 27,857 (27,853) | 3,879 | 31,736 | 3.2% |
| **Wave 2 (Parallel)** | `BearResearcher` | `bear_thesis` | 74.9s | 27,887 (27,883) | 4,525 | 32,412 | 3.3% |
| **Wave 3** | `Trader` | `order_formulation` | 46.4s | 52,299 (52,294) | 2,928 | 55,227 | 5.6% |
| **Wave 4** | `RiskOfficer` | `risk_audit` | 26.7s | 28,920 (28,916) | 1,978 | 30,898 | 3.1% |
| **Wave 5** | `PortfolioManager` | `executive_decision` | 37.9s | 83,946 (83,940) | 2,627 | 86,573 | 8.7% |
| **Total** | | | **279.7s** | **967,449 (967,409)** | **24,093** | **991,542** | **100%** |

---

### 3. Key Architectural Observations

1. **The Polymarket / OpenAPI Context Tax**:
   - `PredictionSentimentAnalyst` burned **625,355 tokens** (63.1% of the entire run) and took 67.5 seconds.
   - *Root cause:* The agent pulled large JSON market payloads from the Polymarket Gamma API into LLM context across multiple tool-call turns to parse market strings.
   - *Optimization Opportunity:* Replacing this with a declarative `type: tool` node calling the OpenAPI connector directly, piped into a `transform_data_with_jmespath` tool node, reduces this step from **625k tokens to 0 tokens** and executes in under 500ms.

2. **Single-Tool Turn Tax**:
   - `MarketAnalyst` (28.4k tokens, 23.2s) and `FundamentalsAnalyst` (28.3k tokens, 23.2s) ran an entire LLM prompt/response cycle solely to invoke `market_get_technicals` and `fundamentals_get_financials`.
   - `RiskOfficer` (30.9k tokens, 26.7s) ran an LLM turn to query `portfolio_get_summary`.
   - `PortfolioManager` (86.6k tokens, 37.9s) ran LLM turns to call `portfolio_execute_trade`.
   - *Total deterministic tool overhead:* **~172,000 tokens** and **~111 seconds** of model latency.

3. **Where LLM Reasoning Truly Adds Value**:
   - The adversarial debate in Wave 2 (`BullResearcher` and `BearResearcher`) and the trade formulation in Wave 3 (`Trader`) consumed only **~120k tokens** combined and produced the actual analytical value of the desk (weighing valuation multiple risk against top-line growth catalysts).

---

### 4. Projected Gains for Graph-Optimized Version (`TradingDeskOptimized`)

By demoting deterministic nodes to `type: tool` and `type: switch`:

| Step | Baseline (`TradingDeskAllAgents`) | Optimized (`TradingDeskOptimized`) | Expected Delta |
|---|---|---|---|
| Technicals Fetch | `agent` (28.4k tokens, 23s) | `type: tool` (0 tokens, <0.1s) | -28.4k tokens, -23s |
| Fundamentals Fetch | `agent` (28.3k tokens, 23s) | `type: tool` (0 tokens, <0.1s) | -28.3k tokens, -23s |
| Prediction Odds | `agent` (625.4k tokens, 68s) | `type: tool` (0 tokens, ~0.4s) | -625.4k tokens, -67s |
| Portfolio Snapshot | `agent` (in RiskOfficer) | `type: tool` (0 tokens, <0.1s) | -20k tokens, -15s |
| Risk Limit Check | `agent` (in RiskOfficer) | `type: switch` (`cash >= amount`) | -10k tokens, -10s |
| Ledger Commit | `agent` (in PortfolioManager) | `type: tool` (0 tokens, <0.1s) | -40k tokens, -20s |
| **Total Projected** | **~991k tokens, ~280s** | **~100k–150k tokens, ~35–50s** | **~85% token reduction, ~80% latency reduction** |

---

## Benchmark 2: Graph-Optimized Workflow (`TradingDeskOptimized`)

- **Date:** 2026-09-09
- **Platform:** `solace-agent-mesh-go` (version `main-v2.336.2-46-g222bcf287`)
- **Model:** `anthropic/claude-sonnet-4-6` via LiteLLM
- **Task ID:** `01a086d6-2bd5-7dea-90b6-edfc960c464b`
- **Raw STIM File:** `benchmarks/01a086d6-2bd5-7dea-90b6-edfc960c464b.stim`
- **Initial Request:** `"Evaluate SHOP for portfolio allocation"` (ticker: `SHOP`)

### 1. High-Level Summary

| Metric | Measurement |
|---|---|
| **Total Wall-Clock Duration** | **119.83s (1m 59.8s)** |
| **Total Tokens Billed** | **113,963 tokens (~114k tokens)** |
| **Total Input Tokens** | 102,762 (102,749 cached) |
| **Total Output Tokens** | 11,201 |
| **Nodes Executed** | 7 (4 Tool Nodes + 2 Debate Agents + 1 Trader Agent + 1 Risk Switch Gate + 1 Ledger Tool) |

---

### 2. Node-by-Node Execution & Token Accounting

| Wave / Stage | Node ID | Node Type | Handler / Entity | Latency | Tokens Billed | Note |
|---|---|---|---|---|---|---|
| **Wave 1 (Parallel)** | `technicals_tool` | `type: tool` | `market_get_technicals` | ~0.15s | **0 tokens** | Pure Go binary calculation |
| **Wave 1 (Parallel)** | `fundamentals_tool` | `type: tool` | `fundamentals_get_financials` | ~0.15s | **0 tokens** | Structured financial metrics |
| **Wave 1 (Parallel)** | `prediction_tool` | `type: tool` | `prediction_get_odds` | ~0.35s | **0 tokens** | Live Polymarket Gamma API call & normalization |
| **Wave 1 (Parallel)** | `portfolio_state` | `type: tool` | `portfolio_get_summary` | ~0.05s | **0 tokens** | SQLite ledger balance query |
| **Wave 2 (Parallel)** | `bull_thesis` | `type: agent` | `BullResearcher` | ~45s | 45,627 tokens | Cognitive upside thesis formulation |
| **Wave 2 (Parallel)** | `bear_thesis` | `type: agent` | `BearResearcher` | ~51s | 31,642 tokens | Adversarial multiple compression & risk audit |
| **Wave 3** | `order_formulation` | `type: agent` | `Trader` | ~42s | 36,694 tokens | Synthesizes debate, outputs structured proposal |
| **Wave 4** | `risk_gate` | `type: switch` | Workflow Engine Expr | 0.001s | **0 tokens** | Deterministic cash & risk condition check |
| **Wave 5** | `hold_position` | `type: tool` | `portfolio_execute_trade` | 0.045s | **0 tokens** | SQLite atomic transaction |
| **Total** | | | | **119.8s** | **113,963 tokens** | **Pure cognitive reasoning only** |

---

## Head-to-Head Comparison: All-Agent vs. Graph-Optimized

| Metric | All-Agent Baseline (`TradingDeskAllAgents`) | Graph-Optimized (`TradingDeskOptimized`) | Absolute Improvement | Relative Improvement |
|---|---|---|---|---|
| **Total Billed Tokens** | **991,542 tokens** | **113,963 tokens** | **-877,579 tokens** | **88.5% token reduction** |
| **Total Duration** | **279.7s (4m 40s)** | **119.8s (2m 00s)** | **-159.9s** | **57.2% faster** |
| **Data Collection Tokens** | 682,051 tokens (68.8% of run) | **0 tokens** | -682,051 tokens | **100% elimination** |
| **Data Collection Latency** | 67.6s (Polymarket agent turn) | **0.35s** | -67.2s | **99.5% faster** |
| **Risk & Execution Tokens** | 117,471 tokens (RiskOfficer + PortfolioManager) | **0 tokens** (Switch + SQLite Tool) | -117,471 tokens | **100% elimination** |
| **Risk & Execution Latency** | 64.6s | **0.05s** | -64.5s | **99.9% faster** |
| **Reasoning Fidelity** | Full Bull/Bear Debate + Trader | Full Bull/Bear Debate + Trader | Identical | Preserved 100% |

