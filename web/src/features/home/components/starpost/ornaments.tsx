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
 * Section furniture: a 云头 rule and a 朱文 mark.
 *
 * Both are shapes, not words. A decorative mark has to survive translation —
 * a vertical Chinese label turns into stacked Latin capitals in `en` and looks
 * like a bug, whereas a small square seal with 驿 in it is a brand mark in
 * every locale.
 */

type CloudRuleProps = {
  className?: string
}

export function CloudRule(props: CloudRuleProps) {
  return (
    <div
      aria-hidden='true'
      className={props.className}
      style={{ display: 'flex', alignItems: 'center', gap: '10px' }}
    >
      <svg viewBox='0 0 64 16' className='sp-cloud-end h-4 w-14 shrink-0' fill='none'>
        <path
          d='M2 8h18c6 0 8-5 13-5 4 0 6 3 6 5s-2 5-6 5c-5 0-7-5-13-5H2'
          stroke='currentColor'
          strokeWidth='1'
          strokeLinecap='round'
        />
      </svg>
      <span className='sp-rule-line h-px flex-1' />
      <svg
        viewBox='0 0 64 16'
        className='sp-cloud-end h-4 w-14 shrink-0 -scale-x-100'
        fill='none'
      >
        <path
          d='M2 8h18c6 0 8-5 13-5 4 0 6 3 6 5s-2 5-6 5c-5 0-7-5-13-5H2'
          stroke='currentColor'
          strokeWidth='1'
          strokeLinecap='round'
        />
      </svg>
    </div>
  )
}

/**
 * A small square seal used as a section marker. `char` is a single character
 * chosen for meaning (驿 for the station, 报 for the bulletin, 牒 for the
 * pass, 规 for the rules) and is decorative, not content.
 */
export function SectionMark(props: { char: string }) {
  return (
    <span className='sp-mark' aria-hidden='true'>
      {props.char}
    </span>
  )
}
