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
 * 远山 — the ink ranges at the foot of the viewport.
 *
 * The first version pinned the ranges to the bottom of the hero with
 * `absolute bottom-0`. Scrolling past the hero therefore cut them off along a
 * hard horizontal edge and exposed their flat underside, which reads as a bug
 * rather than as a landscape.
 *
 * This version is a viewport layer instead: three ranges sit just under the
 * fold, fade in from the top of the band, and drift at a different rate from
 * the page (a few percent of the distance scrolled). Because the band is
 * anchored to the viewport, there is no edge to reach and no underside to see.
 */
import { useEffect, useRef } from 'react'
import { useReducedMotion } from 'motion/react'

type Range = {
  d: string
  opacity: number
  blur: number
  /** Parallax factor — nearer ranges move more. */
  factor: number
  drift: string
  delay: string
}

const RANGES: Range[] = [
  {
    d: 'M0 190 L90 148 L168 172 L252 118 L330 160 L402 132 L470 168 L560 120 L640 158 L720 136 L800 172 L880 130 L960 166 L1040 140 L1120 174 L1200 150 L1200 320 L0 320 Z',
    opacity: 0.035,
    blur: 20,
    factor: -0.012,
    drift: 'sp-ink-far 52s ease-in-out infinite alternate',
    delay: '-7s',
  },
  {
    d: 'M0 226 L74 196 L158 214 L236 178 L318 210 L392 184 L470 216 L548 186 L628 212 L706 190 L790 218 L868 192 L948 214 L1030 188 L1112 216 L1200 196 L1200 320 L0 320 Z',
    opacity: 0.055,
    blur: 11,
    factor: -0.022,
    drift: 'sp-ink-mid 42s ease-in-out infinite alternate',
    delay: '-19s',
  },
  {
    d: 'M0 254 L86 232 L162 246 L246 220 L326 244 L404 226 L482 250 L562 228 L640 248 L722 230 L804 252 L884 232 L966 250 L1048 230 L1130 252 L1200 238 L1200 320 L0 320 Z',
    opacity: 0.075,
    blur: 4,
    factor: -0.034,
    drift: 'sp-ink-near 34s ease-in-out infinite alternate',
    delay: '-11s',
  },
]

export function InkMountains() {
  const reduced = useReducedMotion() ?? false
  const hostRef = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    if (reduced) return
    const host = hostRef.current
    if (!host) return

    let frame = 0
    const apply = () => {
      frame = 0
      const y = window.scrollY
      // Nothing beyond a couple of hundred pixels: past that the drift stops so
      // the ranges cannot slide out of the band.
      const progress = Math.min(1, y / 900)
      host.style.setProperty('--sp-ink-progress', progress.toFixed(4))
    }
    const onScroll = () => {
      if (frame !== 0) return
      frame = requestAnimationFrame(apply)
    }
    apply()
    window.addEventListener('scroll', onScroll, { passive: true })
    return () => {
      window.removeEventListener('scroll', onScroll)
      if (frame !== 0) cancelAnimationFrame(frame)
    }
  }, [reduced])

  return (
    <div
      ref={hostRef}
      aria-hidden='true'
      className='sp-ink-layer pointer-events-none fixed inset-x-0 bottom-0 -z-10 h-[38vh] overflow-hidden'
    >
      <svg
        viewBox='0 0 1200 320'
        preserveAspectRatio='none'
        className='h-full w-full'
      >
        {RANGES.map((range) => (
          <g
            key={range.d}
            style={{
              animation: reduced ? undefined : range.drift,
              animationDelay: range.delay,
              transformOrigin: '50% 100%',
            }}
          >
            <path
              d={range.d}
              fill='var(--foreground)'
              opacity={range.opacity}
              style={{
                filter: `blur(${range.blur}px)`,
                transform: `translateY(calc(var(--sp-ink-progress, 0) * ${
                  range.factor * 100
                }px))`,
              }}
            />
          </g>
        ))}
      </svg>
    </div>
  )
}
