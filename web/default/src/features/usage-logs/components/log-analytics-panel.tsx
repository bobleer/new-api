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
import { useCallback, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { BarChart3, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatTokens } from '@/lib/format'
import { cn } from '@/lib/utils'
import { useIsAdmin } from '@/hooks/use-admin'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { getLogAnalytics, getUserLogAnalytics } from '../api'
import { LOG_ANALYTICS_TIME_PRESETS } from '../constants'
import { buildAnalyticsApiParams } from '../lib/analytics'
import type { LogAnalyticsGroupBy, LogAnalyticsResult } from '../types'
import { CompactDateTimeRangePicker } from './compact-date-time-range-picker'
import { LogAnalyticsVisualizations } from './log-analytics-visualizations'

const route = getRouteApi('/_authenticated/usage-logs/$section')

const EMPTY_ANALYTICS: LogAnalyticsResult = {
  summary: {
    call_count: 0,
    token_count: 0,
    failure_count: 0,
    failure_rate: 0,
  },
  groups: [],
}

function SummaryCard(props: {
  title: string
  value: string
  accent: string
}) {
  return (
    <Card className='border-border/60 shadow-xs'>
      <CardHeader className='pb-2'>
        <CardTitle className='text-muted-foreground flex items-center gap-2 text-xs font-medium'>
          <span className={cn('h-2 w-2 rounded-full', props.accent)} />
          {props.title}
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className='font-mono text-2xl font-semibold tabular-nums'>
          {props.value}
        </div>
      </CardContent>
    </Card>
  )
}

export function LogAnalyticsPanel() {
  const { t } = useTranslation()
  const isAdmin = useIsAdmin()
  const searchParams = route.useSearch()
  const navigate = route.useNavigate()

  const [draftFilters, setDraftFilters] = useState({
    model: searchParams.model ?? '',
    token: searchParams.token ?? '',
    channel: searchParams.channel ?? '',
    group: searchParams.group ?? '',
    username: searchParams.username ?? '',
  })

  const groupBy: LogAnalyticsGroupBy =
    searchParams.groupBy === 'token' ? 'token' : 'channel'

  const timeRange = useMemo(() => {
    const end = searchParams.endTime ? new Date(searchParams.endTime) : new Date()
    const start = searchParams.startTime
      ? new Date(searchParams.startTime)
      : new Date(end.getTime() - LOG_ANALYTICS_TIME_PRESETS[1].hours * 3600 * 1000)
    return { start, end }
  }, [searchParams.endTime, searchParams.startTime])

  const apiParams = useMemo(
    () => buildAnalyticsApiParams(searchParams, isAdmin),
    [isAdmin, searchParams]
  )

  const { data, isLoading, isFetching, refetch } = useQuery({
    queryKey: ['log-analytics', isAdmin, apiParams],
    queryFn: async () => {
      const result = isAdmin
        ? await getLogAnalytics(apiParams)
        : await getUserLogAnalytics(apiParams)
      if (!result.success) {
        toast.error(result.message || t('Failed to load log analytics'))
        return EMPTY_ANALYTICS
      }
      return result.data ?? EMPTY_ANALYTICS
    },
    enabled: Boolean(apiParams.start_timestamp && apiParams.end_timestamp),
  })

  const analytics = data ?? EMPTY_ANALYTICS

  const applyPreset = useCallback(
    (hours: number) => {
      const end = new Date()
      const start = new Date(end.getTime() - hours * 3600 * 1000)
      void navigate({
        search: (prev) => ({
          ...prev,
          startTime: start.getTime(),
          endTime: end.getTime(),
        }),
      })
    },
    [navigate]
  )

  const updateSearch = useCallback(
    (patch: Record<string, unknown>) => {
      void navigate({
        search: (prev) => ({
          ...prev,
          ...patch,
        }),
      })
    },
    [navigate]
  )

  const applyOptionalFilters = () => {
    updateSearch({
      model: draftFilters.model.trim() || undefined,
      token: draftFilters.token.trim() || undefined,
      channel: draftFilters.channel.trim() || undefined,
      group: draftFilters.group.trim() || undefined,
      username: draftFilters.username.trim() || undefined,
    })
  }

  const applyErrorLocalizationFilter = useCallback(
    (filter: { model?: string; channel?: string }) => {
      setDraftFilters((prev) => ({
        ...prev,
        model: filter.model ?? prev.model,
        channel: filter.channel ?? prev.channel,
      }))
      updateSearch({
        model: filter.model || undefined,
        channel: filter.channel || undefined,
      })
    },
    [updateSearch]
  )

  const formatFailureRate = (rate: number) => `${rate.toFixed(2)}%`

  return (
    <div className='flex h-full min-h-0 flex-col gap-4'>
      <div className='flex flex-wrap items-center gap-2'>
        <div className='flex flex-wrap items-center gap-1.5'>
          {LOG_ANALYTICS_TIME_PRESETS.map((preset) => (
            <Button
              key={preset.key}
              variant='outline'
              size='sm'
              className='h-8'
              onClick={() => applyPreset(preset.hours)}
            >
              {t(preset.label)}
            </Button>
          ))}
        </div>
        <CompactDateTimeRangePicker
          start={timeRange.start}
          end={timeRange.end}
          onChange={(range) => {
            updateSearch({
              startTime: range.start?.getTime(),
              endTime: range.end?.getTime(),
            })
          }}
        />
        <Select
          value={groupBy}
          onValueChange={(value) => {
            if (value === 'channel' || value === 'token') {
              updateSearch({ groupBy: value })
            }
          }}
        >
          <SelectTrigger className='h-8 w-[180px]'>
            <SelectValue placeholder={t('Group by')} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value='channel'>{t('Group by Channel')}</SelectItem>
            <SelectItem value='token'>{t('Group by API Key')}</SelectItem>
          </SelectContent>
        </Select>
        <Button
          variant='outline'
          size='sm'
          className='h-8'
          onClick={() => void refetch()}
          disabled={isFetching}
        >
          <RefreshCw
            className={cn('mr-2 size-4', isFetching && 'animate-spin')}
          />
          {t('Analyze')}
        </Button>
      </div>

      <div className='border-border/60 bg-muted/20 rounded-lg border p-3'>
        <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5'>
          {isAdmin && (
            <div className='space-y-1.5'>
              <Label className='text-xs'>{t('Username')}</Label>
              <Input
                value={draftFilters.username}
                onChange={(event) =>
                  setDraftFilters((prev) => ({
                    ...prev,
                    username: event.target.value,
                  }))
                }
                placeholder={t('Username')}
                className='h-8'
              />
            </div>
          )}
          <div className='space-y-1.5'>
            <Label className='text-xs'>{t('Model')}</Label>
            <Input
              value={draftFilters.model}
              onChange={(event) =>
                setDraftFilters((prev) => ({
                  ...prev,
                  model: event.target.value,
                }))
              }
              placeholder={t('Model')}
              className='h-8'
            />
          </div>
          <div className='space-y-1.5'>
            <Label className='text-xs'>{t('Token')}</Label>
            <Input
              value={draftFilters.token}
              onChange={(event) =>
                setDraftFilters((prev) => ({
                  ...prev,
                  token: event.target.value,
                }))
              }
              placeholder={t('Token')}
              className='h-8'
            />
          </div>
          {isAdmin && (
            <div className='space-y-1.5'>
              <Label className='text-xs'>{t('Channel')}</Label>
              <Input
                value={draftFilters.channel}
                onChange={(event) =>
                  setDraftFilters((prev) => ({
                    ...prev,
                    channel: event.target.value,
                  }))
                }
                placeholder={t('Channel')}
                className='h-8'
              />
            </div>
          )}
          <div className='space-y-1.5'>
            <Label className='text-xs'>{t('Group')}</Label>
            <Input
              value={draftFilters.group}
              onChange={(event) =>
                setDraftFilters((prev) => ({
                  ...prev,
                  group: event.target.value,
                }))
              }
              placeholder={t('Group')}
              className='h-8'
            />
          </div>
        </div>
        <div className='mt-3 flex justify-end gap-2'>
          <Button
            variant='outline'
            size='sm'
            onClick={() => {
              setDraftFilters({
                model: '',
                token: '',
                channel: '',
                group: '',
                username: '',
              })
              updateSearch({
                model: undefined,
                token: undefined,
                channel: undefined,
                group: undefined,
                username: undefined,
              })
            }}
          >
            {t('Reset')}
          </Button>
          <Button size='sm' onClick={applyOptionalFilters}>
            {t('Apply Filters')}
          </Button>
        </div>
      </div>

      <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-4'>
        {isLoading ? (
          Array.from({ length: 4 }).map((_, index) => (
            <Skeleton key={index} className='h-24 rounded-xl' />
          ))
        ) : (
          <>
            <SummaryCard
              title={t('Call Count')}
              value={String(analytics.summary.call_count)}
              accent='bg-sky-500'
            />
            <SummaryCard
              title={t('Token Usage')}
              value={formatTokens(analytics.summary.token_count)}
              accent='bg-emerald-500'
            />
            <SummaryCard
              title={t('Failure Count')}
              value={String(analytics.summary.failure_count)}
              accent='bg-rose-500'
            />
            <SummaryCard
              title={t('Failure Rate')}
              value={formatFailureRate(analytics.summary.failure_rate)}
              accent='bg-amber-500'
            />
          </>
        )}
      </div>

      <LogAnalyticsVisualizations
        insights={analytics.insights}
        loading={isLoading}
        onApplyErrorFilter={applyErrorLocalizationFilter}
      />

      <Card className='min-h-0 flex-1 border-border/60 shadow-xs'>
        <CardHeader className='pb-3'>
          <CardTitle className='flex items-center gap-2 text-base'>
            <BarChart3 className='size-4' aria-hidden='true' />
            {groupBy === 'channel'
              ? t('Channel Breakdown')
              : t('API Key Breakdown')}
          </CardTitle>
        </CardHeader>
        <CardContent className='overflow-auto'>
          {isLoading ? (
            <Skeleton className='h-48 w-full rounded-lg' />
          ) : analytics.groups.length === 0 ? (
            <div className='text-muted-foreground py-10 text-center text-sm'>
              {t('No analytics data in this time range')}
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>
                    {groupBy === 'channel' ? t('Channel') : t('API Key')}
                  </TableHead>
                  <TableHead className='text-right'>{t('Call Count')}</TableHead>
                  <TableHead className='text-right'>{t('Token Usage')}</TableHead>
                  <TableHead className='text-right'>{t('Failure Count')}</TableHead>
                  <TableHead className='text-right'>{t('Failure Rate')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {analytics.groups.map((row) => {
                  const key =
                    groupBy === 'channel'
                      ? `channel-${row.channel_id}`
                      : `token-${row.token_id}-${row.token_name}`
                  const label =
                    groupBy === 'channel'
                      ? row.channel_name
                        ? `${row.channel_id} (${row.channel_name})`
                        : String(row.channel_id || '-')
                      : row.token_name || '-'
                  return (
                    <TableRow key={key}>
                      <TableCell className='font-mono text-xs'>{label}</TableCell>
                      <TableCell className='text-right font-mono tabular-nums'>
                        {row.call_count}
                      </TableCell>
                      <TableCell className='text-right font-mono tabular-nums'>
                        {formatTokens(row.token_count)}
                      </TableCell>
                      <TableCell className='text-right font-mono tabular-nums'>
                        {row.failure_count}
                      </TableCell>
                      <TableCell className='text-right font-mono tabular-nums'>
                        {formatFailureRate(row.failure_rate)}
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
  )
}
