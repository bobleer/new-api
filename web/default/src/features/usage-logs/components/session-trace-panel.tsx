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
import { useCallback, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Download, GitBranch, RefreshCw, Search } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatTokens } from '@/lib/format'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'
import {
  downloadSessionTraceTurn,
  getSessionTrace,
} from '../api'
import type { SessionTraceFullView } from '../types'
import { SessionTraceTimeline } from './session-trace-timeline'

function formatTimestamp(ts: number) {
  if (!ts) return '-'
  return new Date(ts * 1000).toLocaleString()
}

function TurnStatusBadge(props: { status: string }) {
  const { t } = useTranslation()
  const isSuccess = props.status === 'success'
  return (
    <Badge variant={isSuccess ? 'default' : 'destructive'}>
      {isSuccess ? t('Success') : t('Failed')}
    </Badge>
  )
}

function SessionTraceSummary(props: { data: SessionTraceFullView }) {
  const { t } = useTranslation()
  const { data } = props
  return (
    <div className='grid gap-4 md:grid-cols-2 xl:grid-cols-4'>
      <Card className='border-border/60 shadow-xs'>
        <CardHeader className='pb-2'>
          <CardTitle className='text-muted-foreground text-xs font-medium'>
            {t('Trace ID')}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className='font-mono text-sm break-all'>{data.trace_id}</div>
        </CardContent>
      </Card>
      <Card className='border-border/60 shadow-xs'>
        <CardHeader className='pb-2'>
          <CardTitle className='text-muted-foreground text-xs font-medium'>
            {t('Turn Count')}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className='font-mono text-2xl font-semibold tabular-nums'>
            {data.turn_count}
          </div>
        </CardContent>
      </Card>
      <Card className='border-border/60 shadow-xs'>
        <CardHeader className='pb-2'>
          <CardTitle className='text-muted-foreground text-xs font-medium'>
            {t('Model')}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className='font-mono text-sm'>{data.model_name || '-'}</div>
        </CardContent>
      </Card>
      <Card className='border-border/60 shadow-xs'>
        <CardHeader className='pb-2'>
          <CardTitle className='text-muted-foreground text-xs font-medium'>
            {t('Last Activity')}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className='text-sm'>{formatTimestamp(data.last_activity_at)}</div>
        </CardContent>
      </Card>
    </div>
  )
}

function TurnDetailBlock(props: {
  title: string
  content?: string
  emptyText: string
}) {
  if (!props.content) {
    return (
      <div className='space-y-2'>
        <Label>{props.title}</Label>
        <p className='text-muted-foreground text-sm'>{props.emptyText}</p>
      </div>
    )
  }
  return (
    <div className='space-y-2'>
      <Label>{props.title}</Label>
      <pre className='bg-muted/40 max-h-80 overflow-auto rounded-md border p-3 text-xs whitespace-pre-wrap break-all'>
        {props.content}
      </pre>
    </div>
  )
}

export function SessionTracePanel() {
  const { t } = useTranslation()
  const [traceIdInput, setTraceIdInput] = useState('')
  const [activeTraceId, setActiveTraceId] = useState('')

  const { data, isLoading, isFetching, refetch, error } = useQuery({
    queryKey: ['session-trace', activeTraceId],
    enabled: activeTraceId.length > 0,
    queryFn: async () => {
      const res = await getSessionTrace(activeTraceId)
      if (!res.success || !res.data) {
        throw new Error(res.message || t('Session trace not found'))
      }
      return res.data
    },
    retry: false,
  })

  const handleSearch = useCallback(() => {
    const trimmed = traceIdInput.trim()
    if (!trimmed) {
      toast.error(t('Please enter a Trace ID'))
      return
    }
    setActiveTraceId(trimmed)
  }, [t, traceIdInput])

  const handleDownloadTurn = useCallback(
    async (traceId: string, turnIndex: number) => {
      try {
        const blob = await downloadSessionTraceTurn(traceId, turnIndex)
        const url = URL.createObjectURL(blob)
        const link = document.createElement('a')
        link.href = url
        link.download = `session-trace-${traceId}-turn-${turnIndex}.json`
        document.body.appendChild(link)
        link.click()
        link.remove()
        URL.revokeObjectURL(url)
      } catch {
        toast.error(t('Failed to download session trace turn'))
      }
    },
    [t]
  )

  return (
    <div className='flex h-full min-h-0 flex-col gap-4'>
      <Card className='border-border/60 shadow-xs'>
        <CardHeader className='pb-3'>
          <CardTitle className='flex items-center gap-2 text-base'>
            <GitBranch className='size-4' />
            {t('Session Trace Lookup')}
          </CardTitle>
        </CardHeader>
        <CardContent className='space-y-4'>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Enter a Trace ID to inspect the full multi-turn conversation context recorded by the gateway.'
            )}
          </p>
          <div className='flex flex-col gap-3 sm:flex-row'>
            <div className='flex-1 space-y-2'>
              <Label htmlFor='session-trace-id'>{t('Trace ID')}</Label>
              <Input
                id='session-trace-id'
                value={traceIdInput}
                onChange={(event) => setTraceIdInput(event.target.value)}
                placeholder='xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx'
                onKeyDown={(event) => {
                  if (event.key === 'Enter') {
                    handleSearch()
                  }
                }}
              />
            </div>
            <div className='flex items-end gap-2'>
              <Button onClick={handleSearch} disabled={isFetching}>
                <Search className='size-4' />
                {t('Search')}
              </Button>
              {activeTraceId ? (
                <Button
                  variant='outline'
                  onClick={() => void refetch()}
                  disabled={isFetching}
                >
                  <RefreshCw className={cn('size-4', isFetching && 'animate-spin')} />
                  {t('Refresh')}
                </Button>
              ) : null}
            </div>
          </div>
        </CardContent>
      </Card>

      {isLoading ? (
        <div className='space-y-4'>
          <Skeleton className='h-28 w-full' />
          <Skeleton className='h-64 w-full' />
        </div>
      ) : null}

      {!isLoading && error ? (
        <Card className='border-destructive/30'>
          <CardContent className='pt-6'>
            <p className='text-destructive text-sm'>
              {error instanceof Error
                ? error.message
                : t('Session trace not found')}
            </p>
          </CardContent>
        </Card>
      ) : null}

      {!isLoading && data ? (
        <>
          <SessionTraceSummary data={data} />
          {data.data_source ? (
            <div className='flex items-center gap-2'>
              <Badge variant='outline'>
                {t('Data source')}: {data.data_source}
              </Badge>
            </div>
          ) : null}
          <SessionTraceTimeline turns={data.turns} />
          <Card className='min-h-0 flex-1 border-border/60 shadow-xs'>
            <CardHeader className='pb-3'>
              <CardTitle className='text-base'>{t('Conversation Turns')}</CardTitle>
            </CardHeader>
            <CardContent>
              {data.turns.length === 0 ? (
                <p className='text-muted-foreground text-sm'>
                  {t('No turns recorded for this session yet.')}
                </p>
              ) : (
                <Accordion className='w-full'>
                  {data.turns.map((turn) => (
                    <AccordionItem
                      key={`${turn.trace_id}-${turn.turn_index}`}
                      value={`turn-${turn.turn_index}`}
                    >
                      <AccordionTrigger className='hover:no-underline'>
                        <div className='flex flex-1 flex-wrap items-center gap-3 pr-4 text-left'>
                          <span className='font-medium'>
                            {t('Turn {{index}}', { index: turn.turn_index })}
                          </span>
                          <TurnStatusBadge status={turn.status} />
                          <span className='text-muted-foreground text-xs'>
                            {formatTimestamp(turn.created_at)}
                          </span>
                          <span className='text-muted-foreground font-mono text-xs'>
                            {turn.request_id}
                          </span>
                          <span className='text-muted-foreground text-xs'>
                            {formatTokens(turn.prompt_tokens)} /{' '}
                            {formatTokens(turn.completion_tokens)}
                          </span>
                        </div>
                      </AccordionTrigger>
                      <AccordionContent className='space-y-4 pt-2'>
                        {turn.error_message ? (
                          <div className='space-y-2'>
                            <Label>{t('Error Message')}</Label>
                            <pre className='bg-destructive/5 text-destructive max-h-40 overflow-auto rounded-md border p-3 text-xs whitespace-pre-wrap break-all'>
                              {turn.error_message}
                            </pre>
                          </div>
                        ) : null}
                        <TurnDetailBlock
                          title={t('Client Request')}
                          content={turn.detail?.client_request}
                          emptyText={t('No request payload saved for this turn.')}
                        />
                        <TurnDetailBlock
                          title={t('Assistant Response')}
                          content={turn.detail?.assistant_response}
                          emptyText={t(
                            'No response payload saved for this turn. Streaming responses may be partial or empty.'
                          )}
                        />
                        {turn.detail ? (
                          <div className='flex flex-wrap gap-2'>
                            <Button
                              variant='outline'
                              size='sm'
                              onClick={() =>
                                void handleDownloadTurn(data.trace_id, turn.turn_index)
                              }
                            >
                              <Download className='size-4' />
                              {t('Download Turn JSON')}
                            </Button>
                            {turn.detail.truncated ? (
                              <Badge variant='secondary'>{t('Truncated')}</Badge>
                            ) : null}
                            {turn.is_stream ? (
                              <Badge variant='outline'>{t('Stream')}</Badge>
                            ) : null}
                          </div>
                        ) : null}
                      </AccordionContent>
                    </AccordionItem>
                  ))}
                </Accordion>
              )}
            </CardContent>
          </Card>
        </>
      ) : null}
    </div>
  )
}
