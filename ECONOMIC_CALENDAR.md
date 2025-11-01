# 📅 Economic Calendar Integration

## Overview

NOFX now includes **Economic Calendar Integration** to enhance AI trading decisions by providing awareness of upcoming macroeconomic events. This feature helps the AI avoid opening positions before high-impact events that could cause extreme market volatility.

## Features

- **Real-time Data Collection**: Automatically scrapes economic events from investing.com
- **SQLite Storage**: Lightweight local database for event storage
- **AI Integration**: Events are displayed in the AI decision prompt
- **Auto-Management**: Economic calendar service starts/stops automatically with NOFX
- **Configurable**: Control update intervals, time ranges, and importance levels

## Quick Start

### 1. Install Python Dependencies

```bash
cd world/经济日历
pip install -r requirements.txt
```

### 2. Enable in Config

Edit `config.json`:

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

### 3. Start NOFX

```bash
./nofx
```

The economic calendar service will start automatically!

## Configuration Options

| Field | Description | Default |
|-------|-------------|---------|
| `enabled` | Enable/disable economic calendar | `true` |
| `db_path` | Path to SQLite database | `world/经济日历/economic_calendar.db` |
| `script_path` | Path to Python scraper | `world/经济日历/economic_calendar_minimal.py` |
| `update_interval_seconds` | Data refresh interval (seconds) | `300` (5 min) |
| `hours_ahead` | Query events in next N hours | `24` |
| `min_importance` | Minimum importance level | `"高"` (High) |

## How It Works

```
┌─────────────────────────────────────────────────┐
│  Python Service (Background)                    │
│  - Scrapes investing.com every 5 minutes        │
│  - Stores events in SQLite database             │
└─────────────────────────────────────────────────┘
                     ↓
┌─────────────────────────────────────────────────┐
│  NOFX Decision Engine (Go)                      │
│  - Reads upcoming high-importance events        │
│  - Includes in AI decision prompt               │
└─────────────────────────────────────────────────┘
                     ↓
┌─────────────────────────────────────────────────┐
│  AI Trading Decision                            │
│  - Sees: "Fed Rate Decision in 2 hours"        │
│  - Decides: Avoid opening new positions         │
└─────────────────────────────────────────────────┘
```

## AI Prompt Example

When economic events are detected, the AI sees:

```markdown
## 📅 Upcoming Economic Events (Next 24 Hours)

1. [2 hours] US Core PCE Price Index MoM (United States) - High Importance
   Expected: 0.3% | Previous: 0.4%

⚠️ Note: High-impact events may cause extreme volatility. Consider:
- Avoid opening positions 1-2 hours before events
- Reduce leverage or position size
- Widen stop-loss ranges
```

## Database Schema

```sql
CREATE TABLE events (
    date TEXT,           -- Format: "dd/mm/yyyy"
    time TEXT,           -- Format: "HH:MM" or "All Day"
    zone TEXT,           -- Region: "United States", "Eurozone", etc.
    currency TEXT,       -- Currency code: "USD", "EUR", etc.
    event TEXT,          -- Event name
    importance TEXT,     -- "高" (High), "中" (Medium), "低" (Low)
    actual TEXT,         -- Actual value (if announced)
    forecast TEXT,       -- Forecast value
    previous TEXT        -- Previous value
);
```

## Manual Testing

Test the Python scraper manually:

```bash
cd world/经济日历
python3 economic_calendar_minimal.py --verbose --days 7
```

Check database:

```bash
sqlite3 economic_calendar.db "SELECT COUNT(*) FROM events WHERE importance = '高';"
```

## Troubleshooting

### Service Not Starting

```bash
# Check Python is installed
python3 --version

# Check script exists
ls -l world/经济日历/economic_calendar_minimal.py

# Check logs
tail -f world/经济日历/calendar.log
```

### No Events in Database

1. **Network Issues**: Script requires internet access to cn.investing.com
2. **Proxy Settings**: Configure in `world/经济日历/.env` if needed
3. **Database Permissions**: Ensure write access to database file

### Events Not Showing in AI Prompt

1. **Check Configuration**: Ensure `enabled: true` in config.json
2. **Check Time Range**: Default is 24 hours ahead
3. **Check Importance**: Only "高" (high) importance events shown by default

## Optional: Proxy Configuration

If you need a proxy to access investing.com:

```bash
cd world/经济日历
cp .env.example .env
nano .env
```

Edit proxy settings:

```env
USE_PROXY=true
HTTP_PROXY=http://your-proxy:port
HTTPS_PROXY=http://your-proxy:port
```

## Disable Feature

To disable economic calendar:

```json
{
  "economic_calendar": {
    "enabled": false
  }
}
```

Or remove the entire `economic_calendar` section from config.json.

## Technical Details

- **Language**: Python 3.6+
- **Dependencies**: `requests`, `lxml`, `pytz`
- **Database**: SQLite3 (no server required)
- **Integration**: Minimal changes to existing codebase
- **Error Handling**: Graceful fallback if service unavailable

## Why Economic Calendar?

Cryptocurrency markets are heavily influenced by macroeconomic events:

- 🇺🇸 **Fed Rate Decisions**: Major Bitcoin price movements
- 📊 **CPI/Inflation Data**: Affects risk appetite
- 💼 **Employment Reports**: Impact market sentiment
- 🏦 **Central Bank Speeches**: Can trigger volatility

By integrating economic calendar awareness, NOFX AI can make more informed decisions and avoid being caught in unexpected market swings.

---

**Note**: This feature is optional and fully backwards-compatible. Existing NOFX installations will continue working without any changes.
