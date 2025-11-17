# Decision Engine & Prompt System Design

This document describes the design of the decision engine and prompt management in NOFX, primarily implemented in:

- `decision/engine.go`
- `decision/prompt_manager.go`
- `mcp/` (Model Context Protocol client)

## 1. Responsibilities

The decision engine is responsible for:

- Translating trader state and market data into an AI-readable context.
- Building robust prompts that encode risk controls and output formats.
- Calling AI models via MCP and parsing responses into `Decision` structs.
- Enforcing minimal structural validity and delegating deeper risk checks to the trading engine.
- Integrating historical performance analysis to guide self-learning behaviour.

## 2. Context Construction

For each decision cycle, `AutoTrader` constructs a `decision.Context`:

- Derived from:
  - Current account state from exchange.
  - Open positions.
  - Candidate symbols from `pool` and OI Top sources.
  - Market data from `market.Get` per symbol.
  - Historical equity and trade performance (via logger/performance analysis).
- The context is serialised to JSON and included in the user message to the AI model.
- Internal-only fields (`MarketDataMap`, `OITopDataMap`, `Performance`, leverage fields) guide prompt construction and risk checks but may not be fully exposed to the model.

## 3. Prompt Templates

`decision/prompt_manager.go` and related code implement:

- Loading of prompt templates from embedded assets or configuration (e.g. `default` template).
- `PromptTemplate` structures with:
  - `Name`, `Content`, and optional metadata (language/market focus).
- API handlers `/api/prompt-templates` and `/api/prompt-templates/:name` expose available templates to the frontend.

### 3.1 System Prompt Composition

- `buildSystemPromptWithCustom(accountEquity, btcEthLeverage, altcoinLeverage, customPrompt, overrideBase, templateName)`:
  - If `overrideBase` is true and `customPrompt` non-empty:
    - System prompt is the custom prompt alone.
  - Otherwise:
    - Loads base template via `GetPromptTemplate(templateName)` (default `"default"`).
    - Appends dynamic risk controls:
      - Leverage caps for BTC/ETH vs altcoins.
      - Position count limits.
      - Recommended per-trade notional ranges derived from `accountEquity`.
      - Margin usage thresholds and minimum notional constraints (e.g. ≥ 12 USDT).
    - Describes exact output format expectations.
    - Appends custom prompt as a separate “Personalised Strategy” section (if present), emphasising it must not violate base risk rules.

### 3.2 Output Format Contract

- System prompt instructs the AI to:
  - Use `<reasoning>` and `<decision>` XML-style tags to separate thought process from machine-readable decisions.
  - Embed a fenced ` ```json ... ``` ` array of decisions inside `<decision>`.
  - Conform to required fields and action enums.
- This contract is enforced by regex patterns in `engine.go`:
  - `reReasoningTag` to extract reasoning.
  - `reDecisionTag` to extract decision block.
  - `reJSONFence`, `reJSONArray`, and related regexes to clean and normalise the JSON content.

## 4. MCP Client Integration

- `mcp/client.go` (not shown here) encapsulates calls to AI providers:
  - DeepSeek, Qwen, and custom OpenAI-compatible APIs.
  - Uses per-user and per-model credentials from `ai_models` table.
- Decision engine specifies:
  - Model identifier (provider or custom name).
  - System prompt (from builder).
  - User message (context JSON).
- On error (network, provider, or rate limiting):
  - Engine logs the failure.
  - The decision cycle may fall back to “no trade” behaviour (e.g. treat as `hold`/`wait`).

## 5. Response Parsing & Normalisation

Key design goals:

- Be tolerant of minor formatting deviations while maintaining strictness on structure.
- Avoid double-parsing or incorrectly executing malformed responses.

### 5.1 Pre-processing

- Strip invisible runes (`reInvisibleRunes`) that often appear in LLM output.
- Locate JSON array:
  - Prefer fenced ```json blocks.
  - Fallback to the first top-level array `[...]` if necessary.
- Ensure array has leading `[` and trailing `]` even if model omits them.

### 5.2 Parsing

- Attempt `json.Unmarshal` into `[]Decision`.
- On success:
  - Attach extracted `Reasoning` string from `<reasoning>` tag to each decision (or to the log entry).
- On failure:
  - Log the raw response snippet and parsing error.
  - Optionally write a diagnostic entry to decision logs.
  - Skip executing any orders for this cycle to avoid unintended trades.

## 6. Historical Feedback & Self-Learning

- Before building the context, the engine queries historical performance:
  - Last N trades (e.g. 20) from logs/DB.
  - Per-symbol win rate, average PnL, worst drawdowns.
- These metrics are included in context and/or summarised in the system prompt so the model can:
  - Avoid repeating patterns with persistent losses.
  - Reinforce trades with consistently positive outcomes.
  - Adjust aggressiveness based on recent performance.

## 7. Extension Points

Areas where the current design intentionally leaves room for extension:

- **Additional prompt templates:**
  - Market-specific templates (e.g. BTC-only, low-liquidity avoidance).
  - Risk profiles (conservative vs aggressive).
- **Model routing:**
  - Auto-select model based on market regime or symbol (ensemble strategies).
- **Critic/reviewer loops:**
  - Use a secondary model to critique primary decisions before execution.
- **Structured logging:**
  - Export reasoning and decisions into external analytics systems.

Any such extensions should:

- Update the prompt template and system prompt builder logic.
- Extend the `Decision` struct and parsing logic if new fields are introduced.
- Be reflected in this design doc and the `trading-engine` spec.

