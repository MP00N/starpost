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
 * 驿路阻断 — the road fails.
 *
 * The break is a single `clip-path` swap that bites a sawtooth out of the line;
 * it never shakes frame by frame. The line withdraws after 200ms, but the
 * text stays: an error is information, not an animation.
 *
 * Every failure surfaces three things together — code, cause, next action —
 * because a red flicker tells a screen reader nothing.
 */
import { useEffect, useState } from 'react'

type RoadBreakProps = {
  /** HTTP status or application error code, e.g. 401. */
  code?: number | string
  /** Human-readable cause. */
  reason: string
  /** What the operator should do next. */
  action?: string
  className?: string
}

const FADE_MS = 200

export function RoadBreak(props: RoadBreakProps) {
  const [fading, setFading] = useState(false)

  useEffect(() => {
    const timer = window.setTimeout(() => setFading(true), FADE_MS)
    return () => window.clearTimeout(timer)
  }, [props.reason])

  return (
    <div className={props.className} role='alert'>
      <div className='sp-road-break' data-fading={fading ? 'true' : 'false'} aria-hidden='true' />
      <p className='text-destructive mt-2 text-[12.5px] leading-relaxed'>
        {props.code !== undefined && (
          <span className='font-mono tabular-nums'>{props.code}</span>
        )}
        {props.code !== undefined && (
          <span aria-hidden='true' className='mx-1.5'>
            ·
          </span>
        )}
        <span>{props.reason}</span>
        {props.action && (
          <>
            <span aria-hidden='true' className='mx-1.5'>
              ·
            </span>
            <span className='text-muted-foreground'>{props.action}</span>
          </>
        )}
      </p>
    </div>
  )
}
