"""Multivariate TPE Bayesian search over NOFX autopilot risk/throttle params.

Optimizes on a train window (first ~70% of history) and reports the untouched
holdout window, plus full-period metrics and baselines for reference.

Run:  .venv/bin/python search.py [--trials 800]
"""

import argparse
import json
import os
from datetime import timedelta

import optuna

from simulate import Params, Simulator, load_dataset

HERE = os.path.dirname(os.path.abspath(__file__))


def suggest_params(trial):
    min_hold = trial.suggest_float("min_hold_h", 0.0, 8.0)
    return Params(
        min_confidence=trial.suggest_int("min_confidence", 70, 95),
        min_hold_h=min_hold,
        noise_hold_extra_h=trial.suggest_float("noise_hold_extra_h", 0.0, 12.0),
        reentry_h=trial.suggest_float("reentry_h", 0.0, 6.0),
        max_opens_per_hour=trial.suggest_int("max_opens_per_hour", 1, 6),
        max_opens_per_cycle=trial.suggest_int("max_opens_per_cycle", 1, 3),
        max_positions=trial.suggest_int("max_positions", 1, 4),
        leverage=trial.suggest_int("leverage", 3, 20),
        ratio=trial.suggest_float("ratio", 1.0, 6.0),
        sl_bypass=trial.suggest_float("sl_bypass", -60.0, -5.0),
        tp_bypass=trial.suggest_float("tp_bypass", 5.0, 60.0),
        noise_floor=trial.suggest_float("noise_floor", -30.0, -2.0),
        noise_ceiling=trial.suggest_float("noise_ceiling", 2.0, 30.0),
        sl_mult=trial.suggest_float("sl_mult", 0.5, 2.5),
        tp_mult=trial.suggest_float("tp_mult", 0.5, 2.5),
    )


def score(metrics):
    if metrics is None:
        return -1000.0
    if metrics["bankrupt"]:
        return -1000.0
    return metrics["ret_pct"] - 0.5 * metrics["max_dd_pct"]


def fmt(metrics):
    if metrics is None:
        return "n/a"
    return (
        f"ret {metrics['ret_pct']:+7.1f}% | dd {metrics['max_dd_pct']:5.1f}%"
        f" | sharpe {metrics['sharpe']:+5.2f} | trades {metrics['trades']:4d}"
        f" | win {metrics['win_rate']:4.1f}% | fees ${metrics['fees']:.0f}"
        f" | liq {metrics['liquidations']}"
    )


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--trials", type=int, default=800)
    ap.add_argument("--seed", type=int, default=42)
    args = ap.parse_args()

    dataset = load_dataset()
    sim = Simulator(dataset)
    cycles = dataset[0]
    t0, t1 = cycles[0][1], cycles[-1][1]
    split = t0 + (t1 - t0) * 0.7
    print(f"history {t0:%Y-%m-%d} .. {t1:%Y-%m-%d}, holdout from {split:%Y-%m-%d}")

    def objective(trial):
        p = suggest_params(trial)
        return score(sim.run(p, end=split))

    sampler = optuna.samplers.TPESampler(
        multivariate=True, group=True, seed=args.seed, n_startup_trials=60
    )
    optuna.logging.set_verbosity(optuna.logging.WARNING)
    study = optuna.create_study(direction="maximize", sampler=sampler)
    study.optimize(objective, n_trials=args.trials, show_progress_bar=True)

    best = Params(**study.best_params)
    baselines = {
        "live config (2x5x @10x)": Params(),
        "old aggressive (4x5x @20x)": Params(
            leverage=20, ratio=5.0, max_positions=4, min_hold_h=1.0,
            noise_hold_extra_h=0.5, reentry_h=0.5, max_opens_per_hour=6,
            max_opens_per_cycle=3, sl_bypass=-2.5, tp_bypass=5.0,
            noise_floor=-1.0, noise_ceiling=2.0,
        ),
    }

    print("\n=== best params (train objective"
          f" {study.best_value:+.1f}) ===")
    for k, v in sorted(study.best_params.items()):
        print(f"  {k:22s} = {v:.2f}" if isinstance(v, float) else
              f"  {k:22s} = {v}")

    print("\n=== best params performance ===")
    print("  train  :", fmt(sim.run(best, end=split)))
    print("  holdout:", fmt(sim.run(best, start=split)))
    print("  full   :", fmt(sim.run(best)))

    print("\n=== baselines ===")
    for name, p in baselines.items():
        print(f"  {name}")
        print("    train  :", fmt(sim.run(p, end=split)))
        print("    holdout:", fmt(sim.run(p, start=split)))

    top = sorted(
        (t for t in study.trials if t.value is not None),
        key=lambda t: t.value, reverse=True
    )[:10]
    print("\n=== top-10 trials: holdout robustness ===")
    for t in top:
        m = sim.run(Params(**t.params), start=split)
        print(f"  train {t.value:+7.1f} | holdout {fmt(m)}")

    try:
        imp = optuna.importance.get_param_importances(study)
        print("\n=== param importances ===")
        for k, v in imp.items():
            print(f"  {k:22s} {v:.3f}")
    except Exception as e:  # sklearn not installed etc.
        print(f"\n(param importances unavailable: {e})")

    out = os.path.join(HERE, "data", "best_params.json")
    with open(out, "w") as f:
        json.dump(study.best_params, f, indent=2)
    print(f"\nbest params saved to {out}")


if __name__ == "__main__":
    main()
