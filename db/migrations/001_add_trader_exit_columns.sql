-- Migration: add per-trader exit columns (stop_loss_pct, take_profit_1_pct, take_profit_2_pct)
-- SQLite
ALTER TABLE traders ADD COLUMN stop_loss_pct REAL DEFAULT 0;
ALTER TABLE traders ADD COLUMN take_profit_1_pct REAL DEFAULT 0;
ALTER TABLE traders ADD COLUMN take_profit_2_pct REAL DEFAULT 0;

-- PostgreSQL (run these if using Postgres)
-- ALTER TABLE traders ADD COLUMN stop_loss_pct DOUBLE PRECISION DEFAULT 0;
-- ALTER TABLE traders ADD COLUMN take_profit_1_pct DOUBLE PRECISION DEFAULT 0;
-- ALTER TABLE traders ADD COLUMN take_profit_2_pct DOUBLE PRECISION DEFAULT 0;

-- MySQL (run these if using MySQL)
-- ALTER TABLE traders ADD COLUMN stop_loss_pct DOUBLE DEFAULT 0;
-- ALTER TABLE traders ADD COLUMN take_profit_1_pct DOUBLE DEFAULT 0;
-- ALTER TABLE traders ADD COLUMN take_profit_2_pct DOUBLE DEFAULT 0;
