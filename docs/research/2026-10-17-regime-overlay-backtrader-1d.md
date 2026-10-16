# US-stock portfolios — Backtrader (1d)

Engine: Backtrader 1.9.78.123; initial cash 100,000.00; commission 0.10% per side; maxpos 10; momentum rebalance every 10 SPY sessions.

Native Cerebro / BackBroker / Market orders; no cheat-on-close/open; **no slippage**. Signals at close t execute at next SPY bar open t+1. Fractional shares; no leverage. Shares are sized at the signal close and capped to actual cash at the fill open; sells precede buys. A missing next-SPY open cancels the order without promoting another candidate. Open positions remain marked to market at the end; no terminal liquidation.

## Strategy comparison

| Strategy | CAGR | Ann vol | Sharpe | Sortino | MaxDD | Exposure | Trades | Win rate | Avg win $ | Avg loss $ | PF | Avg bars | Commission | Last 12m |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| SPY_BH | 17.25% | 16.02% | 1.078 | 1.612 | 18.76% | 100.00% | 0 | n/a | n/a | n/a | n/a | n/a | 99.90 | 18.61% |
| EW_BH | 55.58% | 32.78% | 1.520 | 2.284 | 32.56% | 99.82% | 0 | n/a | n/a | n/a | n/a | n/a | 1,014.68 | 82.61% |
| MOM_TOP+NONE | 94.70% | 54.10% | 1.510 | 2.237 | 51.43% | 97.99% | 175 | 54.29% | 12050.231 | -3815.334 | 3.751 | 52.057 | 15,523.82 | 123.60% |
| MOM_TOP+RV_PCT | 76.05% | 45.21% | 1.484 | 2.229 | 36.98% | 82.60% | 175 | 54.86% | 7378.601 | -2209.570 | 4.058 | 52.057 | 13,640.48 | 121.58% |
| MOM_TOP+RV_RATIO | 84.63% | 49.35% | 1.497 | 2.234 | 43.19% | 89.71% | 175 | 54.86% | 9060.623 | -2724.723 | 4.041 | 52.057 | 13,346.47 | 138.98% |
| MOM_TOP+BREADTH | 61.79% | 44.16% | 1.318 | 1.911 | 36.29% | 85.68% | 175 | 52.57% | 6506.051 | -2374.381 | 3.037 | 52.057 | 12,465.50 | 67.10% |
| MOM_TOP+VIX_TS | 78.37% | 49.15% | 1.431 | 2.110 | 50.04% | 89.99% | 175 | 53.71% | 9122.832 | -3277.007 | 3.231 | 52.057 | 16,532.07 | 91.57% |
| MOM_TOP+BASKET_VOL | 77.30% | 38.45% | 1.690 | 2.566 | 29.53% | 80.39% | 175 | 56.00% | 8317.702 | -2430.336 | 4.356 | 52.057 | 11,476.99 | 77.46% |
| MOM_TOP+COMBO | 71.72% | 45.01% | 1.434 | 2.118 | 41.52% | 85.55% | 175 | 54.29% | 7420.004 | -2475.045 | 3.560 | 52.057 | 11,441.62 | 90.59% |
| MOM_TOP+HARD_RV | 67.59% | 46.68% | 1.346 | 2.011 | 42.26% | 81.00% | 242 | 56.20% | 7725.901 | -2692.572 | 3.681 | 33.331 | 14,726.57 | 80.61% |
| MOM_TOP+HARD_VIX | 68.38% | 51.84% | 1.271 | 1.852 | 51.43% | 93.09% | 340 | 48.53% | 7227.672 | -3201.407 | 2.129 | 26.353 | 21,504.78 | 83.87% |
| MOM_TOP+HARD_BREADTH | 62.54% | 47.15% | 1.273 | 1.862 | 38.51% | 86.47% | 309 | 48.87% | 7335.927 | -2996.097 | 2.340 | 28.227 | 18,107.30 | 112.32% |

Daily TimeReturn equity returns: 252 sessions/year, sample volatility, Sharpe rf=0; Sortino uses RMS negative returns over all sessions. CAGR uses elapsed calendar time (365.25 days/year). DrawDown and TradeAnalyzer are native analyzers; trades mean fully closed Backtrader trades, including breakeven as wins. Average win/loss and profit factor use commission-net dollar P&L. Exposure is averaged over execution bars after the initial mark.

| Strategy | DD peak | DD trough | Open positions | Pending terminal orders | Missing opens / Cash-slot zero fills | Cash/slot size caps |
|---|---|---|---:|---:|---:|---:|
| SPY_BH | 2025-02-19 00:00:00 | 2025-04-08 00:00:00 | 1 | 0 | 0 / 0 | 1 |
| EW_BH | 2025-02-18 00:00:00 | 2025-04-08 00:00:00 | 64 | 0 | 0 / 0 | 7 |
| MOM_TOP+NONE | 2026-06-22 00:00:00 | 2026-07-29 00:00:00 | 10 | 0 | 0 / 3 | 41 |
| MOM_TOP+RV_PCT | 2026-06-04 00:00:00 | 2026-07-29 00:00:00 | 10 | 0 | 0 / 1 | 22 |
| MOM_TOP+RV_RATIO | 2026-06-22 00:00:00 | 2026-07-29 00:00:00 | 10 | 0 | 0 / 1 | 23 |
| MOM_TOP+BREADTH | 2026-06-22 00:00:00 | 2026-07-29 00:00:00 | 10 | 0 | 0 / 1 | 34 |
| MOM_TOP+VIX_TS | 2026-06-22 00:00:00 | 2026-07-29 00:00:00 | 10 | 0 | 0 / 3 | 28 |
| MOM_TOP+BASKET_VOL | 2025-02-14 00:00:00 | 2025-04-08 00:00:00 | 10 | 0 | 0 / 1 | 17 |
| MOM_TOP+COMBO | 2026-06-22 00:00:00 | 2026-07-29 00:00:00 | 10 | 0 | 0 / 1 | 17 |
| MOM_TOP+HARD_RV | 2026-06-04 00:00:00 | 2026-07-29 00:00:00 | 10 | 0 | 0 / 1 | 36 |
| MOM_TOP+HARD_VIX | 2026-06-22 00:00:00 | 2026-07-29 00:00:00 | 10 | 0 | 0 / 2 | 43 |
| MOM_TOP+HARD_BREADTH | 2026-06-22 00:00:00 | 2026-07-16 00:00:00 | 10 | 0 | 0 / 2 | 46 |

## Calendar-year returns

| Year | SPY_BH | EW_BH | MOM_TOP+NONE | MOM_TOP+RV_PCT | MOM_TOP+RV_RATIO | MOM_TOP+BREADTH | MOM_TOP+VIX_TS | MOM_TOP+BASKET_VOL | MOM_TOP+COMBO | MOM_TOP+HARD_RV | MOM_TOP+HARD_VIX | MOM_TOP+HARD_BREADTH |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 2022 | -8.89% | -24.36% | -12.70% | -12.50% | -13.10% | -12.70% | -11.40% | -13.19% | -12.86% | -8.19% | -12.79% | -12.70% |
| 2023 | 26.18% | 70.03% | 67.62% | 64.37% | 64.91% | 63.49% | 66.18% | 68.88% | 65.47% | 67.62% | 67.62% | 67.62% |
| 2024 | 24.89% | 65.37% | 98.16% | 65.68% | 87.93% | 45.74% | 83.18% | 76.90% | 58.05% | 59.06% | 58.17% | 19.31% |
| 2025 | 17.72% | 74.75% | 121.72% | 71.60% | 83.26% | 90.88% | 89.96% | 121.86% | 94.02% | 73.88% | 73.49% | 82.47% |
| 2026 | 14.98% | 70.32% | 151.20% | 159.38% | 162.14% | 87.74% | 118.67% | 89.86% | 116.20% | 102.91% | 119.57% | 138.53% |

First/last calendar years may be partial. Last 12m uses the last mark on/before the prior anniversary; a shorter history is flagged in JSON as last_12_months_partial.

## MA2060 rule breakdown


## 仓位缩放（regime overlay）

| Run | CAGR | Ann vol | Sharpe | Sortino | MaxDD | DD peak → trough | Calmar | Avg exposure | Avg s | s<1 sessions | Scale trades | Scale commission | Total commission | Last 12m |
|---|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|
| MOM_TOP+NONE | 94.70% | 54.10% | 1.510 | 2.237 | 51.43% | 2026-06-22 → 2026-07-29 | 1.841 | 97.99% | 1.000 | 0.00% | 0 | 0.00 | 15,523.82 | 123.60% |
| MOM_TOP+RV_PCT | 76.05% | 45.21% | 1.484 | 2.229 | 36.98% | 2026-06-04 → 2026-07-29 | 2.057 | 82.60% | 0.835 | 42.08% | 416 | 4,000.03 | 13,640.48 | 121.58% |
| MOM_TOP+RV_RATIO | 84.63% | 49.35% | 1.497 | 2.234 | 43.19% | 2026-06-22 → 2026-07-29 | 1.959 | 89.71% | 0.913 | 42.08% | 110 | 1,310.01 | 13,346.47 | 138.98% |
| MOM_TOP+BREADTH | 61.79% | 44.16% | 1.318 | 1.911 | 36.29% | 2026-06-22 → 2026-07-29 | 1.703 | 85.68% | 0.874 | 33.49% | 410 | 3,451.31 | 12,465.50 | 67.10% |
| MOM_TOP+VIX_TS | 78.37% | 49.15% | 1.431 | 2.110 | 50.04% | 2026-06-22 → 2026-07-29 | 1.566 | 89.99% | 0.919 | 40.46% | 400 | 4,357.52 | 16,532.07 | 91.57% |
| MOM_TOP+BASKET_VOL | 77.30% | 38.45% | 1.690 | 2.566 | 29.53% | 2025-02-14 → 2025-04-08 | 2.617 | 80.39% | 0.815 | 59.64% | 130 | 1,287.97 | 11,476.99 | 77.46% |
| MOM_TOP+COMBO | 71.72% | 45.01% | 1.434 | 2.118 | 41.52% | 2026-06-22 → 2026-07-29 | 1.728 | 85.55% | 0.876 | 63.07% | 150 | 1,241.35 | 11,441.62 | 90.59% |
| MOM_TOP+HARD_RV | 67.59% | 46.68% | 1.346 | 2.011 | 42.26% | 2026-06-04 → 2026-07-29 | 1.599 | 81.00% | 0.820 | 18.03% | 195 | 5,588.28 | 14,726.57 | 80.61% |
| MOM_TOP+HARD_VIX | 68.38% | 51.84% | 1.271 | 1.852 | 51.43% | 2026-06-22 → 2026-07-29 | 1.330 | 93.09% | 0.948 | 5.15% | 326 | 10,594.48 | 21,504.78 | 83.87% |
| MOM_TOP+HARD_BREADTH | 62.54% | 47.15% | 1.273 | 1.862 | 38.51% | 2026-06-22 → 2026-07-16 | 1.624 | 86.47% | 0.885 | 11.55% | 310 | 9,998.03 | 18,107.30 | 112.32% |

Fixed a priori rules, not optimized; MOM_TOP only, daily signals. Ordinary rebalances apply s/maxpos; between rebalances a band breach or zero transition resizes existing holdings. A zero-to-positive restart with an empty portfolio selects the current rank basket. Scale trades count filled orders tagged SCALE on these extra resize days (not complete round trips); their actual native commission is shown separately. Rebalance fills retain REBALANCE tags. Avg s and s<1 use every evaluation signal close, including the final unfilled signal; exposure is the broker-observed holding weight after fills.

Percentiles use inclusive rolling average rank/n over 252 observations, minimum 126. Unavailable inputs use neutral s=1; RV_RATIO uses the available trailing median after its first 21 returns. Breadth divides above-SMA200 rank-valid names by all rank-valid names, including names below SMA200. BASKET_VOL uses simple daily returns of a constant current top-maxpos basket; a constituent needs all 21 session returns (22 closes), with no stock-price forward fill. SPY RV uses log returns. All volatility uses sample standard deviation and sqrt(252).

Breadth uses today’s listed universe → survivorship bias. Local VIX and VIX3M are independently forward filled from dates ≤ signal date, never backfilled. VIX closes at 16:15 ET versus stocks at 16:00 ET; orders fill next open, after both closes. All calendar-year returns appear in the table above.

### Calendar-half robustness

First half: 524 sessions through 2024-09-10; second half: 524 sessions from 2024-09-11. Same equity curves, no refit; all thresholds fixed a priori. The second half includes its first session’s return using the preceding close as an anchor. Drawdown restarts at each half’s anchor; CAGR uses elapsed calendar time.

| Run | Half | Return | CAGR | Ann vol | Sharpe | Sortino | MaxDD | Calmar | Exposure |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|
| MOM_TOP+NONE | first_half | 70.00% | 28.96% | 36.38% | 0.884 | 1.329 | 27.28% | 1.062 | 96.21% |
| MOM_TOP+NONE | second_half | 850.03% | 193.80% | 67.17% | 1.952 | 2.875 | 51.43% | 3.768 | 99.77% |
| MOM_TOP+RV_PCT | first_half | 62.65% | 26.26% | 31.54% | 0.900 | 1.350 | 23.00% | 1.142 | 86.29% |
| MOM_TOP+RV_PCT | second_half | 552.14% | 145.37% | 55.53% | 1.904 | 2.858 | 36.98% | 3.931 | 78.91% |
| MOM_TOP+RV_RATIO | first_half | 67.05% | 27.88% | 33.78% | 0.900 | 1.351 | 25.97% | 1.074 | 91.38% |
| MOM_TOP+RV_RATIO | second_half | 674.46% | 166.42% | 60.97% | 1.923 | 2.861 | 43.19% | 3.853 | 88.04% |
| MOM_TOP+BREADTH | first_half | 49.07% | 21.09% | 35.27% | 0.721 | 1.063 | 30.20% | 0.698 | 93.69% |
| MOM_TOP+BREADTH | second_half | 400.07% | 116.09% | 51.48% | 1.765 | 2.540 | 36.29% | 3.199 | 77.69% |
| MOM_TOP+VIX_TS | first_half | 74.54% | 30.60% | 34.04% | 0.958 | 1.463 | 22.66% | 1.350 | 89.83% |
| MOM_TOP+VIX_TS | second_half | 541.84% | 143.51% | 60.52% | 1.784 | 2.601 | 50.04% | 2.868 | 90.15% |
| MOM_TOP+BASKET_VOL | first_half | 73.92% | 30.38% | 33.56% | 0.962 | 1.443 | 26.76% | 1.135 | 90.83% |
| MOM_TOP+BASKET_VOL | second_half | 528.17% | 141.01% | 42.72% | 2.285 | 3.490 | 29.53% | 4.775 | 69.97% |
| MOM_TOP+COMBO | first_half | 56.31% | 23.88% | 33.37% | 0.811 | 1.212 | 28.01% | 0.852 | 90.20% |
| MOM_TOP+COMBO | second_half | 511.58% | 137.95% | 54.12% | 1.884 | 2.767 | 41.52% | 3.323 | 80.91% |
| MOM_TOP+HARD_RV | first_half | 82.51% | 33.43% | 32.44% | 1.055 | 1.606 | 21.39% | 1.563 | 86.83% |
| MOM_TOP+HARD_RV | second_half | 373.20% | 110.45% | 57.46% | 1.590 | 2.362 | 42.26% | 2.613 | 75.18% |
| MOM_TOP+HARD_VIX | first_half | 51.56% | 22.06% | 36.55% | 0.731 | 1.070 | 32.86% | 0.671 | 94.82% |
| MOM_TOP+HARD_VIX | second_half | 481.13% | 132.20% | 63.47% | 1.655 | 2.405 | 51.43% | 2.571 | 91.36% |
| MOM_TOP+HARD_BREADTH | first_half | 39.62% | 17.35% | 35.86% | 0.628 | 0.910 | 34.52% | 0.503 | 93.54% |
| MOM_TOP+HARD_BREADTH | second_half | 444.26% | 125.03% | 56.14% | 1.736 | 2.547 | 38.51% | 3.247 | 79.42% |

### NONE drawdown episodes: same-window returns

Three worst non-overlapping high-water-mark episodes (including an unrecovered final episode), ranked by NONE peak-to-trough drawdown. Entries are close-to-close portfolio returns over the exact same dates, including costs.

| NONE peak → trough | NONE MaxDD | MOM_TOP+NONE | MOM_TOP+RV_PCT | MOM_TOP+RV_RATIO | MOM_TOP+BREADTH | MOM_TOP+VIX_TS | MOM_TOP+BASKET_VOL | MOM_TOP+COMBO | MOM_TOP+HARD_RV | MOM_TOP+HARD_VIX | MOM_TOP+HARD_BREADTH |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 2026-06-22 → 2026-07-29 | 51.43% | -51.43% | -36.49% | -43.19% | -36.29% | -50.04% | -23.69% | -41.52% | -32.32% | -51.43% | -31.26% |
| 2025-02-14 → 2025-04-08 | 42.36% | -42.36% | -30.97% | -36.89% | -24.31% | -39.10% | -29.53% | -29.83% | -23.78% | -39.58% | -18.24% |
| 2025-11-03 → 2025-11-21 | 29.30% | -29.30% | -28.43% | -27.72% | -29.30% | -26.03% | -19.39% | -27.40% | -29.30% | -31.60% | -29.30% |

### Sensitivity (fixed one-factor-at-a-time grid)

Defaults: floor=0.25, band=0.20, target-vol=0.40. Shared default cases run once; the grid is descriptive, with no parameter selection or refitting.

| Scaler | Floor | Band | Target vol | CAGR | Sharpe | MaxDD | Calmar |
|---|---:|---:|---:|---:|---:|---:|---:|
| RV_PCT | 0.00 | 0.20 | 40.00% | 69.04% | 1.438 | 35.54% | 1.942 |
| RV_PCT | 0.25 | 0.20 | 40.00% | 76.05% | 1.484 | 36.98% | 2.057 |
| RV_PCT | 0.50 | 0.20 | 40.00% | 87.66% | 1.570 | 39.76% | 2.205 |
| COMBO | 0.00 | 0.20 | 40.00% | 65.44% | 1.398 | 33.71% | 1.942 |
| COMBO | 0.25 | 0.20 | 40.00% | 71.72% | 1.434 | 41.52% | 1.728 |
| COMBO | 0.50 | 0.20 | 40.00% | 80.24% | 1.478 | 44.15% | 1.818 |
| BASKET_VOL | 0.25 | 0.20 | 30.00% | 62.13% | 1.711 | 22.53% | 2.758 |
| BASKET_VOL | 0.25 | 0.20 | 40.00% | 77.30% | 1.690 | 29.53% | 2.617 |
| BASKET_VOL | 0.25 | 0.20 | 50.00% | 86.35% | 1.657 | 36.08% | 2.393 |
| COMBO | 0.25 | 0.10 | 40.00% | 75.05% | 1.475 | 38.79% | 1.935 |
| COMBO | 0.25 | 0.35 | 40.00% | 76.12% | 1.486 | 40.26% | 1.891 |


## Coverage

75 loaded eligible stocks out of 75 cached candidates; SPY calendar 2022-08-10 00:00:00 → 2026-10-13 00:00:00; 1048 evaluation bars (1300 SPY bars including warmup).

Excluded (SPY/QQQ benchmark-only): KORUBUSDT, MUUBUSDT, QQQBUSDT, SOXLBUSDT, SOXSBUSDT, SPYBUSDT, SQQQBUSDT, TQQQBUSDT, UVXYBUSDT.

| Symbol | Execution bars | Daily bars | Evaluation bars matched | First | Last | Source |
|---|---:|---:|---:|---|---|---|
| AAOIBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| AAPLBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| ADBEBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| AGPUBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| ALABBUSDT | 644 | 644 | 644 | 2024-03-20 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| AMATBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| AMCBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| AMDBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| AMZNBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| ARMBUSDT | 773 | 773 | 773 | 2023-09-14 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| ASMLBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| ASTSBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| AVGOBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| AXTIBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| BABABUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| BEBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| BMNRBUSDT | 341 | 341 | 341 | 2025-06-05 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| BNCBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| COHRBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| COINBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| CRCLBUSDT | 341 | 341 | 341 | 2025-06-05 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| CRDOBUSDT | 1182 | 1182 | 1048 | 2022-01-27 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| CRMBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| CRWDBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| CRWVBUSDT | 388 | 388 | 388 | 2025-03-28 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| CYPHBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| DELLBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| DJTBUSDT | 1264 | 1264 | 1048 | 2021-09-30 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| EWYBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| FLNCBUSDT | 1244 | 1244 | 1048 | 2021-10-28 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| FWDIBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| GLWBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| GMEBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| GOOGLBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| GPROBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| GSBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| HIMSBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| HOODBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| HPEBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| IBMBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| INTCBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| INTWBUSDT | 418 | 418 | 418 | 2025-02-13 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| IRENBUSDT | 1230 | 1230 | 1048 | 2021-11-17 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| JPMBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| LITEBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| LLYBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| MDBBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| METABUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| MRNABUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| MRVLBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| MSFTBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| MSTRBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| MUBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| MVLLBUSDT | 403 | 403 | 403 | 2025-03-07 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| NBISBUSDT | 496 | 496 | 496 | 2024-10-21 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| NFLXBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| NOKBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| NVDABUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| ORCLBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| PDDBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| PENGBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| PLTRBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| PYPLBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| QCOMBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| RDDTBUSDT | 643 | 643 | 643 | 2024-03-21 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| RKLBBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| SMCIBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| SMHBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| SNDKBUSDT | 418 | 418 | 418 | 2025-02-13 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| SPYBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| TSLABUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| TSMBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| USARBUSDT | 815 | 815 | 815 | 2023-07-17 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| WDCBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| WENBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |
| ZMBUSDT | 1300 | 1300 | 1048 | 2021-08-10 00:00:00 | 2026-10-13 00:00:00 | yahoo |

Skipped: {}

Warnings: {}

Removed invalid/duplicate bars: {}

## Caveats

- Survivorship bias: a current cached universe, without historical listing/delisting membership or delisting proceeds.
- Yahoo adjusted underlying regular-session OHLCV differs from executable bStock tokens (hours, spreads, adjustments, liquidity). No bStock trading, volume constraint, taxes, borrow, dividends beyond the vendor adjustment, or market impact is modeled.
- No LLM layer: these are mechanical rules. 1h Yahoo history covers only ~2 years (about 730 days), with 60m/last half-hour bars.
- 1h MA/ATR windows count hourly bars; ranking uses the previous completed daily session. The common warmup is SPY rank-valid (253 daily bars), at least 61 execution bars, and the session’s last bar. New listings use prenext and do not postpone that start; signals only trade current, synchronized feeds.
- MOM_X_MA selects top N before timing veto, without backfill; EW freezes the first-date universe and rebalances every 63 execution bars, with an explicit all-name exception to maxpos. X1–X4 precede E4 TARGET; X4 uses actual entry fill and signal ATR.
