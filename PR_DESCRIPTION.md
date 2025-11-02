# PR: Add Economic Calendar Integration for Risk-Aware Trading

## Summary

This PR adds **Economic Calendar Integration** to help AI make more informed trading decisions by providing awareness of upcoming macroeconomic events (Fed decisions, CPI data, employment reports, etc.).

## Motivation

Cryptocurrency markets are heavily influenced by macroeconomic events. This feature helps avoid opening positions before high-impact events that could cause extreme volatility.

## Changes

### Core Modifications

1. **decision/engine.go**:
   - Added `EconomicEvent` struct for event data
   - Added `EconomicEvents` field to `Context` struct
   - Added public `LoadEconomicEvents()` function for SQLite queries
   - Updated AI prompt builder to display economic events

2. **main.go**:
   - Added `EconomicCalendarConfig` struct for configuration
   - Modified `startEconomicCalendarService()` to accept config parameter
   - Reads `economic_calendar` section from config.json
   - Auto-starts/stops Python scraper service

3. **config.json.example**:
   - Added `economic_calendar` configuration section with sensible defaults

### New Files

- `ECONOMIC_CALENDAR.md` - English documentation
- `world/经济日历/` - Python scraper + SQLite database
- `world/经济日历/.env.example` - Proxy configuration template
- `.gitignore` - Added entries for .db, .log, .env files

## Configuration

```json
{
  "economic_calendar": {
    "enabled": true,
    "db_path": "world/经济日历/economic_calendar.db",
    "script_path": "world/经济日历/economic_calendar_minimal.py",
    "update_interval_seconds": 300,
    "hours_ahead": 24,
    "min_importance": "高"
  }
}
```

## Technical Details

- **Language**: Python 3.6+ (data collection), Go 1.21+ (integration)
- **Database**: SQLite3 (auto-created on first run)
- **Dependencies**: `requests`, `lxml`, `pytz` (Python)
- **Data Source**: investing.com economic calendar
- **Minimal Invasiveness**: Falls back gracefully if service unavailable

## Testing

- ✅ Compiled successfully with Go 1.21.6
- ✅ Backend auto-starts economic calendar service
- ✅ AI receives events in decision context
- ✅ No impact on existing functionality when disabled
- ✅ Database auto-created on first run
- ✅ Graceful handling of missing dependencies

## Breaking Changes

**None** - This feature is:
- Opt-in (requires `enabled: true` in config)
- Backwards compatible (works without Python if disabled)
- No changes to existing APIs or database schema

## Future Enhancements

- [ ] Multi-language support (English investing.com instead of Chinese)
- [ ] Go-based scraper to remove Python dependency
- [ ] Web UI for viewing upcoming events
- [ ] Integration with other data sources (tradingeconomics.com, etc.)

## Screenshots

[Add screenshots showing AI decision prompt with economic events]

---

**Ready for review!** This PR significantly enhances NOFX's risk management capabilities by making AI aware of market-moving events.
