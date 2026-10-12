import unittest

from scripts.backtest.params import derive_baseline


def rc(**kw):
    return dict(kw)


class DeriveBaselineTest(unittest.TestCase):
    def test_live_like_config(self):
        p, w = derive_baseline(rc(
            profit_lock_at_r=1.5, profit_lock_be_offset_r=0.1, tp_close_fraction=0, tp_trim_profit_pct=15,
            breakeven_arm_r=0.75, tp_max_r=2, tp_trim_at_r=1.2, peak_drawdown_arm_r=2, peak_drawdown_giveback_r=0.5,
            tp_trim_yields_to_lock=False, time_stop_hours=0, trailing_stop_enabled=True))
        self.assertEqual(w, [])
        self.assertEqual((p["be_arm"], p["be_off"], p["lock_r"], p["lock_trim"]), (0.75, 0.1, 1.5, False))
        self.assertEqual((p["trim_r"], p["tp_cap"], p["tp_frac_trend"], p["trail"]), (1.2, 2, 0.5, True))
        self.assertAlmostEqual(p["trim_frac"], 1 / 3)
        self.assertEqual((p["pd_arm"], p["pd_give"], p["quick_stop_h"]), (2, 0.5, 4))
        self.assertEqual((p["trail_arm"], p["trail_atr"]), (1.5, 2.0))

    def test_breakeven_arm(self):
        self.assertEqual(derive_baseline(rc(breakeven_arm_r=0.5))[0]["be_arm"], 0.5)
        self.assertEqual(derive_baseline(rc(breakeven_arm_r=0))[0]["be_arm"], 0)
        self.assertEqual(derive_baseline(rc(breakeven_arm_r=-1))[0]["be_arm"], 0)

    def test_profit_lock_at_r(self):
        self.assertEqual(derive_baseline(rc(profit_lock_at_r=-1))[0]["lock_r"], 0)
        self.assertEqual(derive_baseline(rc(profit_lock_at_r=0))[0]["lock_r"], 1.0)
        self.assertEqual(derive_baseline(rc(profit_lock_at_r=2.5))[0]["lock_r"], 2.5)

    def test_profit_lock_offset(self):
        self.assertEqual(derive_baseline(rc(profit_lock_be_offset_r=0))[0]["be_off"], 0.2)
        self.assertEqual(derive_baseline(rc(profit_lock_be_offset_r=-0.5))[0]["be_off"], 0)
        self.assertEqual(derive_baseline(rc(profit_lock_be_offset_r=0.3))[0]["be_off"], 0.3)

    def test_lock_trim_follows_yields_flag(self):
        base = dict(profit_lock_at_r=1.5)
        self.assertTrue(derive_baseline(rc(**base))[0]["lock_trim"])  # null → true
        self.assertTrue(derive_baseline(rc(**base, tp_trim_yields_to_lock=True))[0]["lock_trim"])
        self.assertFalse(derive_baseline(rc(**base, tp_trim_yields_to_lock=False))[0]["lock_trim"])
        # 锁关闭时不会有锁减仓
        self.assertFalse(derive_baseline(rc(profit_lock_at_r=-1))[0]["lock_trim"])

    def test_r_trim_tier_and_legacy_warning(self):
        def trim_warns(w):
            return [x for x in w if "tp_trim_profit_pct" in x]

        p, w = derive_baseline(rc(tp_trim_at_r=1.0))
        self.assertEqual((p["trim_r"], trim_warns(w)), (1.0, []))
        p, w = derive_baseline(rc(tp_trim_at_r=-1, tp_trim_profit_pct=15))
        self.assertEqual((p["trim_r"], trim_warns(w)), (0, []))
        p, w = derive_baseline(rc(tp_trim_at_r=0, tp_trim_profit_pct=15))
        self.assertEqual(p["trim_r"], 0)
        self.assertEqual(len(trim_warns(w)), 1)
        # 旧版 ROE 档被显式关闭(<0)时不告警
        self.assertEqual(trim_warns(derive_baseline(rc(tp_trim_at_r=0, tp_trim_profit_pct=-1))[1]), [])

    def test_tp_max_r(self):
        self.assertEqual(derive_baseline(rc(tp_max_r=2))[0]["tp_cap"], 2)
        self.assertEqual(derive_baseline(rc(tp_max_r=1))[0]["tp_cap"], 1)
        self.assertEqual(derive_baseline(rc(tp_max_r=0.5))[0]["tp_cap"], 0)
        self.assertEqual(derive_baseline(rc(tp_max_r=-1))[0]["tp_cap"], 0)
        self.assertEqual(derive_baseline(rc())[0]["tp_cap"], 0)

    def test_tp_close_fraction(self):
        t = dict(trailing_stop_enabled=True)
        self.assertEqual(derive_baseline(rc(**t, tp_close_fraction=0))[0]["tp_frac_trend"], 0.5)
        self.assertEqual(derive_baseline(rc(**t, tp_close_fraction=-1))[0]["tp_frac_trend"], 1.0)
        self.assertEqual(derive_baseline(rc(**t, tp_close_fraction=0.7))[0]["tp_frac_trend"], 0.7)
        # 追踪止损关闭 → 一律 1.0
        p = derive_baseline(rc(trailing_stop_enabled=False, tp_close_fraction=0.7))[0]
        self.assertEqual((p["trail"], p["tp_frac_trend"]), (False, 1.0))

    def test_peak_drawdown(self):
        p, w = derive_baseline(rc(peak_drawdown_arm_r=2, peak_drawdown_giveback_r=0))
        self.assertEqual((p["pd_arm"], p["pd_give"]), (2, 0.5))
        self.assertEqual([x for x in w if "peak_drawdown" in x], [])
        p, _ = derive_baseline(rc(peak_drawdown_arm_r=1.5, peak_drawdown_giveback_r=0.3))
        self.assertEqual((p["pd_arm"], p["pd_give"]), (1.5, 0.3))
        p, w = derive_baseline(rc(peak_drawdown_arm_r=0))
        self.assertEqual(p["pd_arm"], 0)
        self.assertEqual(len([x for x in w if "peak_drawdown" in x]), 1)

    def test_time_stop_hours(self):
        self.assertEqual(derive_baseline(rc(time_stop_hours=0))[0]["quick_stop_h"], 4)
        self.assertEqual(derive_baseline(rc(time_stop_hours=-1))[0]["quick_stop_h"], 0)
        self.assertEqual(derive_baseline(rc(time_stop_hours=6))[0]["quick_stop_h"], 6)

    def test_horizon_passthrough(self):
        self.assertEqual(derive_baseline(rc(), horizon_h=24)[0]["horizon_h"], 24)


if __name__ == "__main__":
    unittest.main()
