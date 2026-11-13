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
import { useTranslation } from 'react-i18next'

import { CloudRule, SectionMark } from './ornaments'

/**
 * 驿规 — three standing rules of the station.
 *
 * Each entry states one thing the gateway actually does, in the station's
 * vocabulary, with the mechanical detail underneath. No adjectives, no
 * "blazingly fast".
 */
export function StarpostPillars() {
  const { t } = useTranslation()

  const rules = [
    {
      id: 'swap',
      rule: t('Change the horse, not the cart'),
      body: t(
        'The client keeps talking one protocol. Upstream, the station picks whichever provider fits the model you named.'
      ),
      detail: t('One endpoint · many providers'),
    },
    {
      id: 'reroute',
      rule: t('A flooded road is not a closed road'),
      body: t(
        'When a channel fails or rate-limits, the same request is offered to another channel before the reader ever sees an error.'
      ),
      detail: t('Retry · failover · weighted routing'),
    },
    {
      id: 'fare',
      rule: t('Fares by distance travelled'),
      body: t(
        'You are billed for tokens that actually moved, per key, per model. No monthly gate fee and no idle charge.'
      ),
      detail: t('Per-token · per-key · auditable'),
    },
  ]

  return (
    <section className='px-5 py-16 sm:px-8 sm:py-20'>
      <div className='mx-auto max-w-5xl'>
        <CloudRule className='mb-8' />
        <div className='flex items-center gap-2.5'>
          <SectionMark char='规' />
          <p className='sp-eyebrow'>{t('Standing rules')}</p>
        </div>
        <div className='mt-8 grid md:grid-cols-3'>
          {rules.map((rule, index) => (
            <div
              key={rule.id}
              className={
                index === 0
                  ? 'sp-milestone px-0 py-6 md:pr-8 md:py-0'
                  : 'sp-milestone px-0 py-6 md:px-8 md:py-0'
              }
            >
              <span className='text-muted-foreground/80 sp-display text-base'>
                {['壹', '貳', '叁'][index]}
              </span>
              <h3 className='sp-display mt-3 text-lg leading-snug text-balance'>
                {rule.rule}
              </h3>
              <p className='text-muted-foreground mt-3 text-[13.5px] leading-relaxed'>
                {rule.body}
              </p>
              <p className='text-muted-foreground/60 mt-4 text-[11px] tracking-wide'>
                {rule.detail}
              </p>
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}
