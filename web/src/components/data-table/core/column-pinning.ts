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
import { cn } from '@/lib/utils'

import type { DataTableColumnClassName, DataTablePinnedColumn } from './types'

export function getResolvedColumnClassName(
  getColumnClassName?: DataTableColumnClassName,
  pinnedColumns?: DataTablePinnedColumn[]
): DataTableColumnClassName {
  return getResolvedColumnClassNameFromMap(
    getColumnClassName,
    getPinnedColumnMap(pinnedColumns)
  )
}

export function getResolvedColumnClassNameFromMap(
  getColumnClassName?: DataTableColumnClassName,
  pinnedColumnById?: Map<string, DataTablePinnedColumn>
): DataTableColumnClassName {
  return (columnId, kind) => {
    const customClassName = getColumnClassName?.(columnId, kind)
    const pinnedColumn = pinnedColumnById?.get(columnId)

    if (!pinnedColumn) {
      return customClassName
    }

    return cn(customClassName, getPinnedColumnClassName(pinnedColumn, kind))
  }
}

export function getPinnedColumnMap(pinnedColumns?: DataTablePinnedColumn[]) {
  if (!pinnedColumns?.length) {
    return undefined
  }

  return new Map(pinnedColumns.map((column) => [column.columnId, column]))
}

function getPinnedColumnClassName(
  pinnedColumn: DataTablePinnedColumn,
  kind: 'header' | 'cell'
) {
  const edgeClassName =
    pinnedColumn.side === 'left'
      ? 'shadow-[8px_0_10px_-10px_hsl(var(--foreground))]'
      : 'shadow-[-8px_0_10px_-10px_hsl(var(--foreground))]'

  // On a phone the pinned columns are the only ones that always survive, so
  // they must not eat the whole row. 模型 pins both `model_name` (left) and
  // `actions` (right); at 360px those two alone filled the viewport and the
  // middle columns collapsed to zero width — which looked like content being
  // sliced off rather than a table that scrolls.
  //
  // Below `sm` the side shadow is dropped (on paper it is a rule, not a
  // shadow) and the pinned cell is capped so the scrollable middle keeps a
  // usable width.
  const mobilePinClassName =
    'max-sm:max-w-[62vw] max-sm:overflow-hidden max-sm:shadow-none'

  return cn(
    'sticky whitespace-nowrap',
    pinnedColumn.side === 'left' ? 'left-0' : 'right-0',
    edgeClassName,
    mobilePinClassName,
    kind === 'header'
      ? '[background-color:var(--table-header-bg,var(--table-header))] group-hover:[background-color:var(--table-header-hover)] z-30'
      : 'bg-background z-10 group-hover:[background-color:color-mix(in_oklch,var(--muted)_50%,var(--background))] group-data-[state=selected]:bg-muted',
    pinnedColumn.className,
    kind === 'header'
      ? pinnedColumn.headerClassName
      : pinnedColumn.cellClassName
  )
}
