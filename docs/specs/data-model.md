# NOFX Data Model Specification

This document describes the current SQLite schema and main Go structs used by NOFX, based on `config/database.go`. It is intended as the source of truth for database-related behaviour and should be updated whenever schema or persistence logic changes.

## 1. Overview

- **Database engine:** SQLite (via `modernc.org/sqlite`).
- **Mode:** WAL (`PRAGMA journal_mode=WAL`) with `synchronous=FULL` for durability.
- **Primary database file:** `config.db` (default, overridable via CLI arg).
- **Initialization flow:**
  - `NewDatabase`:
    - Opens SQLite connection.
    - Calls `createTables()` to ensure tables/triggers exist.
    - Calls `initDefaultData()` to seed AI models, exchanges, and system_config.

## 2. Core Tables

### 2.1 `users`

Holds user accounts (including potential admin user).

Schema:
```sql
CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  email TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,
  otp_secret TEXT,
  otp_verified BOOLEAN DEFAULT 0,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

Go struct (`User`):
```go
type User struct {
  ID           string    `json:"id"`
  Email        string    `json:"email"`
  PasswordHash string    `json:"-"`
  OTPSecret    string    `json:"-"`
  OTPVerified  bool      `json:"otp_verified"`
  CreatedAt    time.Time `json:"created_at"`
  UpdatedAt    time.Time `json:"updated_at"`
}
```

Key points:
- `id` is an internal identifier (string, not necessarily email).
- Password hash and OTP secret are never exposed to clients.
- `otp_verified` indicates whether 2FA setup/verification has been completed.

### 2.2 `ai_models`

Per-user AI model configurations.

Schema:
```sql
CREATE TABLE IF NOT EXISTS ai_models (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL DEFAULT 'default',
  name TEXT NOT NULL,
  provider TEXT NOT NULL,
  enabled BOOLEAN DEFAULT 0,
  api_key TEXT DEFAULT '',
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
```

Go struct (`AIModelConfig`):
```go
type AIModelConfig struct {
  ID              string    `json:"id"`
  UserID          string    `json:"user_id"`
  Name            string    `json:"name"`
  Provider        string    `json:"provider"`
  Enabled         bool      `json:"enabled"`
  APIKey          string    `json:"apiKey"`
  CustomAPIURL    string    `json:"customApiUrl"`
  CustomModelName string    `json:"customModelName"`
  CreatedAt       time.Time `json:"created_at"`
  UpdatedAt       time.Time `json:"updated_at"`
}
```

Notes:
- `provider` is a logical identifier (e.g. `"deepseek"`, `"qwen"`).
- `api_key`, `custom_api_url`, and `custom_model_name` may be encrypted at rest via `CryptoService`.
- Default entries are seeded in `initDefaultData` for user `"default"`.

### 2.3 `exchanges`

Per-user exchange configurations for Binance, Hyperliquid, Aster, etc.

Schema (initial):
```sql
CREATE TABLE IF NOT EXISTS exchanges (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL DEFAULT 'default',
  name TEXT NOT NULL,
  type TEXT NOT NULL,
  enabled BOOLEAN DEFAULT 0,
  api_key TEXT DEFAULT '',
  secret_key TEXT DEFAULT '',
  testnet BOOLEAN DEFAULT 0,
  hyperliquid_wallet_addr TEXT DEFAULT '',
  aster_user TEXT DEFAULT '',
  aster_signer TEXT DEFAULT '',
  aster_private_key TEXT DEFAULT '',
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
```

Schema (migrated for multi-user support):
```sql
CREATE TABLE exchanges_new (
  id TEXT NOT NULL,
  user_id TEXT NOT NULL DEFAULT 'default',
  name TEXT NOT NULL,
  type TEXT NOT NULL,
  enabled BOOLEAN DEFAULT 0,
  api_key TEXT DEFAULT '',
  secret_key TEXT DEFAULT '',
  testnet BOOLEAN DEFAULT 0,
  hyperliquid_wallet_addr TEXT DEFAULT '',
  aster_user TEXT DEFAULT '',
  aster_signer TEXT DEFAULT '',
  aster_private_key TEXT DEFAULT '',
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id, user_id),
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
```

Go struct (`ExchangeConfig`):
```go
type ExchangeConfig struct {
  ID                   string `json:"id"`
  UserID               string `json:"user_id"`
  Name                 string `json:"name"`
  Type                 string `json:"type"`
  Enabled              bool   `json:"enabled"`
  APIKey               string `json:"apiKey"`
  SecretKey            string `json:"secretKey"`
  Testnet              bool   `json:"testnet"`
  HyperliquidWalletAddr string `json:"hyperliquidWalletAddr"`
  AsterUser            string `json:"asterUser"`
  AsterSigner          string `json:"asterSigner"`
  AsterPrivateKey      string `json:"asterPrivateKey"`
  CreatedAt            time.Time `json:"created_at"`
  UpdatedAt            time.Time `json:"updated_at"`
}
```

Notes:
- After migration, `(id, user_id)` is the logical key; code generally treats `id` as the short exchange identifier (`binance`, `hyperliquid`, `aster`).
- API credentials and private keys are intended to be stored encrypted via `CryptoService`.

### 2.4 `user_signal_sources`

User-level external signal providers.

Schema:
```sql
CREATE TABLE IF NOT EXISTS user_signal_sources (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id TEXT NOT NULL,
  coin_pool_url TEXT DEFAULT '',
  oi_top_url TEXT DEFAULT '',
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  UNIQUE(user_id)
);
```

Go struct (`UserSignalSource`):
```go
type UserSignalSource struct {
  ID          int       `json:"id"`
  UserID      string    `json:"user_id"`
  CoinPoolURL string    `json:"coin_pool_url"`
  OITopURL    string    `json:"oi_top_url"`
  CreatedAt   time.Time `json:"created_at"`
  UpdatedAt   time.Time `json:"updated_at"`
}
```

Notes:
- At most one signal source row per user (`UNIQUE(user_id)`).
- Used when loading traders to decide whether to pull from Coin Pool / OI Top feeds.

### 2.5 `traders`

Trader instances per user.

Schema:
```sql
CREATE TABLE IF NOT EXISTS traders (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL DEFAULT 'default',
  name TEXT NOT NULL,
  ai_model_id TEXT NOT NULL,
  exchange_id TEXT NOT NULL,
  initial_balance REAL NOT NULL,
  scan_interval_minutes INTEGER DEFAULT 3,
  is_running BOOLEAN DEFAULT 0,
  btc_eth_leverage INTEGER DEFAULT 5,
  altcoin_leverage INTEGER DEFAULT 5,
  trading_symbols TEXT DEFAULT '',
  use_coin_pool BOOLEAN DEFAULT 0,
  use_oi_top BOOLEAN DEFAULT 0,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  FOREIGN KEY (ai_model_id) REFERENCES ai_models(id),
  FOREIGN KEY (exchange_id) REFERENCES exchanges(id)
);
```

Go struct (`TraderRecord`):
```go
type TraderRecord struct {
  ID                   string    `json:"id"`
  UserID               string    `json:"user_id"`
  Name                 string    `json:"name"`
  AIModelID            string    `json:"ai_model_id"`
  ExchangeID           string    `json:"exchange_id"`
  InitialBalance       float64   `json:"initial_balance"`
  ScanIntervalMinutes  int       `json:"scan_interval_minutes"`
  IsRunning            bool      `json:"is_running"`
  BTCETHLeverage       int       `json:"btc_eth_leverage"`
  AltcoinLeverage      int       `json:"altcoin_leverage"`
  TradingSymbols       string    `json:"trading_symbols"`
  UseCoinPool          bool      `json:"use_coin_pool"`
  UseOITop             bool      `json:"use_oi_top"`
  CustomPrompt         string    `json:"custom_prompt"`
  OverrideBasePrompt   bool      `json:"override_base_prompt"`
  SystemPromptTemplate string    `json:"system_prompt_template"`
  IsCrossMargin        bool      `json:"is_cross_margin"`
  CreatedAt            time.Time `json:"created_at"`
  UpdatedAt            time.Time `json:"updated_at"`
}
```

Notes:
- Custom prompt / template fields are stored in additional columns managed by `ALTER TABLE` statements (not shown in the initial `CREATE TABLE`).
- `IsCrossMargin` indicates cross vs isolated margin mode when supported by the exchange.
- The `traders` table is the authoritative configuration used by `TraderManager` to reconstruct in-memory traders.

### 2.6 `system_config`

Key–value configuration for global system settings.

Schema:
```sql
CREATE TABLE IF NOT EXISTS system_config (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

Usage:
- Keys populated by `initDefaultData()` include:
  - `"beta_mode"` – `"true"` / `"false"`.
  - `"api_server_port"` – stringified int, default `"8080"`.
  - `"use_default_coins"` – `"true"` / `"false"`.
  - `"default_coins"` – JSON array string of symbols.
  - `"max_daily_loss"` – percentage, e.g. `"10.0"`.
  - `"max_drawdown"` – percentage, e.g. `"20.0"`.
  - `"stop_trading_minutes"` – integer string.
  - `"btc_eth_leverage"`, `"altcoin_leverage"`.
  - `"jwt_secret"`.
  - `"registration_enabled"`.
- Additional keys may be added over time; they should be documented here when they affect external behaviour.

### 2.7 `beta_codes`

Invite / beta access codes.

Schema:
```sql
CREATE TABLE IF NOT EXISTS beta_codes (
  code TEXT PRIMARY KEY,
  used BOOLEAN DEFAULT 0,
  used_by TEXT DEFAULT '',
  used_at DATETIME DEFAULT NULL,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

Usage:
- `LoadBetaCodesFromFile` ingests codes from a text file.
- `ValidateBetaCode` and `UseBetaCode` control registration or feature access gating.
- `GetBetaCodeStats` provides summary counts for observability.

## 3. Triggers

The database defines triggers to keep `updated_at` columns in sync:

```sql
CREATE TRIGGER IF NOT EXISTS update_users_updated_at
  AFTER UPDATE ON users
  BEGIN
    UPDATE users SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
  END;

CREATE TRIGGER IF NOT EXISTS update_ai_models_updated_at
  AFTER UPDATE ON ai_models
  BEGIN
    UPDATE ai_models SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
  END;

CREATE TRIGGER IF NOT EXISTS update_exchanges_updated_at
  AFTER UPDATE ON exchanges
  BEGIN
    UPDATE exchanges SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
  END;

CREATE TRIGGER IF NOT EXISTS update_traders_updated_at
  AFTER UPDATE ON traders
  BEGIN
    UPDATE traders SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
  END;
```

These triggers ensure `updated_at` reflects the time of the last modification even when updates do not set it explicitly.

## 4. Encryption Behaviour

- `Database` holds a `cryptoService`:
  - Set via `SetCryptoService(cs *crypto.CryptoService)`.
  - Used by `encryptSensitiveData` and `decryptSensitiveData`.
- Sensitive fields (e.g. `api_key`, `secret_key`, `aster_private_key`) are stored encrypted when `cryptoService` is configured:
  - Encryption: `EncryptForStorage`.
  - Decryption: `DecryptFromStorage`.
  - Non-encrypted / already-plaintext values are passed through unchanged.

When changing how secrets are handled in the DB, this section should be updated.

## 5. Decision Logs (Filesystem)

Decision logs are stored on disk rather than in the DB but are part of the persistence contract:

- **Directory:** `decision_logs/`.
- **Structure:** `decision_logs/{trader_id}/{timestamp}.json`.
- **Content (typical):**
  - Metadata: trader ID, timestamp, model, exchange.
  - Input context snapshot (account, positions, candidate coins, market data).
  - Raw AI response (string).
  - Parsed decisions (`[]Decision`).
  - Execution results and any risk validation notes.

These logs are used for auditability, performance analysis, and the AI learning visualisations exposed via `/api/performance`.

## 6. Evolution Guidelines

When evolving the data model:

- Prefer `ALTER TABLE` + migration helpers (as done with `exchanges_new`) over destructive changes.
- Keep Go structs (`User`, `AIModelConfig`, `ExchangeConfig`, `TraderRecord`, `UserSignalSource`) aligned with schema:
  - New columns → new struct fields (or vice versa).
  - Document new columns and their semantics in this file.
- For any schema change that affects external behaviour (API responses, trading logic, or UI):
  - Update this data-model spec.
  - Update `docs/specs/api-spec.md` and `docs/specs/trading-engine.md` as needed.

