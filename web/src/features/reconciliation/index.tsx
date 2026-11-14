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
 * 对账 — reconciliation desk.
 *
 * The log list answers "what happened"; this answers "does the account add up".
 *
 * Three columns, three different questions, and keeping them apart is the whole
 * point:
 *
 *   实收   what the user was actually charged
 *   重算   the rule in the request's own log replayed on its own numbers —
 *          an accounting self-check; it should equal 实收 to within rounding
 *   原价   the vendor's published list price on the same token mix — this is
 *          what the request would cost at list, so 实收/原价 is your markup
 *
 * Every figure is quota (500,000 to the dollar) and every one is rendered as
 * money. Printing raw quota is how a dollar of spend reads as half a million.
 *
 * Recomputation happens on the server: cache tokens live inside logs.other and
 * tens of thousands of rows would freeze the browser here.
 */
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Skeleton } from '@/components/ui/skeleton'
import { formatQuota } from '@/lib/format'

import { AnomalyLedger } from './anomalies'
import { getReconciliationAudit } from './api'

const WINDOWS = [1, 7, 30] as const

/** Quota is not money; 500,000 of it is one dollar. */
const money = (quota: number) => formatQuota(quota)

function Stat(props: {
  label: string
  value: string
  hint?: string
  tone?: 'default' | 'warning'
}) {
  return (
    <div className='border-border/70 min-w-0 border-l pl-4 first:border-l-0 first:pl-0'>
      <p className='text-muted-foreground/70 text-[10.5px] tracking-[0.12em] uppercase'>
        {props.label}
      </p>
      <p
        className={
          props.tone === 'warning'
            ? 'text-destructive mt-1 font-mono text-lg font-medium tabular-nums'
            : 'mt-1 font-mono text-lg font-medium tabular-nums'
        }
      >
        {props.value}
      </p>
      {props.hint && (
        <p className='text-muted-foreground/60 mt-0.5 text-[11px]'>{props.hint}</p>
      )}
    </div>
  )
}

export function Reconciliation() {
  const { t } = useTranslation()
  const [days, setDays] = useState<number>(7)

  const { data, isLoading, isError } = useQuery({
    queryKey: ['reconciliation', days],
    queryFn: () => getReconciliationAudit(days),
  })

  const diff = data ? data.quota_total - data.expected_total : 0
  const driftRatio =
    data && data.expected_total > 0 ? Math.abs(diff) / data.expected_total : 0

  return (
    <div className='flex flex-col gap-6 p-4 sm:p-6'>
      <header className='flex flex-wrap items-end justify-between gap-3'>
        <div>
          <h1 className='text-xl font-semibold'>{t('Reconciliation')}</h1>
          <p className='text-muted-foreground mt-1 text-sm'>
            {t('Does the quota charged match the tokens we recorded?')}
          </p>
        </div>
        <div className='flex items-center gap-1'>
          {WINDOWS.map((window) => (
            <button
              key={window}
              type='button'
              onClick={() => setDays(window)}
              className={
                window === days
                  ? 'bg-accent text-accent-foreground rounded-md px-2.5 py-1 text-xs font-medium'
                  : 'text-muted-foreground hover:bg-accent/60 rounded-md px-2.5 py-1 text-xs'
              }
            >
              {t('{{n}}d', { n: window })}
            </button>
          ))}
        </div>
      </header>

      {isLoading && (
        <div className='space-y-3'>
          <Skeleton className='h-16 w-full' />
          <Skeleton className='h-40 w-full' />
        </div>
      )}

      {isError && (
        <p className='text-destructive text-sm'>
          {t('Could not load the audit. Check that you are an administrator.')}
        </p>
      )}

      {data && data.free_model_rows > 0 && (
        <p className='text-muted-foreground text-[12.5px]'>
          {t('{{n}} rows are on zero-priced models — zero quota is expected there.', {
            n: data.free_model_rows,
          })}
        </p>
      )}

      {data && data.unverifiable_rows > 0 && (
        <p className='text-muted-foreground text-[12.5px]'>
          {t(
            '{{n}} rows carry no billing input in their log (older records), so they cannot be recomputed and are left out of the difference below.',
            { n: data.unverifiable_rows }
          )}
        </p>
      )}

      {data && (
        <>
          <section className='border-border grid grid-cols-2 gap-4 rounded-md border p-4 sm:grid-cols-5'>
            <Stat
              label={t('Charged')}
              value={money(data.quota_total)}
              hint={t('{{n}} log rows', { n: data.rows })}
            />
            <Stat
              label={t('Recomputed')}
              value={money(data.expected_total)}
              hint={t('Replayed from each request’s own log')}
            />
            <Stat
              label={t('Difference')}
              value={`${diff >= 0 ? '+' : ''}${money(diff)}`}
              hint={`${(driftRatio * 100).toFixed(2)}%`}
              tone={driftRatio > 0.03 ? 'warning' : 'default'}
            />
            <Stat
              label={t('At official list price')}
              value={data.official_total > 0 ? money(data.official_total) : '—'}
              hint={
                data.official_total > 0
                  ? t('{{n}}× what the vendor charges', {
                      n: (data.quota_total / data.official_total).toFixed(2),
                    })
                  : t('No official price available yet')
              }
            />
            <Stat
              label={t('Cache saved')}
              value={money(data.cache_saved_quota)}
              hint={t('At full input price')}
            />
          </section>

          <section>
            <h2 className='mb-2 text-sm font-medium'>{t('Per model')}</h2>
            <div className='border-border overflow-x-auto rounded-md border'>
              <table className='w-full text-sm'>
                <thead className='bg-muted/40 text-muted-foreground text-[11px] tracking-wide uppercase'>
                  <tr>
                    <th className='px-3 py-2 text-left font-medium'>{t('Model')}</th>
                    <th className='px-3 py-2 text-right font-medium'>{t('Requests')}</th>
                    <th className='px-3 py-2 text-right font-medium'>{t('Cache hit')}</th>
                    <th className='px-3 py-2 text-right font-medium'>{t('Cache read')}</th>
                    <th className='px-3 py-2 text-right font-medium'>{t('Cache write')}</th>
                    <th className='px-3 py-2 text-right font-medium'>{t('Charged')}</th>
                    <th className='px-3 py-2 text-right font-medium'>{t('Recomputed')}</th>
                    <th className='px-3 py-2 text-right font-medium'>
                      {t('At official list price')}
                    </th>
                    <th className='px-3 py-2 text-right font-medium'>
                      {t('Markup')}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {data.models.length === 0 && (
                    <tr>
                      <td colSpan={9} className='text-muted-foreground px-3 py-6 text-center'>
                        {t('No consumption in this window.')}
                      </td>
                    </tr>
                  )}
                  {data.models.map((model) => (
                    <tr key={model.model} className='border-border/60 border-t'>
                      <td className='px-3 py-2 font-mono text-[12.5px]'>{model.model}</td>
                      <td className='px-3 py-2 text-right tabular-nums'>{model.requests}</td>
                      <td className='px-3 py-2 text-right tabular-nums'>
                        {(model.cache_hit_rate * 100).toFixed(1)}%
                      </td>
                      <td className='px-3 py-2 text-right tabular-nums'>
                        {model.cache_read.toLocaleString()}
                      </td>
                      <td className='px-3 py-2 text-right tabular-nums'>
                        {model.cache_write.toLocaleString()}
                      </td>
                      <td className='px-3 py-2 text-right tabular-nums'>
                        {money(model.quota)}
                      </td>
                      <td
                        className={
                          model.drift > 0.03
                            ? 'text-destructive px-3 py-2 text-right tabular-nums'
                            : 'px-3 py-2 text-right tabular-nums'
                        }
                        title={
                          model.priced_by === 'none'
                            ? t('This model’s requests carry no billing rule, so they cannot be recomputed')
                            : undefined
                        }
                      >
                        {model.priced_by === 'none'
                          ? t('cannot recompute')
                          : money(model.expected)}
                      </td>
                      <td className='text-muted-foreground px-3 py-2 text-right tabular-nums'>
                        {model.has_official ? money(model.official) : '—'}
                      </td>
                      <td className='px-3 py-2 text-right tabular-nums'>
                        {model.has_official && model.markup > 0
                          ? `${model.markup.toFixed(2)}×`
                          : '—'}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </section>

          <AnomalyLedger />
        </>
      )}
    </div>
  )
}
