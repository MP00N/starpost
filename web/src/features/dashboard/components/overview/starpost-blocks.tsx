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
 * 驿程 / 驿路指引 — replacements for two stock overview blocks.
 *
 * Why rewrite rather than restyle: the stock blocks are card-in-card stacks.
 * "Get started" is a bordered panel containing a bordered <ol> containing three
 * bordered rows; "Recommended actions" is four bordered boxes inside a bordered
 * box. No amount of radius or colour makes that read as a document.
 *
 * 驿程 — the three setup steps as a route: one vertical rule with numbered nodes
 * on it. State is carried by the node itself (hollow = not started, ring =
 * current, filled vermilion = passed), not by a badge.
 *
 * 驿路指引 — the quick actions as an index of destinations: name on the left,
 * arrow on the right, hairlines between. It reads like a table of contents,
 * which is what it actually is.
 */
import { Link } from '@tanstack/react-router'
import { ArrowUpRight, Check, type LucideIcon } from 'lucide-react'

export type JourneyStep = {
  title: string
  description: string
  to: string
  icon: LucideIcon
  completed: boolean
}

export type GuideEntry = {
  title: string
  description: string
  to: string
  icon: LucideIcon
}

/** 驿程 — the numbered route, nodes on one rule. */
export function JourneyList(props: { steps: JourneyStep[] }) {
  return (
    <ol className='sp-journey'>
      {props.steps.map((step, index) => {
        const isCurrent =
          !step.completed &&
          props.steps.slice(0, index).every((s) => s.completed)
        let state: 'passed' | 'current' | 'todo' = 'todo'
        if (step.completed) {
          state = 'passed'
        } else if (isCurrent) {
          state = 'current'
        }
        const Icon = step.icon

        return (
          <li key={step.title} className='sp-journey-item' data-state={state}>
            <span className='sp-journey-node' aria-hidden='true'>
              {step.completed ? <Check className='size-2.5' /> : null}
            </span>
            <Link to={step.to} className='sp-journey-body'>
              <span className='sp-journey-index' aria-hidden='true'>
                {String(index + 1).padStart(2, '0')}
              </span>
              <span className='sp-journey-text'>
                <span className='sp-journey-title'>{step.title}</span>
                <span className='sp-journey-desc'>{step.description}</span>
              </span>
              <Icon className='sp-journey-icon' aria-hidden='true' />
            </Link>
          </li>
        )
      })}
    </ol>
  )
}

/** 驿路指引 — destinations as an index, not as boxes. */
export function GuideIndex(props: { entries: GuideEntry[] }) {
  return (
    <ul className='sp-guide-index'>
      {props.entries.map((entry) => {
        const Icon = entry.icon
        return (
          <li key={entry.title}>
            <Link to={entry.to} className='sp-guide-entry'>
              <Icon className='sp-guide-entry-icon' aria-hidden='true' />
              <span className='sp-guide-entry-text'>
                <span className='sp-guide-entry-title'>{entry.title}</span>
                <span className='sp-guide-entry-desc'>{entry.description}</span>
              </span>
              <ArrowUpRight
                className='sp-guide-entry-arrow'
                aria-hidden='true'
              />
            </Link>
          </li>
        )
      })}
    </ul>
  )
}

/** 驿规 — the section heading shared by both blocks above. */
export function BoardSectionHead(props: {
  mark: string
  eyebrow: string
  title: string
}) {
  return (
    <div className='sp-section-head'>
      <span className='sp-section-mark' aria-hidden='true'>
        {props.mark}
      </span>
      <span className='sp-section-eyebrow'>{props.eyebrow}</span>
      <h3 className='sp-section-title'>{props.title}</h3>
    </div>
  )
}
