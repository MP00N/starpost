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
 * 白文印 — 星驿, cut in 小篆.
 *
 * What changed and why: the first version set 星驿中转 in the theme's serif
 * inside a red rounded box. That is a badge, not a seal — a real seal is cut in
 * seal script, and 楷书 letterforms are the single biggest reason it read as
 * templated.
 *
 * The two characters are now drawn paths (`seal-glyphs.tsx`) rather than text:
 * no openly licensed 篆书 typeface exists, and drawing them is both lawful and
 * more faithful than any compromise typeface would be.
 *
 * Everything else that makes it an impression is unchanged: solid vermilion
 * ground with the characters knocked out (白文), a thin inner rule, a few chips
 * in the ground, and feTurbulence displacement so the edge looks bitten into
 * paper.
 */
import { GlyphXing, GlyphYi, GlyphZhong, GlyphZhuan } from './seal-glyphs'

type SealProps = {
  size: number
  cx: number
  cy: number
  rotate?: number
}

export function Seal(props: SealProps) {
  const half = props.size / 2
  const inset = props.size * 0.08
  const usable = props.size - inset * 2
  const cell = usable / 2
  const glyphSize = cell * 0.86
  const stroke = props.size * 0.026

  const left = props.cx - half + inset
  const top = props.cy - half + inset

  // 2x2, read right-to-left then top-to-bottom: 星 驿 / 中 转.
  const colRight = left + cell + (cell - glyphSize) / 2
  const colLeft = left + (cell - glyphSize) / 2
  const rowTop = top + (cell - glyphSize) / 2
  const rowBottom = top + cell + (cell - glyphSize) / 2

  return (
    <g transform={`rotate(${props.rotate ?? -2.4} ${props.cx} ${props.cy})`}>
      <defs>
        <filter
          id='sp-seal-roughen'
          x='-15%'
          y='-15%'
          width='130%'
          height='130%'
          colorInterpolationFilters='sRGB'
        >
          <feTurbulence
            type='fractalNoise'
            baseFrequency='0.05'
            numOctaves='3'
            seed='11'
            result='noise'
          />
          <feDisplacementMap
            in='SourceGraphic'
            in2='noise'
            scale='3.2'
            xChannelSelector='R'
            yChannelSelector='G'
          />
        </filter>
      </defs>

      <g style={{ filter: 'url(#sp-seal-roughen)' }}>
        {/* Ground */}
        <rect
          x={props.cx - half}
          y={props.cy - half}
          width={props.size}
          height={props.size}
          fill='var(--sp-seal-ink)'
        />

        {/* Inner rule, knocked out of the ground */}
        <rect
          x={props.cx - half + inset * 0.42}
          y={props.cy - half + inset * 0.42}
          width={props.size - inset * 0.84}
          height={props.size - inset * 0.84}
          fill='none'
          stroke='var(--background)'
          strokeWidth={props.size * 0.014}
          opacity='0.85'
        />

        {/* 界格: the cross that divides a four-character seal into quadrants.
            It is structure, not ornament — without it the four characters float
            and the square reads as empty at the corners. */}
        <g stroke='var(--background)' strokeWidth={props.size * 0.012} opacity='0.75'>
          <line x1={props.cx} y1={top} x2={props.cx} y2={top + usable} />
          <line x1={left} y1={props.cy} x2={left + usable} y2={props.cy} />
        </g>

        <GlyphXing x={colRight} y={rowTop} size={glyphSize} stroke={stroke} color='var(--background)' />
        <GlyphYi x={colRight} y={rowBottom} size={glyphSize} stroke={stroke} color='var(--background)' />
        <GlyphZhong x={colLeft} y={rowTop} size={glyphSize} stroke={stroke} color='var(--background)' />
        <GlyphZhuan x={colLeft} y={rowBottom} size={glyphSize} stroke={stroke} color='var(--background)' />

        {/* Carved wear: a few bites out of the ground */}
        <rect
          x={props.cx - half + props.size * 0.012}
          y={props.cy - half + props.size * 0.055}
          width={props.size * 0.04}
          height={props.size * 0.024}
          fill='var(--background)'
        />
        <rect
          x={props.cx + half - props.size * 0.06}
          y={props.cy + half - props.size * 0.08}
          width={props.size * 0.045}
          height={props.size * 0.028}
          fill='var(--background)'
        />
      </g>
    </g>
  )
}
