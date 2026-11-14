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
 * Human verification widget.
 *
 * The name and props are historical: this started as a Turnstile wrapper and
 * three auth forms import it by that name. Rather than fork the forms three
 * ways, the component reads the configured provider from `/api/status` and
 * renders the matching widget. One token channel (`?turnstile=`) carries all
 * three, so the API layer and the forms stay untouched.
 *
 * Token shape per provider:
 *   turnstile / hcaptcha  opaque string
 *   geetest v4            JSON string of {lot_number, captcha_output,
 *                         pass_token, gen_time} — the server re-derives the
 *                         signature from it
 */
import { useEffect, useRef } from 'react'

import { useStatus } from '@/hooks/use-status'

type GeetestValidate = {
  lot_number: string
  captcha_output: string
  pass_token: string
  gen_time: string
}

type GeetestInstance = {
  appendTo: (element: HTMLElement) => void
  onSuccess: (callback: () => void) => void
  onError: (callback: (error: unknown) => void) => void
  onClose: (callback: () => void) => void
  getValidate: () => GeetestValidate | undefined
}

declare global {
  interface Window {
    turnstile?: {
      render: (element: HTMLElement, options: Record<string, unknown>) => void
    }
    hcaptcha?: {
      render: (element: HTMLElement, options: Record<string, unknown>) => void
    }
    initGeetest4?: (
      options: Record<string, unknown>,
      callback: (captcha: GeetestInstance) => void
    ) => void
  }
}

interface TurnstileProps {
  /** Site key / captcha id for the active provider. */
  siteKey: string
  onVerify: (token: string) => void
  onExpire?: () => void
  className?: string
}

const SCRIPTS = {
  turnstile: {
    id: 'cf-turnstile',
    src: 'https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit',
  },
  hcaptcha: {
    id: 'hcaptcha-api',
    src: 'https://js.hcaptcha.com/1/api.js?render=explicit',
  },
  geetest: {
    id: 'geetest-v4',
    src: 'https://static.geetest.com/v4/gt4.js',
  },
} as const

type Provider = keyof typeof SCRIPTS

function loadScript(provider: Provider, onReady: () => void): void {
  const { id, src } = SCRIPTS[provider]
  if (document.getElementById(id)) {
    onReady()
    return
  }
  const script = document.createElement('script')
  script.id = id
  script.src = src
  script.async = true
  script.defer = true
  script.onload = onReady
  document.head.append(script)
}

export function Turnstile(props: TurnstileProps) {
  const { status } = useStatus()
  const container = useRef<HTMLDivElement | null>(null)

  // Local aliases: the effect below must depend on the individual values, not
  // on the props object, or it re-runs whenever the parent re-renders.
  const siteKey = props.siteKey
  const onVerify = props.onVerify
  const onExpire = props.onExpire

  const configured = status?.captcha_type
  // An install that predates `captcha_type` still reports turnstile_check.
  const provider: Provider =
    configured === 'hcaptcha' || configured === 'geetest'
      ? configured
      : 'turnstile'

  useEffect(() => {
    const node = container.current
    if (!node || !siteKey) return

    const render = () => {
      // Clear before every attempt: these widgets throw when rendered twice
      // into the same element, and React may re-run this effect on re-render.
      node.replaceChildren()

      if (provider === 'turnstile' && window.turnstile) {
        window.turnstile.render(node, {
          sitekey: siteKey,
          callback: (token: string) => onVerify(token),
          'error-callback': () => onExpire?.(),
          'expired-callback': () => onExpire?.(),
        })
        return
      }

      if (provider === 'hcaptcha' && window.hcaptcha) {
        window.hcaptcha.render(node, {
          sitekey: siteKey,
          callback: (token: string) => onVerify(token),
          'error-callback': () => onExpire?.(),
          'expired-callback': () => onExpire?.(),
        })
        return
      }

      if (provider === 'geetest' && window.initGeetest4) {
        window.initGeetest4(
          {
            captchaId: siteKey,
            product: 'bind',
            language: 'zho',
          },
          (captcha) => {
            captcha.appendTo(node)
            captcha.onSuccess(() => {
              const validate = captcha.getValidate()
              if (!validate) {
                onExpire?.()
                return
              }
              // Four fields, one token: the server needs all of them to
              // recompute the HMAC signature.
              onVerify(
                JSON.stringify({
                  lot_number: validate.lot_number,
                  captcha_output: validate.captcha_output,
                  pass_token: validate.pass_token,
                  gen_time: validate.gen_time,
                })
              )
            })
            captcha.onError(() => onExpire?.())
            captcha.onClose(() => onExpire?.())
          }
        )
      }
    }

    loadScript(provider, render)
  }, [provider, siteKey, onVerify, onExpire])

  return <div ref={container} className={props.className} />
}
