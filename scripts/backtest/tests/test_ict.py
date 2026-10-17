"""Hand-built price sequences for ICT gates and prefix invariance."""

import copy
import unittest
from dataclasses import asdict
from datetime import datetime, timezone

import numpy as np

from scripts.backtest.ict import Ctx, FVG, M5, Swing, atr, find_setups, fvgs, load_arrays, resample, swings


def candles(rows, start=0):
    return [[start + i * M5, *row] for i, row in enumerate(rows)]


def impulse(start=0):
    """Low 4 confirms at 6; high 8 at 10; sweep 12, CISD 13, FVG 14, MSS 15."""
    return candles([
        (105, 110, 100, 106), (106, 111, 99, 105), (105, 110, 98, 104), (104, 109, 95, 103),
        (103, 108, 90, 104), (104, 110, 96, 105), (105, 115, 98, 106), (106, 125, 100, 107),
        (107, 160, 101, 108), (108, 120, 100, 107), (107, 112, 98, 103), (103, 108, 94, 99),
        (93, 103, 89, 101), (101, 130, 100, 125), (125, 140, 110, 130), (130, 180, 120, 165),
        (165, 175, 150, 164), (164, 173, 151, 163), (163, 171, 152, 162), (162, 169, 153, 161),
    ], start)


def mirror(bars, axis=400):
    return [[t, axis - o, axis - lo, axis - hi, axis - c] for t, o, hi, lo, c in bars]


def selected(bars, trigger=None, side="long", sweep=None, params=None, reference=None):
    result = find_setups(Ctx(bars, reference), params)
    return [x for x in result if x.side == side and (trigger is None or x.trigger == trigger)
            and (sweep is None or x.s == sweep)]


def hourly_prefix():
    # Strict 1h lows 4=85 and 12=95 (HL), high 8=160, bull FVG 6..8=[100,105].
    rows = [
        (100, 110, 92, 102), (102, 112, 94, 104), (104, 114, 96, 106), (106, 108, 90, 100),
        (100, 104, 85, 98), (98, 102, 90, 96), (96, 100, 92, 98), (98, 115, 95, 112),
        (112, 160, 105, 120), (120, 135, 103, 125), (125, 130, 100, 120), (120, 125, 98, 115),
        (115, 120, 95, 110), (110, 124, 98, 112), (112, 126, 100, 114), (114, 128, 102, 116),
    ]
    return candles([row for row in rows for _ in range(12)])


class ArraysTests(unittest.TestCase):
    def test_load_arrays_copy_and_close_time(self):
        bars = impulse()
        a = load_arrays(bars)
        self.assertEqual(set(a), {"t_open", "t_close", "o", "h", "l", "c"})
        self.assertTrue(all(isinstance(v, np.ndarray) for v in a.values()))
        self.assertEqual(a["t_open"].dtype, np.dtype("int64"))
        np.testing.assert_array_equal(a["t_close"], a["t_open"] + M5)
        a["o"][0] = 0
        self.assertEqual(bars[0][1], 105)
        self.assertEqual(len(load_arrays([])["c"]), 0)

    def test_resample_discards_partial_and_missing_buckets(self):
        bars = candles([(100 + i, 102 + i, 98 + i, 101 + i) for i in range(29)])
        a = resample(bars[1:], 60)  # Missing 00:00; complete 01:00; partial 02:00.
        np.testing.assert_array_equal(a["t_open"], [12 * M5])
        np.testing.assert_array_equal(a["t_close"], [24 * M5])
        self.assertEqual((a["o"][0], a["h"][0], a["l"][0], a["c"][0]), (112, 125, 110, 124))
        self.assertEqual(len(resample(bars[1:18] + bars[19:], 60)["c"]), 0)
        self.assertEqual(len(resample([], 60)["c"]), 0)
        with self.assertRaises(ValueError):
            resample(bars, 7)
        with self.assertRaises(ValueError):
            load_arrays([bars[0], bars[0]])

    def test_swings_strict_ties_and_availability(self):
        h = [1, 2, 5, 2, 1, 6, 6, 1]
        l = [0, -1, -4, -1, 0, -5, -5, 0]
        self.assertEqual(swings(h, l, 2), [Swing(2, "high", 5, 4), Swing(2, "low", -4, 4)])
        self.assertEqual(swings(h[:4], l[:4], 2), [])
        self.assertEqual(swings(h[:5], l[:5], 2), swings(h, l, 2))
        with self.assertRaises(ValueError):
            swings(h, l, 0)

    def test_strict_bull_bear_fvg_and_availability(self):
        bars = candles([(9, 10, 8, 9), (9, 13, 9, 12), (12, 14, 11, 13)])
        a = load_arrays(bars)
        gaps = fvgs(*(a[k] for k in ("o", "h", "l", "c")))
        self.assertEqual(gaps, [FVG(0, 1, 2, "bull", 10, 11, 10.5)])
        self.assertEqual(gaps[0].avail, 2)
        a = load_arrays(mirror(bars, 30))
        self.assertEqual(fvgs(*(a[k] for k in ("o", "h", "l", "c"))),
                         [FVG(0, 1, 2, "bear", 19, 20, 19.5)])

    def test_touch_or_overlap_is_not_fvg(self):
        for low in (10, 9):
            bars = candles([(9, 10, 8, 9), (9, 25, 7, 24), (12, 14, low, 13)])
            for sample in (bars, mirror(bars)):
                a = load_arrays(sample)
                self.assertEqual(fvgs(*(a[k] for k in ("o", "h", "l", "c"))), [])

    def test_atr_simple_partial_window_and_gaps(self):
        np.testing.assert_allclose(atr([11, 13, 14, 17], [9, 10, 12, 13], [10, 12, 13, 16], 3),
                                   [2, 2.5, 7 / 3, 3])
        np.testing.assert_allclose(atr([11, 21], [9, 19], [10, 20]), [2, 6.5])
        self.assertEqual(len(atr([], [], [])), 0)

    def test_ctx_precomputes_required_structures(self):
        ctx = Ctx(impulse())
        self.assertEqual(ctx.swings5, swings(ctx.a5["h"], ctx.a5["l"], 2))
        self.assertEqual(len(ctx.a1h["c"]), 1)
        np.testing.assert_array_equal(ctx.t_close1h, [3_600_000])
        np.testing.assert_allclose(ctx.atr1h, [70])
        self.assertEqual(find_setups(Ctx([])), [])
        self.assertEqual(find_setups(Ctx([], [])), [])


class SetupTests(unittest.TestCase):
    def test_all_five_triggers_and_fields(self):
        bars = impulse()
        out = selected(bars, sweep=12)
        self.assertEqual([(x.trigger, x.idx) for x in out],
                         [("sweep", 12), ("cisd", 13), ("fvg", 14), ("mss", 15), ("full", 15)])
        by = {x.trigger: x for x in out}
        self.assertEqual(by["sweep"].ref, 101)
        self.assertEqual(by["mss"].ref, 165)
        self.assertEqual(by["fvg"].ref, 106.5)
        self.assertEqual(by["full"].tp_erl, 160)
        self.assertGreaterEqual((160 - by["full"].ref) / (by["full"].ref - by["full"].sl), 2)
        for x in out:
            self.assertEqual(x.sym, "")
            self.assertEqual(x.extreme, 89)
            self.assertEqual(x.t_close_ms, (x.idx + 1) * M5)
            self.assertLess(x.sl, x.ref)
            self.assertAlmostEqual(x.sl, 89 - 0.1 * x.atr5)
            self.assertEqual(x.kind, "lim" if x.trigger in ("fvg", "full") else "mkt")
            self.assertEqual(x.valid_bars, 24 if x.kind == "lim" else 0)
            self.assertEqual(set(x.flags), {"erl_swept", "smt", "htf_bias", "in_htf_fvg", "p1", "p2", "p3"})
            self.assertTrue(all(type(v) is bool for v in x.flags.values()))
        by["sweep"].sym = "TEST"
        self.assertEqual(by["sweep"].sym, "TEST")

    def test_sweep_reclaim_vs_close_break_and_equalities(self):
        for mirrored, side in ((False, "long"), (True, "short")):
            for low, close, expected in ((89, 101, [12]), (89, 89, []), (89, 90, []), (90, 101, [])):
                bars = impulse()[:13]
                bars[12][3], bars[12][4] = low, close
                bars[12][1] = 93
                if mirrored:
                    bars = mirror(bars)
                self.assertEqual([x.s for x in selected(bars, "sweep", side)], expected)

    def test_consumed_on_reclaim_and_on_unreclaimed_break(self):
        for close in (101, 89):
            bars = impulse()[:13]
            bars[12][4] = close
            bars += candles([(95, 108, 88 - i, 101) for i in range(6)], 13 * M5)
            for sample, side in ((bars, "long"), (mirror(bars), "short")):
                expected = [12] if close == 101 else []
                self.assertEqual([x.s for x in selected(sample, "sweep", side, params={"dedupe_min": 0})], expected)

    def test_nonselected_older_pivot_is_also_consumed(self):
        # Lows 2=90, 7=95. Bar 10 penetrates both, closing below the latest (95).
        lows = [100, 99, 90, 99, 100, 99, 98, 95, 98, 99, 89, 94, 89.5]
        bars = candles([(100, 110, lo, 94 if i == 10 else 100) for i, lo in enumerate(lows)])
        # After consuming both, bar 12 must not resurrect the older 90 level.
        self.assertEqual(selected(bars, "sweep", params={"dedupe_min": 0}), [])

    def test_mss_needs_close_not_wick_and_reference_is_frozen_at_sweep(self):
        bars = impulse()
        bars[13][2] = bars[14][2] = 170
        bars[15][4] = 160  # Equality does not break high 8=160.
        out = selected(bars, "mss", sweep=12)
        self.assertEqual([x.idx for x in out], [16])
        self.assertEqual(selected(bars, "mss", sweep=12, params={"window": 3}), [])
        # Future high 13=170 is known by 15 but must not replace the sweep-time reference.
        self.assertEqual(out[0].ref, 164)

    def test_cisd_longest_delivery_and_first_close(self):
        bars = impulse()
        bars[9][1] = 110  # Three bearish bars 9..11, first open=110.
        bars[13][4] = 109  # Above open 10=107, below the true delivery level.
        self.assertEqual([x.idx for x in selected(bars, "cisd", sweep=12)], [14])
        bars[13][4] = 110  # Equality still does not cross.
        self.assertEqual([x.idx for x in selected(bars, "cisd", sweep=12)], [14])

    def test_cisd_bearish_sweep_bar_or_doji_and_minimum_length(self):
        for open_ in (106, 101):
            bars = impulse()
            bars[12][1], bars[12][2] = open_, 108
            self.assertEqual([x.idx for x in selected(bars, "cisd", sweep=12)], [13])
        bars = impulse()
        bars[10][1] = bars[10][4]  # Doji terminates delivery, leaving only bar 11.
        self.assertEqual(selected(bars, "cisd", sweep=12), [])
        bars = impulse()
        bars[12][2], bars[12][4] = 110, 109
        self.assertEqual([x.idx for x in selected(bars, "cisd", sweep=12)], [12])

    def test_fvg_requires_direction_body_and_atr_displacement(self):
        for params in ({"body_mult": 1000}, {"atr_mult": 1000}):
            self.assertEqual(selected(impulse(), "fvg", sweep=12, params=params), [])
        bars = impulse()[:15]
        bars[13][1], bars[13][4] = 125, 101  # Same strict gap; bearish middle body.
        self.assertTrue(Ctx(bars).fvgs5)
        self.assertEqual(selected(bars, "fvg", sweep=12), [])

    def test_displacement_needs_nonzero_body_history(self):
        bars = impulse()[:15]
        for bar in bars[:13]:
            bar[1] = bar[4]
        self.assertTrue(Ctx(bars).fvgs5)
        self.assertEqual(selected(bars, "fvg", sweep=12), [])

    def test_unconfirmed_swing_cannot_be_a_target(self):
        bars = impulse()
        bars[13][2], bars[13][4] = 200, 105
        bars[14][2], bars[14][4] = 175, 170
        ctx = Ctx(bars)
        self.assertIn(Swing(13, "high", 200, 15), ctx.swings5)
        out = selected(bars, "cisd", sweep=12)[0]
        self.assertEqual(out.idx, 14)
        self.assertIsNone(out.tp_erl)  # 160 is below ref=170; 200 not yet available.

    def test_full_requires_discount_rr_and_both_confirmations(self):
        bars = impulse()
        bars[8][2] = 130  # Valid MSS target, but RR to 130 is below 2.
        self.assertTrue(selected(bars, "fvg", sweep=12))
        self.assertTrue(selected(bars, "mss", sweep=12))
        self.assertEqual(selected(bars, "full", sweep=12), [])
        bars = impulse()
        bars[12][2] = 130
        bars[13][1:] = [101, 170, 100, 165]
        bars[14][1:] = [165, 175, 160, 170]  # CE=145 > (89+175)/2.
        self.assertTrue(selected(bars, "fvg", sweep=12))
        self.assertEqual(selected(bars, "full", sweep=12), [])
        self.assertEqual(selected(impulse()[:15], "full", sweep=12), [])

    def test_stop_uses_entire_segment_and_limit_validity_parameter(self):
        bars = impulse()
        bars[15][3] = 85
        out = selected(bars, "full", sweep=12, params={"buffer_atr": 0.15, "lim_valid": 7})[0]
        self.assertEqual(out.extreme, 89)
        self.assertAlmostEqual(out.sl, 85 - 0.15 * out.atr5)
        self.assertEqual(out.valid_bars, 7)

    def test_invalid_zero_risk_is_dropped(self):
        # A gap up leaves an old high at 80, then a newer swing low at 90.
        # Sweep 10 reclaims 90; the next close 89 still exceeds the old high,
        # but equals the segment's minimum, giving zero risk with no buffer.
        highs = [70, 75, 80, 75, 74, 100, 105, 110, 115, 120, 125, 130]
        lows = [60, 65, 70, 68, 67, 95, 94, 90, 95, 94, 89, 89]
        bars = candles([((hi + lo) / 2, hi, lo, (hi + lo) / 2) for hi, lo in zip(highs, lows)])
        bars[10][4], bars[11][4] = 100, 89
        for sample, side in ((bars, "long"), (mirror(bars), "short")):
            self.assertEqual([x.idx for x in selected(sample, "mss", side, 10)], [11])
            self.assertEqual(selected(sample, "mss", side, 10, {"buffer_atr": 0}), [])

    def test_custom_swing_width_and_context_not_mutated(self):
        bars = candles([(100, 110, lo, 100) for lo in (99, 98, 90, 95, 89, 96)])
        ctx = Ctx(bars)
        before = copy.deepcopy(ctx.swings5)
        self.assertEqual(find_setups(ctx), [])
        out = find_setups(ctx, {"swing_w5": 1})
        self.assertEqual([(x.side, x.s) for x in out if x.trigger == "sweep"], [("long", 4)])
        self.assertEqual(ctx.swings5, before)

    def test_dedupe_by_decision_time_and_exact_sixty_minute_boundary(self):
        bars = impulse()[:13]
        lows = [97, 96, 94, 96, 97, 93, 98, 99, 95, 98, 99, 94]
        bars += candles([(100, 110, lo, 101) for lo in lows], 13 * M5)
        self.assertEqual([x.s for x in selected(bars, "sweep", params={"dedupe_min": 0})], [12, 18, 24])
        self.assertEqual([x.s for x in selected(bars, "sweep")], [12, 24])

    def test_complete_short_mirror(self):
        long = selected(impulse(), sweep=12)
        short = selected(mirror(impulse()), side="short", sweep=12)
        self.assertEqual(len(short), len(long))
        for a, b in zip(long, short):
            self.assertEqual((a.trigger, a.idx, a.kind, a.flags), (b.trigger, b.idx, b.kind, b.flags))
            for key in ("ref", "sl", "extreme", "tp_erl", "tp_p1", "tp_p2", "tp_p3"):
                value = getattr(a, key)
                if value is None:
                    self.assertIsNone(getattr(b, key))
                else:
                    self.assertAlmostEqual(getattr(b, key), 400 - value)
            self.assertGreater(b.sl, b.ref)


class ContextTests(unittest.TestCase):
    def test_smt_alignment_two_bar_baseline_and_mirror(self):
        bars = candles([(105, 110, 100, 106)] * 84) + impulse(84 * M5)
        ref = candles([(200, 201, 199, 200)] * (len(bars) + 3), start=-3 * M5)
        s = 96
        self.assertTrue(selected(bars, "sweep", sweep=s, reference=ref)[0].flags["smt"])
        self.assertTrue(selected(mirror(bars), "sweep", "short", s,
                                 reference=mirror(ref, 600))[0].flags["smt"])
        # Reference s-1 breaks the old range, then s holds: SMT must remain false.
        ref[s - 1 + 3][3] = 198
        self.assertFalse(selected(bars, "sweep", sweep=s, reference=ref)[0].flags["smt"])
        ref[s - 1 + 3][3] = 199
        del ref[40]
        self.assertFalse(selected(bars, "sweep", sweep=s, reference=ref)[0].flags["smt"])
        self.assertFalse(selected(bars, "sweep", sweep=s)[0].flags["smt"])

    def test_smt_requires_own_new_low(self):
        bars = candles([(105, 110, 100, 106)] * 84) + impulse(84 * M5)
        bars[40][3] = 88
        ref = candles([(200, 201, 199, 200)] * len(bars))
        self.assertFalse(selected(bars, "sweep", sweep=96, reference=ref)[0].flags["smt"])

    def test_htf_paths_and_short_mirror(self):
        prefix = hourly_prefix()
        bars = prefix + impulse(len(prefix) * M5)
        bars[len(prefix) + 11][4] = 100  # Close on FVG boundary survives.
        s = len(prefix) + 12
        out = selected(bars, "sweep", sweep=s)[0]
        self.assertEqual(out.erl_level, 95)
        self.assertTrue(out.flags["erl_swept"])
        self.assertTrue(out.flags["htf_bias"])
        self.assertTrue(out.flags["in_htf_fvg"])
        self.assertEqual((out.tp_p1, out.tp_p2, out.tp_p3), (160, 122.5, 160))
        self.assertTrue(all(out.flags[p] for p in ("p1", "p2", "p3")))
        mirrored = selected(mirror(bars), "sweep", "short", s)[0]
        self.assertEqual(mirrored.flags, out.flags)
        for key in ("erl_level", "tp_p1", "tp_p2", "tp_p3"):
            self.assertAlmostEqual(getattr(mirrored, key), 400 - getattr(out, key))

    def test_htf_fvg_invalidated_by_prior_close_but_not_future_close(self):
        prefix = hourly_prefix()
        bars = prefix + impulse(len(prefix) * M5)
        s = len(prefix) + 12
        # The 16th hour closes at 99 and invalidates the bull FVG before sweep.
        self.assertFalse(selected(bars, "sweep", sweep=s)[0].flags["in_htf_fvg"])
        bars[len(prefix) + 11][4] = 100
        before = selected(bars, "sweep", sweep=s)[0]
        future = bars + candles([(95, 100, 85, 90)] * 28, len(bars) * M5)
        after = selected(future, "sweep", sweep=s)[0]
        self.assertEqual(asdict(before), asdict(after))
        self.assertTrue(after.flags["in_htf_fvg"])

    def test_p2_opposing_fvg_partial_fill_and_full_wick_fill(self):
        prefix = hourly_prefix()
        for i in range(8 * 12, 9 * 12):
            prefix[i][1:] = [145, 160, 140, 150]
        for i in range(10 * 12, 11 * 12):
            prefix[i][1:] = [110, 115, 100, 112]
        bars = prefix + impulse(len(prefix) * M5)
        bars[len(prefix) + 8][2] = 130  # Keep completed hour 16 below far edge=140.
        bars[len(prefix) + 11][4] = 100
        s = len(prefix) + 12
        # Bear gap 8..10=[115,140]; its near edge is closer than range EQ=122.5.
        self.assertEqual(selected(bars, "sweep", sweep=s)[0].tp_p2, 115)
        for i in range(15 * 12, 16 * 12):
            bars[i][2] = 139  # Partial wick fill keeps the FVG target.
        self.assertEqual(selected(bars, "sweep", sweep=s)[0].tp_p2, 115)
        for i in range(15 * 12, 16 * 12):
            bars[i][2] = 140  # Equality at the far edge fully fills the FVG.
        self.assertEqual(selected(bars, "sweep", sweep=s)[0].tp_p2, 122.5)
        self.assertEqual(selected(mirror(bars), "sweep", "short", s)[0].tp_p2, 400 - 122.5)

    def test_htf_fvg_age_and_p3_twenty_four_hour_window(self):
        prefix = hourly_prefix()
        prefix += candles([(110, 128, 102, 116)] * (73 * 12), len(prefix) * M5)
        bars = prefix + impulse(len(prefix) * M5)
        bars[len(prefix) + 11][4] = 100
        out = selected(bars, "sweep", sweep=len(prefix) + 12)[0]
        self.assertTrue(out.flags["erl_swept"])
        self.assertFalse(out.flags["in_htf_fvg"])
        self.assertIsNone(out.tp_p1)
        self.assertIsNone(out.tp_p3)
        self.assertEqual(out.tp_erl, 128)  # Complete prior-day high is nearer than old 160.

    def test_hourly_atr_only_uses_complete_hours(self):
        bars = impulse()
        expected = 70 / 99 * 100
        self.assertAlmostEqual(selected(bars, "sweep", sweep=12)[0].atr1h_pct, expected)
        later = copy.deepcopy(bars)
        later[19][2] = 1000
        self.assertEqual(asdict(selected(bars, "sweep", sweep=12)[0]),
                         asdict(selected(later, "sweep", sweep=12)[0]))
        shifted = impulse(5 * M5)
        self.assertIsNone(selected(shifted, "sweep", sweep=12)[0].atr1h_pct)

    def test_previous_complete_utc_day_and_erl_target(self):
        prefix = candles([(105, 150, 90, 106)] * 288)
        bars = prefix + impulse(288 * M5)
        out = selected(bars, "sweep", sweep=300)[0]
        self.assertTrue(out.flags["erl_swept"])
        self.assertEqual(out.erl_level, 90)
        self.assertEqual(out.tp_erl, 150)  # PDH is closer than 5m high 160.
        # Incomplete prior days are deliberately not called PDL/PDH.
        self.assertFalse(selected(bars[1:], "sweep", sweep=299)[0].flags["erl_swept"])

    def test_ny_hour_uses_decision_close_and_dst(self):
        for date, expected in (((2026, 1, 2, 14), 10), ((2026, 7, 2, 14), 11)):
            start = int(datetime(*date, tzinfo=timezone.utc).timestamp() * 1000)
            out = selected(impulse(start), "sweep", sweep=12)[0]
            self.assertEqual(out.ny_hour, expected)  # Decision is 15:05 UTC.

    def test_prefix_invariance_all_fields_both_sides_and_custom_parameters(self):
        prefix = hourly_prefix()
        bars = prefix + impulse(len(prefix) * M5)
        # Add more pivots, invalidations, and a second impulse in the future.
        bars += candles([(110, 115, 80, 90)] * 28, len(bars) * M5)
        bars += impulse(len(bars) * M5)
        reference = candles([(200, 201, 199, 200)] * len(bars))
        for sample in (bars, mirror(bars)):
            for params in ({}, {"swing_w5": 1, "swing_w1h": 1, "dedupe_min": 0}):
                ctx = Ctx(sample, reference)
                full = find_setups(ctx, params)
                self.assertTrue(full)
                for stop in range(len(prefix) + 9, len(sample) + 1):
                    with self.subTest(stop=stop, mirrored=sample is not bars, params=params):
                        expected = [asdict(x) for x in full if x.idx < stop]
                        got = [asdict(x) for x in find_setups(Ctx(sample[:stop], reference[:stop]), params)]
                        self.assertEqual(got, expected)
                self.assertEqual([asdict(x) for x in find_setups(ctx, params)], [asdict(x) for x in full])


if __name__ == "__main__":
    unittest.main()
