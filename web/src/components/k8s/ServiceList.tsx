import type { K8sService } from '@/lib/k8s'
import { formatRelativeTime } from '@/lib/format'
import { Card, CardContent } from '@/components/ui/card'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { K8sServiceTypePill } from './K8sStatusPill'

interface Props {
  services: K8sService[]
  loading: boolean
  error: Error | null
  search: string
  onOpen?: (namespace: string, name: string) => void
}

function formatPorts(svc: K8sService): string {
  if (svc.ports.length === 0) return '—'
  return svc.ports
    .map((p) => `${p.port}→${p.target_port}/${p.protocol}`)
    .join(', ')
}

export function ServiceList({ services, loading, error, search, onOpen }: Props) {
  if (loading) {
    return (
      <Card>
        <CardContent className="px-4 py-12 text-center text-sm text-muted-foreground">Loading…</CardContent>
      </Card>
    )
  }
  if (error) {
    return (
      <Alert variant="destructive">
        <AlertDescription>{error.message || 'Could not fetch services.'}</AlertDescription>
      </Alert>
    )
  }

  const needle = search.toLowerCase()
  const filtered = needle
    ? services.filter(
        (s) =>
          s.name.toLowerCase().includes(needle) ||
          s.namespace.toLowerCase().includes(needle)
      )
    : services

  if (filtered.length === 0) {
    return (
      <Card>
        <CardContent className="px-4 py-12 text-center text-sm text-muted-foreground">
          {services.length === 0 ? 'No services found' : 'No services match'}
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardContent className="p-0">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b text-left text-xs text-muted-foreground">
                <th className="px-4 py-3 font-medium">Name</th>
                <th className="px-4 py-3 font-medium">Namespace</th>
                <th className="px-4 py-3 font-medium">Type</th>
                <th className="px-4 py-3 font-medium">Cluster IP</th>
                <th className="px-4 py-3 font-medium">External IP</th>
                <th className="px-4 py-3 font-medium">Ports</th>
                <th className="px-4 py-3 font-medium">Age</th>
              </tr>
            </thead>
            <tbody className="divide-y">
              {filtered.map((svc) => (
                <tr key={`${svc.namespace}/${svc.name}`} className={onOpen ? 'cursor-pointer hover:bg-accent/30' : 'hover:bg-accent/30'} onClick={() => onOpen?.(svc.namespace, svc.name)}>
                  <td className="px-4 py-3 font-mono text-xs font-medium">{svc.name}</td>
                  <td className="px-4 py-3 text-xs text-muted-foreground">{svc.namespace}</td>
                  <td className="px-4 py-3">
                    <K8sServiceTypePill type={svc.type} />
                  </td>
                  <td className="px-4 py-3 font-mono text-xs">{svc.cluster_ip || '—'}</td>
                  <td className="px-4 py-3 font-mono text-xs">{svc.external_ip || '—'}</td>
                  <td className="px-4 py-3 font-mono text-xs">{formatPorts(svc)}</td>
                  <td className="px-4 py-3 text-xs text-muted-foreground">{formatRelativeTime(svc.age)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </CardContent>
    </Card>
  )
}
