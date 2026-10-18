"""Frozen market and experiment settings (no network access)."""

MARKETS = {
    'crypto': dict(horizons=(1, 3, 7), one_way_cost=0.0008,
                   periods_per_year=365, universe_n=60, min_names=15,
                   min_history=60, benchmark=None,
                   windows=dict(train=1095, val=365, holdout=180)),
    'us': dict(horizons=(5, 10, 20), one_way_cost=0.0005,
               periods_per_year=252, universe_n=None, min_names=15,
               min_history=60, benchmark='SPY',
               windows=dict(train=756, val=252, holdout=126)),
}
SPLIT = (0.6, 0.2, 0.2)
MIN_COVERAGE = 0.3
CORR_REDUNDANT = 0.8
VAL_MIN_T = 1.65
