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
 * Keeps the browser tab in step with the configured site name.
 *
 * `web/index.html` ships a static `<title>` that belongs to the upstream
 * project and must not be edited here. But the running site already knows its
 * own name — `/api/status` returns `system_name`, which the operator sets in
 * System Settings → Site. Until now nothing read it for the tab, so every
 * deployment showed the upstream title no matter what the operator named it.
 *
 * This sets the title from that setting and leaves the shipped default alone:
 * an unconfigured install still reads exactly as it did before.
 */
import { useEffect } from 'react'

import { useStatus } from '@/hooks/use-status'

export function SiteTitle() {
  const { status } = useStatus()
  const configured = status?.system_name

  useEffect(() => {
    if (typeof document === 'undefined') return
    const name = typeof configured === 'string' ? configured.trim() : ''
    // No setting, no override — keep whatever index.html declared.
    if (name.length === 0) return
    document.title = name
  }, [configured])

  return null
}
