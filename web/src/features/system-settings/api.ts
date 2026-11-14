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
import { api } from '@/lib/api'

import type {
  ConfirmPaymentComplianceResponse,
  IconSyncPreviewResponse,
  IconSyncStatusResponse,
  PricingSyncPreviewResponse,
  PricingSyncStatusResponse,
  FetchUpstreamRatiosRequest,
  LogCleanupTask,
  SystemOptionsResponse,
  SystemTaskListResponse,
  SystemTaskFilters,
  SystemTaskResponse,
  UpdateOptionRequest,
  UpdateOptionResponse,
  UpdatePasskeyDomainsRequest,
  UpdatePasskeyDomainsResponse,
  UpstreamChannelsResponse,
  UpstreamRatiosResponse,
} from './types'

export async function getSystemOptions() {
  const res = await api.get<SystemOptionsResponse>('/api/option/')
  return res.data
}

export async function updateSystemOption(request: UpdateOptionRequest) {
  const res = await api.put<UpdateOptionResponse>('/api/option/', request)
  return res.data
}

export async function updatePasskeyDomains(
  request: UpdatePasskeyDomainsRequest
) {
  const res = await api.put<UpdatePasskeyDomainsResponse>(
    '/api/option/passkey/domains',
    request,
    {
      validateStatus: (status) =>
        (status >= 200 && status < 300) || status === 409,
    }
  )
  return res.data
}

export async function confirmPaymentCompliance() {
  const res = await api.post<ConfirmPaymentComplianceResponse>(
    '/api/option/payment_compliance',
    { confirmed: true }
  )
  return res.data
}

export async function startLogCleanupTask(targetTimestamp: number) {
  const res = await api.post<SystemTaskResponse<LogCleanupTask>>(
    '/api/system-task/log-cleanup',
    null,
    {
      params: { target_timestamp: targetTimestamp },
    }
  )
  return res.data
}

export async function getCurrentLogCleanupTask() {
  const res = await api.get<SystemTaskResponse<LogCleanupTask | null>>(
    '/api/system-task/current',
    {
      params: { type: 'log_cleanup' },
    }
  )
  return res.data
}

export async function getSystemTask(taskId: string) {
  const res = await api.get<SystemTaskResponse<LogCleanupTask>>(
    `/api/system-task/${taskId}`
  )
  return res.data
}

export async function listSystemTasks(
  limit = 20,
  filters: SystemTaskFilters = {}
) {
  const res = await api.get<SystemTaskListResponse>('/api/system-task/list', {
    params: { limit, ...filters },
  })
  return res.data
}

export async function resetModelRatios() {
  const res = await api.post<UpdateOptionResponse>(
    '/api/option/rest_model_ratio'
  )
  return res.data
}

export async function getUpstreamChannels() {
  const res = await api.get<UpstreamChannelsResponse>(
    '/api/ratio_sync/channels'
  )
  return res.data
}

export async function fetchUpstreamRatios(request: FetchUpstreamRatiosRequest) {
  const res = await api.post<UpstreamRatiosResponse>(
    '/api/ratio_sync/fetch',
    request
  )
  return res.data
}

// ---------------------------------------------------------------------------
// Official price synchronization
// ---------------------------------------------------------------------------

export async function getPricingSyncStatus() {
  const res = await api.get<PricingSyncStatusResponse>(
    '/api/ratio_sync/official/status'
  )
  return res.data
}

// `preview` is a dry run: the backend fetches the official list and reports the
// blast radius without writing any price. It is the only safe way to show the
// administrator what a run would change before they enable the schedule.
export async function previewPricingSync() {
  const res = await api.post<PricingSyncPreviewResponse>(
    '/api/ratio_sync/official/preview'
  )
  return res.data
}

export async function runPricingSync() {
  const res = await api.post<SystemTaskResponse<unknown>>(
    '/api/ratio_sync/official/run'
  )
  return res.data
}

export async function getIconSyncStatus() {
  const res = await api.get<IconSyncStatusResponse>(
    '/api/ratio_sync/icons/status'
  )
  return res.data
}

// `preview` is a dry run: it reports how many records would gain an icon and
// writes nothing.
export async function previewIconSync() {
  const res = await api.post<IconSyncPreviewResponse>(
    '/api/ratio_sync/icons/preview'
  )
  return res.data
}

export async function runIconSync() {
  const res = await api.post<SystemTaskResponse<unknown>>(
    '/api/ratio_sync/icons/run'
  )
  return res.data
}
