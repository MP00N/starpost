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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { CheckSquare, RefreshCcw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { handleServerError } from '@/lib/handle-server-error'
import { requireServerSuccess } from '@/lib/server-error-message'

import {
  getPricingSyncStatus,
  previewPricingSync,
  runPricingSync,
} from '../api'
import { useUpdateOption } from '../hooks/use-update-option'
import type { PricingSyncRun } from '../types'

const INTERVAL_OPTIONS = [6, 12, 24, 72, 168] as const

function formatTimestamp(seconds: number): string {
  if (!seconds) return '—'
  return new Date(seconds * 1000).toLocaleString()
}

/**
 * Official price synchronization.
 *
 * Two things this card deliberately does not do. It does not let the
 * administrator type a price — the point is to adopt the vendor's published
 * list. And it does not hide the currency question behind a label: every run
 * measures which currency the feed is in and reports the votes, because a feed
 * silently switching from USD to CNY would multiply every price by the exchange
 * rate.
 */
export function PricingSyncCard() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const updateOption = useUpdateOption()

  const statusQuery = useQuery({
    queryKey: ['pricing-sync-status'],
    queryFn: async () => requireServerSuccess(await getPricingSyncStatus()),
    // A run takes a few seconds; polling while one is in flight keeps the card
    // honest about progress without a websocket.
    refetchInterval: (query) => (query.state.data?.data.running ? 2000 : false),
  })
  const status = statusQuery.data?.data

  const previewMutation = useMutation({
    mutationFn: async () => requireServerSuccess(await previewPricingSync()),
    onSuccess: (response) => {
      toast.success(
        t('A sync would update {{updated}} of {{considered}} models.', {
          updated: response.data.updated,
          considered: response.data.considered,
        })
      )
      void queryClient.invalidateQueries({ queryKey: ['pricing-sync-status'] })
    },
    onError: (error: Error) =>
      handleServerError(error, t('Failed to check the official price list')),
  })

  const runMutation = useMutation({
    mutationFn: async () => requireServerSuccess(await runPricingSync()),
    onSuccess: () => {
      toast.success(t('Price synchronization started'))
      void queryClient.invalidateQueries({ queryKey: ['pricing-sync-status'] })
      void queryClient.invalidateQueries({ queryKey: ['model-pricing'] })
    },
    onError: (error: Error) =>
      handleServerError(error, t('Failed to start price synchronization')),
  })

  // A Select reports `null` when it is cleared. There is no "cleared" state for
  // these settings — an empty interval or currency would be rejected by the
  // backend validator — so a null is simply ignored.
  //
  // `useUpdateOption` refreshes the generic system-options and status queries,
  // but this card reads its own `pricing-sync-status` endpoint. Without the
  // explicit invalidation below, toggling the switch would leave the switch
  // showing the old value until the page is reloaded.
  const setOption = (key: string, value: string | number | boolean | null) => {
    if (value === null) return
    updateOption.mutate(
      { key: `pricing_sync_setting.${key}`, value },
      {
        onSuccess: () => {
          void queryClient.invalidateQueries({
            queryKey: ['pricing-sync-status'],
          })
        },
      }
    )
  }

  const busy = previewMutation.isPending || runMutation.isPending
  const lastRun: PricingSyncRun | null = status?.last_run?.result ?? null
  const lastFailed = status?.last_run?.status === 'failed'

  return (
    <div className='flex flex-col gap-4'>
      <div className='flex flex-wrap items-start justify-between gap-3'>
        <div className='max-w-prose space-y-1'>
          <p className='text-sm font-medium'>
            {t('Sync every model to the official price list')}
          </p>
          <p className='text-muted-foreground text-xs'>
            {t(
              'Adopts the price published by the upstream metadata feed. Each run measures the feed currency before writing, and converts to your site currency when the vendor publishes one.'
            )}
          </p>
        </div>
        <div className='flex shrink-0 items-center gap-2'>
          <span className='text-muted-foreground text-xs'>
            {t('Automatic sync')}
          </span>
          <Switch
            checked={status?.enabled ?? false}
            disabled={!status || updateOption.isPending}
            onCheckedChange={(checked) => setOption('enabled', checked)}
          />
        </div>
      </div>

      <div className='grid gap-3 sm:grid-cols-3'>
        <label className='flex flex-col gap-1.5'>
          <span className='text-xs font-medium'>{t('Sync interval')}</span>
          <Select
            value={String(status?.interval_hours ?? 24)}
            disabled={!status || updateOption.isPending}
            onValueChange={(value) => setOption('interval_hours', value)}
          >
            <SelectTrigger className='w-full'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {INTERVAL_OPTIONS.map((hours) => (
                <SelectItem key={hours} value={String(hours)}>
                  {t('Every {{hours}} hours', { hours })}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </label>

        <label className='flex flex-col gap-1.5'>
          <span className='text-xs font-medium'>{t('Price currency')}</span>
          <Select
            value={status?.currency ?? 'auto'}
            disabled={!status || updateOption.isPending}
            onValueChange={(value) => setOption('currency', value)}
          >
            <SelectTrigger className='w-full'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value='auto'>
                {t('Follow the site display currency ({{currency}})', {
                  currency: status?.site_display_currency ?? 'USD',
                })}
              </SelectItem>
              <SelectItem value='USD'>{t('US dollars')}</SelectItem>
              <SelectItem value='CNY'>
                {t('Chinese yuan (use the vendor CNY list when published)')}
              </SelectItem>
            </SelectContent>
          </Select>
        </label>

        <label className='flex flex-col gap-1.5'>
          <span className='text-xs font-medium'>{t('Models to sync')}</span>
          <Select
            value={status?.scope ?? 'served'}
            disabled={!status || updateOption.isPending}
            onValueChange={(value) => setOption('scope', value)}
          >
            <SelectTrigger className='w-full'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value='served'>
                {t('Only models this site serves')}
              </SelectItem>
              <SelectItem value='all'>
                {t('Every model in the official list')}
              </SelectItem>
            </SelectContent>
          </Select>
        </label>
      </div>

      <div className='flex flex-wrap items-center gap-2'>
        <Button
          variant='outline'
          disabled={busy || !status}
          onClick={() => previewMutation.mutate()}
        >
          <RefreshCcw className='size-4' aria-hidden />
          {t('Check what would change')}
        </Button>
        <Button
          disabled={busy || !status || status.running}
          onClick={() => runMutation.mutate()}
        >
          <CheckSquare className='size-4' aria-hidden />
          {status?.running
            ? t('Synchronizing…')
            : t('Sync official prices now')}
        </Button>
        {status && (
          <span className='text-muted-foreground text-xs'>
            {t('Exchange rate: 1 USD = {{rate}}', {
              rate: String(status.exchange_rate),
            })}
          </span>
        )}
      </div>

      {statusQuery.isError && (
        <p className='text-destructive text-xs'>{statusQuery.error.message}</p>
      )}

      {lastRun && (
        <div className='rounded-none border p-3 text-xs'>
          <div className='flex flex-wrap items-center gap-x-4 gap-y-1'>
            <span className='font-medium'>
              {t('Last run: {{time}}', {
                time: formatTimestamp(lastRun.finished_at),
              })}
            </span>
            <span>
              {t('{{updated}} updated', { updated: lastRun.updated })} ·{' '}
              {t('{{unchanged}} unchanged', { unchanged: lastRun.unchanged })}
              {lastRun.skipped > 0 &&
                ` · ${t('{{skipped}} skipped', { skipped: lastRun.skipped })}`}
              {lastRun.failed > 0 &&
                ` · ${t('{{failed}} failed', { failed: lastRun.failed })}`}
            </span>
            <span>
              {t('Feed currency: {{currency}}', {
                currency: lastRun.feed_currency,
              })}
              {lastRun.currency_verified
                ? ` (${t('confirmed by {{count}} models', { count: lastRun.usd_votes })})`
                : ` (${t('not confirmed')})`}
            </span>
            {lastRun.published_cny_used > 0 && (
              <span>
                {t('{{count}} models used the vendor CNY list', {
                  count: lastRun.published_cny_used,
                })}
              </span>
            )}
          </div>
          {lastRun.warnings && lastRun.warnings.length > 0 && (
            <ul className='text-muted-foreground mt-2 list-disc space-y-0.5 pl-4'>
              {lastRun.warnings.map((warning) => (
                <li key={warning}>{warning}</li>
              ))}
            </ul>
          )}
        </div>
      )}

      {lastFailed && !lastRun && status?.last_run?.error && (
        <p className='text-destructive text-xs'>{status.last_run.error}</p>
      )}
    </div>
  )
}
