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
import { Activity } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { VCHART_OPTION } from '@/lib/vchart'
import { useTheme } from '@/context/theme-provider'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { buildSessionTraceTimelineSpec } from '../lib/analytics-charts'
import type { SessionTraceTurn } from '../types'

let themeManagerPromise: Promise<
  (typeof import('@visactor/vchart'))['ThemeManager']
> | null = null

type SessionTraceTimelineProps = {
  turns: SessionTraceTurn[]
}

export function SessionTraceTimeline({ turns }: SessionTraceTimelineProps) {
  const { t } = useTranslation()
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

  const spec = useMemo(
    () => buildSessionTraceTimelineSpec(turns, t),
    [turns, t]
  )

  if (!spec) return null

  return (
    <Card className='border-border/60 shadow-xs'>
      <CardHeader className='pb-2'>
        <CardTitle className='flex items-center gap-2 text-sm'>
          <Activity className='size-4' aria-hidden='true' />
          {t('Session Turn Timeline')}
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className='h-44 w-full'>
          {themeReady ? (
            <VChart
              key={`trace-timeline-${resolvedTheme}-${turns.length}`}
              spec={spec}
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
