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
 * 通关文牒 — the transit pass.
 *
 * The product's actual onboarding step, staged as a document you stamp: swap
 * one line of `base_url` and the rest of the SDK keeps working. Pressing the
 * stamp is the whole interaction, which is the point — it is one line.
 */
import { useState } from 'react'
import { useReducedMotion } from 'motion/react'
import { useTranslation } from 'react-i18next'

import { SectionMark } from './ornaments'

const FALLBACK_HOST = 'your-station.example'

export function StarpostPass() {
  const { t } = useTranslation()
  const reduced = useReducedMotion() ?? false
  // Read the host once during the first render instead of syncing it in an
  // effect: an effect would paint the placeholder first and then re-render,
  // which is a visible flash on the most important line of the card.
  const [host] = useState(() => {
    if (typeof window === 'undefined') return FALLBACK_HOST
    return window.location.host || FALLBACK_HOST
  })
  const [stamped, setStamped] = useState(false)

  // Keyboard and pointer both land here; the seal is a real button.
  const onStamp = () => {
    setStamped((current) => !current)
  }

  return (
    <section className='px-5 py-16 sm:px-8 sm:py-20'>
      <div className='mx-auto grid max-w-5xl gap-10 md:grid-cols-[minmax(0,1fr)_minmax(0,1.15fr)] md:items-center md:gap-14'>
        <div>
          <div className='flex items-center gap-2.5'>
            <SectionMark char='牒' />
            <p className='sp-eyebrow'>{t('Transit pass')}</p>
          </div>
          <h2 className='sp-display mt-3 text-2xl text-balance sm:text-3xl'>
            {t('One line to change. Nothing else moves.')}
          </h2>
          <p className='text-muted-foreground mt-4 text-[15px] leading-relaxed'>
            {t(
              'Keep the official SDK, keep your code. Point base_url at this station and every model behind it answers the same way.'
            )}
          </p>
          <button
            type='button'
            onClick={onStamp}
            aria-pressed={stamped}
            className='border-border hover:bg-accent focus-visible:ring-ring/50 mt-6 inline-flex h-10 items-center gap-2 rounded-full border px-4 text-sm font-medium transition-colors focus-visible:ring-3 focus-visible:outline-none'
          >
            {stamped ? t('Reopen the pass') : t('Stamp the pass')}
          </button>
        </div>

        <div className='sp-pass relative px-5 py-5 sm:px-6 sm:py-6'>
          <p className='text-muted-foreground/70 text-[10px] font-medium tracking-[0.16em] uppercase'>
            {t('Field guide · step 1')}
          </p>

          <pre className='sp-pass-code mt-3 overflow-x-auto'>
            <code>
              <span className='text-muted-foreground'>{'from '}</span>
              {'openai '}
              <span className='text-muted-foreground'>{'import '}</span>
              {'OpenAI\n\n'}
              {'client = OpenAI(\n'}
              {'  api_key="sk-\u2022\u2022\u2022\u2022",\n'}
              {'  '}
              <span
                className={
                  stamped ? 'sp-pass-struck' : 'text-foreground'
                }
              >
                {'base_url="https://api.openai.com/v1"'}
              </span>
              {',\n'}
              {')\n'}
            </code>
          </pre>

          {stamped ? (
            <>
              <div className='mt-3 flex items-center gap-2'>
                <span
                  aria-hidden='true'
                  className='h-px flex-1'
                  style={{ background: 'var(--sp-rule)' }}
                />
                <span className='text-muted-foreground/70 text-[10px] tracking-[0.16em] uppercase'>
                  {t('now reads')}
                </span>
                <span
                  aria-hidden='true'
                  className='h-px flex-1'
                  style={{ background: 'var(--sp-rule)' }}
                />
              </div>
              <p className='sp-mono mt-3 break-all'>
                {'base_url="https://'}
                <span
                  style={{ color: 'var(--sp-seal-ink)' }}
                  className='font-semibold'
                >
                  {host}
                </span>
                {'/v1"'}
              </p>
            </>
          ) : (
            <p className='text-muted-foreground/60 sp-mono mt-3 text-[11.5px]'>
              {t('Press the stamp to see the one line that changes.')}
            </p>
          )}

          <div
            className='sp-stamp right-5 top-5'
            data-visible={stamped ? 'true' : 'false'}
            aria-hidden='true'
            style={stamped && reduced ? { opacity: 0.9 } : undefined}
          >
            {t('Cleared to pass')}
          </div>
        </div>
      </div>
    </section>
  )
}
