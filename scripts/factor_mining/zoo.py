"""Horizon-independent Alpha158-style seed factors and nofx research factors."""

from .expr import canonical, expr_hash, validate_expr


def seed_factors() -> list[dict]:
    """Build a deterministic, validated library with canonical deduplication.

    Each entry contains expr/family/name and its stable expression-derived id.
    Denominator epsilons protect ratios without changing the engine's NaN rules.
    """
    factors = []
    seen = set()

    def add(name: str, family: str, expr: str) -> None:
        validate_expr(expr)
        key = canonical(expr)
        if key not in seen:
            seen.add(key)
            factors.append({"expr": expr, "family": family, "name": name,
                            "id": expr_hash(expr)})

    body = "($close-$open)"
    length = "($high-$low)"
    upper = "($high-Greater($open,$close))"
    lower = "(Less($open,$close)-$low)"
    shift = "(2*$close-$high-$low)"
    for name, numerator, denominator in [
        ("KMID", body, "$open"), ("KLEN", length, "$open"),
        ("KUP", upper, "$open"), ("KLOW", lower, "$open"),
        ("KSFT", shift, "$open"), ("KMID2", body, length + "+1e-12"),
        ("KUP2", upper, length + "+1e-12"),
        ("KLOW2", lower, length + "+1e-12"),
        ("KSFT2", shift, length + "+1e-12"),
    ]:
        add(name, "candle", f"{numerator}/({denominator})")

    for n in (5, 10, 20, 30, 60):
        specs = [
            ("ROC", "reversal", f"Ref($close,{n})/$close"),
            ("MOM", "momentum", f"$close/Ref($close,{n})"),
            ("MA", "reversal", f"Mean($close,{n})/$close"),
            ("STD", "volatility", f"Std($close,{n})/$close"),
            ("BETA", "momentum", f"Slope($close,{n})/$close"),
            ("RSQR", "other", f"Rsquare($close,{n})"),
            ("RESI", "reversal", f"Resi($close,{n})/$close"),
            ("MAX", "reversal", f"Max($high,{n})/$close"),
            ("MIN", "reversal", f"Min($low,{n})/$close"),
            ("QTLU", "reversal", f"Quantile($close,{n},0.8)/$close"),
            ("QTLD", "reversal", f"Quantile($close,{n},0.2)/$close"),
            ("RANK", "momentum", f"Rank($close,{n})"),
            ("RSV", "momentum", f"($close-Min($low,{n}))/(Max($high,{n})-Min($low,{n})+1e-12)"),
            ("IMAX", "momentum", f"IdxMax($high,{n})"),
            ("IMIN", "reversal", f"IdxMin($low,{n})"),
            ("IMXD", "momentum", f"IdxMax($high,{n})-IdxMin($low,{n})"),
            ("CORR", "price_volume", f"Corr($close,Log($volume+1),{n})"),
            ("CORD", "price_volume", f"Corr($close/Ref($close,1),Log($volume/Ref($volume,1)+1),{n})"),
            ("CNTP", "momentum", f"Mean($close>Ref($close,1),{n})"),
            ("CNTN", "reversal", f"Mean($close<Ref($close,1),{n})"),
            ("CNTD", "momentum", f"Mean($close>Ref($close,1),{n})-Mean($close<Ref($close,1),{n})"),
            ("SUMP", "momentum", f"Sum(Greater($ret,0),{n})/(Sum(Abs($ret),{n})+1e-12)"),
            ("SUMN", "reversal", f"Sum(Greater(-$ret,0),{n})/(Sum(Abs($ret),{n})+1e-12)"),
            ("SUMD", "momentum", f"Sum($ret,{n})/(Sum(Abs($ret),{n})+1e-12)"),
            ("VMA", "volume", f"Mean($volume,{n})/($volume+1e-12)"),
            ("VSTD", "volume", f"Std($volume,{n})/($volume+1e-12)"),
            ("WVMA", "volume", f"Std(Abs($ret)*$volume,{n})/(Mean(Abs($ret)*$volume,{n})+1e-12)"),
            ("VSUMP", "volume", f"Sum(Greater(Delta($volume,1),0),{n})/(Sum(Abs(Delta($volume,1)),{n})+1e-12)"),
            ("VSUMN", "volume", f"Sum(Greater(-Delta($volume,1),0),{n})/(Sum(Abs(Delta($volume,1)),{n})+1e-12)"),
            ("VSUMD", "volume", f"Sum(Delta($volume,1),{n})/(Sum(Abs(Delta($volume,1)),{n})+1e-12)"),
            ("DRAWDOWN", "drawdown", f"$close/Max($close,{n})-1"),
            # DAYS_HIGH intentionally deduplicates with IMAX; IMAX is already
            # the requested normalized number of days since the recent high.
            ("DAYS_HIGH", "momentum", f"IdxMax($high,{n})"),
            ("AMPLITUDE", "volatility", f"Mean(($high-$low)/$close,{n})"),
            ("GAP", "other", f"Mean($open/Ref($close,1)-1,{n})"),
            ("RV", "volatility", f"Std($ret,{n})"),
            ("DOWNSIDE", "volatility", f"Sqrt(Mean(Power(Less($ret,0),2),{n}))"),
            ("SKEW", "volatility", f"Skew($ret,{n})"),
            ("KURT", "volatility", f"Kurt($ret,{n})"),
        ]
        for name, family, expr in specs:
            add(f"{name}{n}", family, expr)
    add("TURNOVER_SHOCK", "price_volume", "Mean($volume*$vwap,5)/Mean($volume*$vwap,60)")
    return factors
