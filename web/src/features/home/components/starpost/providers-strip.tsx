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
 * 已在驿路上 — the providers this station already serves.
 *
 * A returning operator knows what the yard holds. A first-time visitor does
 * not, and a page that only talks about roads gives them nothing to hold on
 * to. This strip is the credibility line: real logos, real names, no invented
 * statistics.
 */
import { useTranslation } from 'react-i18next'

import { getLobeIcon } from '@/lib/lobe-icon'

const PROVIDERS = [
  { icon: 'OpenAI', label: 'OpenAI' },
  { icon: 'Claude', label: 'Claude' },
  { icon: 'Gemini', label: 'Gemini' },
  { icon: 'DeepSeek', label: 'DeepSeek' },
  { icon: 'Qwen', label: 'Qwen' },
  { icon: 'Kimi', label: 'Kimi' },
  { icon: 'Zhipu', label: 'GLM' },
  { icon: 'Mistral', label: 'Mistral' },
] as const

export function ProvidersStrip() {
  const { t } = useTranslation()

  return (
    <div className='mx-auto mt-10 w-full max-w-[42rem]'>
      <div className='mb-4 flex items-center gap-3'>
        <span className='sp-hairline flex-1' />
        <span className='sp-eyebrow'>{t('Already on the road')}</span>
        <span className='sp-hairline flex-1' />
      </div>
      <ul className='flex list-none flex-wrap items-center justify-center gap-x-6 gap-y-3'>
        {PROVIDERS.map((provider) => (
          <li
            key={provider.label}
            className='sp-provider flex items-center gap-1.5'
          >
            <span className='flex shrink-0 items-center opacity-80'>
              {getLobeIcon(provider.icon, 16)}
            </span>
            <span className='sp-provider-name'>{provider.label}</span>
          </li>
        ))}
      </ul>
    </div>
  )
}
