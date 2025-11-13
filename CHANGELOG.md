# Changelog

All notable changes to the NOFX project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

**Languages:** [English](CHANGELOG.md) | [中文](CHANGELOG.zh-CN.md)

---

## [Unreleased]

### Added
- **Upstream Integration: Critical Bug Fixes & Improvements** - Integrated 4 high-value commits from upstream (NoFxAiOS/nofx)
  - **Data Staleness Detection** (#800) - Prevents trading on frozen/outdated market data
    - Detects 5 consecutive periods of identical prices with zero volume
    - Automatically skips stale symbols with warning logs
    - Comprehensive test coverage (8 test cases) for edge cases
  - **Registration Toggle** (#760) - Production-ready user registration control
    - System-level `registration_enabled` configuration flag
    - Backward compatible (defaults to enabled)
    - Seamless integration with existing authentication system
  - **Partial Close Safety Checks** (#713) - Enhanced position management validation
    - Minimum position size threshold (10 USDT) enforcement
    - Automatic full-close when remaining position too small
    - Percentage validation (0-100%) with comprehensive error handling
    - Stop-loss/take-profit recovery after partial close
  - See `openspec/changes/integrate-upstream-low-risk-updates/` for full details
- **Hot Reload Configuration** - Technical indicator configuration can now be updated without restarting the backend
  - Real-time configuration updates through frontend interface
  - AutoTrader.ReloadIndicatorConfig() method for seamless config updates
  - TraderManager integration for distributing config updates to running traders
  - API automatically triggers hot reload after saving configuration
  - New configuration takes effect in next AI decision cycle (typically 3 minutes)
  - See `docs/HOT_RELOAD_CONFIG.md` and `docs/HOT_RELOAD_QUICKSTART.zh-CN.md` for usage guide
- Documentation system with multi-language support (EN/CN/RU/UK)
- Complete getting-started guides (Docker, PM2, Custom API)
- Architecture documentation with system design details
- User guides with FAQ and troubleshooting
- Community documentation with bounty programs
- Verification script for indicator configuration fix (`scripts/verify_indicator_config_fix.sh`)

### Fixed
- **PNL Calculation Accuracy** (#963) - Corrected profit/loss computation errors
  - Fixed calculation logic across API, trader, and manager components
  - Enhanced PNL tracking for both open and closed positions
  - Added comprehensive documentation in `docs/pnl.md`
  - Updated frontend components (ComparisonChart, TraderConfigModal) for accurate display
- **Indicator Configuration Data Flow** - Fixed critical issue where user-configured technical indicator parameters were not being passed to market data layer
  - Fixed 7 market data retrieval points in AutoTrader to pass IndicatorConfig
  - Ensures AI receives accurate data points based on user configuration (e.g., 40 vs 60 3m candles)
  - Front-end configuration now properly persists and loads from database
  - Technical indicators (RSI, MACD, Bollinger Bands) now calculated based on user-selected timeframes
  - Complete data flow: Frontend → API → Database → TraderManager → AutoTrader → market.Get()
  - See `docs/INDICATOR_CONFIG_FIX.md` for detailed analysis and verification

### Changed
- Reorganized documentation structure into logical categories
- Updated all README files with proper navigation links
- Enhanced API response for indicator config update to include `hot_reloaded` flag

---

## [3.0.0] - 2025-10-30

### Added - Major Architecture Transformation 🚀

**Complete System Redesign - Web-Based Configuration Platform**

This is a **major breaking update** that completely transforms NOFX from a static config-based system to a modern web-based trading platform.

#### Database-Driven Architecture
- SQLite integration replacing static JSON config
- Persistent storage with automatic timestamps
- Foreign key relationships and triggers for data consistency
- Separate tables for AI models, exchanges, traders, and system config

#### Web-Based Configuration Interface
- Complete web-based configuration management (no more JSON editing)
- AI Model setup through web interface (DeepSeek/Qwen API keys)
- Exchange management (Binance/Hyperliquid credentials)
- Dynamic trader creation (combine any AI model with any exchange)
- Real-time control (start/stop traders without system restart)

#### Flexible Architecture
- Separation of concerns (AI models and exchanges independent)
- Mix & match capability (unlimited combinations)
- Scalable design (support for unlimited traders)
- Clean slate approach (no default traders)

#### Enhanced API Layer
- RESTful design with complete CRUD operations
- New endpoints:
  - `GET/PUT /api/models` - AI model configuration
  - `GET/PUT /api/exchanges` - Exchange configuration
  - `POST/DELETE /api/traders` - Trader management
  - `POST /api/traders/:id/start|stop` - Trader control
- Updated documentation for all API endpoints

#### Modernized Codebase
- Type safety with proper separation of configuration types
- Database abstraction with prepared statements
- Comprehensive error handling and validation
- Better code organization (database, API, business logic)

### Changed
- **BREAKING**: Old `config.json` files no longer used
- Configuration must be done through web interface
- Much easier setup and better UX
- No more server restarts for configuration changes

### Why This Matters
- 🎯 **User Experience**: Much easier to configure and manage
- 🔧 **Flexibility**: Create any combination of AI models and exchanges
- 📊 **Scalability**: Support for complex multi-trader setups
- 🔒 **Reliability**: Database ensures data persistence and consistency
- 🚀 **Future-Proof**: Foundation for advanced features

---

## [2.0.2] - 2025-10-29

### Fixed - Critical Bug Fixes: Trade History & Performance Analysis

#### PnL Calculation - Major Error Fixed
- **Fixed**: PnL now calculated as actual USDT amount instead of percentage only
- Previously ignored position size and leverage (e.g., 100 USDT @ 5% = 1000 USDT @ 5%)
- Now: `PnL (USDT) = Position Value × Price Change % × Leverage`
- Impact: Win rate, profit factor, and Sharpe ratio now accurate

#### Position Tracking - Missing Critical Data
- **Fixed**: Open position records now store quantity and leverage
- Previously only stored price and time
- Essential for accurate PnL calculations

#### Position Key Logic - Long/Short Conflict
- **Fixed**: Changed from `symbol` to `symbol_side` format
- Now properly distinguishes between long and short positions
- Example: `BTCUSDT_long` vs `BTCUSDT_short`

#### Sharpe Ratio Calculation - Code Optimization
- **Changed**: Replaced custom Newton's method with `math.Sqrt`
- More reliable, maintainable, and efficient

### Why This Matters
- Historical trade statistics now show real USDT profit/loss
- Performance comparison between different leverage trades is accurate
- AI self-learning mechanism receives correct feedback
- Multi-position tracking (long + short simultaneously) works correctly

---

## [2.0.2] - 2025-10-29

### Fixed - Aster Exchange Precision Error

- Fixed Aster exchange precision error (code -1111)
- Improved price and quantity formatting to match exchange requirements
- Added detailed precision processing logs for debugging
- Enhanced all order functions with proper precision handling

#### Technical Details
- Added `formatFloatWithPrecision` function
- Price and quantity formatted according to exchange specifications
- Trailing zeros removed to optimize API requests

---

## [2.0.1] - 2025-10-29

### Fixed - ComparisonChart Data Processing

- Fixed ComparisonChart data processing logic
- Switched from cycle_number to timestamp grouping
- Resolved chart freezing issue when backend restarts
- Improved chart data display (shows all historical data chronologically)
- Enhanced debugging logs

---

## [2.0.0] - 2025-10-28

### Added - Major Updates

- AI self-learning mechanism (historical feedback, performance analysis)
- Multi-trader competition mode (Qwen vs DeepSeek)
- Binance-style UI (complete interface imitation)
- Performance comparison charts (real-time ROI comparison)
- Risk control optimization (per-coin position limit adjustment)

### Fixed

- Fixed hardcoded initial balance issue
- Fixed multi-trader data sync issue
- Optimized chart data alignment (using cycle_number)

---

## [1.0.0] - 2025-10-27

### Added - Initial Release

- Basic AI trading functionality
- Decision logging system
- Simple Web interface
- Support for Binance Futures
- DeepSeek and Qwen AI model integration

---

## How to Use This Changelog

### For Users
- Check the [Unreleased] section for upcoming features
- Review version sections to understand what changed
- Follow migration guides for breaking changes

### For Contributors
When making changes, add them to the [Unreleased] section under appropriate categories:
- **Added** - New features
- **Changed** - Changes to existing functionality
- **Deprecated** - Features that will be removed
- **Removed** - Features that were removed
- **Fixed** - Bug fixes
- **Security** - Security fixes

When releasing a new version, move [Unreleased] items to a new version section with date.

---

## Links

- [Documentation](docs/README.md)
- [Contributing Guidelines](CONTRIBUTING.md)
- [Security Policy](SECURITY.md)
- [GitHub Repository](https://github.com/tinkle-community/nofx)

---

**Last Updated:** 2025-11-01
