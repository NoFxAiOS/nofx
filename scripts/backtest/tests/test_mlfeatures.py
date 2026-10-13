import json
import math
import unittest

from scripts.backtest.mlfeatures import extract_structured, features


def sig():
    return {
        "price": 100.0, "timestamp_utc": "2026-10-01T13:05:00Z",
        "timeframes": {"1h": {
            "rsi": 60, "atr_pct": 2.0, "atr_percentile": 70, "ema_fast": 98.0, "ema_slow": 95.0, "trend": "up",
            "macd_trend": "rising", "volume_ratio": 1.5, "trend_window_return_pct": 3.0,
            "last_closed_candle": "bullish", "resistance_dist_pct": [2.5, 4.0], "support_dist_pct": [-1.5, -3.0],
            "structure_high_dist_pct": 4.0, "structure_low_dist_pct": -2.0}},
        "market_regime": {"regime": "TREND_UP", "confidence": 0.8, "inputs": {"adx_1h": 30, "adx_4h": 25,
                                                                             "bb_width_percentile_1h": 40}},
        "derivatives": {"funding_annualized_pct": 20.0, "price_change_24h_live_pct": 5.0,
                        "price_change_60m_live_pct": -1.0, "taker_buy_sell_ratio": 1.25},
        "bias": {"scanner": "long", "structure": "short"},
        "signal_conflict": {"directional_conflict": True, "directional_score": 3},
        "breakout": {"status": "extended"}, "liquidity": {"quote_volume_24h_usd": 1e6},
        "btc_correlation": {"beta": 1.4},
        "hard_entry_gate": {"long": {"stop_plan_pct": 3.0, "rr_scan": {"best_rr": 2.2}},
                            "short": {"stop_plan_pct": 4.0, "rr_scan": {"best_rr": 1.1}}},
    }


class FeaturesTest(unittest.TestCase):
    def test_long_values(self):
        f = features(sig(), "long", {"confidence": 75, "exit_mode": "range"})
        self.assertEqual(f["ema_align_1h"], 1)
        self.assertAlmostEqual(f["dist_emaf_1h"], (100 / 98 - 1) * 100)
        self.assertEqual((f["trend_with_1h"], f["macd_rising_1h"], f["lastcandle_with_1h"]), (1, 1, 1))
        self.assertEqual(f["retwin_1h"], 3.0)
        self.assertEqual(f["wall_dist_1h"], 2.5)      # 做多看上方阻力
        self.assertEqual(f["room_struct_1h"], 4.0)
        self.assertEqual(f["funding_ann_with"], 20.0)
        self.assertEqual((f["chg24_with"], f["chg60_with"]), (5.0, -1.0))
        self.assertEqual(f["taker_ratio_with"], 1.25)
        self.assertEqual(f["dir_score_with"], 3)
        self.assertEqual((f["stop_pct"], f["best_rr"]), (3.0, 2.2))
        self.assertEqual((f["is_long"], f["ai_conf"], f["exit_trend"], f["hour_utc"]), (1, 75, 0, 13))
        self.assertAlmostEqual(f["qvol24_log"], 6.0)
        self.assertEqual((f["btc_beta"], f["conflict"], f["breakout_extended"]), (1.4, 1, 1))

    def test_short_mirrors_sign(self):
        f = features(sig(), "short", {})
        self.assertEqual(f["ema_align_1h"], -1)       # EMA 快线在慢线上方 = 与做空方向相反
        self.assertAlmostEqual(f["dist_emaf_1h"], -(100 / 98 - 1) * 100)
        self.assertEqual((f["trend_with_1h"], f["macd_rising_1h"], f["lastcandle_with_1h"]), (-1, -1, -1))
        self.assertEqual(f["retwin_1h"], -3.0)
        self.assertEqual(f["wall_dist_1h"], 1.5)      # 做空看下方支撑(取绝对值)
        self.assertEqual(f["room_struct_1h"], 2.0)
        self.assertEqual(f["funding_ann_with"], -20.0)
        self.assertEqual((f["chg24_with"], f["chg60_with"]), (-5.0, 1.0))
        self.assertAlmostEqual(f["taker_ratio_with"], 1 / 1.25)
        self.assertEqual(f["dir_score_with"], -3)
        self.assertEqual((f["stop_pct"], f["best_rr"]), (4.0, 1.1))  # 取对应方向的 gate
        self.assertEqual((f["is_long"], f["ai_conf"], f["exit_trend"]), (0, None, 1))

    def test_missing_fields_are_none(self):
        f = features({}, "long", {})
        self.assertIsNone(f["rsi_1h"])
        self.assertIsNone(f["ema_align_4h"])
        self.assertIsNone(f["dist_emaf_1h"])
        self.assertIsNone(f["trend_with_1h"])
        self.assertIsNone(f["wall_dist_1h"])
        self.assertIsNone(f["funding_ann_with"])
        self.assertIsNone(f["taker_ratio_with"])
        self.assertIsNone(f["regime_trend_with"])
        self.assertIsNone(f["scanner_with"])
        self.assertIsNone(f["qvol24_log"])
        self.assertIsNone(f["hour_utc"])
        self.assertIsNone(f["btc_beta"])
        # 缺失时布尔类特征默认为 0
        self.assertEqual((f["conflict"], f["breakout_extended"]), (0, 0))
        # 非数字字符串也不能抛异常
        self.assertIsNone(features({"timeframes": {"1h": {"rsi": "n/a"}}}, "short", {})["rsi_1h"])

    def test_regime_encoding(self):
        def reg(r, side):
            return features({"market_regime": {"regime": r}}, side, {})
        self.assertEqual(reg("TREND_UP", "long")["regime_trend_with"], 1)
        self.assertEqual(reg("TREND_DOWN", "long")["regime_trend_with"], -1)
        self.assertEqual(reg("TREND_DOWN", "short")["regime_trend_with"], 1)
        self.assertEqual(reg("TREND_UP", "short")["regime_trend_with"], -1)
        self.assertEqual(reg("CHOP_LOW", "long")["regime_trend_with"], 0)
        self.assertEqual(reg("CHOP_LOW", "long")["regime_chop"], 1)
        self.assertEqual(reg("TREND_UP", "long")["regime_chop"], 0)

    def test_scanner_structure_encoding(self):
        def bias(sc, st, side):
            f = features({"bias": {"scanner": sc, "structure": st}}, side, {})
            return f["scanner_with"], f["structure_with"]
        self.assertEqual(bias("long", "short", "long"), (1, -1))
        self.assertEqual(bias("long", "short", "short"), (-1, 1))
        self.assertEqual(bias("neutral", None, "long"), (0, 0))

    def test_trend_encoding(self):
        def tw(tr, side):
            return features({"timeframes": {"4h": {"trend": tr}}}, side, {})["trend_with_4h"]
        self.assertEqual((tw("up", "long"), tw("pullback", "long"), tw("down", "long"), tw("range", "long")), (1, 1, -1, 0))
        self.assertEqual((tw("down", "short"), tw("up", "short"), tw("range", "short")), (1, -1, 0))

    def test_extract_structured(self):
        s = {"price": 1.0}
        prompt = f"x\n=== ABCUSDT Structured Signal (1h) ===\n{json.dumps(s)}\ny"
        self.assertEqual(extract_structured(prompt, "ABCUSDT"), s)
        self.assertIsNone(extract_structured(prompt, "OTHERUSDT"))
        self.assertIsNone(extract_structured(None, "ABCUSDT"))
        self.assertIsNone(extract_structured("=== ABCUSDT Structured Signal\n{bad json}\n", "ABCUSDT"))


if __name__ == "__main__":
    unittest.main()
