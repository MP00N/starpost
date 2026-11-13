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
 * 概览 — the station board.
 *
 * This replaces the stock overview's opening block. The stock page is a stack of
 * equally-weighted cards, which is why it read as "stock" no matter how much the
 * colours were changed: structure, not colour, is what carries the language.
 *
 * What is different here, concretely:
 *
 *   1. 驿头 — a real page head. 印 (the seal), a serif title at display size, a
 *      subtitle, and a 竖排 (vertical) label on the right edge. The stock page
 *      had a 16px bold h2 with no room around it.
 *   2. 界格 — vertical rules behind the content, on the same 96px rhythm as the
 *      landing page. Everything sits *on* a grid instead of floating.
 *   3. 云头 rule — the divider is a cloud-head line (a long stroke with a hook
 *      at each end), drawn in SVG rather than a 1px border.
 *   4. 驿报 rows — the figures are set as a ledger: label above, number in
 *      tabular figures with a small unit, separated by hairlines.
 *
 * The数据 comes from the same query the stock page uses, passed in as props, so
 * this is a presentation layer only — no new requests, no new state.
 */
import { useTranslation } from 'react-i18next'

import { Seal } from '@/features/home/components/starpost/seal'

export type BoardFigure = {
  label: string
  value: string
  unit?: string
}

export type BoardRoute = {
  label: string
  detail: string
  state: 'online' | 'waiting' | 'pending'
}

/** 云头 rule — a horizontal stroke with a hook curling up at each end. */
function CloudHeadRule(props: { className?: string }) {
  return (
    <svg
      className={props.className}
      viewBox='0 0 600 12'
      preserveAspectRatio='none'
      aria-hidden='true'
      focusable='false'
    >
      <path
        d='M2 10 C 40 10, 60 3, 96 3 L 504 3 C 540 3, 560 10, 598 10'
        fill='none'
        stroke='currentColor'
        strokeWidth='1'
        vectorEffect='non-scaling-stroke'
      />
      {/* 云头: two small curls sitting on the ends */}
      <path
        d='M2 10 C 6 6, 10 6, 12 9'
        fill='none'
        stroke='currentColor'
        strokeWidth='1'
        vectorEffect='non-scaling-stroke'
      />
      <path
        d='M598 10 C 594 6, 590 6, 588 9'
        fill='none'
        stroke='currentColor'
        strokeWidth='1'
        vectorEffect='non-scaling-stroke'
      />
    </svg>
  )
}

/** 竖排 label — real vertical writing, not a rotated element. */
function VerticalLabel(props: { children: string }) {
  return (
    <span
      className='sp-vertical'
      style={{
        // 竖排用 writing-mode 而不是 rotate：旋转的文字基线与盒模型都不对
        writingMode: 'vertical-rl',
        textOrientation: 'upright',
      }}
    >
      {props.children}
    </span>
  )
}

function Figure(props: { figure: BoardFigure }) {
  return (
    <div className='sp-board-figure'>
      <p className='sp-board-figure-label'>{props.figure.label}</p>
      <p className='sp-board-figure-value'>
        <span className='sp-board-figure-number'>{props.figure.value}</span>
        {props.figure.unit && (
          <span className='sp-board-figure-unit'>{props.figure.unit}</span>
        )}
      </p>
    </div>
  )
}

export function StarPostOverview(props: {
  figures: BoardFigure[]
  routes: BoardRoute[]
}) {
  const { t } = useTranslation()

  return (
    <section className='sp-board relative'>
      {/* 界格：竖线在内容之下，不是装饰层 */}
      <div className='sp-board-grid' aria-hidden='true' />

      <header className='sp-board-head'>
        <div className='sp-board-seal'>
          {/* 复用首页那枚篆书印，尺寸按页头缩放 */}
          <svg viewBox='0 0 200 200' aria-hidden='true' focusable='false'>
            <Seal size={186} cx={100} cy={100} />
          </svg>
        </div>

        <div className='sp-board-heading'>
          <p className='sp-board-eyebrow'>{t('STATION BOARD')}</p>
          <h1 className='sp-board-title'>{t('Overview')}</h1>
          <p className='sp-board-subtitle'>
            {t('Requests in, horses changed, roads out — the day at a glance.')}
          </p>
        </div>

        <div className='sp-board-aside'>
          <VerticalLabel>{t('驿报')}</VerticalLabel>
        </div>
      </header>

      <CloudHeadRule className='sp-board-rule' />

      <div className='sp-board-figures'>
        {props.figures.map((figure) => (
          <Figure key={figure.label} figure={figure} />
        ))}
      </div>

      <CloudHeadRule className='sp-board-rule sp-board-rule--short' />

      <div className='sp-board-routes'>
        {props.routes.map((route) => (
          <div key={route.label} className='sp-board-route'>
            <span
              className='sp-board-route-dot'
              data-state={route.state}
              aria-hidden='true'
            />
            <span className='sp-board-route-label'>{route.label}</span>
            <span className='sp-board-route-detail'>{route.detail}</span>
          </div>
        ))}
      </div>
    </section>
  )
}
