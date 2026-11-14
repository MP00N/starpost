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
import { Link } from '@tanstack/react-router'
import { ArrowRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

import { SectionMark } from './ornaments'

type StarpostClosingProps = {
  isAuthenticated: boolean
}

export function StarpostClosing(props: StarpostClosingProps) {
  const { t } = useTranslation()

  return (
    <section className='px-5 pt-8 pb-24 sm:px-8 sm:pb-32'>
      <div className='mx-auto max-w-5xl'>
        <div
          className='relative overflow-hidden rounded-lg border px-6 py-14 text-center sm:py-20'
          style={{
            borderColor: 'var(--sp-rule)',
            background:
              'radial-gradient(ellipse 60% 120% at 50% 120%, color-mix(in oklch, var(--chart-3) 16%, transparent) 0%, transparent 70%)',
          }}
        >
          <div className='mb-4 flex justify-center'>
            <SectionMark char='驿' />
          </div>
          <p className='sp-eyebrow'>{t('The gate is open')}</p>
          <h2 className='sp-display mx-auto mt-4 max-w-xl text-3xl text-balance sm:text-4xl'>
            {t('The horses are already saddled.')}
          </h2>
          <p className='text-muted-foreground mx-auto mt-4 max-w-md text-sm leading-relaxed'>
            {t(
              'Register, copy a key, change one line. The first courier leaves the same minute.'
            )}
          </p>
          <div className='mt-8 flex flex-wrap items-center justify-center gap-3'>
            {props.isAuthenticated ? (
              <Button
                className='group h-11 px-6 text-sm'
                render={<Link to='/dashboard' />}
              >
                {t('Go to Dashboard')}
                <ArrowRight className='ml-1 size-4 transition-transform duration-200 group-hover:translate-x-0.5' />
              </Button>
            ) : (
              <Button
                className='group h-11 px-6 text-sm'
                render={<Link to='/sign-up' />}
              >
                {t('Set out')}
                <ArrowRight className='ml-1 size-4 transition-transform duration-200 group-hover:translate-x-0.5' />
              </Button>
            )}
            <Button
              variant='outline'
              className='border-border hover:bg-accent h-11 px-5 text-sm'
              render={<Link to='/pricing' />}
            >
              {t('Read the fares')}
            </Button>
          </div>
        </div>
      </div>
    </section>
  )
}
