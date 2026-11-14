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
 * The round route diagram — a 罗盘 of roads.
 *
 * Requests rise from the foot of the diagram, are reined in at the seal, and
 * leave along the radius that belongs to their upstream. Movement runs on
 * requestAnimationFrame against the real path geometry so each courier sits on
 * the line the reader can see; positions are written straight to the DOM
 * because a React render per frame would be pure waste.
 */
import { useEffect, useRef } from 'react'
import { motion, useReducedMotion } from 'motion/react'

import { Seal } from './seal'
import {
  CENTER,
  PROVIDER_RING,
  ROUTE_CLIENTS,
  ROUTE_PROVIDERS,
  SEAL_SIZE,
  VIEW,
  clientPath,
  providerLabelAnchor,
  providerPath,
  providerPoint,
} from './route-layout'

type Packet = {
  pathIndex: number
  progress: number
  speed: number
  node: SVGCircleElement
  halo: SVGCircleElement
}

type RoutingDiagramProps = {
  labels: {
    station: string
    stationNote: string
    clients: string[]
    providers: string[]
  }
}

const COURIER_COUNT = 9

export function RoutingDiagram(props: RoutingDiagramProps) {
  const reduced = useReducedMotion() ?? false
  const svgRef = useRef<SVGSVGElement | null>(null)
  const packetsRef = useRef<Packet[]>([])
  const rafRef = useRef(0)

  useEffect(() => {
    if (reduced) return
    const svg = svgRef.current
    if (!svg) return

    const inbound = [
      ...svg.querySelectorAll<SVGPathElement>('path[data-kind="inbound"]'),
    ]
    const outbound = [
      ...svg.querySelectorAll<SVGPathElement>('path[data-kind="outbound"]'),
    ]
    const layer = svg.querySelector<SVGGElement>('g[data-kind="couriers"]')
    if (inbound.length === 0 || outbound.length === 0 || !layer) return

    const paths = [...inbound, ...outbound]
    const lengths = paths.map((path) => path.getTotalLength())
    const packets: Packet[] = []

    for (let index = 0; index < COURIER_COUNT; index += 1) {
      const pathIndex = index % paths.length
      const halo = document.createElementNS(
        'http://www.w3.org/2000/svg',
        'circle'
      )
      const node = document.createElementNS(
        'http://www.w3.org/2000/svg',
        'circle'
      )
      halo.setAttribute('r', '8')
      halo.setAttribute('class', 'sp-courier-halo')
      node.setAttribute('r', '2.8')
      node.setAttribute('class', 'sp-courier')
      const stroke = paths[pathIndex].getAttribute('stroke')
      node.style.fill = stroke ?? 'currentColor'
      halo.style.color = stroke ?? 'currentColor'
      layer.append(halo, node)
      packets.push({
        pathIndex,
        progress: index / COURIER_COUNT,
        speed: 0.0042 + (index % 3) * 0.0011,
        node,
        halo,
      })
    }
    packetsRef.current = packets

    let last = performance.now()
    const tick = (now: number) => {
      const delta = Math.min(64, now - last)
      last = now
      for (const packet of packetsRef.current) {
        packet.progress += packet.speed * (delta / 16.67)
        if (packet.progress >= 1) {
          // Alternate inbound and outbound so the seal is seen to work both ways.
          const useInbound = Math.random() < 0.4
          const pool = useInbound ? inbound : outbound
          const chosen = pool[Math.floor(Math.random() * pool.length)]
          const nextIndex = paths.indexOf(chosen)
          if (nextIndex >= 0) {
            packet.pathIndex = nextIndex
            packet.progress = 0
            const stroke = chosen.getAttribute('stroke')
            packet.node.style.fill = stroke ?? 'currentColor'
            packet.halo.style.color = stroke ?? 'currentColor'
          }
        }
        const path = paths[packet.pathIndex]
        const point = path.getPointAtLength(
          lengths[packet.pathIndex] * packet.progress
        )
        packet.node.setAttribute('cx', point.x.toFixed(2))
        packet.node.setAttribute('cy', point.y.toFixed(2))
        packet.halo.setAttribute('cx', point.x.toFixed(2))
        packet.halo.setAttribute('cy', point.y.toFixed(2))
        const edge = Math.min(packet.progress, 1 - packet.progress) / 0.07
        const opacity = Math.max(0, Math.min(1, edge))
        packet.node.style.opacity = opacity.toFixed(2)
        packet.halo.style.opacity = (opacity * 0.3).toFixed(2)
      }
      rafRef.current = requestAnimationFrame(tick)
    }
    rafRef.current = requestAnimationFrame(tick)

    return () => {
      cancelAnimationFrame(rafRef.current)
      for (const packet of packetsRef.current) {
        packet.node.remove()
        packet.halo.remove()
      }
      packetsRef.current = []
    }
  }, [reduced])

  // Arrows are information, not decoration: they must appear even when the
  // reader asked for reduced motion, so they live in their own effect.
  useEffect(() => {
    const svg = svgRef.current
    if (!svg) return
    const layer = svg.querySelector<SVGGElement>('g[data-kind="arrows"]')
    if (!layer) return
    layer.replaceChildren()

    const draw = (selector: string, at: number) => {
      for (const path of svg.querySelectorAll<SVGPathElement>(selector)) {
        const length = path.getTotalLength()
        const point = path.getPointAtLength(length * at)
        const before = path.getPointAtLength(Math.max(0, length * at - 2))
        const after = path.getPointAtLength(Math.min(length, length * at + 2))
        const angle =
          (Math.atan2(after.y - before.y, after.x - before.x) * 180) / Math.PI
        const head = document.createElementNS(
          'http://www.w3.org/2000/svg',
          'polygon'
        )
        head.setAttribute('points', '0,-4.6 8.4,0 0,4.6')
        head.setAttribute('fill', path.getAttribute('stroke') ?? 'currentColor')
        head.setAttribute('opacity', '0.7')
        head.setAttribute(
          'transform',
          `translate(${point.x.toFixed(2)} ${point.y.toFixed(2)}) rotate(${angle.toFixed(1)})`
        )
        layer.appendChild(head)
      }
    }
    draw('path[data-kind="inbound"]', 0.52)
    draw('path[data-kind="outbound"]', 0.62)
  }, [])

  return (
    <svg
      ref={svgRef}
      viewBox={`0 0 ${VIEW} ${VIEW}`}
      role='img'
      aria-label={props.labels.stationNote}
      className='sp-map-svg mx-auto w-full max-w-[560px] overflow-visible'
    >
      {/* 云纹: a slow ring of dashes, the only ambient motion on the page. */}
      <g className={reduced ? undefined : 'sp-cloud-ring'}>
        <circle
          cx={CENTER}
          cy={CENTER}
          r={PROVIDER_RING + 56}
          className='sp-cloud'
        />
        <circle
          cx={CENTER}
          cy={CENTER}
          r={PROVIDER_RING - 62}
          className='sp-cloud sp-cloud-outer'
        />
      </g>

      {/* The ring the roads end on. */}
      <circle
        cx={CENTER}
        cy={CENTER}
        r={PROVIDER_RING}
        className='sp-ring'
      />

      {/* Roads */}
      {ROUTE_CLIENTS.map((client) => (
        <path
          key={client.id}
          data-kind='inbound'
          d={clientPath(client)}
          className='sp-road'
          stroke='var(--muted-foreground)'
        />
      ))}
      {ROUTE_PROVIDERS.map((provider) => (
        <path
          key={provider.id}
          data-kind='outbound'
          d={providerPath(provider.angle)}
          className='sp-road'
          stroke={provider.tone}
        />
      ))}

      {/* Arrows, then couriers, then the seal — in that stacking order. */}
      <g data-kind='arrows' />

      {/* Courier layer sits above the roads, below the seal. */}
      <g data-kind='couriers' />

      {/* Inbound entry marks */}
      {ROUTE_CLIENTS.map((client) => (
        <circle
          key={`${client.id}-dot`}
          cx={client.x}
          cy={client.y}
          r='3.4'
          className='sp-node'
        />
      ))}
      {ROUTE_CLIENTS.map((client, index) => (
        <text
          key={`${client.id}-label`}
          x={client.x}
          y={client.y + 30}
          textAnchor='middle'
          className='sp-map-label'
        >
          {props.labels.clients[index] ?? client.label}
        </text>
      ))}

      {/* Provider nodes + labels */}
      {ROUTE_PROVIDERS.map((provider, index) => {
        const point = providerPoint(provider.angle)
        const anchor = providerLabelAnchor(provider.angle)
        return (
          <g key={provider.id}>
            <circle
              cx={point.x}
              cy={point.y}
              r='7.5'
              className='sp-provider-halo'
              style={{ color: provider.tone }}
            />
            <circle
              cx={point.x}
              cy={point.y}
              r='3.6'
              fill={provider.tone}
            />
            <text
              x={anchor.x}
              y={anchor.y}
              textAnchor={anchor.anchor}
              dominantBaseline='middle'
              className='sp-map-label sp-map-label-strong'
            >
              {props.labels.providers[index] ?? provider.name}
            </text>
          </g>
        )
      })}

      {/* The seal, stamped last so it lands on top of everything. */}
      <motion.g
        initial={reduced ? false : { opacity: 0, scale: 0.62 }}
        animate={reduced ? undefined : { opacity: 1, scale: 1 }}
        transition={{
          duration: 0.42,
          delay: 0.55,
          ease: [0.34, 1.56, 0.64, 1],
        }}
        style={{ transformOrigin: `${CENTER}px ${CENTER}px` }}
      >
        <Seal size={SEAL_SIZE * 0.98} cx={CENTER} cy={CENTER} />
      </motion.g>
    </svg>
  )
}
