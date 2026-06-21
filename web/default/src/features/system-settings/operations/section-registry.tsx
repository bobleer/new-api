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
import { SystemBehaviorSection } from '../general/system-behavior-section'
import { EmailSettingsSection } from '../integrations/email-settings-section'
import { MonitoringSettingsSection } from '../integrations/monitoring-settings-section'
import { WorkerSettingsSection } from '../integrations/worker-settings-section'
import { LogSettingsSection } from '../maintenance/log-settings-section'
import { LogExportSettingsSection } from '../maintenance/log-export-settings-section'
import { PerformanceSection } from '../maintenance/performance-section'
import { UpdateCheckerSection } from '../maintenance/update-checker-section'
import type { OperationsSettings } from '../types'
import { createSectionRegistry } from '../utils/section-registry'

const OPERATIONS_SECTIONS = [
  {
    id: 'behavior',
    titleKey: 'System Behavior',
    build: (settings: OperationsSettings) => (
      <SystemBehaviorSection
        defaultValues={{
          DefaultCollapseSidebar: settings.DefaultCollapseSidebar,
          DemoSiteEnabled: settings.DemoSiteEnabled,
          SelfUseModeEnabled: settings.SelfUseModeEnabled,
        }}
      />
    ),
  },
  {
    id: 'alerts',
    titleKey: 'Monitoring & Alerts',
    build: (settings: OperationsSettings) => (
      <MonitoringSettingsSection
        defaultValues={{
          QuotaRemindThreshold: settings.QuotaRemindThreshold,
          'perf_metrics_setting.enabled':
            settings['perf_metrics_setting.enabled'] ?? true,
          'perf_metrics_setting.flush_interval':
            settings['perf_metrics_setting.flush_interval'] ?? 5,
          'perf_metrics_setting.bucket_time':
            settings['perf_metrics_setting.bucket_time'] ?? 'hour',
          'perf_metrics_setting.retention_days':
            settings['perf_metrics_setting.retention_days'] ?? 0,
        }}
      />
    ),
  },
  {
    id: 'email',
    titleKey: 'SMTP Email',
    build: (settings: OperationsSettings) => (
      <EmailSettingsSection
        defaultValues={{
          SMTPServer: settings.SMTPServer,
          SMTPPort: settings.SMTPPort,
          SMTPAccount: settings.SMTPAccount,
          SMTPFrom: settings.SMTPFrom,
          SMTPToken: settings.SMTPToken,
          SMTPSSLEnabled: settings.SMTPSSLEnabled,
          SMTPForceAuthLogin: settings.SMTPForceAuthLogin,
        }}
      />
    ),
  },
  {
    id: 'worker',
    titleKey: 'Worker Proxy',
    build: (settings: OperationsSettings) => (
      <WorkerSettingsSection
        defaultValues={{
          WorkerUrl: settings.WorkerUrl,
          WorkerValidKey: settings.WorkerValidKey,
          WorkerAllowHttpImageRequestEnabled:
            settings.WorkerAllowHttpImageRequestEnabled,
        }}
      />
    ),
  },
  {
    id: 'log-export',
    titleKey: 'Log Export Integration',
    build: (settings: OperationsSettings) => (
      <LogExportSettingsSection
        defaultValues={{
          'log_export_setting.enabled':
            settings['log_export_setting.enabled'] ?? false,
          'log_export_setting.export_consume_logs':
            settings['log_export_setting.export_consume_logs'] ?? true,
          'log_export_setting.export_error_logs':
            settings['log_export_setting.export_error_logs'] ?? true,
          'log_export_setting.export_session_turns':
            settings['log_export_setting.export_session_turns'] ?? true,
          'log_export_setting.prefer_external_for_trace_query':
            settings['log_export_setting.prefer_external_for_trace_query'] ??
            false,
          'log_export_setting.elasticsearch_enabled':
            settings['log_export_setting.elasticsearch_enabled'] ?? false,
          'log_export_setting.elasticsearch_url':
            settings['log_export_setting.elasticsearch_url'] ?? '',
          'log_export_setting.elasticsearch_index':
            settings['log_export_setting.elasticsearch_index'] ?? 'new-api-logs',
          'log_export_setting.elasticsearch_username':
            settings['log_export_setting.elasticsearch_username'] ?? '',
          'log_export_setting.elasticsearch_secret':
            settings['log_export_setting.elasticsearch_secret'] ?? '',
          'log_export_setting.elasticsearch_api_key':
            settings['log_export_setting.elasticsearch_api_key'] ?? '',
          'log_export_setting.clickhouse_enabled':
            settings['log_export_setting.clickhouse_enabled'] ?? false,
          'log_export_setting.clickhouse_url':
            settings['log_export_setting.clickhouse_url'] ?? '',
          'log_export_setting.clickhouse_database':
            settings['log_export_setting.clickhouse_database'] ?? 'default',
          'log_export_setting.clickhouse_table':
            settings['log_export_setting.clickhouse_table'] ??
            'new_api_log_events',
          'log_export_setting.clickhouse_username':
            settings['log_export_setting.clickhouse_username'] ?? '',
          'log_export_setting.clickhouse_secret':
            settings['log_export_setting.clickhouse_secret'] ?? '',
        }}
      />
    ),
  },
  {
    id: 'logs',
    titleKey: 'Log Maintenance',
    build: (settings: OperationsSettings) => (
      <LogSettingsSection
        defaultEnabled={Boolean(settings.LogConsumeEnabled)}
      />
    ),
  },
  {
    id: 'performance',
    titleKey: 'Performance',
    build: (settings: OperationsSettings) => (
      <PerformanceSection
        defaultValues={{
          'performance_setting.disk_cache_enabled':
            settings['performance_setting.disk_cache_enabled'] ?? false,
          'performance_setting.disk_cache_threshold_mb':
            settings['performance_setting.disk_cache_threshold_mb'] ?? 10,
          'performance_setting.disk_cache_max_size_mb':
            settings['performance_setting.disk_cache_max_size_mb'] ?? 1024,
          'performance_setting.disk_cache_path':
            settings['performance_setting.disk_cache_path'] ?? '',
          'performance_setting.monitor_enabled':
            settings['performance_setting.monitor_enabled'] ?? false,
          'performance_setting.monitor_cpu_threshold':
            settings['performance_setting.monitor_cpu_threshold'] ?? 90,
          'performance_setting.monitor_memory_threshold':
            settings['performance_setting.monitor_memory_threshold'] ?? 90,
          'performance_setting.monitor_disk_threshold':
            settings['performance_setting.monitor_disk_threshold'] ?? 95,
        }}
      />
    ),
  },
  {
    id: 'update-checker',
    titleKey: 'System maintenance',
    build: (
      _settings: OperationsSettings,
      currentVersion?: string | null,
      startTime?: number | null
    ) => (
      <UpdateCheckerSection
        currentVersion={currentVersion}
        startTime={startTime}
      />
    ),
  },
] as const

export type OperationsSectionId = (typeof OPERATIONS_SECTIONS)[number]['id']

const operationsRegistry = createSectionRegistry<
  OperationsSectionId,
  OperationsSettings,
  [string | null | undefined, number | null | undefined]
>({
  sections: OPERATIONS_SECTIONS,
  defaultSection: 'behavior',
  basePath: '/system-settings/operations',
  urlStyle: 'path',
})

export const OPERATIONS_SECTION_IDS = operationsRegistry.sectionIds
export const OPERATIONS_DEFAULT_SECTION = operationsRegistry.defaultSection
export const getOperationsSectionNavItems =
  operationsRegistry.getSectionNavItems
export const getOperationsSectionContent = operationsRegistry.getSectionContent
export const getOperationsSectionMeta = operationsRegistry.getSectionMeta
