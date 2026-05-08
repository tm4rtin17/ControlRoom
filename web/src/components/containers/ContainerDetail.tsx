import { useState } from 'react'
import { Eye, EyeOff } from 'lucide-react'

import { useContainerDetail } from '@/lib/containers'
import { Button } from '@/components/ui/button'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { ContainerStatusBadge } from './StatusBadge'
import { ContainerLogStream } from './LogStream'

export function ContainerDetail({
  id,
  open,
  onOpenChange,
}: {
  id: string | null
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const detail = useContainerDetail(open ? id : null)
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="overflow-y-auto">
        <SheetHeader>
          <SheetTitle className="font-mono break-all">{detail.data?.name ?? id}</SheetTitle>
          <SheetDescription>{detail.data?.image}</SheetDescription>
        </SheetHeader>

        {detail.data && (
          <div className="mt-4 flex flex-col gap-6">
            <div className="flex flex-wrap items-center gap-2">
              <ContainerStatusBadge state={detail.data.state} />
              <span className="text-xs text-muted-foreground">{detail.data.status}</span>
            </div>

            <div className="grid grid-cols-2 gap-3 text-sm">
              <Field label="ID" value={detail.data.id} mono />
              <Field label="Compose" value={detail.data.compose_project || '—'} />
              <Field label="Service" value={detail.data.compose_service || '—'} />
              <Field label="Restart" value={detail.data.restart_policy || '—'} />
              {detail.data.health && <Field label="Health" value={detail.data.health} />}
            </div>

            {detail.data.command?.length > 0 && (
              <Section title="Command">
                <pre className="rounded-md border bg-muted/40 p-2 font-mono text-[11px] whitespace-pre-wrap break-all">
                  {detail.data.command.join(' ')}
                </pre>
              </Section>
            )}

            {detail.data.env?.length > 0 && (
              <EnvSection env={detail.data.env} />
            )}

            {detail.data.mounts?.length > 0 && (
              <Section title="Mounts">
                <ul className="space-y-1 text-xs">
                  {detail.data.mounts.map((m, i) => (
                    <li key={i} className="font-mono break-all">
                      {m.source}{' '}→{' '}{m.destination}
                      <span className="text-muted-foreground"> · {m.type}{m.rw ? ' rw' : ' ro'}</span>
                    </li>
                  ))}
                </ul>
              </Section>
            )}

            {detail.data.networks?.length > 0 && (
              <Section title="Networks">
                <ul className="space-y-1 text-xs">
                  {detail.data.networks.map((n) => (
                    <li key={n.name} className="font-mono">
                      {n.name} · {n.ip_address || '—'}
                    </li>
                  ))}
                </ul>
              </Section>
            )}

            <Section title="Logs">
              {id && <ContainerLogStream id={id} />}
            </Section>
          </div>
        )}
      </SheetContent>
    </Sheet>
  )
}

function Field({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div>
      <div className="text-xs uppercase tracking-wider text-muted-foreground">{label}</div>
      <div className={mono ? 'font-mono text-xs' : 'text-xs'}>{value}</div>
    </div>
  )
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <p className="mb-2 text-xs uppercase tracking-wider text-muted-foreground">{title}</p>
      {children}
    </div>
  )
}

const SENSITIVE_KEY = /(?:PASSWORD|SECRET|TOKEN|KEY|API|CREDENTIAL)/i

function EnvSection({ env }: { env: string[] }) {
  const [revealed, setRevealed] = useState(false)
  return (
    <div>
      <div className="mb-2 flex items-center justify-between">
        <p className="text-xs uppercase tracking-wider text-muted-foreground">Environment</p>
        <Button variant="ghost" size="sm" onClick={() => setRevealed((r) => !r)}>
          {revealed ? <EyeOff className="h-3.5 w-3.5" aria-hidden /> : <Eye className="h-3.5 w-3.5" aria-hidden />}
          {revealed ? 'Hide secrets' : 'Reveal secrets'}
        </Button>
      </div>
      <ul className="space-y-1 text-[11px] font-mono">
        {env.map((e, i) => {
          const eq = e.indexOf('=')
          if (eq < 0) return <li key={i}>{e}</li>
          const key = e.slice(0, eq)
          const val = e.slice(eq + 1)
          const sensitive = SENSITIVE_KEY.test(key)
          return (
            <li key={i} className="break-all">
              <span className="text-muted-foreground">{key}=</span>
              {sensitive && !revealed ? <span className="text-amber-500">••••••••</span> : <span>{val}</span>}
            </li>
          )
        })}
      </ul>
    </div>
  )
}
