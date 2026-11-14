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
 * Geometry for the circular route diagram.
 *
 * The composition is a 圆相 — one station at the exact centre, roads leaving
 * on evenly spaced radii. A circle has a single centre of gravity, which is
 * the point: the previous left-to-right spread read as two unrelated columns.
 *
 * Angles are degrees in SVG space (y grows downwards), so -90 is north.
 */

export const VIEW = 800
export const CENTER = VIEW / 2
export const PROVIDER_RING = 248
export const SEAL_SIZE = 168

export type RouteClient = {
  id: string
  label: string
  /** Entry point at the foot of the scroll. */
  x: number
  y: number
}

export type RouteProvider = {
  id: string
  name: string
  /** Radians on the ring. */
  angle: number
  tone: string
}

/** Three ways a request arrives, lined up along the foot of the diagram. */
export const ROUTE_CLIENTS: RouteClient[] = [
  { id: 'app', label: 'Your app', x: 236, y: 676 },
  { id: 'agent', label: 'AI agents', x: 400, y: 726 },
  { id: 'ide', label: 'IDE and CLI', x: 564, y: 676 },
]

/**
 * Six roads, evenly spaced but rotated half a step so the southern arc stays
 * clear for the three inbound entries.
 */
export const ROUTE_PROVIDERS: RouteProvider[] = [
  { id: 'openai', name: 'OpenAI', angle: -60, tone: 'var(--chart-1)' },
  { id: 'claude', name: 'Claude', angle: 0, tone: 'var(--chart-3)' },
  { id: 'gemini', name: 'Gemini', angle: 60, tone: 'var(--chart-2)' },
  { id: 'deepseek', name: 'DeepSeek', angle: 120, tone: 'var(--chart-5)' },
  { id: 'qwen', name: 'Qwen', angle: 180, tone: 'var(--chart-4)' },
  { id: 'kimi', name: 'Kimi', angle: 240, tone: 'var(--chart-2)' },
]

const RAD = Math.PI / 180

export function providerPoint(angle: number): { x: number; y: number } {
  return {
    x: CENTER + PROVIDER_RING * Math.cos(angle * RAD),
    y: CENTER + PROVIDER_RING * Math.sin(angle * RAD),
  }
}

/** Inbound: from the client's point up into the seal, bowing along the axis. */
export function clientPath(client: RouteClient): string {
  // Each entry bows out on its own side and then joins the centre line, so the
  // three roads read as three roads instead of one bundle.
  const c1y = client.y - (client.y - CENTER) * 0.45
  const c2y = CENTER + SEAL_SIZE * 1.45
  return `M ${client.x} ${client.y} C ${client.x} ${c1y}, ${CENTER} ${c2y}, ${CENTER} ${
    CENTER + SEAL_SIZE / 2
  }`
}

/** Outbound: a straight radius from the seal edge to the ring. */
export function providerPath(angle: number): string {
  const inner = SEAL_SIZE / 2 + 2
  const x1 = CENTER + inner * Math.cos(angle * RAD)
  const y1 = CENTER + inner * Math.sin(angle * RAD)
  const outer = providerPoint(angle)
  return `M ${x1} ${y1} L ${outer.x} ${outer.y}`
}

/** Label anchors: text sits just outside the ring, aligned away from it. */
export function providerLabelAnchor(angle: number): {
  x: number
  y: number
  anchor: 'start' | 'middle' | 'end'
} {
  const p = providerPoint(angle)
  const dx = Math.cos(angle * RAD)
  const dy = Math.sin(angle * RAD)
  const x = p.x + dx * 24
  const y = p.y + dy * 24
  let anchor: 'start' | 'middle' | 'end' = 'middle'
  if (Math.abs(dx) >= 0.3) {
    anchor = dx > 0 ? 'start' : 'end'
  }
  return { x, y, anchor }
}
