import { useK8sNodeDetail } from '@/lib/k8s'
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription } from '@/components/ui/sheet'
import { Badge } from '@/components/ui/badge'
import { K8sNodeStatusPill } from './K8sStatusPill'
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

export function NodeDetail({
  name,
  open,
  onOpenChange,
}: {
  name: string | null
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { data } = useK8sNodeDetail(open ? name : null)

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="overflow-y-auto">
        <SheetHeader>
          <SheetTitle className="font-mono break-all">{data?.node.name ?? name}</SheetTitle>
          <SheetDescription>
            {data ? `${data.node.os} / ${data.node.arch} · ${data.node.version}` : ''}
          </SheetDescription>
        </SheetHeader>

        {data && (
          <div className="mt-4 flex flex-col gap-6">
            <div className="flex items-center gap-2">
              <K8sNodeStatusPill status={data.node.status} />
            </div>

            {/* Capacity / Allocatable */}
            <Section title="Capacity / Allocatable">
              <div className="grid grid-cols-3 gap-3 text-xs">
                {(['cpu', 'memory', 'pods'] as const).map((k) => (
                  <div key={k}>
                    <div className="uppercase tracking-wider text-muted-foreground">{k}</div>
                    <div className="font-mono">{data.node.capacity[k]}</div>
                    <div className="font-mono text-muted-foreground">{data.allocatable[k]}</div>
                  </div>
                ))}
              </div>
              <p className="mt-1 text-[10px] text-muted-foreground">capacity / allocatable</p>
            </Section>

            <Section title="Conditions">
              <ConditionsTable conditions={data.conditions} />
            </Section>

            {data.taints.length > 0 && (
              <Section title="Taints">
                <div className="flex flex-wrap gap-1.5">
                  {data.taints.map((t, i) => (
                    <Badge key={i} variant="warn">
                      {t.key}{t.value ? `=${t.value}` : ''}:{t.effect}
                    </Badge>
                  ))}
                </div>
              </Section>
            )}

            <Section title="Events">
              <EventsTable events={data.events} />
            </Section>
          </div>
        )}
      </SheetContent>
    </Sheet>
  )
}
