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
import { useEffect, useMemo, useState } from 'react'
import { VChart } from '@visactor/react-vchart'
import {
  AlertTriangle,
  GitBranch,
  Grid3X3,
  LineChart,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useThemeRadiusPx } from '@/lib/theme-radius'
import { VCHART_OPTION } from '@/lib/vchart'
import { useThemeCustomization } from '@/context/theme-customization-provider'
import { useTheme } from '@/context/theme-provider'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import type { LogAnalyticsInsights } from '../types'
import {
  buildAnalyticsHeatmapSpec,
  buildAnalyticsTimeSeriesSpec,
  buildAnalyticsTopologySpec,
} from '../lib/analytics-charts'

let themeManagerPromise: Promise<
  (typeof import('@visactor/vchart'))['ThemeManager']
> | null = null

function useVChartThemeReady() {
  const { resolvedTheme } = useTheme()
  const [themeReady, setThemeReady] = useState(false)

  useEffect(() => {
    let cancelled = false
    const updateTheme = async () => {
      setThemeReady(false)
      if (!themeManagerPromise) {
        themeManagerPromise = import('@visactor/vchart').then(
          (m) => m.ThemeManager
        )
      }
      const ThemeManager = await themeManagerPromise
      ThemeManager.setCurrentTheme(resolvedTheme === 'dark' ? 'dark' : 'light')
      if (!cancelled) setThemeReady(true)
    }
    void updateTheme()
    return () => {
      cancelled = true
    }
  }, [resolvedTheme])

  return themeReady
}

function AnalyticsChartCard(props: {
  title: string
  icon: React.ReactNode
  loading?: boolean
  empty?: boolean
  emptyMessage: string
  spec: Record<string, unknown> | null
  chartKey: string
  heightClass?: string
}) {
  const themeReady = useVChartThemeReady()
  const { resolvedTheme } = useTheme()
  const { customization } = useThemeCustomization()

  if (props.loading) {
    return (
      <Card className='border-border/60 shadow-xs'>
        <CardHeader className='pb-2'>
          <CardTitle className='flex items-center gap-2 text-sm'>
            {props.icon}
            {props.title}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <Skeleton className='h-56 w-full rounded-lg' />
        </CardContent>
      </Card>
    )
  }

  if (props.empty || !props.spec) {
    return (
      <Card className='border-border/60 shadow-xs'>
        <CardHeader className='pb-2'>
          <CardTitle className='flex items-center gap-2 text-sm'>
            {props.icon}
            {props.title}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className='text-muted-foreground flex h-40 items-center justify-center text-sm'>
            {props.emptyMessage}
          </div>
        </CardContent>
      </Card>
    )
  }

  return (
    <Card className='border-border/60 shadow-xs'>
      <CardHeader className='pb-2'>
        <CardTitle className='flex items-center gap-2 text-sm'>
          {props.icon}
          {props.title}
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className={props.heightClass ?? 'h-56 w-full'}>
          {themeReady ? (
            <VChart
              key={`${props.chartKey}-${resolvedTheme}-${customization.preset}`}
              spec={props.spec}
              option={VCHART_OPTION}
            />
          ) : (
            <Skeleton className='h-full w-full rounded-lg' />
          )}
        </div>
      </CardContent>
    </Card>
  )
}

type LogAnalyticsVisualizationsProps = {
  insights?: LogAnalyticsInsights | null
  loading?: boolean
  onApplyErrorFilter?: (filter: {
    model?: string
    channel?: string
  }) => void
}

export function LogAnalyticsVisualizations({
  insights,
  loading,
  onApplyErrorFilter,
}: LogAnalyticsVisualizationsProps) {
  const { t } = useTranslation()
  const { customization } = useThemeCustomization()
  const chartRadius = useThemeRadiusPx(
    '--radius-md',
    `${customization.preset}:${customization.radius}`
  )

  const timeSeriesSpec = useMemo(
    () =>
      insights?.time_series?.length
        ? buildAnalyticsTimeSeriesSpec(
            insights.time_series,
            insights.bucket_seconds,
            t,
            chartRadius
          )
        : null,
    [chartRadius, insights, t]
  )

  const heatmapSpec = useMemo(
    () =>
      insights?.heatmap?.length
        ? buildAnalyticsHeatmapSpec(insights.heatmap, t)
        : null,
    [insights?.heatmap, t]
  )

  const topologySpec = useMemo(
    () =>
      insights?.flow_links?.length
        ? buildAnalyticsTopologySpec(insights.flow_links, t)
        : null,
    [insights?.flow_links, t]
  )

  return (
    <div className='space-y-4'>
      <div className='grid gap-4 xl:grid-cols-2'>
        <AnalyticsChartCard
          title={t('Call Trend Over Time')}
          icon={<LineChart className='size-4' aria-hidden='true' />}
          loading={loading}
          empty={!insights?.time_series?.length}
          emptyMessage={t('No time-series data in this range')}
          spec={timeSeriesSpec}
          chartKey='analytics-time-series'
        />
        <AnalyticsChartCard
          title={t('Activity Heatmap (UTC)')}
          icon={<Grid3X3 className='size-4' aria-hidden='true' />}
          loading={loading}
          empty={!insights?.heatmap?.length}
          emptyMessage={t('No heatmap data in this range')}
          spec={heatmapSpec}
          chartKey='analytics-heatmap'
        />
      </div>

      <div className='grid gap-4 xl:grid-cols-2'>
        <AnalyticsChartCard
          title={t('Request Path Topology')}
          icon={<GitBranch className='size-4' aria-hidden='true' />}
          loading={loading}
          empty={!topologySpec}
          emptyMessage={t('No topology data in this range')}
          spec={topologySpec}
          chartKey='analytics-topology'
          heightClass='h-72 w-full'
        />

        <Card className='border-border/60 shadow-xs'>
          <CardHeader className='space-y-1 pb-2'>
            <CardTitle className='flex items-center gap-2 text-sm'>
              <AlertTriangle className='size-4 text-rose-500' aria-hidden='true' />
              {t('Error Localization')}
            </CardTitle>
            {onApplyErrorFilter ? (
              <p className='text-muted-foreground text-xs font-normal'>
                {t('Click a row to filter breakdown by model and channel.')}
              </p>
            ) : null}
          </CardHeader>
          <CardContent className='overflow-auto'>
            {loading ? (
              <Skeleton className='h-72 w-full rounded-lg' />
            ) : !insights?.errors?.length ? (
              <div className='text-muted-foreground flex h-72 items-center justify-center text-sm'>
                {t('No errors in this time range')}
              </div>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('Error Message')}</TableHead>
                    <TableHead>{t('Model')}</TableHead>
                    <TableHead>{t('Channel')}</TableHead>
                    <TableHead className='text-right'>{t('Count')}</TableHead>
                    <TableHead className='text-right'>{t('Last Seen')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {insights.errors.map((row, index) => {
                    const channelLabel = row.channel_name
                      ? `${row.channel_id} (${row.channel_name})`
                      : row.channel_id
                        ? String(row.channel_id)
                        : '-'
                    return (
                      <TableRow
                        key={`${row.message}-${row.model_name}-${row.channel_id}-${index}`}
                        className={
                          onApplyErrorFilter
                            ? 'hover:bg-muted/40 cursor-pointer'
                            : undefined
                        }
                        onClick={() => {
                          if (!onApplyErrorFilter) return
                          onApplyErrorFilter({
                            model: row.model_name || undefined,
                            channel: row.channel_id
                              ? String(row.channel_id)
                              : undefined,
                          })
                        }}
                      >
                        <TableCell className='max-w-[280px]'>
                          <div className='line-clamp-2 font-mono text-xs'>
                            {row.message}
                          </div>
                        </TableCell>
                        <TableCell className='font-mono text-xs'>
                          {row.model_name || '-'}
                        </TableCell>
                        <TableCell className='font-mono text-xs'>
                          {channelLabel}
                        </TableCell>
                        <TableCell className='text-right'>
                          <Badge variant='destructive'>{row.count}</Badge>
                        </TableCell>
                        <TableCell className='text-muted-foreground text-right font-mono text-xs'>
                          {row.latest_at
                            ? new Date(row.latest_at * 1000).toLocaleString()
                            : '-'}
                        </TableCell>
                      </TableRow>
                    )
                  })}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
