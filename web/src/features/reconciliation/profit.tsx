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
 * 盈亏 — what each model actually earns, from real request records.
 *
 * The configured ratio is what you think you charge. The margin is what is left
 * after the upstream bills you — and those differ by cache hit rate, output/input
 * mix, group discounts and the upstream's own price changes. A model priced at
 * 1.0x can still be losing money if every request is a long completion with no
 * cache.
 *
 * Cost comes from an upstream price table the admin configures (or imports from
 * a new-api upstream's /api/pricing). Models without a cost entry are reported as
 * "unknown" rather than guessed at — a wrong verdict here turns off a model that
 * was fine.
 *
 * The guard: a model found losing money is taken off the shelves automatically.
 * It comes back only when its ratio changes, or when an admin explicitly chooses
 * to keep it listed and accept the loss.
 */
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AlertTriangle, ArrowDownRight, ArrowUpRight, Lock } from 'lucide-react'
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
import { Skeleton } from '@/components/ui/skeleton'
import { api } from '@/lib/api'

import { CostDrawer } from './cost-drawer'

type ProfitRow = {
  model: string
  requests: number
  prompt: number
  completion: number
  cache_read: number
  sell_ratio: number
  cost_ratio: number
  has_cost: boolean
  revenue: number
  cost: number
  margin: number
  margin_rate: number | null
  verdict: 'profitable' | 'loss' | 'unknown'
}

type Hold = {
  model: string
  hold_since: number
  loss_quota: number
  reason: string
  force_listed: boolean
}

function fmt(n: number): string {
  return Math.round(n).toLocaleString()
}

export function ModelProfit() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [days, setDays] = useState(7)
  const [confirmForce, setConfirmForce] = useState<Hold | null>(null)
  const [costOpen, setCostOpen] = useState(false)

  const { data, isLoading } = useQuery({
    queryKey: ['model-profit', days],
    queryFn: async () => {
      const res = await api.get('/api/user/model-profit', { params: { days } })
      return res.data.data as {
        models: ProfitRow[]
        loss_count: number
        unknown_count: number
      }
    },
  })

  const { data: holdsData } = useQuery({
    queryKey: ['model-loss-holds'],
    queryFn: async () => {
      const res = await api.get('/api/user/model-loss-holds')
      return res.data.data as { holds: Hold[] }
    },
  })

  const forceMutation = useMutation({
    mutationFn: async (model: string) => {
      await api.post('/api/user/model-loss-holds/force-list', { model })
    },
    onSuccess: () => {
      setConfirmForce(null)
      queryClient.invalidateQueries({ queryKey: ['model-loss-holds'] })
      queryClient.invalidateQueries({ queryKey: ['model-profit'] })
    },
  })

  const holds = holdsData?.holds ?? []

  return (
    <div className='flex flex-col gap-6 p-4 sm:p-6'>
      <header className='flex flex-wrap items-end justify-between gap-3'>
        <div>
          <h1 className='text-xl font-semibold'>{t('Profit & loss')}</h1>
          <p className='text-muted-foreground mt-1 text-sm'>
            {t(
              'Revenue from what you charge, cost from what the upstream bills. From real requests, not from the price table.'
            )}
          </p>
        </div>
        <div className='flex items-center gap-2'>
          <Button size='sm' variant='outline' onClick={() => setCostOpen(true)}>
            {t('Upstream cost')}
          </Button>
          {[1, 7, 30].map((window) => (
            <button
              key={window}
              type='button'
              onClick={() => setDays(window)}
              className={
                window === days
                  ? 'bg-accent text-accent-foreground rounded px-2.5 py-1 text-xs font-medium'
                  : 'text-muted-foreground hover:bg-accent/60 rounded px-2.5 py-1 text-xs'
              }
            >
              {t('{{n}}d', { n: window })}
            </button>
          ))}
        </div>
      </header>

      {/* 被下架的模型：必须在这里处理，否则一直不在架上 */}
      {holds.length > 0 && (
        <section className='border-destructive/40 bg-destructive/5 rounded border p-3'>
          <h2 className='text-destructive flex items-center gap-2 text-sm font-medium'>
            <Lock className='size-3.5' aria-hidden='true' />
            {t('{{n}} models taken off the shelves for loss', { n: holds.length })}
          </h2>
          <p className='text-muted-foreground mt-1 text-[11.5px]'>
            {t(
              'They come back automatically once you change their ratio. Keeping one listed means accepting the loss.'
            )}
          </p>
          <ul className='mt-2 space-y-1'>
            {holds.map((hold) => (
              <li
                key={hold.model}
                className='flex flex-wrap items-center gap-2 text-[12.5px]'
              >
                <span className='font-mono'>{hold.model}</span>
                <span className='text-muted-foreground'>
                  {t('lost')} {fmt(hold.loss_quota)}
                </span>
                {hold.force_listed ? (
                  <Badge variant='secondary'>{t('Kept listed by admin')}</Badge>
                ) : (
                  <Button
                    size='sm'
                    variant='outline'
                    className='ml-auto h-6 px-2 text-[11.5px]'
                    onClick={() => setConfirmForce(hold)}
                  >
                    {t('Keep listed anyway')}
                  </Button>
                )}
              </li>
            ))}
          </ul>
        </section>
      )}

      {isLoading && <Skeleton className='h-40 w-full' />}

      {data && (
        <>
          <section className='border-border grid grid-cols-2 gap-4 rounded border p-4 sm:grid-cols-3'>
            <div>
              <p className='text-muted-foreground text-[10.5px] tracking-[0.12em] uppercase'>
                {t('Loss-making models')}
              </p>
              <p className='mt-1 font-mono text-lg'>
                {data.loss_count}
                {data.loss_count > 0 && (
                  <AlertTriangle
                    className='text-destructive ml-1 inline size-3.5'
                    aria-hidden='true'
                  />
                )}
              </p>
            </div>
            <div className='border-border border-l pl-4'>
              <p className='text-muted-foreground text-[10.5px] tracking-[0.12em] uppercase'>
                {t('Without upstream cost')}
              </p>
              <p className='mt-1 font-mono text-lg'>{data.unknown_count}</p>
            </div>
            <div className='border-border border-l pl-4'>
              <p className='text-muted-foreground text-[10.5px] tracking-[0.12em] uppercase'>
                {t('Models tracked')}
              </p>
              <p className='mt-1 font-mono text-lg'>{data.models.length}</p>
            </div>
          </section>

          <div className='border-border overflow-x-auto rounded border'>
            <table className='w-full text-sm'>
              <thead className='bg-muted/40 text-muted-foreground text-[11px] tracking-wide uppercase'>
                <tr>
                  <th className='px-3 py-2 text-left font-medium'>{t('Model')}</th>
                  <th className='px-3 py-2 text-right font-medium'>{t('Requests')}</th>
                  <th className='px-3 py-2 text-right font-medium'>{t('Revenue')}</th>
                  <th className='px-3 py-2 text-right font-medium'>{t('Cost')}</th>
                  <th className='px-3 py-2 text-right font-medium'>{t('Margin')}</th>
                  <th className='px-3 py-2 text-right font-medium'>{t('Rate')}</th>
                  <th className='px-3 py-2 text-right font-medium'>{t('Verdict')}</th>
                </tr>
              </thead>
              <tbody>
                {data.models.length === 0 && (
                  <tr>
                    <td colSpan={7} className='text-muted-foreground px-3 py-6 text-center'>
                      {t('No requests in this window.')}
                    </td>
                  </tr>
                )}
                {data.models.map((row) => (
                  <tr key={row.model} className='border-border/60 border-t'>
                    <td className='px-3 py-2 font-mono text-[12.5px]'>{row.model}</td>
                    <td className='px-3 py-2 text-right tabular-nums'>{row.requests}</td>
                    <td className='px-3 py-2 text-right tabular-nums'>{fmt(row.revenue)}</td>
                    <td className='px-3 py-2 text-right tabular-nums'>
                      {row.has_cost ? fmt(row.cost) : '—'}
                    </td>
                    <td
                      className={
                        row.verdict === 'loss'
                          ? 'text-destructive px-3 py-2 text-right tabular-nums'
                          : 'px-3 py-2 text-right tabular-nums'
                      }
                    >
                      {row.has_cost ? (
                        <span className='inline-flex items-center gap-0.5'>
                          {row.margin >= 0 ? (
                            <ArrowUpRight className='size-3' aria-hidden='true' />
                          ) : (
                            <ArrowDownRight className='size-3' aria-hidden='true' />
                          )}
                          {fmt(row.margin)}
                        </span>
                      ) : (
                        '—'
                      )}
                    </td>
                    <td className='px-3 py-2 text-right tabular-nums'>
                      {row.margin_rate === null
                        ? '—'
                        : `${(row.margin_rate * 100).toFixed(1)}%`}
                    </td>
                    <td className='px-3 py-2 text-right'>
                      {row.verdict === 'loss' && (
                        <Badge variant='destructive'>{t('Loss')}</Badge>
                      )}
                      {row.verdict === 'profitable' && (
                        <Badge variant='secondary'>{t('Profitable')}</Badge>
                      )}
                      {row.verdict === 'unknown' && (
                        <span className='text-muted-foreground text-[11px]'>
                          {t('No cost set')}
                        </span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}

      <CostDrawer
        open={costOpen}
        onOpenChange={setCostOpen}
        knownModels={data?.models.map((m) => m.model) ?? []}
      />

      {/* 强制上架要确认：这是接受亏损的决定，不该一点就过 */}
      <Dialog
        open={confirmForce !== null}
        onOpenChange={(open) => !open && setConfirmForce(null)}
      >
        <DialogContent className='sm:max-w-md'>
          <DialogHeader>
            <DialogTitle>{t('Keep this model listed at a loss?')}</DialogTitle>
            <DialogDescription>
              {t(
                'The guard will stop taking it off the shelves. It stays marked as loss-making until you change its ratio.'
              )}
            </DialogDescription>
          </DialogHeader>
          {confirmForce && (
            <p className='text-muted-foreground text-[12.5px]'>
              <span className='text-foreground font-mono'>{confirmForce.model}</span>
              {' · '}
              {t('lost')} {fmt(confirmForce.loss_quota)}
            </p>
          )}
          <DialogFooter>
            <Button variant='outline' onClick={() => setConfirmForce(null)}>
              {t('Cancel')}
            </Button>
            <Button
              variant='destructive'
              onClick={() => confirmForce && forceMutation.mutate(confirmForce.model)}
              disabled={forceMutation.isPending}
            >
              {t('Keep it listed')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
