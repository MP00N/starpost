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
 * 换马 — the environment label changes.
 *
 * The old label leaves along the axis, the new one arrives from the same side,
 * so the change reads as one continuous movement rather than two unrelated
 * swaps. No flip, no scale.
 */
import { useEffect, useRef, useState } from 'react'

type MountSwapProps = {
  value: string
  className?: string
}

const SWAP_MS = 200

export function MountSwap(props: MountSwapProps) {
  const [current, setCurrent] = useState(props.value)
  const [previous, setPrevious] = useState<string | null>(null)
  const timer = useRef(0)

  useEffect(() => {
    if (props.value === current) return
    setPrevious(current)
    setCurrent(props.value)
    window.clearTimeout(timer.current)
    timer.current = window.setTimeout(() => setPrevious(null), SWAP_MS)
    return () => window.clearTimeout(timer.current)
    // `current` is intentionally not a dependency: including it would re-run
    // the swap the moment setCurrent lands, restarting the animation.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [props.value])

  return (
    <span
      className={`relative inline-flex items-center overflow-hidden ${props.className ?? ''}`}
    >
      {previous !== null && (
        <span className='sp-swap-out absolute inset-0 inline-flex items-center'>
          {previous}
        </span>
      )}
      <span key={current} className='sp-swap-in inline-flex items-center'>
        {current}
      </span>
    </span>
  )
}
