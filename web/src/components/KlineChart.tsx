import { useEffect, useRef, useCallback } from 'react';
import { createChart, type IChartApi, type ISeriesApi, CandlestickSeries, HistogramSeries, type CandlestickData, type HistogramData, type Time } from 'lightweight-charts';
import type { KlineData } from '../types';

interface KlineChartProps {
  klines: KlineData[];
  symbol: string;
  height?: number;
}

export function KlineChart({ klines, symbol, height = 300 }: KlineChartProps) {
  const chartContainerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi | null>(null);
  const candleSeriesRef = useRef<ISeriesApi<'Candlestick'> | null>(null);
  const volumeSeriesRef = useRef<ISeriesApi<'Histogram'> | null>(null);

  const initChart = useCallback(() => {
    if (!chartContainerRef.current) return;

    // Clean up existing chart
    if (chartRef.current) {
      chartRef.current.remove();
      chartRef.current = null;
    }

    const chart = createChart(chartContainerRef.current, {
      height,
      layout: {
        background: { color: '#0B0E11' },
        textColor: '#848E9C',
        fontSize: 11,
      },
      grid: {
        vertLines: { color: 'rgba(43, 49, 57, 0.5)' },
        horzLines: { color: 'rgba(43, 49, 57, 0.5)' },
      },
      crosshair: {
        mode: 0,
        vertLine: { color: 'rgba(240, 185, 11, 0.4)', width: 1, style: 2 },
        horzLine: { color: 'rgba(240, 185, 11, 0.4)', width: 1, style: 2 },
      },
      rightPriceScale: {
        borderColor: '#2B3139',
        scaleMargins: { top: 0.1, bottom: 0.25 },
      },
      timeScale: {
        borderColor: '#2B3139',
        timeVisible: true,
        secondsVisible: false,
      },
    });

    const candleSeries = chart.addSeries(CandlestickSeries, {
      upColor: '#0ECB81',
      downColor: '#F6465D',
      borderUpColor: '#0ECB81',
      borderDownColor: '#F6465D',
      wickUpColor: '#0ECB81',
      wickDownColor: '#F6465D',
    });

    const volumeSeries = chart.addSeries(HistogramSeries, {
      priceFormat: { type: 'volume' },
      priceScaleId: 'volume',
    });

    chart.priceScale('volume').applyOptions({
      scaleMargins: { top: 0.8, bottom: 0 },
    });

    chartRef.current = chart;
    candleSeriesRef.current = candleSeries;
    volumeSeriesRef.current = volumeSeries;

    // Handle resize
    const resizeObserver = new ResizeObserver((entries) => {
      for (const entry of entries) {
        chart.applyOptions({ width: entry.contentRect.width });
      }
    });
    resizeObserver.observe(chartContainerRef.current);

    return () => {
      resizeObserver.disconnect();
      chart.remove();
    };
  }, [height]);

  // Initialize chart once
  useEffect(() => {
    const cleanup = initChart();
    return () => cleanup?.();
  }, [initChart]);

  // Update data when klines change
  useEffect(() => {
    if (!candleSeriesRef.current || !volumeSeriesRef.current || !klines.length) return;

    const candleData: CandlestickData<Time>[] = klines.map((k) => ({
      time: (k.openTime / 1000) as Time,
      open: k.open,
      high: k.high,
      low: k.low,
      close: k.close,
    }));

    const volumeData: HistogramData<Time>[] = klines.map((k) => ({
      time: (k.openTime / 1000) as Time,
      value: k.volume,
      color: k.close >= k.open ? 'rgba(14, 203, 129, 0.3)' : 'rgba(246, 70, 93, 0.3)',
    }));

    candleSeriesRef.current.setData(candleData);
    volumeSeriesRef.current.setData(volumeData);
    chartRef.current?.timeScale().fitContent();
  }, [klines, symbol]);

  return <div ref={chartContainerRef} style={{ width: '100%' }} />;
}
