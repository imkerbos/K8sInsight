import ReactECharts from 'echarts-for-react'
import dayjs from '../utils/dayjs'

export type TimeValue = {
  ts: number
  value: number
}

type Props = {
  samples: TimeValue[]
  color: string
  height?: number
  unit: string
  seriesLabel: string
  limitValue?: number | null
  limitLabel?: string
}

function fmtNumber(n: number, digits = 2) {
  return n.toFixed(digits)
}

export default function TimeSeriesChart({
  samples,
  color,
  height = 280,
  unit,
  seriesLabel,
  limitValue,
  limitLabel,
}: Props) {
  if (!samples.length) {
    return (
      <div style={{ height, display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#999' }}>
        暂无曲线数据
      </div>
    )
  }

  const times = samples.map((s) => dayjs.unix(s.ts).format('HH:mm:ss'))
  const values = samples.map((s) => s.value)

  const series: Record<string, unknown>[] = [
    {
      name: seriesLabel,
      type: 'line',
      data: values,
      smooth: true,
      symbol: 'none',
      lineStyle: { width: 2, color },
      areaStyle: {
        color: {
          type: 'linear',
          x: 0, y: 0, x2: 0, y2: 1,
          colorStops: [
            { offset: 0, color: color + '40' },
            { offset: 1, color: color + '05' },
          ],
        },
      },
    },
  ]

  // Memory limit line
  if (limitValue && Number.isFinite(limitValue)) {
    series.push({
      name: limitLabel || `Limit: ${fmtNumber(limitValue)} ${unit}`,
      type: 'line',
      data: new Array(samples.length).fill(limitValue),
      symbol: 'none',
      lineStyle: { width: 1.5, color: '#ff4d4f', type: 'dashed' },
      itemStyle: { color: '#ff4d4f' },
    })
  }

  const option = {
    tooltip: {
      trigger: 'axis',
      backgroundColor: 'rgba(255,255,255,0.96)',
      borderColor: '#e8e8e8',
      textStyle: { color: '#333', fontSize: 12 },
      formatter: (params: { axisValueLabel?: string; color?: string; seriesName?: string; value?: number }[]) => {
        const time = params[0]?.axisValueLabel || ''
        let html = `<div style="font-weight:600;margin-bottom:4px">${time}</div>`
        params.forEach((p) => {
          html += `<div style="display:flex;align-items:center;gap:6px">
            <span style="display:inline-block;width:8px;height:8px;border-radius:50%;background:${p.color}"></span>
            <span>${p.seriesName}: <b>${fmtNumber(p.value)}</b> ${unit}</span>
          </div>`
        })
        return html
      },
    },
    grid: { left: 60, right: 20, top: 30, bottom: 40 },
    xAxis: {
      type: 'category',
      data: times,
      axisLabel: { fontSize: 10, color: '#8c8c8c', interval: Math.max(0, Math.floor(times.length / 6)) },
      axisLine: { lineStyle: { color: '#e8e8e8' } },
      axisTick: { show: false },
    },
    yAxis: {
      type: 'value',
      axisLabel: {
        fontSize: 10,
        color: '#8c8c8c',
        formatter: (v: number) => `${fmtNumber(v)} ${unit}`,
      },
      splitLine: { lineStyle: { color: '#f0f0f0', type: 'dashed' } },
    },
    legend: {
      show: true,
      top: 0,
      right: 0,
      textStyle: { fontSize: 12, color: '#595959' },
    },
    series,
  }

  return (
    <ReactECharts
      option={option}
      style={{ height, width: '100%' }}
      notMerge
      lazyUpdate
    />
  )
}
