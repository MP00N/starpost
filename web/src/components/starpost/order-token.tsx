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
 * 棨信 — the credential token.
 *
 * A narrow tally with four scratch marks. As the operator types, the marks
 * light up one by one; that is the only feedback, and it is opacity only. On a
 * failed check the token halts and the caller renders the reason next to it —
 * the mark never carries the verdict on its own.
 *
 * Line drawing, no fill, no glow: this sits inside a form and must not compete
 * with the field itself.
 */

export type OrderTokenState = 'idle' | 'verifying' | 'ok' | 'fail'

type OrderTokenProps = {
  /** 0–4 lit marks. */
  filled: number
  state?: OrderTokenState
  className?: string
}

const MARKS = [0, 1, 2, 3]

export function OrderToken(props: OrderTokenProps) {
  const lit = Math.max(0, Math.min(MARKS.length, props.filled))
  const state = props.state ?? 'idle'

  return (
    <svg
      viewBox='0 0 20 28'
      width='18'
      height='26'
      aria-hidden='true'
      className={`${state === 'fail' ? 'sp-token-halt' : ''} ${
        props.className ?? ''
      }`}
      fill='none'
    >
      {/* Tally body: a narrow rectangle with a notched head. */}
      <path
        d='M3 1h14v20.5l-3 5.5-3-3-3 3-3-5.5V1Z'
        stroke='currentColor'
        strokeWidth='1.2'
        strokeLinejoin='round'
        opacity={state === 'ok' ? 1 : 0.55}
      />
      {/* Scratch marks. */}
      {MARKS.map((index) => (
        <line
          key={index}
          x1='6.2'
          x2='13.8'
          y1={6 + index * 4}
          y2={6 + index * 4}
          stroke='currentColor'
          strokeWidth='1.4'
          strokeLinecap='round'
          className='sp-token-tick'
          data-lit={index < lit ? 'true' : 'false'}
        />
      ))}
    </svg>
  )
}
