**Key results**  
Main 10d residual Spearman: mean IC **-0.0142**; ICIR **-0.10**.  
Main residual IC: NW t **-1.91**; hit rate **48.13%** (1014 dates).  
Long-setup-only IC: **-0.0094**; NW t **-0.49** (943 dates).  
Top-10 annualized residual: **1.16%**; Sharpe-like **0.03**.  
LongSetup vs rest residual spread: **-0.84%** per 10d; NW t **-3.31**.

# Kell price-cycle IC study

Signal: production `ClassifyCycle` score for LongSetup, otherwise zero. History: last 300 bars, at least 260. Entry: signal date + 1 close; exit: entry + horizon close. Beta: OLS on 126 daily SPY dates ending at signal, at least 60 matched return pairs. Cross-sections need 20 names. Main horizon: 10; rebalance/hold: 10; top N: 10; one-way cost: 10.00 bps.

## 1–2. IC series and summaries

| Horizon | Target | Method | Dates | Mean IC | Std | ICIR | NW t | 95% CI | Hit rate | Non-overlap n | Non-overlap mean | Plain t |
|---:|---|---|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|
| 1 | residual | Spearman | 1014 | -0.0040 | 0.1402 | -0.03 | -0.91 | [-0.0126, 0.0046] | 49.41% | 1014 | -0.0040 | -0.91 |
| 1 | residual | Pearson | 1014 | -0.0034 | 0.1417 | -0.02 | -0.77 | [-0.0121, 0.0053] | 50.00% | 1014 | -0.0034 | -0.77 |
| 1 | raw | Spearman | 1014 | -0.0041 | 0.1404 | -0.03 | -0.94 | [-0.0128, 0.0045] | 49.80% | 1014 | -0.0041 | -0.94 |
| 1 | raw | Pearson | 1014 | -0.0038 | 0.1430 | -0.03 | -0.84 | [-0.0126, 0.0050] | 50.20% | 1014 | -0.0038 | -0.84 |
| 5 | residual | Spearman | 1014 | -0.0139 | 0.1381 | -0.10 | -2.26 | [-0.0260, -0.0019] | 44.38% | 202 | -0.0177 | -1.83 |
| 5 | residual | Pearson | 1014 | -0.0148 | 0.1384 | -0.11 | -2.34 | [-0.0272, -0.0024] | 43.00% | 202 | -0.0233 | -2.42 |
| 5 | raw | Spearman | 1014 | -0.0153 | 0.1389 | -0.11 | -2.43 | [-0.0276, -0.0030] | 45.27% | 202 | -0.0154 | -1.55 |
| 5 | raw | Pearson | 1014 | -0.0162 | 0.1392 | -0.12 | -2.50 | [-0.0288, -0.0035] | 42.90% | 202 | -0.0218 | -2.21 |
| 10 | residual | Spearman | 1014 | -0.0142 | 0.1353 | -0.10 | -1.91 | [-0.0287, 0.0004] | 48.13% | 101 | -0.0208 | -1.48 |
| 10 | residual | Pearson | 1014 | -0.0203 | 0.1317 | -0.15 | -2.78 | [-0.0346, -0.0060] | 43.10% | 101 | -0.0306 | -2.12 |
| 10 | raw | Spearman | 1014 | -0.0165 | 0.1369 | -0.12 | -2.16 | [-0.0314, -0.0015] | 46.25% | 101 | -0.0174 | -1.22 |
| 10 | raw | Pearson | 1014 | -0.0217 | 0.1338 | -0.16 | -2.88 | [-0.0365, -0.0069] | 43.79% | 101 | -0.0273 | -1.85 |
| 20 | residual | Spearman | 1014 | -0.0225 | 0.1376 | -0.16 | -2.81 | [-0.0382, -0.0068] | 44.28% | 50 | -0.0104 | -0.57 |
| 20 | residual | Pearson | 1014 | -0.0290 | 0.1276 | -0.23 | -3.82 | [-0.0439, -0.0141] | 37.87% | 50 | -0.0239 | -1.31 |
| 20 | raw | Spearman | 1014 | -0.0239 | 0.1355 | -0.18 | -3.10 | [-0.0390, -0.0088] | 42.21% | 50 | -0.0118 | -0.68 |
| 20 | raw | Pearson | 1014 | -0.0295 | 0.1288 | -0.23 | -3.87 | [-0.0444, -0.0145] | 37.57% | 50 | -0.0210 | -1.17 |

Constant signal/target dates have undefined correlation and are excluded from the corresponding summary. NW uses Bartlett weights with lag h−1 and an n/(n−1) correction; at lag zero it equals the plain sample standard error. Missing dates retain their SPY calendar spacing. The non-overlapping check uses every h-th SPY date from warmup, even when intermediate dates are missing. Daily IC series are included in the optional JSON.

## 3. IC by calendar year

Main-horizon residual Spearman.

| Year | Dates | Mean IC | ICIR |
|---|---:|---:|---:|
| 2022 | 90 | -0.0071 | -0.05 |
| 2023 | 250 | -0.0323 | -0.25 |
| 2024 | 252 | 0.0194 | 0.15 |
| 2025 | 247 | -0.0311 | -0.23 |
| 2026 | 175 | -0.0163 | -0.12 |

## 4. Long-setup-only IC

Main-horizon residual; at least five long setups per date.

| Method | Dates | Mean IC | Std | ICIR | NW t | 95% CI | Hit rate | Non-overlap n | Non-overlap mean | Plain t |
|---|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|
| Spearman | 943 | -0.0094 | 0.3161 | -0.03 | -0.49 | [-0.0472, 0.0284] | 46.77% | 93 | -0.0291 | -0.85 |
| Pearson | 943 | -0.0092 | 0.3253 | -0.03 | -0.45 | [-0.0496, 0.0312] | 48.04% | 93 | -0.0299 | -0.92 |

## 5. Phase event study

Main-horizon residual returns; each date has equal weight.

| Phase | Mean return | NW t | Dates | Avg names/date |
|---|---:|---:|---:|---:|
| all stocks | 0.92% | 3.29 | 1020 | 67.5 |
| base_forming | 0.34% | 0.71 | 795 | 2.8 |
| base_n_break | 0.89% | 1.39 | 789 | 3.0 |
| downtrend | 0.65% | 1.47 | 1018 | 18.3 |
| ema_crossback | 0.11% | 0.24 | 996 | 9.3 |
| exhaustion_extension | 1.71% | 1.73 | 708 | 3.5 |
| insufficient_data | n/a | n/a | 0 | 0.0 |
| neutral | 0.93% | 2.69 | 1016 | 8.4 |
| reversal_extension | -3.13% | -0.74 | 7 | 1.0 |
| uptrend | 1.07% | 2.75 | 992 | 14.2 |
| wedge_drop | 0.25% | 0.52 | 999 | 8.2 |
| wedge_pop | -0.02% | -0.03 | 747 | 4.0 |

The all-stocks row is the residual baseline. Beta adjustment does not force its cross-sectional mean to zero.

## 6. LongSetup vs rest spread

| Dates | Mean residual spread | NW t |
|---:|---:|---:|
| 1014 | -0.84% | -3.31 |

## 7. Top-N portfolio

| Return series | Periods | Annualized mean | Annualized vol | Sharpe-like | Hit rate |
|---|---:|---:|---:|---:|---:|
| Residual | 103 | 1.16% | 37.75% | 0.03 | 50.49% |
| Excess vs EW universe | 103 | -21.39% | 31.75% | -0.67 | 47.57% |
| Excess net of costs | 103 | -25.52% | 31.78% | -0.80 | 46.60% |

Average names held: 9.07; average one-way turnover: 82.09%; skipped periods: 0.

Rank: LongSetup first, Score descending, RS63Pct descending, Symbol ascending; buy only LongSetup names, equal-weight fewer than N if necessary, cash if none. Rank before checking future prices; a missing selected entry/exit skips the period without substituting another stock. EW universe uses stocks with valid signal, beta and holding-period returns. One-way turnover is half the absolute change in equal weights including cash (fraction replaced for fixed-size baskets); it compares consecutive scheduled baskets, including skipped periods, and charges the initial purchase from cash. Net excess subtracts turnover × 2 × cost-bps / 10000. Annualization is arithmetic mean × 252/rebalance and sample std × sqrt(252/rebalance).

| Signal date | Entry | Exit | Names | Portfolio raw | Residual | EW universe raw | Excess | Turnover | Net excess |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|
| 2022-08-19 | 2022-08-22 | 2022-09-06 | 10 | -6.91% | 1.14% | -7.75% | 0.84% | 100.00% | 0.64% |
| 2022-09-02 | 2022-09-06 | 2022-09-20 | 1 | -37.04% | -34.51% | -2.11% | -34.93% | 100.00% | -35.13% |
| 2022-09-19 | 2022-09-20 | 2022-10-04 | 3 | -6.50% | -3.90% | -1.86% | -4.64% | 100.00% | -4.84% |
| 2022-10-03 | 2022-10-04 | 2022-10-18 | 3 | 0.06% | 1.96% | -6.55% | 6.60% | 66.67% | 6.47% |
| 2022-10-17 | 2022-10-18 | 2022-11-01 | 7 | 7.84% | 3.89% | 4.16% | 3.68% | 71.43% | 3.53% |
| 2022-10-31 | 2022-11-01 | 2022-11-15 | 10 | 0.36% | -5.33% | 9.07% | -8.71% | 80.00% | -8.87% |
| 2022-11-14 | 2022-11-15 | 2022-11-30 | 10 | -0.75% | -3.92% | -1.13% | 0.38% | 80.00% | 0.22% |
| 2022-11-29 | 2022-11-30 | 2022-12-14 | 10 | 3.85% | 6.38% | -1.89% | 5.75% | 90.00% | 5.57% |
| 2022-12-13 | 2022-12-14 | 2022-12-29 | 10 | -8.02% | -3.98% | -9.51% | 1.49% | 90.00% | 1.31% |
| 2022-12-28 | 2022-12-29 | 2023-01-13 | 4 | 5.44% | 0.75% | 15.16% | -9.71% | 100.00% | -9.91% |
| 2023-01-12 | 2023-01-13 | 2023-01-30 | 10 | -0.87% | -1.57% | 3.05% | -3.92% | 100.00% | -4.12% |
| 2023-01-27 | 2023-01-30 | 2023-02-13 | 10 | 4.33% | -0.78% | 2.90% | 1.43% | 80.00% | 1.27% |
| 2023-02-10 | 2023-02-13 | 2023-02-28 | 10 | -6.46% | 0.35% | -3.37% | -3.09% | 50.00% | -3.19% |
| 2023-02-27 | 2023-02-28 | 2023-03-14 | 9 | -9.69% | -7.95% | -4.25% | -5.44% | 80.00% | -5.60% |
| 2023-03-13 | 2023-03-14 | 2023-03-28 | 2 | 2.03% | 0.19% | 1.29% | 0.73% | 100.00% | 0.53% |
| 2023-03-27 | 2023-03-28 | 2023-04-12 | 10 | -0.67% | -5.45% | 3.39% | -4.06% | 90.00% | -4.24% |
| 2023-04-11 | 2023-04-12 | 2023-04-26 | 10 | -3.66% | -1.94% | -5.08% | 1.42% | 100.00% | 1.22% |
| 2023-04-25 | 2023-04-26 | 2023-05-10 | 7 | 2.50% | 0.37% | 4.65% | -2.15% | 100.00% | -2.35% |
| 2023-05-09 | 2023-05-10 | 2023-05-24 | 10 | 2.88% | 3.50% | 4.41% | -1.53% | 70.00% | -1.67% |
| 2023-05-23 | 2023-05-24 | 2023-06-08 | 10 | 4.90% | -2.33% | 10.41% | -5.51% | 60.00% | -5.63% |
| 2023-06-07 | 2023-06-08 | 2023-06-23 | 10 | 0.72% | -1.51% | 1.85% | -1.13% | 60.00% | -1.25% |
| 2023-06-22 | 2023-06-23 | 2023-07-10 | 10 | 7.32% | 4.98% | 5.12% | 2.19% | 80.00% | 2.03% |
| 2023-07-07 | 2023-07-10 | 2023-07-24 | 10 | 1.32% | -4.74% | 3.75% | -2.43% | 70.00% | -2.57% |
| 2023-07-21 | 2023-07-24 | 2023-08-07 | 10 | -6.93% | -5.44% | -0.93% | -6.00% | 90.00% | -6.18% |
| 2023-08-04 | 2023-08-07 | 2023-08-21 | 10 | -6.17% | -2.20% | -6.69% | 0.52% | 80.00% | 0.36% |
| 2023-08-18 | 2023-08-21 | 2023-09-05 | 1 | -16.13% | -20.21% | 4.36% | -20.48% | 90.00% | -20.66% |
| 2023-09-01 | 2023-09-05 | 2023-09-19 | 10 | -5.50% | -3.91% | -5.92% | 0.42% | 90.00% | 0.24% |
| 2023-09-18 | 2023-09-19 | 2023-10-03 | 10 | -6.79% | 1.84% | -4.75% | -2.04% | 70.00% | -2.18% |
| 2023-10-02 | 2023-10-03 | 2023-10-17 | 10 | 2.26% | -3.88% | 1.88% | 0.38% | 60.00% | 0.26% |
| 2023-10-16 | 2023-10-17 | 2023-10-31 | 10 | -5.01% | 1.25% | -5.71% | 0.70% | 70.00% | 0.56% |
| 2023-10-30 | 2023-10-31 | 2023-11-14 | 4 | 8.30% | -0.05% | 11.87% | -3.57% | 100.00% | -3.77% |
| 2023-11-13 | 2023-11-14 | 2023-11-29 | 10 | 9.64% | 7.52% | 6.78% | 2.86% | 100.00% | 2.66% |
| 2023-11-28 | 2023-11-29 | 2023-12-13 | 10 | 0.06% | -6.37% | 4.31% | -4.25% | 80.00% | -4.41% |
| 2023-12-12 | 2023-12-13 | 2023-12-28 | 10 | 3.86% | 1.23% | 6.96% | -3.10% | 90.00% | -3.28% |
| 2023-12-27 | 2023-12-28 | 2024-01-12 | 10 | -3.33% | -3.33% | -5.15% | 1.82% | 80.00% | 1.66% |
| 2024-01-11 | 2024-01-12 | 2024-01-29 | 10 | 10.81% | 5.46% | 5.69% | 5.12% | 60.00% | 5.00% |
| 2024-01-26 | 2024-01-29 | 2024-02-12 | 10 | 5.74% | 2.90% | 6.25% | -0.51% | 80.00% | -0.67% |
| 2024-02-09 | 2024-02-12 | 2024-02-27 | 10 | 0.77% | -1.81% | 2.55% | -1.78% | 80.00% | -1.94% |
| 2024-02-26 | 2024-02-27 | 2024-03-12 | 10 | 4.25% | 0.55% | 3.82% | 0.43% | 90.00% | 0.25% |
| 2024-03-11 | 2024-03-12 | 2024-03-26 | 10 | 5.48% | 4.12% | 1.60% | 3.88% | 70.00% | 3.74% |
| 2024-03-25 | 2024-03-26 | 2024-04-10 | 10 | -6.42% | -5.15% | -5.50% | -0.92% | 90.00% | -1.10% |
| 2024-04-09 | 2024-04-10 | 2024-04-24 | 10 | -3.70% | -1.12% | -3.69% | -0.00% | 90.00% | -0.18% |
| 2024-04-23 | 2024-04-24 | 2024-05-08 | 10 | 0.91% | -2.99% | 5.56% | -4.65% | 60.00% | -4.77% |
| 2024-05-07 | 2024-05-08 | 2024-05-22 | 10 | 12.29% | 7.41% | 8.68% | 3.61% | 60.00% | 3.49% |
| 2024-05-21 | 2024-05-22 | 2024-06-06 | 10 | 17.66% | 16.14% | 5.18% | 12.48% | 60.00% | 12.36% |
| 2024-06-05 | 2024-06-06 | 2024-06-21 | 10 | 1.30% | -2.10% | -1.04% | 2.34% | 70.00% | 2.20% |
| 2024-06-20 | 2024-06-21 | 2024-07-08 | 10 | 5.50% | 1.35% | 3.99% | 1.51% | 90.00% | 1.33% |
| 2024-07-05 | 2024-07-08 | 2024-07-22 | 10 | -6.39% | -6.17% | -1.19% | -5.20% | 50.00% | -5.30% |
| 2024-07-19 | 2024-07-22 | 2024-08-05 | 8 | -14.84% | -3.71% | -13.39% | -1.46% | 90.00% | -1.64% |
| 2024-08-02 | 2024-08-05 | 2024-08-19 | 4 | -1.23% | -5.42% | 13.90% | -15.13% | 100.00% | -15.33% |
| 2024-08-16 | 2024-08-19 | 2024-09-03 | 10 | -5.37% | -2.52% | -4.78% | -0.59% | 100.00% | -0.79% |
| 2024-08-30 | 2024-09-03 | 2024-09-17 | 10 | 4.76% | 1.10% | 2.91% | 1.85% | 90.00% | 1.67% |
| 2024-09-16 | 2024-09-17 | 2024-10-01 | 10 | 2.36% | -0.29% | 3.21% | -0.85% | 70.00% | -0.99% |
| 2024-09-30 | 2024-10-01 | 2024-10-15 | 10 | 9.19% | 5.00% | 4.45% | 4.74% | 70.00% | 4.60% |
| 2024-10-14 | 2024-10-15 | 2024-10-29 | 10 | 3.22% | 2.58% | 6.73% | -3.51% | 70.00% | -3.65% |
| 2024-10-28 | 2024-10-29 | 2024-11-12 | 10 | 7.56% | 3.14% | 3.88% | 3.68% | 70.00% | 3.54% |
| 2024-11-11 | 2024-11-12 | 2024-11-26 | 10 | -1.98% | -3.08% | 3.25% | -5.23% | 80.00% | -5.39% |
| 2024-11-25 | 2024-11-26 | 2024-12-11 | 10 | -2.04% | -4.42% | 4.51% | -6.55% | 80.00% | -6.71% |
| 2024-12-10 | 2024-12-11 | 2024-12-26 | 10 | 3.55% | 4.68% | -1.34% | 4.89% | 90.00% | 4.71% |
| 2024-12-24 | 2024-12-26 | 2025-01-13 | 10 | -7.43% | -2.28% | -3.15% | -4.28% | 100.00% | -4.48% |
| 2025-01-10 | 2025-01-13 | 2025-01-28 | 10 | -7.28% | -13.24% | 1.60% | -8.89% | 80.00% | -9.05% |
| 2025-01-27 | 2025-01-28 | 2025-02-11 | 10 | 7.50% | 7.25% | 3.26% | 4.24% | 70.00% | 4.10% |
| 2025-02-10 | 2025-02-11 | 2025-02-26 | 10 | -9.17% | -5.65% | -4.14% | -5.03% | 80.00% | -5.19% |
| 2025-02-25 | 2025-02-26 | 2025-03-12 | 8 | -6.95% | 0.66% | -10.11% | 3.15% | 100.00% | 2.95% |
| 2025-03-11 | 2025-03-12 | 2025-03-26 | 5 | -12.97% | -15.25% | 2.25% | -15.23% | 87.50% | -15.40% |
| 2025-03-25 | 2025-03-26 | 2025-04-09 | 10 | -12.52% | -5.09% | -8.39% | -4.13% | 90.00% | -4.31% |
| 2025-04-08 | 2025-04-09 | 2025-04-24 | 0 | 0.00% | 0.00% | 5.23% | -5.23% | 100.00% | -5.43% |
| 2025-04-23 | 2025-04-24 | 2025-05-08 | 10 | 5.87% | 2.04% | 5.41% | 0.46% | 100.00% | 0.26% |
| 2025-05-07 | 2025-05-08 | 2025-05-22 | 10 | 4.57% | -0.87% | 7.27% | -2.69% | 70.00% | -2.83% |
| 2025-05-21 | 2025-05-22 | 2025-06-06 | 10 | 1.51% | -1.87% | 5.28% | -3.77% | 70.00% | -3.91% |
| 2025-06-05 | 2025-06-06 | 2025-06-23 | 10 | 3.61% | 2.82% | 3.50% | 0.12% | 80.00% | -0.04% |
| 2025-06-20 | 2025-06-23 | 2025-07-08 | 10 | 4.90% | -0.52% | 7.55% | -2.65% | 90.00% | -2.83% |
| 2025-07-07 | 2025-07-08 | 2025-07-22 | 9 | 2.64% | 0.15% | 5.14% | -2.50% | 80.00% | -2.66% |
| 2025-07-21 | 2025-07-22 | 2025-08-05 | 10 | -2.77% | -2.52% | 1.29% | -4.05% | 70.00% | -4.19% |
| 2025-08-04 | 2025-08-05 | 2025-08-19 | 7 | -6.71% | -9.20% | 0.69% | -7.41% | 70.00% | -7.55% |
| 2025-08-18 | 2025-08-19 | 2025-09-03 | 10 | 0.73% | -0.21% | 3.40% | -2.67% | 90.00% | -2.85% |
| 2025-09-02 | 2025-09-03 | 2025-09-17 | 10 | 25.76% | 22.93% | 13.72% | 12.04% | 90.00% | 11.86% |
| 2025-09-16 | 2025-09-17 | 2025-10-01 | 10 | 8.58% | 5.88% | 6.63% | 1.94% | 80.00% | 1.78% |
| 2025-09-30 | 2025-10-01 | 2025-10-15 | 10 | 15.31% | 16.02% | 5.20% | 10.11% | 60.00% | 9.99% |
| 2025-10-14 | 2025-10-15 | 2025-10-29 | 10 | -0.18% | -6.60% | 3.44% | -3.62% | 90.00% | -3.80% |
| 2025-10-28 | 2025-10-29 | 2025-11-12 | 10 | 5.30% | 6.56% | 0.18% | 5.11% | 100.00% | 4.91% |
| 2025-11-11 | 2025-11-12 | 2025-11-26 | 6 | -7.07% | -6.49% | -5.15% | -1.92% | 90.00% | -2.10% |
| 2025-11-25 | 2025-11-26 | 2025-12-11 | 6 | 5.12% | 2.78% | 8.89% | -3.77% | 100.00% | -3.97% |
| 2025-12-10 | 2025-12-11 | 2025-12-26 | 10 | -3.93% | -5.06% | -2.76% | -1.17% | 100.00% | -1.37% |
| 2025-12-24 | 2025-12-26 | 2026-01-12 | 10 | 6.77% | 5.16% | 5.48% | 1.29% | 80.00% | 1.13% |
| 2026-01-09 | 2026-01-12 | 2026-01-27 | 10 | 4.12% | 4.03% | 1.98% | 2.14% | 90.00% | 1.96% |
| 2026-01-26 | 2026-01-27 | 2026-02-10 | 10 | -9.65% | -8.43% | -5.47% | -4.19% | 70.00% | -4.33% |
| 2026-02-09 | 2026-02-10 | 2026-02-25 | 10 | 3.28% | 2.96% | 2.65% | 0.63% | 90.00% | 0.45% |
| 2026-02-24 | 2026-02-25 | 2026-03-11 | 10 | 10.77% | 18.76% | 0.53% | 10.24% | 70.00% | 10.10% |
| 2026-03-10 | 2026-03-11 | 2026-03-25 | 10 | -0.30% | 6.16% | -1.58% | 1.29% | 80.00% | 1.13% |
| 2026-03-24 | 2026-03-25 | 2026-04-09 | 10 | 15.00% | 4.30% | 8.29% | 6.70% | 90.00% | 6.52% |
| 2026-04-08 | 2026-04-09 | 2026-04-23 | 10 | 23.22% | 7.62% | 15.49% | 7.73% | 90.00% | 7.55% |
| 2026-04-22 | 2026-04-23 | 2026-05-07 | 10 | 21.69% | 13.40% | 12.12% | 9.57% | 100.00% | 9.37% |
| 2026-05-06 | 2026-05-07 | 2026-05-21 | 10 | 10.74% | 6.09% | 6.76% | 3.98% | 80.00% | 3.82% |
| 2026-05-20 | 2026-05-21 | 2026-06-05 | 10 | 6.10% | 7.43% | 1.29% | 4.81% | 100.00% | 4.61% |
| 2026-06-04 | 2026-06-05 | 2026-06-22 | 10 | 10.44% | 7.17% | 10.77% | -0.33% | 80.00% | -0.49% |
| 2026-06-18 | 2026-06-22 | 2026-07-07 | 10 | -15.07% | -16.37% | -8.71% | -6.36% | 100.00% | -6.56% |
| 2026-07-06 | 2026-07-07 | 2026-07-21 | 10 | 1.01% | 0.85% | -0.92% | 1.93% | 100.00% | 1.73% |
| 2026-07-20 | 2026-07-21 | 2026-08-04 | 10 | 2.03% | -2.64% | 2.14% | -0.11% | 60.00% | -0.23% |
| 2026-08-03 | 2026-08-04 | 2026-08-18 | 10 | 1.93% | 3.14% | 0.01% | 1.93% | 80.00% | 1.77% |
| 2026-08-17 | 2026-08-18 | 2026-09-01 | 10 | 12.16% | 14.04% | 3.14% | 9.02% | 60.00% | 8.90% |
| 2026-08-31 | 2026-09-01 | 2026-09-16 | 10 | 6.52% | 8.79% | 2.78% | 3.74% | 40.00% | 3.66% |
| 2026-09-15 | 2026-09-16 | 2026-09-30 | 10 | 5.51% | 2.34% | 7.77% | -2.26% | 80.00% | -2.42% |

## 8. Coverage

| Measure | Value |
|---|---|
| Universe after exclusions | 84 |
| Symbols loaded / skipped | 77 / 7 |
| SPY dates | 1300 |
| Candidate / valid main-horizon dates | 1020 / 1020 |
| First / last valid evaluation date | 2022-08-19 / 2026-09-14 |
| Average stocks/date | 67.49 |
| Average long setups/date | 16.47 |

Loaded: AAOIBUSDT, AAPLBUSDT, ADBEBUSDT, AGPUBUSDT, ALABBUSDT, AMATBUSDT, AMCBUSDT, AMDBUSDT, AMZNBUSDT, ARMBUSDT, ASMLBUSDT, ASTSBUSDT, AVGOBUSDT, AXTIBUSDT, BABABUSDT, BEBUSDT, BMNRBUSDT, BNCBUSDT, COHRBUSDT, COINBUSDT, CRCLBUSDT, CRDOBUSDT, CRMBUSDT, CRWDBUSDT, CRWVBUSDT, CYPHBUSDT, DELLBUSDT, DJTBUSDT, EWYBUSDT, FLNCBUSDT, FWDIBUSDT, GLWBUSDT, GMEBUSDT, GOOGLBUSDT, GPROBUSDT, GSBUSDT, HIMSBUSDT, HOODBUSDT, HPEBUSDT, IBMBUSDT, INTCBUSDT, INTWBUSDT, IRENBUSDT, JPMBUSDT, KORUBUSDT, LITEBUSDT, LLYBUSDT, MDBBUSDT, METABUSDT, MRNABUSDT, MRVLBUSDT, MSFTBUSDT, MSTRBUSDT, MUBUSDT, MUUBUSDT, MVLLBUSDT, NBISBUSDT, NFLXBUSDT, NOKBUSDT, NVDABUSDT, ORCLBUSDT, PDDBUSDT, PENGBUSDT, PLTRBUSDT, PYPLBUSDT, QCOMBUSDT, RDDTBUSDT, RKLBBUSDT, SMCIBUSDT, SMHBUSDT, SNDKBUSDT, TSLABUSDT, TSMBUSDT, USARBUSDT, WDCBUSDT, WENBUSDT, ZMBUSDT.

Excluded: QQQBUSDT, SOXLBUSDT, SOXSBUSDT, SPYBUSDT, SQQQBUSDT, TQQQBUSDT, UVXYBUSDT. SPY is the benchmark and always excluded from the cross-section.

| Skipped symbol | Reason |
|---|---|
| CBRSBUSDT | insufficient closed bars: CBRSBUSDT 1d source=yahoo have=105 need=1300 |
| DRAMBUSDT | insufficient closed bars: DRAMBUSDT 1d source=yahoo have=134 need=1300 |
| SECZBUSDT | insufficient closed bars: SECZBUSDT 1d source=yahoo have=72 need=1300 |
| SHAZBUSDT | insufficient closed bars: SHAZBUSDT 1d source=yahoo have=165 need=1300 |
| SKHYBUSDT | insufficient closed bars: SKHYBUSDT 1d source=yahoo have=67 need=1300 |
| SNXXBUSDT | insufficient closed bars: SNXXBUSDT 1d source=yahoo have=180 need=1300 |
| SPCXBUSDT | insufficient closed bars: SPCXBUSDT 1d source=yahoo have=85 need=1300 |

| Loaded symbol | Source | Warning |
|---|---|---|
| AAOIBUSDT | yahoo |  |
| AAPLBUSDT | yahoo |  |
| ADBEBUSDT | yahoo |  |
| AGPUBUSDT | yahoo |  |
| ALABBUSDT | yahoo | insufficient closed bars: ALABBUSDT 1d source=yahoo have=644 need=1300 |
| AMATBUSDT | yahoo |  |
| AMCBUSDT | yahoo |  |
| AMDBUSDT | yahoo |  |
| AMZNBUSDT | yahoo |  |
| ARMBUSDT | yahoo | insufficient closed bars: ARMBUSDT 1d source=yahoo have=773 need=1300 |
| ASMLBUSDT | yahoo |  |
| ASTSBUSDT | yahoo |  |
| AVGOBUSDT | yahoo |  |
| AXTIBUSDT | yahoo |  |
| BABABUSDT | yahoo |  |
| BEBUSDT | yahoo |  |
| BMNRBUSDT | yahoo | insufficient closed bars: BMNRBUSDT 1d source=yahoo have=341 need=1300 |
| BNCBUSDT | yahoo |  |
| COHRBUSDT | yahoo |  |
| COINBUSDT | yahoo |  |
| CRCLBUSDT | yahoo | insufficient closed bars: CRCLBUSDT 1d source=yahoo have=341 need=1300 |
| CRDOBUSDT | yahoo | insufficient closed bars: CRDOBUSDT 1d source=yahoo have=1182 need=1300 |
| CRMBUSDT | yahoo |  |
| CRWDBUSDT | yahoo |  |
| CRWVBUSDT | yahoo | insufficient closed bars: CRWVBUSDT 1d source=yahoo have=388 need=1300 |
| CYPHBUSDT | yahoo |  |
| DELLBUSDT | yahoo |  |
| DJTBUSDT | yahoo | insufficient closed bars: DJTBUSDT 1d source=yahoo have=1264 need=1300 |
| EWYBUSDT | yahoo |  |
| FLNCBUSDT | yahoo | insufficient closed bars: FLNCBUSDT 1d source=yahoo have=1244 need=1300 |
| FWDIBUSDT | yahoo |  |
| GLWBUSDT | yahoo |  |
| GMEBUSDT | yahoo |  |
| GOOGLBUSDT | yahoo |  |
| GPROBUSDT | yahoo |  |
| GSBUSDT | yahoo |  |
| HIMSBUSDT | yahoo |  |
| HOODBUSDT | yahoo |  |
| HPEBUSDT | yahoo |  |
| IBMBUSDT | yahoo |  |
| INTCBUSDT | yahoo |  |
| INTWBUSDT | yahoo | insufficient closed bars: INTWBUSDT 1d source=yahoo have=418 need=1300 |
| IRENBUSDT | yahoo | insufficient closed bars: IRENBUSDT 1d source=yahoo have=1230 need=1300 |
| JPMBUSDT | yahoo |  |
| KORUBUSDT | yahoo |  |
| LITEBUSDT | yahoo |  |
| LLYBUSDT | yahoo |  |
| MDBBUSDT | yahoo |  |
| METABUSDT | yahoo |  |
| MRNABUSDT | yahoo |  |
| MRVLBUSDT | yahoo |  |
| MSFTBUSDT | yahoo |  |
| MSTRBUSDT | yahoo |  |
| MUBUSDT | yahoo |  |
| MUUBUSDT | yahoo | insufficient closed bars: MUUBUSDT 1d source=yahoo have=503 need=1300 |
| MVLLBUSDT | yahoo | insufficient closed bars: MVLLBUSDT 1d source=yahoo have=403 need=1300 |
| NBISBUSDT | yahoo | insufficient closed bars: NBISBUSDT 1d source=yahoo have=496 need=1300 |
| NFLXBUSDT | yahoo |  |
| NOKBUSDT | yahoo |  |
| NVDABUSDT | yahoo |  |
| ORCLBUSDT | yahoo |  |
| PDDBUSDT | yahoo |  |
| PENGBUSDT | yahoo |  |
| PLTRBUSDT | yahoo |  |
| PYPLBUSDT | yahoo |  |
| QCOMBUSDT | yahoo |  |
| RDDTBUSDT | yahoo | insufficient closed bars: RDDTBUSDT 1d source=yahoo have=643 need=1300 |
| RKLBBUSDT | yahoo |  |
| SMCIBUSDT | yahoo |  |
| SMHBUSDT | yahoo |  |
| SNDKBUSDT | yahoo | insufficient closed bars: SNDKBUSDT 1d source=yahoo have=418 need=1300 |
| SPYBUSDT | yahoo |  |
| TSLABUSDT | yahoo |  |
| TSMBUSDT | yahoo |  |
| USARBUSDT | yahoo | insufficient closed bars: USARBUSDT 1d source=yahoo have=815 need=1300 |
| WDCBUSDT | yahoo |  |
| WENBUSDT | yahoo |  |
| ZMBUSDT | yahoo |  |

## Caveats

- Survivorship/selection bias: the universe is currently listed bStock names.
- Yahoo adjusted data can differ from bStock prices.
- ETFs remain in the universe after the explicit exclusions.
- Overlapping forward returns are handled by Newey–West HAC; the non-overlapping check is also shown.
- Single-period beta is estimated at the signal date and held fixed for the forward horizon; residuals are not regression alpha.
- Missing future prices can select the evaluable sample; unpriced selected portfolios are reported as skipped periods.
