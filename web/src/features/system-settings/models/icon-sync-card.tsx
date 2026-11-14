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

import { getIconSyncStatus, previewIconSync, runIconSync } from '../api'
import { useUpdateOption } from '../hooks/use-update-option'
import type { IconSyncRun } from '../types'

const INTERVAL_OPTIONS = [6, 12, 24, 72, 168] as const

function formatTimestamp(seconds: number): string {
  if (!seconds) return '—'
  return new Date(seconds * 1000).toLocaleString()
}

/**
 * Official icon synchronization.
 *
 * Model and vendor records carry a @lobehub/icons name; when it is empty the
 * console falls back to the first letter of the name, which is why a fresh
 * install shows letters instead of brand marks. This card fills those in from
 * the same feed the manual "Sync metadata" wizard reads.
 *
 * The overwrite switch is off by default and is the one setting here that can
 * destroy information: an icon someone chose by hand is replaced only when this
 * is on. The description says so rather than leaving it to be discovered.
 */
export function IconSyncCard() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const updateOption = useUpdateOption()

  const statusQuery = useQuery({
    queryKey: ['icon-sync-status'],
    queryFn: async () => requireServerSuccess(await getIconSyncStatus()),
    refetchInterval: (query) => (query.state.data?.data.running ? 2000 : false),
  })
  const status = statusQuery.data?.data

  const previewMutation = useMutation({
    mutationFn: async () => requireServerSuccess(await previewIconSync()),
    onSuccess: (response) => {
      toast.success(
        t('{{models}} models and {{vendors}} vendors would gain an icon.', {
          models: response.data.models_updated,
          vendors: response.data.vendors_updated,
        })
      )
      void queryClient.invalidateQueries({ queryKey: ['icon-sync-status'] })
    },
    onError: (error: Error) =>
      handleServerError(error, t('Failed to check the official icon list')),
  })

  const runMutation = useMutation({
    mutationFn: async () => requireServerSuccess(await runIconSync()),
    onSuccess: () => {
      toast.success(t('Icon synchronization started'))
      void queryClient.invalidateQueries({ queryKey: ['icon-sync-status'] })
      void queryClient.invalidateQueries({ queryKey: ['models'] })
    },
    onError: (error: Error) =>
      handleServerError(error, t('Failed to start icon synchronization')),
  })

  // A Select reports `null` when cleared; there is no "cleared" state for these
  // settings, so a null is ignored. The card reads its own status endpoint, so
  // the query must be invalidated explicitly — `useUpdateOption` only refreshes
  // the generic system-options and status queries.
  const setOption = (key: string, value: string | number | boolean | null) => {
    if (value === null) return
    updateOption.mutate(
      { key: `icon_sync_setting.${key}`, value },
      {
        onSuccess: () => {
          void queryClient.invalidateQueries({ queryKey: ['icon-sync-status'] })
        },
      }
    )
  }

  const busy = previewMutation.isPending || runMutation.isPending
  const lastRun: IconSyncRun | null = status?.last_run?.result ?? null

  return (
    <div className='flex flex-col gap-4'>
      <div className='flex flex-wrap items-start justify-between gap-3'>
        <div className='max-w-prose space-y-1'>
          <p className='text-sm font-medium'>
            {t('Fill in model and vendor icons automatically')}
          </p>
          <p className='text-muted-foreground text-xs'>
            {t(
              'Reads the brand mark each model publishes upstream and stores it locally. Models without one keep the letter fallback.'
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

      <div className='grid gap-3 sm:grid-cols-2'>
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

        <div className='flex items-start justify-between gap-3 rounded-none border p-3'>
          <div className='space-y-0.5'>
            <p className='text-xs font-medium'>{t('Replace existing icons')}</p>
            <p className='text-muted-foreground text-xs'>
              {t(
                'Off: only records with no icon are filled in. On: every icon is replaced with the published one, including ones you set by hand.'
              )}
            </p>
          </div>
          <Switch
            checked={status?.overwrite ?? false}
            disabled={!status || updateOption.isPending}
            onCheckedChange={(checked) => setOption('overwrite', checked)}
          />
        </div>
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
          {status?.running ? t('Synchronizing…') : t('Sync icons now')}
        </Button>
      </div>

      {statusQuery.isError && (
        <p className='text-destructive text-xs'>{statusQuery.error.message}</p>
      )}

      {lastRun && (
        <div className='rounded-none border p-3 text-xs'>
          <div className='flex flex-wrap items-center gap-x-4 gap-y-1'>
            <span className='font-medium'>
              {t('Last run: {{time}}', {
                time: formatTimestamp(status?.last_run?.updated_at ?? 0),
              })}
            </span>
            <span>
              {t('{{count}} models updated', {
                count: lastRun.models_updated,
              })}
              {lastRun.vendors_updated > 0 &&
                ` · ${t('{{count}} vendors updated', {
                  count: lastRun.vendors_updated,
                })}`}
            </span>
            {lastRun.already_set > 0 && (
              <span>
                {t('{{count}} already had an icon', {
                  count: lastRun.already_set,
                })}
              </span>
            )}
            {lastRun.no_upstream_icon > 0 && (
              <span>
                {t('{{count}} have no official icon', {
                  count: lastRun.no_upstream_icon,
                })}
              </span>
            )}
          </div>
          {lastRun.sample && lastRun.sample.length > 0 && (
            <p className='text-muted-foreground mt-2'>
              {t('Examples: {{names}}', {
                names: lastRun.sample.join('、'),
              })}
            </p>
          )}
        </div>
      )}
    </div>
  )
}
