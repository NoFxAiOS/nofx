# Feature Proposal: AI Learning System - Phase 1 (Data Foundation)

## 1. Context & Objectives
To enable AI agents to learn from their trading history, we need to establish a feedback loop. The first step (Phase 1) is to build the data foundation for recording analysis, reflections, and parameter changes, and to implement the core analysis logic.

**Goal**: Enable the system to analyze trade data, identify basic patterns, and expose this information via API.

## 2. Technical Architecture

### 2.1 Database Schema
We will introduce three new tables to store learning artifacts.

```sql
-- 1. Trade Analysis Records
CREATE TABLE trade_analysis_records (
    id TEXT PRIMARY KEY,
    trader_id TEXT NOT NULL,
    analysis_date TIMESTAMPTZ NOT NULL,
    total_trades INTEGER,
    win_rate REAL,
    profit_factor REAL,
    risk_reward_ratio REAL,
    analysis_data JSONB, -- Stores detailed breakdown (e.g., best pair, hourly stats)
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (trader_id) REFERENCES traders(id) ON DELETE CASCADE,
    UNIQUE(trader_id, analysis_date)
);

-- 2. Learning Reflections
CREATE TABLE learning_reflections (
    id TEXT PRIMARY KEY,
    trader_id TEXT NOT NULL,
    reflection_type VARCHAR(50), -- strategy, risk, timing
    severity VARCHAR(20),        -- critical, high, medium, low
    problem_title TEXT NOT NULL,
    problem_description TEXT,
    root_cause TEXT,
    recommended_action TEXT,
    priority INTEGER,            -- 1-10
    is_applied BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (trader_id) REFERENCES traders(id) ON DELETE CASCADE
);

-- 3. Parameter Change History
CREATE TABLE parameter_change_history (
    id TEXT PRIMARY KEY,
    trader_id TEXT NOT NULL,
    parameter_name VARCHAR(100),
    old_value TEXT,
    new_value TEXT,
    change_reason TEXT,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (trader_id) REFERENCES traders(id) ON DELETE CASCADE
);
```

### 2.2 Go Modules

#### `decision/analysis` Package
This new package will contain the core logic for analyzing trades.

**`TradeAnalyzer` Struct**:
-   **Responsibility**: Fetch trade records for a given period and calculate statistics (Win Rate, Profit Factor, etc.).
-   **Input**: `traderID`, `startDate`, `endDate`.
-   **Output**: `TradeAnalysisResult` struct.

**`PatternDetector` Struct**:
-   **Responsibility**: Analyze `TradeAnalysisResult` to identify specific failure patterns (e.g., "High Leverage Risk").
-   **Input**: `TradeAnalysisResult`.
-   **Output**: `[]FailurePattern`.

### 2.3 API Endpoints
-   `GET /api/traders/:id/analysis`: Returns the latest analysis or triggers an on-demand analysis.
-   `GET /api/traders/:id/reflections`: Returns a list of generated reflections.

## 3. Implementation Plan

1.  **Database Migration**: Create `database/migrations/20251216_ai_learning_phase1.sql`.
2.  **Core Logic**:
    -   Create `decision/analysis/types.go` (Data structures).
    -   Implement `decision/analysis/trade_analyzer.go` (Stats calculation).
    -   Implement `decision/analysis/pattern_detector.go` (Rule-based detection).
3.  **Data Access**: Update `database/` package to support the new tables (or add repository methods in `decision/analysis` if using a repository pattern, but adhering to existing project style usually means methods on `*Database`).
4.  **API**:
    -   Create `api/handlers/learning.go`.
    -   Register routes in `api/server.go`.
5.  **Testing**:
    -   Unit tests for `TradeAnalyzer` (Pure logic tests).
    -   Unit tests for `PatternDetector`.

## 4. Verification
-   **Unit Tests**: 100% coverage for the new analysis logic.
-   **Integration Verification**: Verify that the API returns calculated stats for a test trader.
