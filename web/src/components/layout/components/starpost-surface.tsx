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
 * 驿纸 — one paper surface shared by every console page.
 *
 * Why this exists: restyling page by page does not converge. s3ai's approach is
 * the right one — lay a single surface behind the whole console so that every
 * page, including the ones nobody has hand-touched, sits on the same material.
 * Its pages look consistent because the *frame* is consistent, not because every
 * component was individually rebuilt.
 *
 * This does the same thing in the station's language, three layers:
 *
 *   1. 纸底 — a warm tint and a very faint grain, so the page is paper rather
 *      than a white plane. Generated in CSS (two stacked gradients), so there is
 *      no image to load and it costs nothing on a phone.
 *   2. 界格 — vertical columns on a 96px rhythm, the same grid as the landing
 *      page. Masked to fade toward the bottom so it reads as writing paper
 *      rather than a table.
 *   3. 远山 — a thin ink wash along the top edge only. A full mountain range
 *      behind a data table would fight the content; one band reads as a
 *      watermark and stays out of the way.
 *
 * It renders nothing at all under another preset, so switching presets restores
 * the stock console exactly.
 */
export function StarPostSurface() {
  return (
    <div className='sp-surface' aria-hidden='true'>
      <div className='sp-surface-grid' />
      <div className='sp-surface-wash' />
    </div>
  )
}
