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
 * 进价管理 — the upstream cost table, finally editable.
 *
 * Without this the profit page is decorative: every model reads "no cost set",
 * nothing is ever judged a loss, and the guard never fires. So this drawer is not
 * a convenience, it is the missing half of the feature.
 *
 * Two ways in:
 *   从上游导入 — paste a new-api provider's address and key, pick the group, done.
 *   手工改一条 — for providers that are not new-api, or to correct one row.
 *
 * Two facts the UI has to make obvious, both learned the hard way against a real
 * provider:
 *   · 分组倍率会乘进成本。同一个上游不同分组差 12 倍（0.08 到 1.0），选错分组
 *     成本就全错。
 *   · 有些模型不是倍率计费（billing_mode: tiered_expr），它们的 ratio 是占位值，
 *     算出来会差好几倍。这类模型会明确标注「按表达式计费，无法按倍率核算」，
 *     不允许手工填一个假数字上去。
 */
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Download, Pencil } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import { api } from '@/lib/api'

type CostEntry = {
  ratio: number
  completion_ratio: number
  cache_ratio: number
  billing_mode?: string
  billing_expr?: string
  source?: string
}

type CostResponse = {
  entries: Record<string, CostEntry>
  count: number
}

type ImportResult = {
  imported: number
  skipped?: number
  unsupported: string[]
  group: string
  group_ratio: number
}

export function CostDrawer(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** 盈亏页已知的模型名，用于标明「哪些模型还没有进价」 */
  knownModels?: string[]
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const [baseUrl, setBaseUrl] = useState('')
  const [key, setKey] = useState('')
  const [group, setGroup] = useState('')
  const [lastImport, setLastImport] = useState<ImportResult | null>(null)
  const [editing, setEditing] = useState<string | null>(null)
  const [editRatio, setEditRatio] = useState('')
  const [editCompletion, setEditCompletion] = useState('')
  const [editCache, setEditCache] = useState('')

  const { data, isLoading } = useQuery({
    queryKey: ['upstream-cost'],
    queryFn: async () => {
      const res = await api.get('/api/user/upstream-cost')
      return res.data.data as CostResponse
    },
    enabled: props.open,
  })

  const importMutation = useMutation({
    mutationFn: async () => {
      const res = await api.post('/api/user/upstream-cost/import', {
        base_url: baseUrl,
        key,
        group,
      })
      return res.data.data as ImportResult
    },
    onSuccess: (result) => {
      setLastImport(result)
      queryClient.invalidateQueries({ queryKey: ['upstream-cost'] })
      queryClient.invalidateQueries({ queryKey: ['model-profit'] })
    },
  })

  const setMutation = useMutation({
    mutationFn: async (payload: {
      model: string
      ratio: number
      completion_ratio: number
      cache_ratio: number
    }) => {
      await api.post('/api/user/upstream-cost/set', payload)
    },
    onSuccess: () => {
      setEditing(null)
      queryClient.invalidateQueries({ queryKey: ['upstream-cost'] })
      queryClient.invalidateQueries({ queryKey: ['model-profit'] })
    },
  })

  const entries = data?.entries ?? {}
  const configured = Object.keys(entries).length
  const missing = (props.knownModels ?? []).filter((name) => !entries[name])

  const startEdit = (model: string) => {
    const existing = entries[model]
    setEditing(model)
    setEditRatio(existing ? String(existing.ratio) : '')
    setEditCompletion(existing ? String(existing.completion_ratio) : '')
    setEditCache(existing ? String(existing.cache_ratio) : '')
  }

  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent className='w-full overflow-y-auto sm:max-w-2xl'>
        <SheetHeader>
          <SheetTitle>{t('Upstream cost')}</SheetTitle>
          <SheetDescription>
            {t(
              'What the provider charges you. Profit is revenue minus this cost — without it nothing can be judged.'
            )}
          </SheetDescription>
        </SheetHeader>

        {/* 导入 */}
        <section className='mt-4 space-y-2'>
          <h3 className='text-sm font-medium'>{t('Import from a new-api provider')}</h3>
          <p className='text-muted-foreground text-[11.5px]'>
            {t(
              'The group matters: the same provider can differ 12x between groups. Import at the group your key actually uses.'
            )}
          </p>

          <div className='grid gap-2 sm:grid-cols-2'>
            <div>
              <Label htmlFor='upstream-url' className='text-[11.5px]'>
                {t('Base URL')}
              </Label>
              <Input
                id='upstream-url'
                placeholder='https://api.example.com'
                value={baseUrl}
                onChange={(event) => setBaseUrl(event.target.value)}
              />
            </div>
            <div>
              <Label htmlFor='upstream-key' className='text-[11.5px]'>
                {t('API key (optional)')}
              </Label>
              <Input
                id='upstream-key'
                placeholder='sk-...'
                value={key}
                onChange={(event) => setKey(event.target.value)}
              />
            </div>
            <div className='sm:col-span-2'>
              <Label htmlFor='upstream-group' className='text-[11.5px]'>
                {t('Group')}
              </Label>
              <Input
                id='upstream-group'
                placeholder={t('e.g. Codex (Special Offer) — exactly as the provider spells it')}
                value={group}
                onChange={(event) => setGroup(event.target.value)}
              />
            </div>
          </div>

          <Button
            onClick={() => importMutation.mutate()}
            disabled={!baseUrl || importMutation.isPending}
          >
            <Download data-icon='inline-start' />
            {importMutation.isPending ? t('Importing…') : t('Import prices')}
          </Button>

          {importMutation.isError && (
            <p className='text-destructive text-[12px]'>
              {t('Import failed. Check the address, key and group name.')}
            </p>
          )}

          {lastImport && (
            <div className='border-border/70 rounded border p-2 text-[12px]'>
              <p>
                {t('Imported {{n}} models at group {{group}} ({{ratio}}x).', {
                  n: lastImport.imported,
                  group: lastImport.group || '—',
                  ratio: lastImport.group_ratio,
                })}
              </p>
              {lastImport.skipped ? (
                <p className='text-muted-foreground mt-1'>
                  {t('{{n}} models skipped — your group cannot reach them.', {
                    n: lastImport.skipped,
                  })}
                </p>
              ) : null}
              {lastImport.unsupported.length > 0 && (
                <p className='text-muted-foreground mt-1'>
                  {t(
                    '{{n}} models are billed by expression or per call, not by ratio, so they are excluded: {{list}}',
                    {
                      n: lastImport.unsupported.length,
                      list: lastImport.unsupported.slice(0, 4).join(', '),
                    }
                  )}
                </p>
              )}
            </div>
          )}
        </section>

        {/* 现状 */}
        <section className='mt-6'>
          <h3 className='text-sm font-medium'>
            {t('Configured')} <span className='text-muted-foreground'>({configured})</span>
          </h3>

          {isLoading && <Skeleton className='mt-2 h-20 w-full' />}

          {configured === 0 && !isLoading && (
            <p className='text-muted-foreground mt-2 text-[12.5px]'>
              {t('No cost configured yet. Import above, or set one by hand.')}
            </p>
          )}

          {configured > 0 && (
            <ul className='border-border/70 mt-2 border-t'>
              {Object.entries(entries)
                .sort((a, b) => a[0].localeCompare(b[0]))
                .map(([model, entry]) => {
                  const expressionPriced = entry.billing_mode === 'tiered_expr'
                  return (
                    <li
                      key={model}
                      className='border-border/60 flex flex-wrap items-center gap-2 border-b px-1 py-2 text-[12.5px]'
                    >
                      <span className='font-mono'>{model}</span>

                      {expressionPriced ? (
                        <Badge variant='secondary'>
                          {t('Expression-priced, cannot be computed by ratio')}
                        </Badge>
                      ) : (
                        <span className='text-muted-foreground font-mono tabular-nums'>
                          {entry.ratio}× / {entry.completion_ratio}× /{' '}
                          {entry.cache_ratio}×
                        </span>
                      )}

                      {entry.source && (
                        <span className='text-muted-foreground ml-auto text-[11px]'>
                          {entry.source}
                        </span>
                      )}

                      {!expressionPriced && (
                        <Button
                          size='sm'
                          variant='ghost'
                          className='h-6 px-2'
                          onClick={() => startEdit(model)}
                        >
                          <Pencil className='size-3' />
                        </Button>
                      )}
                    </li>
                  )
                })}
            </ul>
          )}
        </section>

        {/* 缺进价的模型 */}
        {missing.length > 0 && (
          <section className='mt-6'>
            <h3 className='text-sm font-medium'>
              {t('Models with traffic but no cost')}{' '}
              <span className='text-muted-foreground'>({missing.length})</span>
            </h3>
            <p className='text-muted-foreground mt-1 text-[11.5px]'>
              {t('These are billed by you but the cost is unknown, so profit cannot be judged.')}
            </p>
            <ul className='mt-2 space-y-1'>
              {missing.map((model) => (
                <li key={model} className='flex items-center gap-2 text-[12.5px]'>
                  <span className='font-mono'>{model}</span>
                  <Button
                    size='sm'
                    variant='outline'
                    className='ml-auto h-6 px-2 text-[11.5px]'
                    onClick={() => startEdit(model)}
                  >
                    {t('Set cost')}
                  </Button>
                </li>
              ))}
            </ul>
          </section>
        )}

        {/* 手工编辑 */}
        {editing && (
          <section className='border-border mt-6 rounded border p-3'>
            <h3 className='text-sm font-medium'>
              {t('Set cost for')} <span className='font-mono'>{editing}</span>
            </h3>
            <p className='text-muted-foreground mt-1 text-[11.5px]'>
              {t(
                'Same unit as the provider: ratio 1.0 means $2 per 1M input tokens. Leave 0 to remove.'
              )}
            </p>
            <div className='mt-2 grid gap-2 sm:grid-cols-3'>
              <div>
                <Label className='text-[11.5px]'>{t('Input ratio')}</Label>
                <Input
                  value={editRatio}
                  onChange={(event) => setEditRatio(event.target.value)}
                />
              </div>
              <div>
                <Label className='text-[11.5px]'>{t('Output ratio')}</Label>
                <Input
                  value={editCompletion}
                  onChange={(event) => setEditCompletion(event.target.value)}
                />
              </div>
              <div>
                <Label className='text-[11.5px]'>{t('Cache ratio')}</Label>
                <Input
                  value={editCache}
                  onChange={(event) => setEditCache(event.target.value)}
                />
              </div>
            </div>
            <div className='mt-2 flex gap-2'>
              <Button
                size='sm'
                onClick={() =>
                  setMutation.mutate({
                    model: editing,
                    ratio: Number(editRatio) || 0,
                    completion_ratio: Number(editCompletion) || 0,
                    cache_ratio: Number(editCache) || 0,
                  })
                }
                disabled={setMutation.isPending}
              >
                {t('Save')}
              </Button>
              <Button size='sm' variant='outline' onClick={() => setEditing(null)}>
                {t('Cancel')}
              </Button>
            </div>
          </section>
        )}
      </SheetContent>
    </Sheet>
  )
}
