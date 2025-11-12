package backtest

import (
	"fmt"
	"sort"
	"time"

	"nofx/market"
)

type timeframeSeries struct {
	klines     []market.Kline
	closeTimes []int64
}

type symbolSeries struct {
	byTF map[string]*timeframeSeries
}

// DataFeed 管理历史K线数据，为回测提供按时间推进的快照。
type DataFeed struct {
	cfg           BacktestConfig
	symbols       []string
	timeframes    []string
	symbolSeries  map[string]*symbolSeries
	decisionTimes []int64
	primaryTF     string
	longerTF      string
}

func NewDataFeed(cfg BacktestConfig) (*DataFeed, error) {
	df := &DataFeed{
		cfg:          cfg,
		symbols:      make([]string, len(cfg.Symbols)),
		timeframes:   append([]string(nil), cfg.Timeframes...),
		symbolSeries: make(map[string]*symbolSeries),
		primaryTF:    cfg.DecisionTimeframe,
	}
	copy(df.symbols, cfg.Symbols)

	if err := df.loadAll(); err != nil {
		return nil, err
	}

	return df, nil
}

func (df *DataFeed) loadAll() error {
	start := time.Unix(df.cfg.StartTS, 0)
	end := time.Unix(df.cfg.EndTS, 0)

	// longest timeframe用于辅助指标
	var longestDur time.Duration
	for _, tf := range df.timeframes {
		dur, err := market.TFDuration(tf)
		if err != nil {
			return err
		}
		if dur > longestDur {
			longestDur = dur
			df.longerTF = tf
		}
	}

	for _, symbol := range df.symbols {
		ss := &symbolSeries{byTF: make(map[string]*timeframeSeries)}
		for _, tf := range df.timeframes {
			dur, _ := market.TFDuration(tf)
			buffer := dur * 200
			fetchStart := start.Add(-buffer)
			if fetchStart.Before(time.Unix(0, 0)) {
				fetchStart = time.Unix(0, 0)
			}
			fetchEnd := end.Add(dur)

			klines, err := market.GetKlinesRange(symbol, tf, fetchStart, fetchEnd)
			if err != nil {
				return fmt.Errorf("fetch klines for %s %s: %w", symbol, tf, err)
			}
			if len(klines) == 0 {
				return fmt.Errorf("no klines for %s %s", symbol, tf)
			}

			series := &timeframeSeries{
				klines:     klines,
				closeTimes: make([]int64, len(klines)),
			}
			for i, k := range klines {
				series.closeTimes[i] = k.CloseTime
			}
			ss.byTF[tf] = series
		}
		df.symbolSeries[symbol] = ss
	}

	// 以第一个符号的主周期生成回测进度时间轴
	firstSymbol := df.symbols[0]
	primarySeries := df.symbolSeries[firstSymbol].byTF[df.primaryTF]
	startMs := start.UnixMilli()
	endMs := end.UnixMilli()
	for _, ts := range primarySeries.closeTimes {
		if ts < startMs {
			continue
		}
		if ts > endMs {
			break
		}
		df.decisionTimes = append(df.decisionTimes, ts)
		// 对齐其他符号，如果缺数据则提前报错
		for _, symbol := range df.symbols[1:] {
			if _, ok := df.symbolSeries[symbol].byTF[df.primaryTF]; !ok {
				return fmt.Errorf("symbol %s missing timeframe %s", symbol, df.primaryTF)
			}
		}
	}
	if len(df.decisionTimes) == 0 {
		return fmt.Errorf("no decision bars in range")
	}
	return nil
}

func (df *DataFeed) DecisionBarCount() int {
	return len(df.decisionTimes)
}

func (df *DataFeed) DecisionTimestamp(index int) int64 {
	return df.decisionTimes[index]
}

func (df *DataFeed) sliceUpTo(symbol, tf string, ts int64) []market.Kline {
	series := df.symbolSeries[symbol].byTF[tf]
	idx := sort.Search(len(series.closeTimes), func(i int) bool {
		return series.closeTimes[i] > ts
	})
	if idx <= 0 {
		return nil
	}
	return series.klines[:idx]
}

func (df *DataFeed) BuildMarketData(ts int64) (map[string]*market.Data, map[string]map[string]*market.Data, error) {
	result := make(map[string]*market.Data, len(df.symbols))
	multi := make(map[string]map[string]*market.Data, len(df.symbols))

	for _, symbol := range df.symbols {
		perTF := make(map[string]*market.Data, len(df.timeframes))
		for _, tf := range df.timeframes {
			series := df.sliceUpTo(symbol, tf, ts)
			if len(series) == 0 {
				continue
			}
			var longer []market.Kline
			if df.longerTF != "" && df.longerTF != tf {
				longer = df.sliceUpTo(symbol, df.longerTF, ts)
			}
			data, err := market.BuildDataFromKlines(symbol, series, longer)
			if err != nil {
				return nil, nil, err
			}
			perTF[tf] = data
			if tf == df.primaryTF {
				result[symbol] = data
			}
		}
		if _, ok := perTF[df.primaryTF]; !ok {
			return nil, nil, fmt.Errorf("no primary data for %s at %d", symbol, ts)
		}
		multi[symbol] = perTF
	}
	return result, multi, nil
}

func (df *DataFeed) decisionBarSnapshot(symbol string, ts int64) (*market.Kline, *market.Kline) {
	ss, ok := df.symbolSeries[symbol]
	if !ok {
		return nil, nil
	}
	series, ok := ss.byTF[df.primaryTF]
	if !ok {
		return nil, nil
	}
	idx := sort.Search(len(series.closeTimes), func(i int) bool {
		return series.closeTimes[i] >= ts
	})
	if idx >= len(series.closeTimes) || series.closeTimes[idx] != ts {
		return nil, nil
	}
	curr := &series.klines[idx]
	var next *market.Kline
	if idx+1 < len(series.klines) {
		next = &series.klines[idx+1]
	}
	return curr, next
}
