/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { getDashboardChartColors } from '@/features/dashboard/lib/charts'
import type {
  LogAnalyticsFlowLink,
  LogAnalyticsHeatmapCell,
  LogAnalyticsInsights,
  LogAnalyticsTimeSeriesPoint,
} from '../types'

// eslint-disable-next-line @typescript-eslint/no-explicit-any
type VChartSpec = Record<string, any>

type TFunction = (key: string) => string

const WEEKDAY_KEYS = [
  'Sunday',
  'Monday',
  'Tuesday',
  'Wednesday',
  'Thursday',
  'Friday',
  'Saturday',
] as const

function formatBucketLabel(bucketStart: number, bucketSeconds: number) {
  const date = new Date(bucketStart * 1000)
  if (bucketSeconds >= 86400) {
    return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
  }
  return date.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

export function buildAnalyticsTimeSeriesSpec(
  points: LogAnalyticsTimeSeriesPoint[],
  bucketSeconds: number,
  t: TFunction,
  chartCornerRadius?: number
): VChartSpec {
  const callsLabel = t('Calls')
  const failuresLabel = t('Failures')
  const values = points.flatMap((point) => [
    {
      bucket: formatBucketLabel(point.bucket_start, bucketSeconds),
      bucket_start: point.bucket_start,
      series: callsLabel,
      value: point.call_count,
    },
    {
      bucket: formatBucketLabel(point.bucket_start, bucketSeconds),
      bucket_start: point.bucket_start,
      series: failuresLabel,
      value: point.failure_count,
    },
  ])

  const colors = getDashboardChartColors(2)

  return {
    type: 'area',
    data: [{ id: 'time-series', values }],
    xField: 'bucket',
    yField: 'value',
    seriesField: 'series',
    stack: false,
    point: { visible: points.length <= 48 },
    line: { style: { lineWidth: 2 } },
    area: {
      style: {
        fillOpacity: 0.18,
        ...(chartCornerRadius ? { cornerRadius: chartCornerRadius } : {}),
      },
    },
    color: colors,
    legends: [{ visible: true, orient: 'top', position: 'start' }],
    tooltip: {
      mark: {
        content: [
          {
            key: (datum: Record<string, unknown>) => String(datum.series ?? ''),
            value: (datum: Record<string, unknown>) =>
              Number(datum.value ?? 0).toLocaleString(),
          },
        ],
      },
    },
  }
}

export function buildAnalyticsHeatmapSpec(
  cells: LogAnalyticsHeatmapCell[],
  t: TFunction
): VChartSpec {
  const weekdayLabels = WEEKDAY_KEYS.map((key) => t(key))
  const values = cells.map((cell) => ({
    hour: String(cell.hour).padStart(2, '0') + ':00',
    hour_num: cell.hour,
    weekday: weekdayLabels[cell.weekday] ?? String(cell.weekday),
    weekday_num: cell.weekday,
    call_count: cell.call_count,
    failure_count: cell.failure_count,
    failure_rate: cell.failure_rate,
    intensity: cell.call_count + cell.failure_count,
  }))

  return {
    type: 'heatmap',
    data: [{ id: 'heatmap', values }],
    xField: 'hour',
    yField: 'weekday',
    valueField: 'intensity',
    cell: {
      style: {
        fill: {
          field: 'failure_rate',
          scale: 'color',
        },
      },
    },
    scales: [
      {
        id: 'color',
        type: 'linear',
        domain: [0, Math.max(...values.map((v) => v.failure_rate), 1)],
        range: ['#dbeafe', '#fca5a5', '#dc2626'],
      },
    ],
    tooltip: {
      mark: {
        content: [
          {
            key: t('Call Count'),
            value: (datum: Record<string, unknown>) =>
              Number(datum.call_count ?? 0).toLocaleString(),
          },
          {
            key: t('Failure Count'),
            value: (datum: Record<string, unknown>) =>
              Number(datum.failure_count ?? 0).toLocaleString(),
          },
          {
            key: t('Failure Rate'),
            value: (datum: Record<string, unknown>) =>
              `${Number(datum.failure_rate ?? 0).toFixed(2)}%`,
          },
        ],
      },
    },
  }
}

function flowNodeID(kind: string, label: string) {
  return `${kind}:${label}`
}

export function buildAnalyticsTopologySpec(
  links: LogAnalyticsFlowLink[],
  t: TFunction
): VChartSpec | null {
  if (links.length === 0) return null

  const nodeMap = new Map<string, { id: string; label: string; kind: string; value: number }>()
  const sankeyLinks: Array<Record<string, unknown>> = []

  for (const link of links) {
    const sourceID = flowNodeID(link.source_kind, link.source)
    const targetID = flowNodeID(link.target_kind, link.target)
    const value = link.call_count + link.failure_count
    if (value <= 0) continue

    if (!nodeMap.has(sourceID)) {
      nodeMap.set(sourceID, {
        id: sourceID,
        label: link.source,
        kind: link.source_kind,
        value: 0,
      })
    }
    if (!nodeMap.has(targetID)) {
      nodeMap.set(targetID, {
        id: targetID,
        label: link.target,
        kind: link.target_kind,
        value: 0,
      })
    }
    nodeMap.get(sourceID)!.value += value
    nodeMap.get(targetID)!.value += value

    sankeyLinks.push({
      source: sourceID,
      target: targetID,
      value,
      call_count: link.call_count,
      failure_count: link.failure_count,
      failure_rate:
        value > 0
          ? Math.round((link.failure_count / value) * 10000) / 100
          : 0,
    })
  }

  if (sankeyLinks.length === 0) return null

  const colors = getDashboardChartColors(nodeMap.size)
  const kindColor: Record<string, string> = {
    group: colors[0] ?? '#38bdf8',
    model: colors[1] ?? '#34d399',
    channel: colors[2] ?? '#fbbf24',
  }

  const nodes = Array.from(nodeMap.values()).map((node) => ({
    key: node.id,
    name: node.label,
    kind: node.kind,
    value: node.value,
    color: kindColor[node.kind] ?? colors[0],
  }))

  return {
    type: 'sankey',
    data: [
      {
        id: 'topology',
        values: [{ nodes, links: sankeyLinks }],
      },
    ],
    categoryField: 'name',
    valueField: 'value',
    sourceField: 'source',
    targetField: 'target',
    node: {
      style: {
        fill: (datum: Record<string, unknown>) =>
          String(datum.color ?? kindColor.group),
      },
    },
    link: {
      style: {
        fillOpacity: 0.35,
      },
    },
    tooltip: {
      mark: {
        title: t('Request Path'),
        content: [
          {
            key: t('Call Count'),
            value: (datum: Record<string, unknown>) =>
              Number(datum.call_count ?? datum.value ?? 0).toLocaleString(),
          },
          {
            key: t('Failure Count'),
            value: (datum: Record<string, unknown>) =>
              Number(datum.failure_count ?? 0).toLocaleString(),
          },
          {
            key: t('Failure Rate'),
            value: (datum: Record<string, unknown>) =>
              `${Number(datum.failure_rate ?? 0).toFixed(2)}%`,
          },
        ],
      },
    },
  }
}

export function buildSessionTraceTimelineSpec(
  turns: Array<{ turn_index: number; status: string; created_at: number }>,
  t: TFunction
): VChartSpec | null {
  if (turns.length === 0) return null

  const successLabel = t('Success')
  const failedLabel = t('Failed')
  const values = turns.map((turn) => ({
    turn: `#${turn.turn_index + 1}`,
    turn_index: turn.turn_index,
    status: turn.status === 'success' ? successLabel : failedLabel,
    value: 1,
    created_at: turn.created_at,
  }))

  return {
    type: 'bar',
    data: [{ id: 'trace-timeline', values }],
    xField: 'turn',
    yField: 'value',
    seriesField: 'status',
    stack: true,
    color: ['#22c55e', '#ef4444'],
    legends: [{ visible: true, orient: 'top', position: 'start' }],
    tooltip: {
      mark: {
        content: [
          {
            key: t('Status'),
            value: (datum: Record<string, unknown>) => String(datum.status ?? ''),
          },
          {
            key: t('Time'),
            value: (datum: Record<string, unknown>) => {
              const ts = Number(datum.created_at ?? 0)
              if (!ts) return '-'
              return new Date(ts * 1000).toLocaleString()
            },
          },
        ],
      },
    },
  }
}

export function hasAnalyticsInsights(insights?: LogAnalyticsInsights | null) {
  if (!insights) return false
  return (
    insights.time_series.length > 0 ||
    insights.heatmap.length > 0 ||
    insights.errors.length > 0 ||
    insights.flow_links.length > 0
  )
}
