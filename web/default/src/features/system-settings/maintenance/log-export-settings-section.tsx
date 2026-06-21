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
import { useCallback, useEffect, useState } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Database, RefreshCw, Search } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { api } from '@/lib/api'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormLabel,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const logExportSchema = z.object({
  'log_export_setting.enabled': z.boolean(),
  'log_export_setting.export_consume_logs': z.boolean(),
  'log_export_setting.export_error_logs': z.boolean(),
  'log_export_setting.export_session_turns': z.boolean(),
  'log_export_setting.prefer_external_for_trace_query': z.boolean(),
  'log_export_setting.elasticsearch_enabled': z.boolean(),
  'log_export_setting.elasticsearch_url': z.string(),
  'log_export_setting.elasticsearch_index': z.string(),
  'log_export_setting.elasticsearch_username': z.string(),
  'log_export_setting.elasticsearch_secret': z.string(),
  'log_export_setting.elasticsearch_api_key': z.string(),
  'log_export_setting.clickhouse_enabled': z.boolean(),
  'log_export_setting.clickhouse_url': z.string(),
  'log_export_setting.clickhouse_database': z.string(),
  'log_export_setting.clickhouse_table': z.string(),
  'log_export_setting.clickhouse_username': z.string(),
  'log_export_setting.clickhouse_secret': z.string(),
})

type LogExportFormValues = z.infer<typeof logExportSchema>

type LogExportSettingsSectionProps = {
  defaultValues: LogExportFormValues
}

type LogExportTestResult = {
  elasticsearch?: { configured?: boolean; healthy?: boolean; message?: string }
  clickhouse?: { configured?: boolean; healthy?: boolean; message?: string }
}

export function LogExportSettingsSection({
  defaultValues,
}: LogExportSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [testing, setTesting] = useState(false)
  const [testResult, setTestResult] = useState<LogExportTestResult | null>(null)

  const form = useForm<LogExportFormValues>({
    resolver: zodResolver(logExportSchema),
    defaultValues,
  })

  useEffect(() => {
    form.reset(defaultValues)
  }, [defaultValues, form])

  const onSubmit = async (values: LogExportFormValues) => {
    for (const [key, value] of Object.entries(values)) {
      if (defaultValues[key as keyof LogExportFormValues] === value) {
        continue
      }
      await updateOption.mutateAsync({ key, value })
    }
  }

  const handleTestConnections = useCallback(async () => {
    setTesting(true)
    try {
      const res = await api.post('/api/log/export/test')
      if (!res.data.success) {
        throw new Error(res.data.message || t('Failed to test log export connections'))
      }
      setTestResult(res.data.data)
      toast.success(t('Log export connection test completed'))
    } catch (error) {
      const message =
        error instanceof Error
          ? error.message
          : t('Failed to test log export connections')
      toast.error(message)
    } finally {
      setTesting(false)
    }
  }, [t])

  return (
    <SettingsSection title={t('Log Export Integration')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel={t('Save log export settings')}
          />

          <FormField
            control={form.control}
            name='log_export_setting.enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable log export')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Export consume logs, error logs, and session turns to Elasticsearch and/or ClickHouse for analytics and trace lookup.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch checked={field.value} onCheckedChange={field.onChange} />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <FormField
            control={form.control}
            name='log_export_setting.prefer_external_for_trace_query'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Prefer external stores for Trace ID lookup')}</FormLabel>
                  <FormDescription>
                    {t(
                      'When enabled, session trace queries prefer Elasticsearch or ClickHouse before local storage.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch checked={field.value} onCheckedChange={field.onChange} />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <div className='grid gap-4 md:grid-cols-3'>
            {(
              [
                ['log_export_setting.export_consume_logs', t('Export consume logs')],
                ['log_export_setting.export_error_logs', t('Export error logs')],
                ['log_export_setting.export_session_turns', t('Export session turns')],
              ] as const
            ).map(([name, label]) => (
              <FormField
                key={name}
                control={form.control}
                name={name}
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>{label}</FormLabel>
                    </SettingsSwitchContent>
                    <FormControl>
                      <Switch checked={field.value} onCheckedChange={field.onChange} />
                    </FormControl>
                  </SettingsSwitchItem>
                )}
              />
            ))}
          </div>

          <div className='space-y-4 rounded-lg border p-4'>
            <div className='flex items-center gap-2'>
              <Search className='size-4' />
              <h4 className='font-medium'>{t('Elasticsearch')}</h4>
            </div>
            <FormField
              control={form.control}
              name='log_export_setting.elasticsearch_enabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Enable Elasticsearch export')}</FormLabel>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch checked={field.value} onCheckedChange={field.onChange} />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />
            <div className='grid gap-4 md:grid-cols-2'>
              <FormField
                control={form.control}
                name='log_export_setting.elasticsearch_url'
                render={({ field }) => (
                  <div className='space-y-2'>
                    <FormLabel>{t('Elasticsearch URL')}</FormLabel>
                    <FormControl>
                      <Input {...field} placeholder='http://127.0.0.1:9200' />
                    </FormControl>
                  </div>
                )}
              />
              <FormField
                control={form.control}
                name='log_export_setting.elasticsearch_index'
                render={({ field }) => (
                  <div className='space-y-2'>
                    <FormLabel>{t('Elasticsearch index')}</FormLabel>
                    <FormControl>
                      <Input {...field} placeholder='new-api-logs' />
                    </FormControl>
                  </div>
                )}
              />
              <FormField
                control={form.control}
                name='log_export_setting.elasticsearch_username'
                render={({ field }) => (
                  <div className='space-y-2'>
                    <FormLabel>{t('Elasticsearch username')}</FormLabel>
                    <FormControl>
                      <Input {...field} />
                    </FormControl>
                  </div>
                )}
              />
              <FormField
                control={form.control}
                name='log_export_setting.elasticsearch_secret'
                render={({ field }) => (
                  <div className='space-y-2'>
                    <FormLabel>{t('Elasticsearch secret')}</FormLabel>
                    <FormControl>
                      <Input {...field} type='password' autoComplete='off' />
                    </FormControl>
                  </div>
                )}
              />
              <FormField
                control={form.control}
                name='log_export_setting.elasticsearch_api_key'
                render={({ field }) => (
                  <div className='space-y-2 md:col-span-2'>
                    <FormLabel>{t('Elasticsearch API key')}</FormLabel>
                    <FormDescription>
                      {t(
                        'Use an API key instead of username/password when connecting to Elasticsearch.'
                      )}
                    </FormDescription>
                    <FormControl>
                      <Input {...field} type='password' autoComplete='off' />
                    </FormControl>
                  </div>
                )}
              />
            </div>
          </div>

          <div className='space-y-4 rounded-lg border p-4'>
            <div className='flex items-center gap-2'>
              <Database className='size-4' />
              <h4 className='font-medium'>{t('ClickHouse')}</h4>
            </div>
            <FormField
              control={form.control}
              name='log_export_setting.clickhouse_enabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Enable ClickHouse export')}</FormLabel>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch checked={field.value} onCheckedChange={field.onChange} />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />
            <div className='grid gap-4 md:grid-cols-2'>
              <FormField
                control={form.control}
                name='log_export_setting.clickhouse_url'
                render={({ field }) => (
                  <div className='space-y-2'>
                    <FormLabel>{t('ClickHouse URL')}</FormLabel>
                    <FormControl>
                      <Input {...field} placeholder='http://127.0.0.1:8123' />
                    </FormControl>
                  </div>
                )}
              />
              <FormField
                control={form.control}
                name='log_export_setting.clickhouse_database'
                render={({ field }) => (
                  <div className='space-y-2'>
                    <FormLabel>{t('ClickHouse database')}</FormLabel>
                    <FormControl>
                      <Input {...field} placeholder='default' />
                    </FormControl>
                  </div>
                )}
              />
              <FormField
                control={form.control}
                name='log_export_setting.clickhouse_table'
                render={({ field }) => (
                  <div className='space-y-2'>
                    <FormLabel>{t('ClickHouse table')}</FormLabel>
                    <FormControl>
                      <Input {...field} placeholder='new_api_log_events' />
                    </FormControl>
                  </div>
                )}
              />
              <FormField
                control={form.control}
                name='log_export_setting.clickhouse_username'
                render={({ field }) => (
                  <div className='space-y-2'>
                    <FormLabel>{t('ClickHouse username')}</FormLabel>
                    <FormControl>
                      <Input {...field} />
                    </FormControl>
                  </div>
                )}
              />
              <FormField
                control={form.control}
                name='log_export_setting.clickhouse_secret'
                render={({ field }) => (
                  <div className='space-y-2'>
                    <FormLabel>{t('ClickHouse secret')}</FormLabel>
                    <FormControl>
                      <Input {...field} type='password' autoComplete='off' />
                    </FormControl>
                  </div>
                )}
              />
            </div>
          </div>

          <div className='flex flex-wrap items-center gap-3'>
            <Button type='button' variant='outline' onClick={() => void handleTestConnections()} disabled={testing}>
              <RefreshCw className={cn('size-4', testing && 'animate-spin')} />
              {t('Test log export connections')}
            </Button>
            {testResult ? (
              <div className='flex flex-wrap gap-2'>
                <Badge variant={testResult.elasticsearch?.healthy ? 'default' : 'secondary'}>
                  ES: {testResult.elasticsearch?.message || t('Not configured')}
                </Badge>
                <Badge variant={testResult.clickhouse?.healthy ? 'default' : 'secondary'}>
                  CH: {testResult.clickhouse?.message || t('Not configured')}
                </Badge>
              </div>
            ) : null}
          </div>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
