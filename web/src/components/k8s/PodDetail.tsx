import { useState } from 'react'

import { useK8sPodDetail } from '@/lib/k8s'
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription } from '@/components/ui/sheet'
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import { K8sPodStatusPill } from './K8sStatusPill'
import { ConditionsTable } from './ConditionsTable'
import { EventsTable } from './EventsTable'
import { PodLogStream } from './PodLogStream'

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <p className="mb-2 text-xs uppercase tracking-wider text-muted-foreground">{title}</p>
      {children}
    </div>
  )
}

export function PodDetail({
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
  const { data } = useK8sPodDetail(open ? namespace : null, open ? name : null)
  const [activeContainer, setActiveContainer] = useState<string | null>(null)

  const containers = data?.containers ?? []
  // Resolve active container: use state if valid, else first container
  const resolvedContainer =
    (activeContainer && containers.find((c) => c.name === activeContainer)?.name) ||
    containers[0]?.name ||
    null

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="overflow-y-auto">
        <SheetHeader>
          <SheetTitle className="font-mono break-all">{data?.pod.name ?? name}</SheetTitle>
          <SheetDescription>
            {data ? `${data.pod.namespace} · ${data.node_name || '—'} · ${data.pod.pod_ip || '—'} · ${data.qos_class}` : (namespace ?? '')}
          </SheetDescription>
        </SheetHeader>

        {data && (
          <div className="mt-4 flex flex-col gap-6">
            <div className="flex items-center gap-2">
              <K8sPodStatusPill status={data.pod.status} />
            </div>

            <Section title="Containers">
              <div className="overflow-x-auto">
                <table className="w-full text-xs">
                  <thead>
                    <tr className="border-b text-left text-muted-foreground">
                      <th className="py-1.5 pr-3 font-medium">Name</th>
                      <th className="py-1.5 pr-3 font-medium">Image</th>
                      <th className="py-1.5 pr-3 font-medium">Ready</th>
                      <th className="py-1.5 pr-3 font-medium">Restarts</th>
                      <th className="py-1.5 pr-3 font-medium">State</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y">
                    {containers.map((c) => (
                      <tr
                        key={c.name}
                        onClick={() => setActiveContainer(c.name)}
                        className={cn(
                          'cursor-pointer hover:bg-accent/30',
                          resolvedContainer === c.name && 'bg-accent/50'
                        )}
                      >
                        <td className="py-1.5 pr-3 font-mono font-medium">{c.name}</td>
                        <td className="py-1.5 pr-3 font-mono text-muted-foreground max-w-[160px] truncate" title={c.image}>
                          {c.image}
                        </td>
                        <td className="py-1.5 pr-3">
                          <Badge variant={c.ready ? 'success' : 'danger'}>{c.ready ? 'Yes' : 'No'}</Badge>
                        </td>
                        <td className="py-1.5 pr-3">
                          <span className={cn('tabular-nums', c.restart_count > 0 && 'text-amber-500')}>
                            {c.restart_count}
                          </span>
                        </td>
                        <td className="py-1.5 pr-3">
                          <Badge variant={c.state === 'running' ? 'success' : c.state === 'terminated' ? 'muted' : 'warn'}>
                            {c.state}
                          </Badge>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </Section>

            <Section title="Conditions">
              <ConditionsTable conditions={data.conditions} />
            </Section>

            <Section title="Events">
              <EventsTable events={data.events} />
            </Section>

            {namespace && name && resolvedContainer && (
              <Section title="Logs">
                {/* container picker */}
                {containers.length > 1 && (
                  <div className="mb-2 flex flex-wrap gap-1.5">
                    {containers.map((c) => (
                      <button
                        key={c.name}
                        onClick={() => setActiveContainer(c.name)}
                        className={cn(
                          'rounded-full border px-2 py-0.5 text-xs transition-colors',
                          resolvedContainer === c.name
                            ? 'border-primary/30 bg-primary/10 text-foreground'
                            : 'border-transparent bg-muted text-muted-foreground hover:text-foreground'
                        )}
                      >
                        {c.name}
                      </button>
                    ))}
                  </div>
                )}
                <PodLogStream
                  namespace={namespace}
                  podName={name}
                  container={resolvedContainer}
                />
              </Section>
            )}
          </div>
        )}
      </SheetContent>
    </Sheet>
  )
}
