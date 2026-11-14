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
 * How many requests are in flight right now.
 *
 * A bare counter plus a subscriber set — deliberately not a state library and
 * not a React context. The HTTP client is not a component, so it cannot own
 * React state; and a context would re-render a subtree on every request, which
 * is exactly the kind of cost the motion spec forbids.
 *
 * Consumers subscribe with `useSyncExternalStore`, so only the components that
 * actually draw the in-flight line re-render.
 */

type Listener = () => void

let inFlight = 0
const listeners = new Set<Listener>()

function emit(): void {
  for (const listener of listeners) listener()
}

export function beginRequest(): void {
  inFlight += 1
  emit()
}

export function endRequest(): void {
  inFlight = Math.max(0, inFlight - 1)
  emit()
}

export function getInFlightCount(): number {
  return inFlight
}

export function subscribeRequestActivity(listener: Listener): () => void {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}
