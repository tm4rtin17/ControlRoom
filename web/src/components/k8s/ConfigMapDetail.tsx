import { useState, useEffect, useRef, useCallback } from 'react'
import { Trash2, Plus, RefreshCw } from 'lucide-react'

import { useK8sConfigMapDetail, useUpdateConfigMap } from '@/lib/k8s'
import { ApiError } from '@/lib/api'
import { formatBytes } from '@/lib/format'
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription } from '@/components/ui/sheet'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Alert, AlertDescription } from '@/components/ui/alert'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { EventsTable } from './EventsTable'

const KEY_PATTERN = /^[a-zA-Z0-9._-]+$/

interface DataEntry {
  key: string
  value: string
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <p className="mb-2 text-xs uppercase tracking-wider text-muted-foreground">{title}</p>
      {children}
    </div>
  )
}

function dataToEntries(data: Record<string, string>): DataEntry[] {
  return Object.entries(data)
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([key, value]) => ({ key, value }))
}

function entriesToSortedJson(entries: DataEntry[]): string {
  const sorted = [...entries].sort((a, b) => a.key.localeCompare(b.key))
  return JSON.stringify(sorted.map((e) => ({ key: e.key, value: e.value })))
}

function totalBytes(entries: DataEntry[]): number {
  return entries.reduce((sum, e) => sum + new TextEncoder().encode(e.value).length, 0)
}

function CollapsibleSection({ title, children }: { title: string; children: React.ReactNode }) {
  const [open, setOpen] = useState(false)
  return (
    <div>
      <button
        className="mb-2 flex items-center gap-1 text-xs uppercase tracking-wider text-muted-foreground hover:text-foreground"
        onClick={() => setOpen((o) => !o)}
        type="button"
      >
        <span>{open ? '▾' : '▸'}</span>
        {title}
      </button>
      {open && children}
    </div>
  )
}

export function ConfigMapDetail({
  namespace,
  name,
  open,
  onOpenChange,
}: {
  namespace: string | null
  name: string | null
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const detailQ = useK8sConfigMapDetail(open ? namespace : null, open ? name : null)
  const updateMut = useUpdateConfigMap()

  const [entries, setEntries] = useState<DataEntry[]>([])
  const [originalJson, setOriginalJson] = useState('')
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)
  const [saveSuccess, setSaveSuccess] = useState(false)
  const [conflictError, setConflictError] = useState(false)

  const newKeyRefs = useRef<HTMLInputElement[]>([])

  const detail = detailQ.data

  // Sync entries when detail loads or reloads
  useEffect(() => {
    if (!detail) return
    const initial = dataToEntries(detail.data)
    setEntries(initial)
    setOriginalJson(entriesToSortedJson(initial))
    setSaveError(null)
    setSaveSuccess(false)
    setConflictError(false)
  }, [detail])

  const dirty = entries.length > 0
    ? entriesToSortedJson(entries) !== originalJson
    : originalJson !== '[]'

  const duplicateKeys = (() => {
    const seen = new Set<string>()
    const dupes = new Set<string>()
    for (const e of entries) {
      if (seen.has(e.key)) dupes.add(e.key)
      seen.add(e.key)
    }
    return dupes
  })()

  const invalidKeys = entries.filter((e) => e.key && !KEY_PATTERN.test(e.key)).map((e) => e.key)

  const canSave = dirty && duplicateKeys.size === 0 && invalidKeys.length === 0

  const handleReload = useCallback(() => {
    detailQ.refetch()
  }, [detailQ])

  function updateEntry(idx: number, field: 'key' | 'value', val: string) {
    setEntries((prev) => prev.map((e, i) => i === idx ? { ...e, [field]: val } : e))
    setSaveError(null)
    setSaveSuccess(false)
  }

  function removeEntry(idx: number) {
    setEntries((prev) => prev.filter((_, i) => i !== idx))
  }

  function addEntry() {
    setEntries((prev) => {
      const next = [...prev, { key: '', value: '' }]
      // Focus the new key input after render
      setTimeout(() => {
        const el = newKeyRefs.current[next.length - 1]
        el?.focus()
      }, 50)
      return next
    })
  }

  function handleSave() {
    if (!canSave) return
    setConfirmOpen(true)
  }

  function confirmSave() {
    if (!namespace || !name) return
    const data: Record<string, string> = {}
    for (const e of entries) {
      data[e.key] = e.value
    }
    updateMut.mutate(
      { namespace, name, data },
      {
        onSuccess: () => {
          setSaveSuccess(true)
          setSaveError(null)
          setConflictError(false)
          // originalJson will be updated when detail refetches via invalidation
        },
        onError: (err) => {
          if (err instanceof ApiError && err.status === 409) {
            setConflictError(true)
            setSaveError(null)
          } else {
            setSaveError(err instanceof Error ? err.message : 'Save failed.')
            setConflictError(false)
          }
        },
      }
    )
  }

  const cm = detail?.configmap

  return (
    <>
      <Sheet open={open} onOpenChange={onOpenChange}>
        <SheetContent className="overflow-y-auto sm:max-w-2xl">
          <SheetHeader>
            <div className="flex items-start justify-between gap-2">
              <div className="min-w-0">
                <SheetTitle className="font-mono break-all">{cm?.name ?? name}</SheetTitle>
                <SheetDescription>
                  {cm
                    ? `${cm.namespace}${detail && detail.binary_keys.length > 0 ? ` · ${cm.keys.length} data keys · ${detail.binary_keys.length} binary keys (read-only)` : ` · ${cm.keys.length} data keys`}`
                    : (namespace ?? '')}
                </SheetDescription>
              </div>
              <div className="flex shrink-0 items-center gap-2">
                {(updateMut.isPending) && (
                  <span className="text-xs text-muted-foreground">Saving…</span>
                )}
                {saveSuccess && !updateMut.isPending && (
                  <Badge variant="success">Saved</Badge>
                )}
                <Button
                  size="sm"
                  variant="outline"
                  onClick={handleReload}
                  title="Reload and discard local edits"
                >
                  <RefreshCw className="h-3.5 w-3.5" />
                </Button>
                <Button
                  size="sm"
                  disabled={!canSave || updateMut.isPending}
                  onClick={handleSave}
                >
                  Save
                </Button>
              </div>
            </div>
          </SheetHeader>

          {conflictError && (
            <Alert variant="destructive" className="mt-4">
              <AlertDescription>
                This configmap was edited elsewhere. Reload to see the latest version, then re-apply your changes.
                <Button size="sm" variant="outline" className="ml-2 h-6 text-xs" onClick={handleReload}>
                  Reload
                </Button>
              </AlertDescription>
            </Alert>
          )}

          {saveError && !conflictError && (
            <Alert variant="destructive" className="mt-4">
              <AlertDescription>{saveError}</AlertDescription>
            </Alert>
          )}

          {detail && cm && (
            <div className="mt-4 flex flex-col gap-6">
              {/* Data editor */}
              <Section title="Data">
                <div className="overflow-x-auto">
                  <table className="w-full text-xs">
                    <thead>
                      <tr className="border-b text-left text-muted-foreground">
                        <th className="py-1.5 pr-2 font-medium w-[35%]">Key</th>
                        <th className="py-1.5 pr-2 font-medium">Value</th>
                        <th className="py-1.5 w-8" />
                      </tr>
                    </thead>
                    <tbody className="divide-y">
                      {entries.map((entry, idx) => {
                        const keyInvalid = entry.key && !KEY_PATTERN.test(entry.key)
                        const keyDupe = duplicateKeys.has(entry.key)
                        return (
                          <tr key={idx}>
                            <td className="py-1.5 pr-2 align-top">
                              <Input
                                ref={(el) => { if (el) newKeyRefs.current[idx] = el }}
                                value={entry.key}
                                onChange={(e) => updateEntry(idx, 'key', e.target.value)}
                                className={`h-7 font-mono text-xs ${keyInvalid || keyDupe ? 'border-destructive ring-destructive' : ''}`}
                                placeholder="key"
                                aria-label="Entry key"
                              />
                              {keyInvalid && (
                                <p className="mt-0.5 text-[10px] text-destructive">Invalid characters</p>
                              )}
                              {keyDupe && (
                                <p className="mt-0.5 text-[10px] text-destructive">Duplicate key</p>
                              )}
                            </td>
                            <td className="py-1.5 pr-2 align-top">
                              <textarea
                                value={entry.value}
                                onChange={(e) => updateEntry(idx, 'value', e.target.value)}
                                rows={Math.min(10, Math.max(1, entry.value.split('\n').length))}
                                className="flex w-full rounded-md border border-input bg-background px-3 py-1.5 font-mono text-xs ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 resize-y"
                                placeholder="value"
                                aria-label="Entry value"
                              />
                            </td>
                            <td className="py-1.5 align-top">
                              <Button
                                size="icon"
                                variant="ghost"
                                className="h-7 w-7"
                                onClick={() => removeEntry(idx)}
                                title="Remove entry"
                              >
                                <Trash2 className="h-3.5 w-3.5 text-muted-foreground" />
                              </Button>
                            </td>
                          </tr>
                        )
                      })}
                    </tbody>
                  </table>
                </div>
                <Button size="sm" variant="outline" className="mt-2 h-7 text-xs gap-1" onClick={addEntry}>
                  <Plus className="h-3.5 w-3.5" />
                  Add key
                </Button>
              </Section>

              {/* Labels */}
              {Object.keys(detail.labels).length > 0 && (
                <Section title="Labels">
                  <div className="flex flex-wrap gap-1.5">
                    {Object.entries(detail.labels).map(([k, v]) => (
                      <Badge key={k} variant="muted">{k}={v}</Badge>
                    ))}
                  </div>
                </Section>
              )}

              {/* Annotations */}
              {Object.keys(detail.annotations).length > 0 && (
                <CollapsibleSection title="Annotations">
                  <div className="flex flex-wrap gap-1.5">
                    {Object.entries(detail.annotations).map(([k, v]) => (
                      <Badge key={k} variant="muted" className="max-w-xs truncate" title={`${k}=${v}`}>
                        {k}={v}
                      </Badge>
                    ))}
                  </div>
                </CollapsibleSection>
              )}

              {/* Binary keys */}
              {detail.binary_keys.length > 0 && (
                <Section title="Binary Keys">
                  <div className="flex flex-col gap-1">
                    {detail.binary_keys.map((bk) => (
                      <span key={bk} className="font-mono text-xs text-muted-foreground">{bk}</span>
                    ))}
                    <p className="mt-1 text-xs text-muted-foreground">Edit via kubectl; not editable here.</p>
                  </div>
                </Section>
              )}

              {/* Events */}
              <Section title="Events">
                <EventsTable events={detail.events} />
              </Section>
            </div>
          )}

          {detailQ.isLoading && !detail && (
            <p className="mt-6 text-sm text-muted-foreground">Loading…</p>
          )}
          {detailQ.error && !detail && (
            <Alert variant="destructive" className="mt-4">
              <AlertDescription>{detailQ.error instanceof Error ? detailQ.error.message : 'Failed to load configmap.'}</AlertDescription>
            </Alert>
          )}
        </SheetContent>
      </Sheet>

      {/* Save confirmation dialog */}
      <AlertDialog open={confirmOpen} onOpenChange={setConfirmOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Update ConfigMap</AlertDialogTitle>
            <AlertDialogDescription>
              Update configmap{' '}
              <span className="font-mono">
                {namespace}/{name}
              </span>
              ? {entries.length} {entries.length === 1 ? 'key' : 'keys'}, total{' '}
              {formatBytes(totalBytes(entries))}.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={confirmSave}>Update</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
