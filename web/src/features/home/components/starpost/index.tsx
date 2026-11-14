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
 * 星驿 StarPost — the relay-station homepage.
 *
 * Composition order is deliberate: the station introduces itself, shows the
 * roads, shows what passes through them, hands you the pass, then states the
 * rules. The CTA repeats only at the end, once the reader knows what it is
 * agreeing to.
 */
import { Footer } from '@/components/layout/components/footer'
import { useStatus } from '@/hooks/use-status'

import './starpost.css'

import { InkMountains } from './ink-mountains'
import { StarpostBulletin } from './bulletin'
import { StarpostClosing } from './closing'
import { StarpostHero } from './hero'
import { StarpostPass } from './pass'
import { StarpostPillars } from './pillars'

type StarpostHomeProps = {
  isAuthenticated: boolean
}

const DEFAULT_DOCS = 'https://docs.newapi.pro'

export function StarpostHome(props: StarpostHomeProps) {
  const { status } = useStatus()
  const configuredDocs = status?.docs_link
  const docsUrl =
    typeof configuredDocs === 'string' && configuredDocs.length > 0
      ? configuredDocs
      : DEFAULT_DOCS

  return (
    <div className='sp-page relative isolate w-full'>
      {/* One 界格 for the whole scroll, not per section: the ruled columns are
          the page's spine and they must run continuously. */}
      <div aria-hidden='true' className='sp-kaige sp-kaige-page' />
      {/* 山影 anchor to the viewport, not to the hero, so scrolling can never
          expose the flat underside of the ranges. */}
      <InkMountains />
      <StarpostHero isAuthenticated={props.isAuthenticated} docsUrl={docsUrl} />
      <StarpostBulletin />
      <StarpostPass />
      <StarpostPillars />
      <StarpostClosing isAuthenticated={props.isAuthenticated} />
      <Footer />
    </div>
  )
}
