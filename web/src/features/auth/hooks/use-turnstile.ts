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
import i18next from 'i18next'
import { useState } from 'react'
import { toast } from 'sonner'

import { useStatus } from '@/hooks/use-status'

/**
 * Hook for managing Turnstile verification
 */
export function useTurnstile() {
  const { status } = useStatus()
  const [turnstileToken, setTurnstileToken] = useState('')

  // The operator picks one provider in settings; the key that matters is the
  // one belonging to that provider. Turnstile keeps its historical
  // `turnstile_check` flag so an install that never touched the new setting
  // behaves exactly as before.
  const provider = status?.captcha_type ?? 'turnstile'

  let turnstileSiteKey = ''
  let isTurnstileEnabled = false
  if (provider === 'hcaptcha') {
    turnstileSiteKey = status?.hcaptcha_site_key ?? ''
    isTurnstileEnabled = turnstileSiteKey.length > 0
  } else if (provider === 'geetest') {
    turnstileSiteKey = status?.geetest_captcha_id ?? ''
    isTurnstileEnabled = turnstileSiteKey.length > 0
  } else if (provider === 'none') {
    isTurnstileEnabled = false
  } else {
    turnstileSiteKey = status?.turnstile_site_key ?? ''
    isTurnstileEnabled = !!status?.turnstile_check && turnstileSiteKey.length > 0
  }

  /**
   * Validate if turnstile is ready when required
   */
  const validateTurnstile = (): boolean => {
    if (isTurnstileEnabled && !turnstileToken) {
      toast.info(
        i18next.t('Please wait a moment, human check is initializing...')
      )
      return false
    }
    return true
  }

  return {
    isTurnstileEnabled,
    turnstileSiteKey,
    turnstileToken,
    setTurnstileToken,
    validateTurnstile,
  }
}
