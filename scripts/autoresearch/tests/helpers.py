from pathlib import Path
import numpy as np
import pandas as pd
from scripts.autoresearch import build_dataset as bd


def bars(days=50):
    i = np.arange(days * 288)
    o = 200 + .001 * i + np.sin(i / 37) * .2
    return np.column_stack([i * bd.sf.M5, o, o + .3, o - .3, o + .0005]).tolist()


def synthetic_dataset(root, seed=57):
    """30 coins x 240 decision days, correlated signal + independent noise."""
    rng = np.random.default_rng(seed)
    times = np.repeat(np.arange(240 * 6) * 4 * bd.HOUR, 30)
    n = len(times)
    x = rng.normal(size=n)
    shock = np.repeat(rng.normal(0, .002, 240 * 6), 30)
    ret = .02 * x + rng.normal(0, .003, n) + shock
    state = np.repeat(np.arange(240 * 6) % 3, 30)
    frame = pd.DataFrame(dict(t_ms=times, sym=np.tile([f'C{i}' for i in range(30)], 240 * 6),
                              ret_24h_xs=x, noise=rng.normal(size=n),
                              btc_bear=(state == 0).astype(float), btc_chop=(state == 1).astype(float),
                              btc_bull=(state == 2).astype(float), missing=np.full(n, np.nan)))
    for h in bd.HORIZONS:
        for side, sign in [('long', 1), ('short', -1)]:
            frame[f'{side}_ret_{h}'] = sign * ret
            frame[f'{side}_net_{h}'] = sign * ret - .0016
            frame[f'{side}_exc_{h}'] = sign * ret
    features = ['ret_24h_xs', 'noise', 'btc_bear', 'btc_chop', 'btc_bull', 'missing']
    meta = bd.save_dataset(frame, root, features)
    return frame, meta


def candidate(path, feature='ret_24h_xs', sign=''):
    path = Path(path)
    path.write_text(f'import numpy as np\nFEATURES = ["{feature}"]\ndef score(X):\n    return {sign}X["{feature}"]\n')
    return path
