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
/**
 * 异常台账 — persisted anomalies, with a two-step disposal.
 *
 * Why not compute anomalies on the fly and show them: a live calculation can only
 * see the current window. Once the window slides past, the problem evaporates —
 * even though nothing was fixed. This list is stored in `billing_anomalies`,
 * which is deliberately outside the usage-log retention policy.
 *
 * The record carries a machine `kind` and a written `detail`. The card shows the
 * written one as the headline: a row that reads `billing_drift` tells whoever
 * opens this page nothing about what happened or what to do, which is exactly
 * how it was reported back. The machine token stays, small and secondary, so the
 * row can still be matched against logs and grouped.
 *
 * Disposal is two-step, because "delete" should never be one click:
 *
 *   open      → 点「已解决」→ 弹窗确认（可填写处置说明）→ resolved
 *   resolved  → 进回收站，超过保留天数才物理删除
 *
 * open 的记录不会过期。这是和用量日志最大的区别：没处理完的账目问题永远在。
 */
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { Check, Clock, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { api } from '@/lib/api'
import { formatQuota } from '@/lib/format'

type Anomaly = {
  id: number
  kind: string
  status: string
  model_name: string
  user_id: number
  username: string
  token_name: string
  group: string
  channel_name: string
  ip: string
  request_id: string
  expected: number
  actual: number
  detail: string
  occurred_at: number
  created_at: number
  resolved_at: number
  resolution: string
}

/**
 * What each machine kind means, in words. The server already writes a full
 * sentence into `detail`; this is the fallback for rows recorded by an older
 * build that stored only the token, and it is what the badge is titled with so
 * hovering explains the category without opening the record.
 */
const KIND_LABELS: Record<string, string> = {
  quota_zero: '有 token 却没收钱',
  billing_drift: '实收与规则不符',
  cache_exceeds_prompt: '缓存数异常',
  negative_tokens: 'token 数为负',
  broken_other: '计费数据损坏',
  no_ratio: '缺计费规则',
  loss_model: '模型在亏损',
}

function formatTime(unix: number): string {
  if (!unix) return '—'
  // 精确到秒
  const d = new Date(unix * 1000)
  const pad = (n: number) => String(n).padStart(2, '0')
  return (
    `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ` +
    `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
  )
}

async function fetchAnomalies(
  status: string,
  page: number
): Promise<{ items: Anomaly[]; total: number }> {
  const res = await api.get('/api/user/billing-anomalies', {
    params: { status, page, page_size: 50 },
  })
  return res.data.data
}

export function AnomalyLedger() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [tab, setTab] = useState<'open' | 'resolved'>('open')
  const [pending, setPending] = useState<Anomaly | null>(null)
  const [resolution, setResolution] = useState('')

  const { data, isLoading } = useQuery({
    queryKey: ['billing-anomalies', tab],
    queryFn: () => fetchAnomalies(tab, 1),
  })

  const resolveMutation = useMutation({
    mutationFn: async (payload: { id: number; resolution: string }) => {
      await api.post(`/api/user/billing-anomalies/${payload.id}/resolve`, {
        resolution: payload.resolution,
      })
    },
    onSuccess: () => {
      setPending(null)
      setResolution('')
      queryClient.invalidateQueries({ queryKey: ['billing-anomalies'] })
    },
  })

  /** The written explanation, or a local label for records that predate it. */
  const explain = (item: Anomaly) =>
    item.detail?.trim() ||
    KIND_LABELS[item.kind] ||
    t('No explanation was recorded for this one.')

  return (
    <section className='mt-8'>
      <div className='mb-3 flex items-center gap-3'>
        <h2 className='text-sm font-medium'>{t('Anomaly ledger')}</h2>
        <div className='flex items-center gap-1'>
          <button
            type='button'
            onClick={() => setTab('open')}
            className={
              tab === 'open'
                ? 'bg-accent text-accent-foreground rounded px-2.5 py-1 text-xs'
                : 'text-muted-foreground hover:bg-accent/60 rounded px-2.5 py-1 text-xs'
            }
          >
            {t('Open')}
          </button>
          <button
            type='button'
            onClick={() => setTab('resolved')}
            className={
              tab === 'resolved'
                ? 'bg-accent text-accent-foreground rounded px-2.5 py-1 text-xs'
                : 'text-muted-foreground hover:bg-accent/60 rounded px-2.5 py-1 text-xs'
            }
          >
            {t('Recycle bin')}
          </button>
        </div>
      </div>

      {tab === 'open' && (
        <p className='text-muted-foreground mb-2 text-[11.5px]'>
          {t(
            'Open records never expire. They leave this list only after you mark them resolved.'
          )}
        </p>
      )}
      {tab === 'resolved' && (
        <p className='text-muted-foreground mb-2 text-[11.5px]'>
          {t(
            'Resolved records stay here for 30 days, then are deleted automatically.'
          )}
        </p>
      )}

      {isLoading && <Skeleton className='h-28 w-full' />}

      {data && data.items.length === 0 && (
        <p className='text-muted-foreground border-border/60 rounded border border-dashed px-3 py-8 text-center text-sm'>
          {tab === 'open'
            ? t('Nothing open. The books balance.')
            : t('The recycle bin is empty.')}
        </p>
      )}

      {data && data.items.length > 0 && (
        <ul className='border-border/70 border-t'>
          {data.items.map((item) => (
            <li
              key={item.id}
              className='border-border/60 flex flex-wrap items-start gap-x-3 gap-y-1 border-b px-1 py-2.5 text-[12.5px]'
            >
              <Badge variant='secondary' title={item.kind}>
                {KIND_LABELS[item.kind] ?? item.kind}
              </Badge>
              <span className='font-mono'>{item.model_name}</span>

              {/* 说清楚发生了什么，这才是这一行对管理员有用的部分 */}
              <span className='text-foreground/85 min-w-0 basis-full sm:basis-auto'>
                {explain(item)}
              </span>

              <span className='text-muted-foreground flex items-center gap-1'>
                <Clock className='size-3' aria-hidden='true' />
                {formatTime(item.occurred_at)}
              </span>

              <span className='text-muted-foreground ml-auto flex items-center gap-2'>
                <Link
                  to='/users'
                  search={{ filter: item.username }}
                  className='text-foreground font-medium hover:underline'
                >
                  {item.username || `#${item.user_id}`}
                </Link>
                {item.token_name && <span>{item.token_name}</span>}
                {/* 金额按货币显示；额度本身不是钱 */}
                {item.expected > 0 && (
                  <span className='font-mono tabular-nums'>
                    {formatQuota(item.expected)} → {formatQuota(item.actual)}
                  </span>
                )}
              </span>

              {item.status === 'open' ? (
                <Button
                  size='sm'
                  variant='outline'
                  className='h-6 px-2 text-[11.5px]'
                  onClick={() => setPending(item)}
                >
                  <Check data-icon='inline-start' />
                  {t('Resolved')}
                </Button>
              ) : (
                <span className='text-muted-foreground text-[11px]'>
                  {t('Resolved at')} {formatTime(item.resolved_at)}
                  {item.resolution ? ` · ${item.resolution}` : ''}
                </span>
              )}
            </li>
          ))}
        </ul>
      )}

      {/* 两步处置：先确认，不是一点就删 */}
      <Dialog
        open={pending !== null}
        onOpenChange={(open) => {
          if (!open) {
            setPending(null)
            setResolution('')
          }
        }}
      >
        <DialogContent className='sm:max-w-md'>
          <DialogHeader>
            <DialogTitle>{t('Mark this anomaly as resolved?')}</DialogTitle>
            <DialogDescription>
              {t(
                'It moves to the recycle bin and is deleted after 30 days. If you are not sure, leave it open — open records never expire.'
              )}
            </DialogDescription>
          </DialogHeader>

          {pending && (
            <div className='text-muted-foreground space-y-1 text-[12px]'>
              <p className='text-foreground'>
                {KIND_LABELS[pending.kind] ?? pending.kind} ·{' '}
                <span className='font-mono'>{pending.model_name}</span>
              </p>
              <p className='text-foreground/85'>{explain(pending)}</p>
              <p>{formatTime(pending.occurred_at)}</p>
              <p>
                {pending.username}
                {pending.token_name ? ` · ${pending.token_name}` : ''}
                {pending.channel_name ? ` · ${pending.channel_name}` : ''}
              </p>
              {pending.request_id && (
                <p className='font-mono text-[11px]'>req {pending.request_id}</p>
              )}
            </div>
          )}

          <Input
            placeholder={t('How was it handled? (optional)')}
            value={resolution}
            onChange={(event) => setResolution(event.target.value)}
          />

          <DialogFooter>
            <Button variant='outline' onClick={() => setPending(null)}>
              {t('Cancel')}
            </Button>
            <Button
              onClick={() =>
                pending &&
                resolveMutation.mutate({ id: pending.id, resolution })
              }
              disabled={resolveMutation.isPending}
            >
              <Trash2 data-icon='inline-start' />
              {t('Resolved, move to recycle bin')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  )
}
