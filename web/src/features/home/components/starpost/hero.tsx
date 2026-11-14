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
 * 星驿 hero — commercial layout, both audiences.
 *
 * Two columns, two jobs:
 *
 *   left  — the pitch. A first-time visitor needs to know what this is, what
 *           it costs, and that it speaks their existing SDK, inside one
 *           screen. Two actions only: set out, or read the field guide.
 *   right — the proof. The route diagram lives inside a glass panel so it
 *           reads as an object on the page rather than as a decorative
 *           illustration.
 *
 * A returning operator does not read the pitch; they go straight for the
 * primary action, which is why it sits at the top of the left column instead
 * of below three paragraphs of copy.
 */
import { Link } from '@tanstack/react-router'
import { motion, useReducedMotion } from 'motion/react'
import { ArrowRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

import { Grain } from './grain'
import { LensFilter } from './lens-filter'
import { cascade, rise } from './motion'
import { ProvidersStrip } from './providers-strip'
import { ROUTE_CLIENTS, ROUTE_PROVIDERS } from './route-layout'
import { RoutingDiagram } from './routing'

type StarpostHeroProps = {
  isAuthenticated: boolean
  docsUrl: string
}

export function StarpostHero(props: StarpostHeroProps) {
  const { t } = useTranslation()
  const reduced = useReducedMotion() ?? false
  const container = cascade(reduced)
  const item = rise(reduced)
  const docsIsExternal = props.docsUrl.startsWith('http')

  return (
    <section className='relative isolate overflow-hidden px-5 pt-16 pb-10 sm:px-8 sm:pt-20'>
      <Grain />

      {/* Colour for the glass to refract. Without this the panel blurs a flat
          surface and turns grey. */}
      <div
        aria-hidden='true'
        className='pointer-events-none absolute inset-0 -z-10'
      >
        <div
          className='absolute top-[2%] left-[50%] h-[32rem] w-[32rem] rounded-full opacity-90 blur-[70px] dark:opacity-60'
          style={{
            background:
              'radial-gradient(closest-side, color-mix(in oklch, var(--chart-3) 55%, transparent), transparent 72%)',
          }}
        />
        <div
          className='absolute top-[30%] left-[34%] h-[24rem] w-[24rem] rounded-full opacity-80 blur-[80px] dark:opacity-50'
          style={{
            background:
              'radial-gradient(closest-side, color-mix(in oklch, var(--chart-1) 45%, transparent), transparent 72%)',
          }}
        />
      </div>

      <div className='mx-auto grid max-w-6xl items-center gap-10 lg:grid-cols-[minmax(0,1.05fr)_minmax(0,1fr)] lg:gap-14'>
        {/* ── The pitch ─────────────────────────────────────────────────── */}
        <motion.div
          className='flex flex-col items-start text-left'
          variants={container}
          initial='hidden'
          animate='visible'
        >
          <motion.p
            variants={item}
            className='sp-eyebrow flex items-center gap-3'
          >
            <span aria-hidden='true' className='sp-hairline w-8' />
            {t('StarPost Relay')}
          </motion.p>

          <motion.h1
            variants={item}
            className='sp-display mt-5 text-[clamp(2.1rem,4.4vw,3.35rem)] text-balance'
          >
            {t('One station,')}
            <br />
            {t('every road onward.')}
          </motion.h1>

          <motion.p
            variants={item}
            className='text-muted-foreground mt-5 max-w-[30rem] text-[15px] leading-[1.8] text-pretty'
          >
            {t(
              'Requests leave your app and change horses here. One address at the front door, every upstream model harnessed in the yard.'
            )}
          </motion.p>

          <motion.div
            variants={item}
            className='mt-7 flex flex-wrap items-center gap-3'
          >
            {props.isAuthenticated ? (
              <Button
                className='group h-12 px-7 text-[15px]'
                render={<Link to='/dashboard' />}
              >
                {t('Go to Dashboard')}
                <ArrowRight className='ml-1.5 size-4 transition-transform duration-150 group-hover:translate-x-0.5' />
              </Button>
            ) : (
              <Button
                className='group h-12 px-7 text-[15px]'
                render={<Link to='/sign-up' />}
              >
                {t('Set out')}
                <ArrowRight className='ml-1.5 size-4 transition-transform duration-150 group-hover:translate-x-0.5' />
              </Button>
            )}
            {docsIsExternal ? (
              <Button
                variant='outline'
                className='border-border hover:bg-accent h-12 px-5 text-sm'
                render={
                  <a
                    href={props.docsUrl}
                    target='_blank'
                    rel='noopener noreferrer'
                  />
                }
              >
                {t('Field guide')}
              </Button>
            ) : (
              <Button
                variant='outline'
                className='border-border hover:bg-accent h-12 px-5 text-sm'
                render={<Link to={props.docsUrl} />}
              >
                {t('Field guide')}
              </Button>
            )}
          </motion.div>

          {/* Live strip. Three facts, one of them measured a moment ago. */}
          <motion.dl
            variants={item}
            className='border-border/70 mt-8 grid w-full max-w-[30rem] grid-cols-3 gap-4 border-t pt-5'
          >
            <div>
              <dt className='text-muted-foreground/70 text-[10.5px] tracking-[0.12em] uppercase'>
                {t('Gate')}
              </dt>
              <dd className='mt-1 flex items-center gap-1.5 text-[15px] font-medium'>
                <span
                  aria-hidden='true'
                  className='size-1.5 rounded-full'
                  style={{ background: 'var(--chart-2)' }}
                />
                {t('Open')}
              </dd>
            </div>
            <div>
              <dt className='text-muted-foreground/70 text-[10.5px] tracking-[0.12em] uppercase'>
                {t('Roads out')}
              </dt>
              <dd className='mt-1 font-mono text-[15px] font-medium tabular-nums'>
                {ROUTE_PROVIDERS.length}
              </dd>
            </div>
            <div>
              <dt className='text-muted-foreground/70 text-[10.5px] tracking-[0.12em] uppercase'>
                {t('Kinds of client')}
              </dt>
              <dd className='mt-1 font-mono text-[15px] font-medium tabular-nums'>
                {ROUTE_CLIENTS.length}
              </dd>
            </div>
          </motion.dl>
        </motion.div>

        {/* ── The proof ─────────────────────────────────────────────────── */}
        <motion.div
          variants={item}
          initial='hidden'
          animate='visible'
          className='w-full'
        >
          <div className='sp-glass px-4 py-6 sm:px-6 sm:py-8'>
            {/* Refraction: the SVG filter plus the layer it bends. Decorative
                only — the diagram above it is untouched and stays crisp. */}
            <LensFilter />
            <div className='sp-glass-lens' aria-hidden='true' />
            <RoutingDiagram
              labels={{
                station: t('StarPost'),
                stationNote: t(
                  'Route diagram: requests leave your apps, change at StarPost, and continue to the upstream models.'
                ),
                clients: [t('Your app'), t('AI agents'), t('IDE and CLI')],
                providers: ROUTE_PROVIDERS.map((provider) => provider.name),
              }}
            />
            <p className='text-muted-foreground/70 mt-1 text-center text-[11px] tracking-[0.06em]'>
              {t('{{n}} roads out', { n: ROUTE_PROVIDERS.length })}
              <span aria-hidden='true' className='mx-1.5'>
                ·
              </span>
              {t('{{n}} kinds of client', { n: ROUTE_CLIENTS.length })}
            </p>
          </div>
        </motion.div>
      </div>

      <ProvidersStrip />
    </section>
  )
}
