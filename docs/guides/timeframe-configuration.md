# K-Line Timeframe Configuration Guide

## Overview

NOFX supports a comprehensive set of K-line timeframes for AI trading analysis. You can select multiple timeframes to give your AI trader different perspectives on market movements.

## Supported Timeframes

### Scalping (Ultra-Short Term)
- **1m** - 1 minute
- **3m** - 3 minutes
- **5m** - 5 minutes ✨

### Intraday (Day Trading)
- **15m** - 15 minutes
- **30m** - 30 minutes ✨
- **1h** - 1 hour ✨

### Swing Trading
- **2h** - 2 hours
- **4h** - 4 hours
- **6h** - 6 hours
- **12h** - 12 hours

### Position Trading (Long-Term)
- **1d** - 1 day
- **3d** - 3 days
- **1w** - 1 week

✨ = Commonly requested timeframes now confirmed supported

## Default Configuration

The system comes pre-configured with these timeframes:
- **Primary**: 5m (5 minutes)
- **Selected**: 5m, 15m, 1h, 4h
- **K-line Count**: 30 candles per timeframe

## How to Configure Timeframes

### 1. Via Strategy Studio (Web Interface)

1. Navigate to **Strategy Studio** in the web interface
2. Select your strategy or create a new one
3. Find the **Indicator Configuration** section
4. Locate the **Timeframes** panel
5. Click timeframes to select/deselect them
6. Double-click a timeframe to set it as **Primary** (marked with ★)
7. Adjust **K-line Count** (10-200) to control how many historical candles to analyze
8. Save your strategy

### 2. Via Configuration File

Edit your strategy configuration JSON:

```json
{
  "indicators": {
    "klines": {
      "primary_timeframe": "5m",
      "primary_count": 30,
      "longer_timeframe": "4h",
      "longer_count": 10,
      "enable_multi_timeframe": true,
      "selected_timeframes": ["5m", "15m", "1h", "4h"]
    }
  }
}
```

### 3. Via API

When creating a trader via API:

```bash
curl -X POST http://localhost:8080/api/traders \
  -H "Content-Type: application/json" \
  -d '{
    "config": {
      "indicators": {
        "klines": {
          "selected_timeframes": ["5m", "30m", "1h", "4h"],
          "primary_timeframe": "5m",
          "primary_count": 30
        }
      }
    }
  }'
```

## Strategy Recommendations

### For Different Trading Styles

**Scalping** (Quick in-and-out trades):
```json
"selected_timeframes": ["1m", "3m", "5m", "15m"]
```

**Day Trading** (Intraday positions):
```json
"selected_timeframes": ["5m", "15m", "30m", "1h"]
```

**Swing Trading** (Multi-day holds):
```json
"selected_timeframes": ["1h", "4h", "6h", "1d"]
```

**Position Trading** (Long-term):
```json
"selected_timeframes": ["4h", "1d", "3d", "1w"]
```

## Multi-Timeframe Analysis

### Why Use Multiple Timeframes?

AI trading benefits from **multi-timeframe analysis** because:

1. **Trend Confirmation**: Shorter timeframes confirm entries, longer timeframes confirm trends
2. **Noise Reduction**: Longer timeframes filter out market noise
3. **Entry Precision**: Shorter timeframes provide precise entry/exit points
4. **Risk Management**: Multiple perspectives improve risk assessment

### Best Practices

1. **Combine Different Categories**: Use at least one short + one long timeframe
   - Example: `5m + 1h` or `15m + 4h`

2. **Primary Timeframe**: Choose based on your trading style
   - Scalpers: 1m or 3m
   - Day traders: 5m or 15m
   - Swing traders: 1h or 4h

3. **K-line Count**: More history = better context, but slower analysis
   - Fast decisions: 20-30 candles
   - Comprehensive: 50-100 candles
   - Deep analysis: 100-200 candles

4. **Don't Overload**: 3-5 timeframes is optimal
   - Too many = slower AI analysis
   - Too few = missing important context

## Technical Implementation

### Backend Support

File: `market/timeframe.go`

```go
var supportedTimeframes = map[string]time.Duration{
    "1m":  time.Minute,
    "3m":  3 * time.Minute,
    "5m":  5 * time.Minute,
    "15m": 15 * time.Minute,
    "30m": 30 * time.Minute,
    "1h":  time.Hour,
    "2h":  2 * time.Hour,
    "4h":  4 * time.Hour,
    "6h":  6 * time.Hour,
    "12h": 12 * time.Hour,
    "1d":  24 * time.Hour,
}
```

### Frontend Support

File: `web/src/components/strategy/IndicatorEditor.tsx`

The UI categorizes timeframes for easy selection:
- **Scalp** (超短): 1m, 3m, 5m
- **Intraday** (日内): 15m, 30m, 1h
- **Swing** (波段): 2h, 4h, 6h, 8h, 12h
- **Position** (趋势): 1d, 3d, 1w

### Validation

All timeframes are validated in:
- Backend: `NormalizeTimeframe()` function
- Frontend: Dropdown/button selection
- Config: Schema validation

## Examples

### Example 1: Conservative Day Trading Strategy

```json
{
  "selected_timeframes": ["15m", "30m", "1h", "4h"],
  "primary_timeframe": "30m",
  "primary_count": 50
}
```

**Analysis**: Uses 30-minute primary with 50 candles (25 hours history), plus 15m for entries, 1h/4h for trend confirmation.

### Example 2: Aggressive Scalping Strategy

```json
{
  "selected_timeframes": ["1m", "3m", "5m", "15m"],
  "primary_timeframe": "3m",
  "primary_count": 30
}
```

**Analysis**: Fast 3-minute primary with 30 candles (90 minutes history), plus 1m for quick entries, 5m/15m for trend context.

### Example 3: Balanced Swing Trading Strategy

```json
{
  "selected_timeframes": ["1h", "4h", "6h", "1d"],
  "primary_timeframe": "4h",
  "primary_count": 40
}
```

**Analysis**: 4-hour primary with 40 candles (160 hours = ~7 days history), plus 1h for precision, 6h/1d for major trends.

## Performance Considerations

### Data Volume

- Each timeframe requires fetching and processing K-line data
- More timeframes = More API calls = Slightly slower analysis
- Recommended: 3-5 timeframes for optimal balance

### Memory Usage

- System caches K-line data for performance
- Each symbol × timeframe combination is cached
- WebSocket subscriptions automatically optimized

### AI Analysis Time

- More timeframes = Richer context for AI
- AI processing time increases linearly with data volume
- Default scan interval (3 minutes) provides enough time for analysis

## Troubleshooting

### "Unsupported timeframe" Error

**Cause**: Typo or invalid timeframe string

**Solution**: Check spelling and use lowercase (e.g., `5m` not `5M`)

### Slow AI Analysis

**Cause**: Too many timeframes or too many K-lines

**Solution**: Reduce to 3-5 timeframes and K-line count to 30-50

### Missing Data

**Cause**: Exchange doesn't provide certain timeframes

**Solution**: Use supported timeframes (all Binance futures timeframes work)

## FAQ

**Q: Can I use only one timeframe?**
A: Yes, but multi-timeframe analysis improves decision quality.

**Q: What's the best timeframe for beginners?**
A: Start with `5m, 15m, 1h, 4h` - balanced approach for day trading.

**Q: How do I know which timeframe AI is using?**
A: The ★ (star) indicates the primary timeframe. All selected timeframes are analyzed.

**Q: Can I change timeframes while trader is running?**
A: Update strategy configuration, then restart the trader for changes to take effect.

**Q: Do I need different timeframes for backtest vs live?**
A: No - same timeframes work for both. Consistency ensures backtest results predict live performance.

## Summary

✅ **11 timeframes fully supported** (1m to 1w)
✅ **5m, 30m, 1h specifically confirmed working**
✅ **Default configuration uses 5m, 15m, 1h, 4h**
✅ **Easy configuration via web UI or JSON**
✅ **Comprehensive test coverage**

For more information, see:
- [Strategy Configuration Guide](../guides/strategy-configuration.md)
- [Backtest Configuration](../guides/backtest-guide.md)
- [API Documentation](../api/README.md)
