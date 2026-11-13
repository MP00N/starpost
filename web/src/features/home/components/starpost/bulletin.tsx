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
 * 驿报 — the station bulletin.
 *
 * A ledger of requests passing through, newest first. This is an illustration
 * of what the gateway logs look like, not a live feed, and the caption says so
 * — a landing page that passes invented latency off as telemetry is lying to
 * the reader.
 */
import { useEffect, useRef, useState } from 'react'
import { useReducedMotion } from 'motion/react'
import { useTranslation } from 'react-i18next'

import { CloudRule, SectionMark } from './ornaments'

type BulletinRow = {
  key: number
  time: string
  endpoint: string
  model: string
  ms: number
  ok: boolean
}

const ENDPOINTS = [
  '/v1/chat/completions',
  '/v1/messages',
  '/v1/responses',
  '/v1/embeddings',
  '/v1/chat/completions',
  '/v1beta/models',
]

const MODELS = [
  'gpt-5',
  'claude-sonnet-4-5',
  'gemini-2.5-pro',
  'deepseek-chat',
  'qwen3-max',
  'kimi-k2',
]

const ROW_COUNT = 6
const TICK_MS = 2600

function stamp(): string {
  const now = new Date()
  const hh = String(now.getHours()).padStart(2, '0')
  const mm = String(now.getMinutes()).padStart(2, '0')
  const ss = String(now.getSeconds()).padStart(2, '0')
  return `${hh}:${mm}:${ss}`
}

function makeRow(key: number): BulletinRow {
  const index = key % ENDPOINTS.length
  return {
    key,
    time: stamp(),
    endpoint: ENDPOINTS[index],
    model: MODELS[index],
    ms: 180 + Math.floor(Math.random() * 420),
    ok: Math.random() > 0.06,
  }
}

export function StarpostBulletin() {
  const { t } = useTranslation()
  const reduced = useReducedMotion() ?? false
  const counter = useRef(ROW_COUNT)
  const [rows, setRows] = useState<BulletinRow[]>(() =>
    Array.from({ length: ROW_COUNT }, (_, index) => makeRow(index))
  )

  useEffect(() => {
    if (reduced) return
    const timer = window.setInterval(() => {
      if (document.hidden) return
      const next = counter.current
      counter.current += 1
      setRows((current) => [makeRow(next), ...current].slice(0, ROW_COUNT))
    }, TICK_MS)
    return () => window.clearInterval(timer)
  }, [reduced])

  return (
    <section className='px-5 py-16 sm:px-8 sm:py-20'>
      <div className='mx-auto max-w-3xl'>
        <CloudRule className='mb-8' />
        <div className='mb-6 flex items-end justify-between gap-6'>
          <div>
            <div className='flex items-center gap-2.5'>
              <SectionMark char='报' />
              <p className='sp-eyebrow'>{t('Station bulletin')}</p>
            </div>
            <h2 className='sp-display mt-3 text-2xl sm:text-3xl'>
              {t('Everything passing through, in order.')}
            </h2>
          </div>
          <span className='text-muted-foreground hidden shrink-0 text-[11px] tracking-wide sm:block'>
            {t('Illustration')}
          </span>
        </div>

        <div className='sp-bulletin px-4 py-3 sm:px-6 sm:py-4'>
          <div className='sp-bulletin-row text-muted-foreground border-none text-[10px] font-medium tracking-[0.14em] uppercase'>
            <span aria-hidden='true'>#</span>
            <span>{t('Endpoint')}</span>
            <span className='hidden sm:block'>{t('Model')}</span>
            <span className='text-right'>{t('Latency')}</span>
            <span className='text-right'>{t('Result')}</span>
          </div>

          <div role='table' aria-label={t('Recent requests')}>
            {rows.map((row, index) => (
              <div
                key={row.key}
                role='row'
                className={
                  index === 0 && !reduced
                    ? 'sp-bulletin-row sp-row-enter'
                    : 'sp-bulletin-row'
                }
                style={index === 0 ? undefined : { opacity: 1 - index * 0.1 }}
              >
                <span className='text-muted-foreground/60 sp-mono' role='cell'>
                  {String(ROW_COUNT - index).padStart(2, '0')}
                </span>
                <span className='sp-mono truncate' role='cell'>
                  <span className='text-muted-foreground/70 mr-2 hidden sm:inline'>
                    {row.time}
                  </span>
                  {row.endpoint}
                </span>
                <span
                  className='text-muted-foreground sp-mono hidden truncate sm:block'
                  role='cell'
                >
                  {row.model}
                </span>
                <span className='sp-mono text-right' role='cell'>
                  {row.ms}
                  <span className='text-muted-foreground/60 ml-0.5'>ms</span>
                </span>
                <span
                  className='sp-mono text-right'
                  role='cell'
                  style={{
                    color: row.ok
                      ? 'var(--chart-2)'
                      : 'var(--sp-seal-ink)',
                  }}
                >
                  {row.ok ? '200' : '429'}
                </span>
              </div>
            ))}
          </div>
        </div>

        <p className='text-muted-foreground/70 mt-3 text-[11px] leading-relaxed'>
          {t(
            'Sample rows. Your own logs live in the dashboard, one line per request.'
          )}
        </p>
      </div>
    </section>
  )
}
