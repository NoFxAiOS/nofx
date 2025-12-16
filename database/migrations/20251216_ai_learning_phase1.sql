-- AI Learning Phase 1: Data Foundation

-- 1. Trade Analysis Records
CREATE TABLE IF NOT EXISTS trade_analysis_records (
    id TEXT PRIMARY KEY,
    trader_id TEXT NOT NULL,
    analysis_date TIMESTAMPTZ NOT NULL,
    
    -- Basic Stats
    total_trades INTEGER DEFAULT 0,
    winning_trades INTEGER DEFAULT 0,
    losing_trades INTEGER DEFAULT 0,
    win_rate REAL DEFAULT 0,
    
    -- Risk/Reward
    avg_profit_per_win REAL DEFAULT 0,
    avg_loss_per_loss REAL DEFAULT 0,
    profit_factor REAL DEFAULT 0,
    risk_reward_ratio REAL DEFAULT 0,
    
    -- Detailed Data (JSON)
    analysis_data JSONB,
    
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    
    -- Constraints
    UNIQUE(trader_id, analysis_date)
);

CREATE INDEX IF NOT EXISTS idx_trade_analysis_trader_date ON trade_analysis_records(trader_id, analysis_date DESC);

-- 2. Learning Reflections
CREATE TABLE IF NOT EXISTS learning_reflections (
    id TEXT PRIMARY KEY,
    trader_id TEXT NOT NULL,
    
    reflection_type VARCHAR(50), -- 'strategy', 'risk', 'timing', 'pattern'
    severity VARCHAR(20),        -- 'critical', 'high', 'medium', 'low'
    
    problem_title TEXT NOT NULL,
    problem_description TEXT,
    
    root_cause TEXT,
    recommended_action TEXT,
    
    priority INTEGER DEFAULT 0,
    is_applied BOOLEAN DEFAULT FALSE,
    
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_learning_reflections_trader ON learning_reflections(trader_id);

-- 3. Parameter Change History
CREATE TABLE IF NOT EXISTS parameter_change_history (
    id TEXT PRIMARY KEY,
    trader_id TEXT NOT NULL,
    
    parameter_name VARCHAR(100),
    old_value TEXT,
    new_value TEXT,
    change_reason TEXT,
    
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_parameter_change_trader ON parameter_change_history(trader_id);
