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
import type { GetLogAnalyticsParams } from '../types'

type AnalyticsSearchParams = {
  startTime?: number
  endTime?: number
  groupBy?: string
  model?: string
  token?: string
  channel?: string
  group?: string
  username?: string
}

export function buildAnalyticsApiParams(
  searchParams: AnalyticsSearchParams,
  isAdmin: boolean
): GetLogAnalyticsParams {
  const endMs = searchParams.endTime ?? Date.now()
  const startMs =
    searchParams.startTime ?? endMs - 24 * 3600 * 1000

  const params: GetLogAnalyticsParams = {
    start_timestamp: Math.floor(startMs / 1000),
    end_timestamp: Math.floor(endMs / 1000),
    group_by: searchParams.groupBy === 'token' ? 'token' : 'channel',
  }

  if (searchParams.model) params.model_name = searchParams.model
  if (searchParams.token) params.token_name = searchParams.token
  if (searchParams.group) params.group = searchParams.group
  if (isAdmin && searchParams.username) params.username = searchParams.username
  if (isAdmin && searchParams.channel) {
    const channelId = Number.parseInt(searchParams.channel, 10)
    if (!Number.isNaN(channelId) && channelId > 0) {
      params.channel = channelId
    }
  }

  return params
}
