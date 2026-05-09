import { useState, useCallback, type ReactNode } from 'react'
import { Eye, EyeOff, Copy } from 'lucide-react'

import { useK8sSecretDetail } from '@/lib/k8s'
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription } from '@/components/ui/sheet'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { EventsTable } from './EventsTable'

const MASK = '••••••••'

function isTlsLike(v: string): boolean {
  return v.includes('-----BEGIN') || (v.length > 200 && v.includes('\n'))
}

function parseJsonPretty(v: string): string | null {
  if (!v.startsWith('{') && !v.startsWith('[')) return null
  try {
    return JSON.stringify(JSON.parse(v), null, 2)
  } catch {
    return null
  }
}

async function copyToClipboard(text: string): Promise<boolean> {
  if (navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text)
      return true
    } catch {
      // fall through to execCommand
    }
  }
  // Fallback for insecure contexts
  try {
    const ta = document.createElement('textarea')
    ta.value = text
    ta.style.position = 'fixed'
    ta.style.opacity = '0'
    document.body.appendChild(ta)
    ta.focus()
    ta.select()
    const ok = document.execCommand('copy')
    document.body.removeChild(ta)
    return ok
  } catch {
    return false
  }
}

function typeLabel(t: string): string {
  const prefix = 'kubernetes.io/'
  if (t.startsWith(prefix)) return t.slice(prefix.length)
  return t
}

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div>
      <p className="mb-2 text-xs uppercase tracking-wider text-muted-foreground">{title}</p>
      {children}
    </div>
  )
}

function CollapsibleSection({ title, children }: { title: string; children: ReactNode }) {
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

interface SecretRowProps {
  secretKey: string
  value: string
  revealAll: boolean
}

function SecretRow({ secretKey, value, revealAll }: SecretRowProps) {
  const [revealed, setRevealed] = useState(false)
  const [copyState, setCopyState] = useState<'idle' | 'copied' | 'failed'>('idle')

  const isRevealed = revealAll || revealed

  const handleCopy = useCallback(async () => {
    const ok = await copyToClipboard(value)
    setCopyState(ok ? 'copied' : 'failed')
    setTimeout(() => setCopyState('idle'), 1500)
  }, [value])

  let displayValue: ReactNode = null
  if (isRevealed) {
    const pretty = parseJsonPretty(value)
    const usePre = isTlsLike(value) || pretty !== null
    const text = pretty ?? value
    if (usePre) {
      displayValue = (
        <pre className="whitespace-pre-wrap break-all font-mono text-[11px] text-foreground">{text}</pre>
      )
    } else {
      displayValue = <span className="font-mono text-xs text-foreground break-all">{text}</span>
    }
  } else {
    displayValue = <span className="font-mono text-xs text-muted-foreground select-none">{MASK}</span>
  }

  return (
    <tr className="border-b last:border-0 align-top">
      <td className="py-2 pr-3 w-[35%]">
        <span className="font-mono text-xs">{secretKey}</span>
      </td>
      <td className="py-2 pr-2 min-w-0 max-w-[300px]">
        <div className="break-all">{displayValue}</div>
      </td>
      <td className="py-2 whitespace-nowrap">
        <div className="flex items-center gap-1">
          <Button
            size="icon"
            variant="ghost"
            className="h-6 w-6"
            title={isRevealed ? 'Hide value' : 'Reveal value'}
            onClick={() => setRevealed((r) => !r)}
          >
            {isRevealed ? <EyeOff className="h-3.5 w-3.5" /> : <Eye className="h-3.5 w-3.5" />}
          </Button>
          <Button
            size="icon"
            variant="ghost"
            className="h-6 w-6"
            title="Copy value"
            onClick={handleCopy}
          >
            <Copy className="h-3.5 w-3.5" />
          </Button>
          {copyState !== 'idle' && (
            <span className={`text-[10px] ${copyState === 'copied' ? 'text-emerald-500' : 'text-destructive'}`}>
              {copyState === 'copied' ? 'Copied' : 'Copy failed'}
            </span>
          )}
        </div>
      </td>
    </tr>
  )
}

export function SecretDetail({
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
  const detailQ = useK8sSecretDetail(open ? namespace : null, open ? name : null)
  const [revealAll, setRevealAll] = useState(false)

  // Reset reveal state each time drawer opens
  const handleOpenChange = useCallback(
    (o: boolean) => {
      if (!o) setRevealAll(false)
      onOpenChange(o)
    },
    [onOpenChange]
  )

  const detail = detailQ.data
  const secret = detail?.secret

  const dataEntries = detail
    ? Object.entries(detail.data).sort(([a], [b]) => a.localeCompare(b))
    : []

  return (
    <Sheet open={open} onOpenChange={handleOpenChange}>
      <SheetContent className="overflow-y-auto sm:max-w-2xl">
        <SheetHeader>
          <div className="flex items-start justify-between gap-2">
            <div className="min-w-0">
              <SheetTitle className="font-mono break-all">{secret?.name ?? name}</SheetTitle>
              <SheetDescription>
                {secret
                  ? `${secret.namespace} · ${secret.keys.length} key${secret.keys.length === 1 ? '' : 's'}`
                  : (namespace ?? '')}
              </SheetDescription>
            </div>
            {secret && (
              <div className="flex shrink-0 items-center gap-2">
                <Badge variant="muted">{typeLabel(secret.type)}</Badge>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => setRevealAll((r) => !r)}
                >
                  {revealAll ? <><EyeOff className="mr-1.5 h-3.5 w-3.5" />Hide All</> : <><Eye className="mr-1.5 h-3.5 w-3.5" />Reveal All</>}
                </Button>
              </div>
            )}
          </div>
        </SheetHeader>

        {/* Informational banner */}
        <div className="mt-4 rounded-md border border-border bg-muted/40 px-3 py-2 text-xs text-muted-foreground">
          Read-only. Values are sensitive — every detail view writes an audit row.
        </div>

        {detail && secret && (
          <div className="mt-4 flex flex-col gap-6">
            {/* Data viewer */}
            <Section title="Data">
              {dataEntries.length === 0 ? (
                <p className="text-xs text-muted-foreground">No data keys.</p>
              ) : (
                <div className="overflow-x-auto">
                  <table className="w-full text-xs">
                    <thead>
                      <tr className="border-b text-left text-muted-foreground">
                        <th className="py-1.5 pr-3 font-medium w-[35%]">Key</th>
                        <th className="py-1.5 pr-3 font-medium">Value</th>
                        <th className="py-1.5 font-medium" />
                      </tr>
                    </thead>
                    <tbody>
                      {dataEntries.map(([k, v]) => (
                        <SecretRow key={k} secretKey={k} value={v} revealAll={revealAll} />
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
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
            <AlertDescription>
              {detailQ.error instanceof Error ? detailQ.error.message : 'Failed to load secret.'}
            </AlertDescription>
          </Alert>
        )}
      </SheetContent>
    </Sheet>
  )
}
