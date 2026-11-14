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
 * 篆书字形 — drawn, not typeset.
 *
 * A real seal is cut in 小篆, and that is the whole difference between a seal
 * and a red box with a character in it. No openly licensed 篆书 typeface exists
 * (the ones on sale are commercial), so the two characters the brand needs are
 * drawn here as paths.
 *
 * What makes a form read as 小篆 rather than 楷书:
 *   - every stroke is one weight; no thick-thin modulation, no brush entry;
 *   - strokes are straight or evenly curved, and cut square at the ends;
 *   - the character fills a tall rectangle with even margins, and internal
 *     spacing is regular — a seal is a *layout*, not handwriting;
 *   - crossings are T-junctions and grid joints, never 撇捺 flourishes.
 *
 * Each glyph is authored in a 0..100 box and drawn with a single stroke width,
 * so they can be scaled together without the weights drifting apart.
 */

type GlyphProps = {
  /** Box origin in the parent's coordinate space. */
  x: number
  y: number
  /** Edge length of the 0..100 design box. */
  size: number
  stroke: number
  color: string
}

function frame(props: GlyphProps) {
  const scale = props.size / 100
  return `translate(${props.x} ${props.y}) scale(${scale})`
}

/** 星 — 日 over 生. */
export function GlyphXing(props: GlyphProps) {
  return (
    <g transform={frame(props)}>
      <g
        fill='none'
        stroke={props.color}
        strokeWidth={props.stroke / (props.size / 100)}
        strokeLinecap='butt'
        strokeLinejoin='miter'
      >
        {/* 日: a squarish sun, one internal bar. */}
        <rect x='30' y='4' width='40' height='26' rx='3' />
        <line x1='31' y1='17' x2='69' y2='17' />
        {/* 生: one base line, one mid line, one upright, one rising stroke. */}
        <line x1='14' y1='88' x2='86' y2='88' />
        <line x1='22' y1='64' x2='78' y2='64' />
        <line x1='50' y1='38' x2='50' y2='88' />
        {/* Left-falling stroke, cut straight — a seal has no 撇 tail. */}
        <path d='M50 54 L30 88' />
      </g>
    </g>
  )
}

/** 驿 (驛) — 馬 on the left, 睪 on the right, both squared off. */
export function GlyphYi(props: GlyphProps) {
  return (
    <g transform={frame(props)}>
      <g
        fill='none'
        stroke={props.color}
        strokeWidth={props.stroke / (props.size / 100)}
        strokeLinecap='butt'
        strokeLinejoin='miter'
      >
        {/* 馬: mane bar, body box, two legs. */}
        <line x1='8' y1='8' x2='44' y2='8' />
        <line x1='40' y1='8' x2='40' y2='30' />
        <rect x='10' y='30' width='32' height='32' rx='2' />
        <line x1='18' y1='62' x2='18' y2='92' />
        <line x1='34' y1='62' x2='34' y2='92' />
        {/* 睪: 目 above, 幸 below, on one upright. */}
        <rect x='58' y='4' width='30' height='24' rx='2' />
        <line x1='59' y1='16' x2='87' y2='16' />
        <line x1='72' y1='28' x2='72' y2='92' />
        <line x1='54' y1='50' x2='90' y2='50' />
        <line x1='54' y1='92' x2='90' y2='92' />
      </g>
    </g>
  )
}

/** 中 — a box pierced by one upright that runs past both edges. */
export function GlyphZhong(props: GlyphProps) {
  return (
    <g transform={frame(props)}>
      <g
        fill='none'
        stroke={props.color}
        strokeWidth={props.stroke / (props.size / 100)}
        strokeLinecap='butt'
        strokeLinejoin='miter'
      >
        <rect x='22' y='16' width='56' height='68' rx='2' />
        <line x1='50' y1='0' x2='50' y2='100' />
      </g>
    </g>
  )
}

/** 转 (轉) — 車 on the left, 專 reduced to its frame on the right. */
export function GlyphZhuan(props: GlyphProps) {
  return (
    <g transform={frame(props)}>
      <g
        fill='none'
        stroke={props.color}
        strokeWidth={props.stroke / (props.size / 100)}
        strokeLinecap='butt'
        strokeLinejoin='miter'
      >
        {/* 車: one box, one upright, two cross bars. */}
        <rect x='6' y='6' width='40' height='88' rx='2' />
        <line x1='26' y1='6' x2='26' y2='94' />
        <line x1='6' y1='36' x2='46' y2='36' />
        <line x1='6' y1='64' x2='46' y2='64' />
        {/* 專: a head bar, an upright, and a foot box. */}
        <line x1='56' y1='10' x2='94' y2='10' />
        <line x1='75' y1='10' x2='75' y2='56' />
        <rect x='56' y='56' width='38' height='38' rx='2' />
      </g>
    </g>
  )
}
