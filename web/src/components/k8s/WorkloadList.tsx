import type { K8sWorkload } from '@/lib/k8s'
import { formatRelativeTime } from '@/lib/format'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { cn } from '@/lib/utils'

interface Props {
  workloads: K8sWorkload[]
  loading: boolean
  error: Error | null
  search: string
  onOpen?: (namespace: string, kind: string, name: string) => void
}

const KIND_ORDER: K8sWorkload['kind'][] = ['Deployment', 'StatefulSet', 'DaemonSet']

const KIND_VARIANT: Record<K8sWorkload['kind'], 'default' | 'warn' | 'muted'> = {
  Deployment: 'default',
  StatefulSet: 'warn',
  DaemonSet: 'muted',
}

export function WorkloadList({ workloads, loading, error, search, onOpen }: Props) {
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
        <AlertDescription>{error.message || 'Could not fetch workloads.'}</AlertDescription>
      </Alert>
    )
  }

  const needle = search.toLowerCase()
  const filtered = needle
    ? workloads.filter(
        (w) =>
          w.name.toLowerCase().includes(needle) ||
          w.namespace.toLowerCase().includes(needle)
      )
    : workloads

  if (filtered.length === 0) {
    return (
      <Card>
        <CardContent className="px-4 py-12 text-center text-sm text-muted-foreground">
          {workloads.length === 0 ? 'No workloads found' : 'No workloads match'}
        </CardContent>
      </Card>
    )
  }

  const byKind = KIND_ORDER.map((kind) => ({
    kind,
    items: filtered.filter((w) => w.kind === kind),
  })).filter((g) => g.items.length > 0)

  return (
    <div className="flex flex-col gap-4">
      {byKind.map(({ kind, items }) => (
        <section key={kind}>
          <div className="mb-2 flex items-center gap-2">
            <Badge variant={KIND_VARIANT[kind]}>{kind}</Badge>
            <span className="text-xs text-muted-foreground">{items.length}</span>
          </div>
          <Card>
            <CardContent className="p-0">
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b text-left text-xs text-muted-foreground">
                      <th className="px-4 py-3 font-medium">Name</th>
                      <th className="px-4 py-3 font-medium">Namespace</th>
                      <th className="px-4 py-3 font-medium">Ready</th>
                      <th className="px-4 py-3 font-medium">Image</th>
                      <th className="px-4 py-3 font-medium">Age</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y">
                    {items.map((w) => {
                      const pct = w.ready.desired > 0 ? (w.ready.current / w.ready.desired) * 100 : 0
                      const healthy = w.ready.current >= w.ready.desired
                      return (
                        <tr key={`${w.namespace}/${w.name}`} className={onOpen ? 'cursor-pointer hover:bg-accent/30' : 'hover:bg-accent/30'} onClick={() => onOpen?.(w.namespace, w.kind, w.name)}>
                          <td className="px-4 py-3 font-mono text-xs font-medium">{w.name}</td>
                          <td className="px-4 py-3 text-xs text-muted-foreground">{w.namespace}</td>
                          <td className="px-4 py-3">
                            <div className="flex items-center gap-2">
                              <span className={cn('text-xs tabular-nums', healthy ? 'text-emerald-500' : 'text-amber-500')}>
                                {w.ready.current}/{w.ready.desired}
                              </span>
                              <div className="h-1.5 w-16 overflow-hidden rounded-full bg-muted">
                                <div
                                  className={cn('h-full rounded-full', healthy ? 'bg-emerald-500' : 'bg-amber-500')}
                                  style={{ width: `${Math.min(pct, 100)}%` }}
                                />
                              </div>
                            </div>
                          </td>
                          <td className="px-4 py-3 font-mono text-xs">
                            {w.images.length === 0 ? '—' : (
                              <>
                                <span className="truncate">{w.images[0]}</span>
                                {w.images.length > 1 && (
                                  <span className="ml-1 text-muted-foreground">+{w.images.length - 1} more</span>
                                )}
                              </>
                            )}
                          </td>
                          <td className="px-4 py-3 text-xs text-muted-foreground">{formatRelativeTime(w.age)}</td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              </div>
            </CardContent>
          </Card>
        </section>
      ))}
    </div>
  )
}
