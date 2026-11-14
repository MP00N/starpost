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
 * Two halves:
 *
 *   左：按模型汇总。命中率、缓存读写 token、实收额度、按倍率重算的额度。
 *   右：可疑行。漏计费、缓存读数大于 prompt、实收与重算不符。
 *
 * 重算在服务端做，因为缓存 token 藏在 logs.other 的 JSON 里，几万行在前端算
 * 会卡死浏览器。
 */
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Skeleton } from '@/components/ui/skeleton'

import { AnomalyLedger } from './anomalies'
import { getReconciliationAudit } from './api'

const WINDOWS = [1, 7, 30] as const

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

      {data && (
        <>
          <section className='border-border grid grid-cols-2 gap-4 rounded-md border p-4 sm:grid-cols-4'>
            <Stat
              label={t('Charged')}
              value={data.quota_total.toLocaleString()}
              hint={t('{{n}} log rows', { n: data.rows })}
            />
            <Stat
              label={t('Recomputed')}
              value={Math.round(data.expected_total).toLocaleString()}
              hint={t('From token counts and ratios')}
            />
            <Stat
              label={t('Difference')}
              value={`${diff >= 0 ? '+' : ''}${Math.round(diff).toLocaleString()}`}
              hint={`${(driftRatio * 100).toFixed(2)}%`}
              tone={driftRatio > 0.03 ? 'warning' : 'default'}
            />
            <Stat
              label={t('Cache saved')}
              value={(data.cache_saved_quota / data.quota_per_unit).toFixed(2)}
              hint={t('USD, at full price')}
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
                  </tr>
                </thead>
                <tbody>
                  {data.models.length === 0 && (
                    <tr>
                      <td colSpan={7} className='text-muted-foreground px-3 py-6 text-center'>
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
                        {model.quota.toLocaleString()}
                      </td>
                      <td
                        className={
                          model.drift > 0.03
                            ? 'text-destructive px-3 py-2 text-right tabular-nums'
                            : 'px-3 py-2 text-right tabular-nums'
                        }
                      >
                        {Math.round(model.expected).toLocaleString()}
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
