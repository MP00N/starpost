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

export type AuditModel = {
  model: string
  requests: number
  prompt: number
  completion: number
  cache_read: number
  cache_write: number
  quota: number
  expected: number
  cache_hit_rate: number
  drift: number
  free: boolean
}

export type AuditAnomaly = {
  kind: string
  model: string
  detail: string
  expected?: number
  actual?: number
  // 追人用
  user_id: number
  username: string
  token_name: string
  group: string
  channel_id: number
  channel_name: string
  ip: string
  request_id: string
  created_at: number
  log_id: number
}

export type AuditResult = {
  days: number
  rows: number
  quota_total: number
  expected_total: number
  cache_saved_quota: number
  quota_per_unit: number
  models: AuditModel[]
  anomalies: AuditAnomaly[]
  free_model_rows: number
}

export async function getReconciliationAudit(
  days: number
): Promise<AuditResult> {
  const res = await api.get<{ success: boolean; data: AuditResult }>(
    '/api/user/reconciliation',
    { params: { days } }
  )
  return res.data.data
}
