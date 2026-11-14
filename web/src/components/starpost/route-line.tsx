/**
 * 驿路线 — a request is on the road.
 *
 * One pixel of horizontal line at the head of a result area, advancing by
 * `scaleX` on a linear loop. On completion the two ends collapse toward the
 * middle in 150ms, so the line ends where the result appears.
 *
 * It subscribes to the global in-flight counter rather than taking a prop, so a
 * caller only has to render it and can never disagree with the network. The
 * transition between states is written straight to `data-state` on the node:
 * the line is pure decoration, and routing it through React state would cost a
 * re-render of the surrounding panel on every request that starts or ends.
 */
import { useEffect, useRef, useSyncExternalStore } from 'react'
import { useTranslation } from 'react-i18next'

import {
  getInFlightCount,
  subscribeRequestActivity,
} from '@/lib/request-activity'

type RouteLineProps = {
  /** Show a percentage figure beside the line. */
  progress?: number
  className?: string
}

const CLOSE_MS = 150

export function RouteLine(props: RouteLineProps) {
  const { t } = useTranslation()
  const count = useSyncExternalStore(
    subscribeRequestActivity,
    getInFlightCount,
    () => 0
  )
  const lineRef = useRef<HTMLDivElement | null>(null)
  const wasActive = useRef(false)

  useEffect(() => {
    const element = lineRef.current
    if (!element) return

    let timer = 0
    if (count > 0) {
      wasActive.current = true
      element.dataset.state = 'active'
    } else if (wasActive.current) {
      wasActive.current = false
      element.dataset.state = 'closing'
      timer = window.setTimeout(() => {
        element.dataset.state = 'idle'
      }, CLOSE_MS)
    }
    return () => window.clearTimeout(timer)
  }, [count])

  return (
    <div className={props.className}>
      {/* Always mounted: removing it would shift the panel by one pixel on
          every request. Idle state draws nothing. */}
      <div
        ref={lineRef}
        className='sp-route-line text-foreground'
        data-state='idle'
        aria-hidden='true'
      />
      {typeof props.progress === 'number' && (
        <span className='text-muted-foreground mt-1 block text-right font-mono text-[11px] tabular-nums'>
          {Math.max(0, Math.min(100, Math.round(props.progress)))}%
        </span>
      )}
      <span className='sr-only' role='status' aria-live='polite'>
        {count > 0 ? t('Request in progress') : t('Request complete')}
      </span>
    </div>
  )
}
