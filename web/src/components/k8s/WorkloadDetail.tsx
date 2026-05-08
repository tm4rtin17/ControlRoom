import { useK8sWorkloadDetail } from '@/lib/k8s'
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription } from '@/components/ui/sheet'
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import { ConditionsTable } from './ConditionsTable'
import { EventsTable } from './EventsTable'

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <p className="mb-2 text-xs uppercase tracking-wider text-muted-foreground">{title}</p>
      {children}
    </div>
  )
}

export function WorkloadDetail({
  namespace,
  kind,
  name,
  open,
  onOpenChange,
}: {
  namespace: string | null
  kind: string | null
  name: string | null
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { data } = useK8sWorkloadDetail(
    open ? namespace : null,
    open ? kind : null,
    open ? name : null
  )

  const w = data?.workload
  const pct = w && w.ready.desired > 0 ? (w.ready.current / w.ready.desired) * 100 : 0
  const healthy = w ? w.ready.current >= w.ready.desired : false

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="overflow-y-auto">
        <SheetHeader>
          <SheetTitle className="font-mono break-all">{w?.name ?? name}</SheetTitle>
          <SheetDescription>
            {w ? `${w.kind} · ${w.namespace}` : (kind && namespace ? `${kind} · ${namespace}` : '')}
          </SheetDescription>
        </SheetHeader>

        {data && w && (
          <div className="mt-4 flex flex-col gap-6">
            {/* Ready bar */}
            <div className="flex items-center gap-3">
              <span className={cn('text-sm tabular-nums font-medium', healthy ? 'text-emerald-500' : 'text-amber-500')}>
                {w.ready.current}/{w.ready.desired} ready
              </span>
              <div className="h-2 flex-1 overflow-hidden rounded-full bg-muted">
                <div
                  className={cn('h-full rounded-full', healthy ? 'bg-emerald-500' : 'bg-amber-500')}
                  style={{ width: `${Math.min(pct, 100)}%` }}
                />
              </div>
            </div>

            <Section title="Strategy">
              <span className="text-sm">{data.strategy || '—'}</span>
            </Section>

            {Object.keys(data.selector).length > 0 && (
              <Section title="Selector">
                <div className="flex flex-wrap gap-1.5">
                  {Object.entries(data.selector).map(([k, v]) => (
                    <Badge key={k} variant="muted">{k}={v}</Badge>
                  ))}
                </div>
              </Section>
            )}

            <Section title="Conditions">
              <ConditionsTable conditions={data.conditions} />
            </Section>

            <Section title="Events">
              <EventsTable events={data.events} />
            </Section>
          </div>
        )}
      </SheetContent>
    </Sheet>
  )
}
