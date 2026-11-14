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
 * Shared motion vocabulary for the StarPost homepage.
 *
 * The page uses exactly one orchestrated entrance — a single staggered
 * cascade on first paint — and then only motion the reader causes (hover,
 * press). Per-section fade-ups on scroll are deliberately absent: repeating
 * the same reveal on every block is the generic landing-page default and it
 * would compete with the route diagram, which is the one thing worth
 * watching.
 */
import type { Transition, Variants } from 'motion/react'

/** Long, settling ease. Matches the ink-settling feel of the diagram strokes. */
export const EASE_OUT: [number, number, number, number] = [0.22, 1, 0.36, 1]

/** Slightly overshooting ease, used only for the vermilion seal press. */
export const EASE_PRESS: [number, number, number, number] = [
  0.34, 1.56, 0.64, 1,
]

export const STAGGER = 0.06

/**
 * A staggered container. Children opt in with `rise()`.
 *
 * When `reduced` is set every variant collapses to the visible state so the
 * page renders complete on the first frame instead of waiting on animation.
 */
export function cascade(reduced: boolean): Variants {
  if (reduced) {
    return {
      hidden: { opacity: 1 },
      visible: { opacity: 1 },
    }
  }
  return {
    hidden: { opacity: 0 },
    visible: {
      opacity: 1,
      transition: { staggerChildren: STAGGER, delayChildren: 0.05 },
    },
  }
}

/** One element of the entrance cascade. */
export function rise(reduced: boolean): Variants {
  if (reduced) {
    return { hidden: { opacity: 1 }, visible: { opacity: 1 } }
  }
  // No blur here on purpose. Animating a filter on body copy looks like a
  // broken font render for the first half second, and it re-rasterises every
  // frame. Opacity plus a short rise reads as arrival without the smear.
  return {
    hidden: { opacity: 0, y: 14 },
    visible: {
      opacity: 1,
      y: 0,
      transition: { duration: 0.5, ease: EASE_OUT },
    },
  }
}

/** Diagram strokes draw in after the copy has settled. */
export const DRAW_DELAY = 0.45
export const DRAW_DURATION = 1.1

export const strokeDraw: Transition = {
  duration: DRAW_DURATION,
  delay: DRAW_DELAY,
  ease: EASE_OUT,
}
