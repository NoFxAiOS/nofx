# Market Regime Classification Framework

> A comprehensive market state identification system for quantitative trading strategy matching

---

## 1. Classification Dimensions Overview

Market state identification requires analysis across multiple dimensions:

| Dimension | Sub-dimensions | Description |
|-----------|---------------|-------------|
| **Trend** | Direction, Strength | Determine market movement direction and momentum |
| **Volatility** | Amplitude, Frequency | Measure price fluctuation characteristics |
| **Structure** | Pattern, Phase | Identify market structure and cycle position |

---

## 2. Primary Classification (5 Categories)

### 2.1 Classification Overview

| Code | Name | Key Characteristics | Suitable Strategies |
|------|------|---------------------|---------------------|
| `TREND_UP` | Uptrend | Higher highs & higher lows | Trend following, Breakout |
| `TREND_DOWN` | Downtrend | Lower highs & lower lows | Trend following, Short selling |
| `RANGE` | Range-bound | Price oscillates within bounds | Grid trading, Mean reversion |
| `TRANSITION` | Transition | Uncertain directional period | Wait & watch, Small positions |
| `BREAKOUT` | Breakout | Price breaks key levels | Breakout trading |

### 2.2 Identification Indicators

- **ADX (Average Directional Index)**: Measures trend strength
  - ADX > 25: Clear trend exists
  - ADX < 20: Range-bound market
- **EMA Alignment**: Determines trend direction
  - EMA20 > EMA50 > EMA200: Bullish alignment
  - EMA20 < EMA50 < EMA200: Bearish alignment

---

## 3. Secondary Classification (18 Sub-categories)

### 3.1 Uptrend Sub-categories (5 Types)

| Code | Name | Technical Features | Quantitative Indicators |
|------|------|-------------------|------------------------|
| `TU_STRONG_LOW_VOL` | Strong Uptrend · Low Vol | Steady rise, shallow pullbacks | ADX>40, ATR%<2%, Pullback<38.2% |
| `TU_STRONG_HIGH_VOL` | Strong Uptrend · High Vol | Rapid surge, high volatility | ADX>40, ATR%>4%, MACD histogram expanding |
| `TU_WEAK_CHOPPY` | Weak Uptrend · Choppy | Two steps forward, one back | ADX 20-30, RSI oscillating 50-70 |
| `TU_PARABOLIC` | Parabolic Acceleration | Exponential price increase | Price far from MA, RSI>80, Volume surge |
| `TU_EXHAUSTION` | Uptrend Exhaustion | New highs but weakening momentum | Price new high + MACD/RSI divergence |

**Strategy Matching:**
- Strong Low Vol: Heavy trend following, pyramid adding
- Strong High Vol: Medium position, trailing stops
- Weak Choppy: Light swing trading
- Parabolic: Cautious, prepare to exit
- Exhaustion: Reduce positions, prepare for reversal

### 3.2 Downtrend Sub-categories (5 Types)

| Code | Name | Technical Features | Quantitative Indicators |
|------|------|-------------------|------------------------|
| `TD_STRONG_LOW_VOL` | Strong Downtrend · Low Vol | Steady decline, weak bounces | ADX>40, ATR%<2%, Bounce<38.2% |
| `TD_STRONG_HIGH_VOL` | Strong Downtrend · High Vol | Panic selling, wild swings | ADX>40, ATR%>5%, VIX spike |
| `TD_WEAK_CHOPPY` | Weak Downtrend · Choppy | Grinding lower with bounces | ADX 20-30, RSI oscillating 30-50 |
| `TD_CAPITULATION` | Capitulation | High volume crash, extreme fear | RSI<20, Volume>3x average |
| `TD_EXHAUSTION` | Downtrend Exhaustion | New lows but selling pressure fading | Price new low + MACD/RSI divergence |

**Strategy Matching:**
- Strong Low Vol: Short trend following
- Strong High Vol: Stay flat or light hedge
- Weak Choppy: Wait for stabilization
- Capitulation: Light bottom fishing possible
- Exhaustion: Gradually build long positions

### 3.3 Range Sub-categories (4 Types)

| Code | Name | Technical Features | Quantitative Indicators |
|------|------|-------------------|------------------------|
| `RG_TIGHT_LOW_VOL` | Tight Range · Low Vol | Extreme contraction, coiling | BB Width<2%, ATR at new lows |
| `RG_TIGHT_HIGH_VOL` | Tight Range · High Vol | Violent swings within range | BB Width<3%, ATR%>3% |
| `RG_WIDE_LOW_VOL` | Wide Range · Low Vol | Large range, slow movement | BB Width>5%, ATR%<2% |
| `RG_WIDE_HIGH_VOL` | Wide Range · High Vol | Large range, fast movement | BB Width>5%, ATR%>3% |

**Strategy Matching:**
- Tight Low Vol: Dense grid, wait for breakout
- Tight High Vol: Fast grid, small frequent profits
- Wide Low Vol: Sparse grid, patient holding
- Wide High Vol: Swing trading, high profit targets

### 3.4 Transition (2 Types)

| Code | Name | Technical Features | Quantitative Indicators |
|------|------|-------------------|------------------------|
| `TR_BOTTOM_FORMING` | Bottom Forming | Decline slowing, testing support | Price stabilizing + Volume drying up + RSI divergence |
| `TR_TOP_FORMING` | Top Forming | Rally slowing, testing resistance | Price stalling + Volume drying up + RSI divergence |

### 3.5 Breakout (2 Types)

| Code | Name | Technical Features | Quantitative Indicators |
|------|------|-------------------|------------------------|
| `BK_UPWARD` | Upward Breakout | Breaking resistance with volume | Price>Previous high, Volume>2x, BB breakout |
| `BK_DOWNWARD` | Downward Breakout | Breaking support with volume | Price<Previous low, Volume>2x, BB breakdown |

---

## 4. Tertiary Classification (36 Ultra-fine Categories)

### 4.1 Trend Phase Classification

Uptrend lifecycle consists of 5 phases:

| Phase Code | Name | Description | Quantitative Criteria |
|------------|------|-------------|----------------------|
| `TU_S1_INITIATION` | Uptrend Initiation | First break above MA or previous high | MACD bullish cross, Price>EMA20 |
| `TU_S2_ACCELERATION` | Uptrend Acceleration | Momentum increasing, slope steepening | MACD histogram expanding, ADX rising |
| `TU_S3_MAIN_WAVE` | Main Wave | Sustained rise, shallow pullbacks | RSI 60-80, Pullbacks hold EMA20 |
| `TU_S4_EXHAUSTION` | Uptrend Exhaustion | Slowing momentum, divergences appearing | RSI divergence, MACD divergence |
| `TU_S5_REVERSAL` | Trend Reversal | Breakdown, trend ending | Break below EMA50, MACD bearish cross |

Downtrend phases follow same pattern: `TD_S1` through `TD_S5`

### 4.2 Range Position Classification

| Position Code | Name | Description | Strategy Suggestion |
|---------------|------|-------------|---------------------|
| `RG_UPPER` | Upper Range | Price near resistance | Bias toward short |
| `RG_MIDDLE` | Mid Range | Price near middle band | Neutral grid trading |
| `RG_LOWER` | Lower Range | Price near support | Bias toward long |
| `RG_SQUEEZE` | Squeeze Pattern | Highs and lows converging | Wait for direction |
| `RG_EXPAND` | Expanding Pattern | Highs and lows diverging | Boundary reversal |

### 4.3 Volatility Grades

| Code | Name | ATR% | BB Width | Strategy Suggestion |
|------|------|------|----------|---------------------|
| `VOL_EXTREME_LOW` | Extreme Low Vol | <1% | <1.5% | Option selling |
| `VOL_LOW` | Low Volatility | 1-2% | 1.5-2.5% | Grid / Mean reversion |
| `VOL_NORMAL` | Normal Volatility | 2-3% | 2.5-4% | Trend following |
| `VOL_HIGH` | High Volatility | 3-5% | 4-6% | Momentum / Breakout |
| `VOL_EXTREME_HIGH` | Extreme High Vol | >5% | >6% | Reduce exposure / Hedge |

---

## 5. Complete State Encoding Rules

### 5.1 Encoding Format

```
{Primary}_{Volatility}_{Phase}_{Position}
```

### 5.2 Encoding Examples

| Full Code | Interpretation |
|-----------|----------------|
| `TU_LV_S3_M` | Uptrend_LowVol_MainWave_Middle |
| `TD_HV_S2_L` | Downtrend_HighVol_Acceleration_Lower |
| `RG_NV_SQ_U` | Range_NormalVol_Squeeze_Upper |
| `BK_HV_UP_M` | Breakout_HighVol_Upward_Middle |

---

## 6. Core Identification Indicators

### 6.1 Trend Indicators

| Indicator | Calculation | Criteria |
|-----------|-------------|----------|
| ADX | 14-period Average Directional Index | >40 Strong, 25-40 Medium, <25 Weak/Range |
| Trend Score | Composite EMA/MACD/Price structure | -100 to +100, Positive=Bullish, Negative=Bearish |
| EMA Alignment | Relative position of EMA20/50/200 | Bullish/Bearish/Mixed alignment |

### 6.2 Volatility Indicators

| Indicator | Calculation | Purpose |
|-----------|-------------|---------|
| ATR Percent | ATR(14) / Current Price × 100% | Measure relative volatility |
| BB Width | (Upper - Lower) / Middle × 100% | Measure price range |
| Volatility Rank | Current vol percentile in history | Determine vol level |

### 6.3 Momentum Indicators

| Indicator | Calculation | Criteria |
|-----------|-------------|----------|
| RSI | 14-period Relative Strength Index | >70 Overbought, <30 Oversold, 50 Neutral |
| MACD Histogram | MACD - Signal | Positive=Bullish momentum, Negative=Bearish |
| Momentum Score | Composite RSI/MACD/Volume | Measure current momentum |

### 6.4 Structure Indicators

| Indicator | Description | Purpose |
|-----------|-------------|---------|
| Swing Structure | HH/HL/LH/LL sequence | Determine trend structure |
| Support/Resistance | Key price levels | Define trading range |
| Volume Profile | Volume-price relationship | Validate price action |

---

## 7. Strategy Matching Matrix

### 7.1 Regime-Strategy Mapping

| Regime Type | Recommended Strategy | Position Size | Stop Loss |
|-------------|---------------------|---------------|-----------|
| Strong Uptrend · Low Vol | Trend following + Pyramid | 60-80% | ATR×2 |
| Strong Uptrend · High Vol | Momentum + Quick profit | 40-60% | ATR×1.5 |
| Uptrend Exhaustion | Reduce + Reversal short | 20-30% | Previous high |
| Panic Decline | Wait or light bottom fish | 10-20% | Wide stop |
| Low Vol Range | Grid trading | 50-70% | Range boundary |
| High Vol Range | Swing trading | 30-50% | ATR×2 |
| Squeeze Pattern | Wait for breakout | 10-20% | - |
| Upward Breakout | Chase + Add on pullback | 50-70% | Breakout level |
| Bottom Formation | Scale in gradually | 20-40% | New low |

### 7.2 Grid Strategy Parameter Matching

| Range Type | Grid Levels | Grid Spacing | Other Parameters |
|------------|-------------|--------------|------------------|
| Tight Low Vol | 30-50 levels | Small spacing | Enable Maker Only |
| Tight High Vol | 15-25 levels | Small spacing | Fast execution mode |
| Wide Low Vol | 10-20 levels | Large spacing | Patient execution |
| Wide High Vol | 15-25 levels | Large spacing | High profit targets |
| Squeeze Pattern | Pause grid | - | Wait for breakout signal |
| Upper Range | Short bias | Medium | Increase sell weight |
| Lower Range | Long bias | Medium | Increase buy weight |

---

## 8. Real-time Monitoring Guidelines

### 8.1 State Transition Triggers

| Current State | Trigger Condition | Transitions To |
|---------------|-------------------|----------------|
| Range | Price breakout + Volume + ADX rising | Breakout |
| Uptrend | RSI divergence + Volume decline | Exhaustion |
| Downtrend | RSI divergence + Volume decline | Exhaustion |
| Breakout | Failed breakout, price returns | Range |
| Exhaustion | Confirmed reversal breakout | Opposite trend |

### 8.2 Risk Control Rules

| Regime State | Max Position | Risk Per Trade | Special Rules |
|--------------|--------------|----------------|---------------|
| Strong Trend | 80% | 2% | Adding allowed |
| Weak Trend | 50% | 1.5% | No adding |
| Range | 60% | 1% | Diversified holding |
| Transition | 30% | 1% | Reduce activity |
| High Volatility | 40% | 0.5% | Wide stops |

---

## 9. Appendix

### 9.1 Abbreviation Reference

| Abbrev | Full Form | Description |
|--------|-----------|-------------|
| TU | Trend Up | Upward trend |
| TD | Trend Down | Downward trend |
| RG | Range | Range-bound market |
| TR | Transition | Trend transition |
| BK | Breakout | Breakout pattern |
| LV | Low Volatility | Low volatility regime |
| HV | High Volatility | High volatility regime |
| NV | Normal Volatility | Normal volatility regime |
| XLV | Extreme Low Vol | Extremely low volatility |
| XHV | Extreme High Vol | Extremely high volatility |

### 9.2 Document Information

- Version: v1.0
- Created: January 2026
- Applicable: Cryptocurrency, Forex, Stocks, and other financial markets

---

*This document is designed for market state identification and strategy matching in quantitative trading systems*
