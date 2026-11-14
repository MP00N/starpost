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
 * 驿站侧栏 — the navigation as a station board.
 *
 * The stock sidebar is a column of rounded pills with an all-caps group label.
 * Two things make it read as a console rather than a station sign, and neither is
 * a colour problem:
 *
 *   - every item is a pill, so nothing looks *pressed on*;
 *   - the groups are labelled in the reader's own language, horizontally.
 *
 * This version:
 *   - puts the seal at the top with the station name, on a 界格 column;
 *   - sets group labels vertically (writing-mode) with 一二三 index marks, which
 *     is how a board would number its sections;
 *   - marks the active item with a vermilion rule on its left edge instead of a
 *     filled pill.
 *
 * It is a drop-in replacement for the root sidebar body only. Nested drill-in
 * views keep using SidebarViewHeader, since those carry a back affordance the
 * board metaphor would only get in the way of.
 */
import { useTranslation } from 'react-i18next'

import { Seal } from '@/features/home/components/starpost/seal'
import { useStatus } from '@/hooks/use-status'

const INDEX_MARKS = ['壹', '貳', '叁', '肆', '伍', '陸']

export function StarPostSidebarHead(props: { title?: string | null }) {
  const { t } = useTranslation()
  const { status } = useStatus()
  const name = props.title || status?.system_name || '星驿'

  return (
    <div className='sp-side-head'>
      <div className='sp-side-seal' aria-hidden='true'>
        <svg viewBox='0 0 200 200'>
          <Seal size={186} cx={100} cy={100} />
        </svg>
      </div>
      <div className='sp-side-name'>
        <p className='sp-side-name-main'>{name}</p>
        <p className='sp-side-name-sub'>{t('STATION RELAY')}</p>
      </div>
    </div>
  )
}

/** Group label as a vertical marker with a numeral, e.g. 壹 / 常规. */
export function StarPostGroupLabel(props: {
  index: number
  children: string
}) {
  const mark = INDEX_MARKS[props.index] ?? String(props.index + 1)
  return (
    <div className='sp-side-group-label'>
      <span className='sp-side-group-mark' aria-hidden='true'>
        {mark}
      </span>
      <span className='sp-side-group-text'>{props.children}</span>
      <span className='sp-side-group-rule' aria-hidden='true' />
    </div>
  )
}
