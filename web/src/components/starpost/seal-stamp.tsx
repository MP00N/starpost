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
 * 驿印 — a result lands.
 *
 * Square, thin edge, narrow face; the corners are bitten out with `clip-path`
 * so the edge looks eaten by paper. The face carries the environment, never
 * the word "success": an operator scanning a table needs to know *where* the
 * answer came from, and "success" is already obvious from the data.
 *
 * One shot, 140ms, then nothing. No bounce, no rotation, no glow.
 */

export type SealEnv = 'prod' | 'staging' | 'dev'

type SealStampProps = {
  env: SealEnv
  /** Plays the landing animation on mount; set false for static repeats. */
  entering?: boolean
  className?: string
}

const LABEL: Record<SealEnv, string> = {
  prod: 'prod',
  staging: 'stg',
  dev: 'dev',
}

export function SealStamp(props: SealStampProps) {
  return (
    <span
      className={`sp-seal-stamp ${props.className ?? ''}`}
      data-env={props.env}
      data-entering={props.entering === false ? 'false' : 'true'}
      // The stamp is a result marker, so it must be announced, not hidden.
      role='img'
      aria-label={`结果已落定 · 环境 ${props.env}`}
      title={`${props.env}`}
    >
      {LABEL[props.env]}
    </span>
  )
}
