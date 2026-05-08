import type { K8sPod } from '@/lib/k8s'
import { formatRelativeTime } from '@/lib/format'
import { Card, CardContent } from '@/components/ui/card'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { cn } from '@/lib/utils'
import { K8sPodStatusPill } from './K8sStatusPill'

interface Props {
  pods: K8sPod[]
  loading: boolean
  error: Error | null
  search: string
  onOpen?: (namespace: string, name: string) => void
}

export function PodList({ pods, loading, error, search, onOpen }: Props) {
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
        <AlertDescription>{error.message || 'Could not fetch pods.'}</AlertDescription>
      </Alert>
    )
  }

  const needle = search.toLowerCase()
  const filtered = needle
    ? pods.filter(
        (p) =>
          p.name.toLowerCase().includes(needle) ||
          p.namespace.toLowerCase().includes(needle)
      )
    : pods

  if (filtered.length === 0) {
    return (
      <Card>
        <CardContent className="px-4 py-12 text-center text-sm text-muted-foreground">
          {pods.length === 0 ? 'No pods found' : 'No pods match'}
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
                <th className="px-4 py-3 font-medium">Status</th>
                <th className="px-4 py-3 font-medium">Ready</th>
                <th className="px-4 py-3 font-medium">Restarts</th>
                <th className="px-4 py-3 font-medium">Node</th>
                <th className="px-4 py-3 font-medium">Age</th>
              </tr>
            </thead>
            <tbody className="divide-y">
              {filtered.map((pod) => (
                <tr key={`${pod.namespace}/${pod.name}`} className={onOpen ? 'cursor-pointer hover:bg-accent/30' : 'hover:bg-accent/30'} onClick={() => onOpen?.(pod.namespace, pod.name)}>
                  <td className="px-4 py-3 font-mono text-xs font-medium">{pod.name}</td>
                  <td className="px-4 py-3 text-xs text-muted-foreground">{pod.namespace}</td>
                  <td className="px-4 py-3">
                    <K8sPodStatusPill status={pod.status} />
                  </td>
                  <td className="px-4 py-3 text-xs tabular-nums">
                    {pod.ready.current}/{pod.ready.total}
                  </td>
                  <td className="px-4 py-3 text-xs tabular-nums">
                    <span className={cn(pod.restarts > 0 ? 'text-amber-500' : '')}>{pod.restarts}</span>
                  </td>
                  <td className="px-4 py-3 font-mono text-xs text-muted-foreground">{pod.node || '—'}</td>
                  <td className="px-4 py-3 text-xs text-muted-foreground">{formatRelativeTime(pod.age)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </CardContent>
    </Card>
  )
}
