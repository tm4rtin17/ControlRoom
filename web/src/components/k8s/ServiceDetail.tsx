import { useK8sServiceDetail } from '@/lib/k8s'
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription } from '@/components/ui/sheet'
import { Badge } from '@/components/ui/badge'
import { K8sServiceTypePill } from './K8sStatusPill'
import { EventsTable } from './EventsTable'

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <p className="mb-2 text-xs uppercase tracking-wider text-muted-foreground">{title}</p>
      {children}
    </div>
  )
}

export function ServiceDetail({
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
  const { data } = useK8sServiceDetail(open ? namespace : null, open ? name : null)
  const svc = data?.service

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="overflow-y-auto">
        <SheetHeader>
          <SheetTitle className="font-mono break-all">{svc?.name ?? name}</SheetTitle>
          <SheetDescription>
            {svc ? `${svc.namespace} · ${svc.cluster_ip || '—'}` : (namespace ?? '')}
          </SheetDescription>
        </SheetHeader>

        {data && svc && (
          <div className="mt-4 flex flex-col gap-6">
            <div className="flex items-center gap-2">
              <K8sServiceTypePill type={svc.type} />
              <span className="font-mono text-xs text-muted-foreground">{svc.cluster_ip}</span>
            </div>

            {svc.ports.length > 0 && (
              <Section title="Ports">
                <div className="overflow-x-auto">
                  <table className="w-full text-xs">
                    <thead>
                      <tr className="border-b text-left text-muted-foreground">
                        <th className="py-1.5 pr-3 font-medium">Name</th>
                        <th className="py-1.5 pr-3 font-medium">Port</th>
                        <th className="py-1.5 pr-3 font-medium">Target</th>
                        <th className="py-1.5 pr-3 font-medium">Protocol</th>
                        <th className="py-1.5 font-medium">NodePort</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y">
                      {svc.ports.map((p, i) => (
                        <tr key={i}>
                          <td className="py-1.5 pr-3 font-mono">{p.name || '—'}</td>
                          <td className="py-1.5 pr-3 tabular-nums">{p.port}</td>
                          <td className="py-1.5 pr-3 tabular-nums">{p.target_port}</td>
                          <td className="py-1.5 pr-3 text-muted-foreground">{p.protocol}</td>
                          <td className="py-1.5 text-muted-foreground">{p.node_port ?? '—'}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </Section>
            )}

            <Section title="Endpoints">
              {data.endpoints.length === 0 ? (
                <p className="text-xs text-muted-foreground">No endpoints.</p>
              ) : (
                <div className="overflow-x-auto">
                  <table className="w-full text-xs">
                    <thead>
                      <tr className="border-b text-left text-muted-foreground">
                        <th className="py-1.5 pr-3 font-medium">IP</th>
                        <th className="py-1.5 pr-3 font-medium">Node</th>
                        <th className="py-1.5 font-medium">Ready</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y">
                      {data.endpoints.map((ep, i) => (
                        <tr key={i}>
                          <td className="py-1.5 pr-3 font-mono">{ep.ip}</td>
                          <td className="py-1.5 pr-3 font-mono text-muted-foreground">{ep.node_name || '—'}</td>
                          <td className="py-1.5">
                            <Badge variant={ep.ready ? 'success' : 'danger'}>{ep.ready ? 'Yes' : 'No'}</Badge>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
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

            <Section title="Events">
              <EventsTable events={data.events} />
            </Section>
          </div>
        )}
      </SheetContent>
    </Sheet>
  )
}
