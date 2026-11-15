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
import { describe, expect, it } from 'vitest'

import { getGroupDescription } from '../model-helpers'

// `/api/pricing` sends `usable_group` as `{group: "description"}` — a plain
// string per group. Reading `.desc` off it returns undefined and no type error,
// which is how a group description once shipped showing nothing at all while
// every test and the build stayed green.
describe('getGroupDescription', () => {
  it('reads the plain string form that /api/pricing sends', () => {
    const usableGroup = {
      国模自营: '自营出问题了，能及时解决并且速度最快',
      free: '稳定性差，但什么价格你自己心里清楚',
    }
    expect(getGroupDescription(usableGroup, '国模自营')).toBe(
      '自营出问题了，能及时解决并且速度最快'
    )
    expect(getGroupDescription(usableGroup, 'free')).toBe(
      '稳定性差，但什么价格你自己心里清楚'
    )
  })

  // `/api/user/groups` (the API-key drawer endpoint) uses the object form. The
  // helper accepts both so the two endpoints cannot be confused again.
  it('also accepts the {desc, ratio} form', () => {
    const usableGroup = { default: { desc: 'Shared pool', ratio: 1 } }
    expect(getGroupDescription(usableGroup, 'default')).toBe('Shared pool')
  })

  it('returns empty for the all pseudo-group', () => {
    expect(getGroupDescription({ all: 'nope' }, 'all')).toBe('')
  })

  it('returns empty for a missing, blank, or unknown group', () => {
    expect(getGroupDescription({}, 'premium')).toBe('')
    expect(getGroupDescription({ blank: '   ' }, 'blank')).toBe('')
    expect(getGroupDescription(undefined, 'anything')).toBe('')
    expect(getGroupDescription({ odd: null }, 'odd')).toBe('')
    expect(getGroupDescription({ n: 42 }, 'n')).toBe('')
  })

  it('trims surrounding whitespace', () => {
    expect(getGroupDescription({ g: '  spaced  ' }, 'g')).toBe('spaced')
  })
})
