import type { K8sNode } from '@/lib/k8s'
import { formatRelativeTime } from '@/lib/format'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { K8sNodeStatusPill } from './K8sStatusPill'

interface Props {
  nodes: K8sNode[]
  loading: boolean
  error: Error | null
  search: string
  onOpen?: (name: string) => void
}

export function NodeList({ nodes, loading, error, search, onOpen }: Props) {
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
        <AlertDescription>{error.message || 'Could not fetch nodes.'}</AlertDescription>
      </Alert>
    )
  }

  const needle = search.toLowerCase()
  const filtered = needle
    ? nodes.filter((n) => n.name.toLowerCase().includes(needle))
    : nodes

  if (filtered.length === 0) {
    return (
      <Card>
        <CardContent className="px-4 py-12 text-center text-sm text-muted-foreground">
          {nodes.length === 0 ? 'No nodes found' : 'No nodes match'}
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
                <th className="px-4 py-3 font-medium">Status</th>
                <th className="px-4 py-3 font-medium">Roles</th>
                <th className="px-4 py-3 font-medium">Version</th>
                <th className="px-4 py-3 font-medium">OS/Arch</th>
                <th className="px-4 py-3 font-medium">Internal IP</th>
                <th className="px-4 py-3 font-medium">Age</th>
              </tr>
            </thead>
            <tbody className="divide-y">
              {filtered.map((node) => {
                const internalIP = node.addresses.find((a) => a.type === 'InternalIP')?.address ?? '—'
                return (
                  <tr key={node.name} className={onOpen ? 'cursor-pointer hover:bg-accent/30' : 'hover:bg-accent/30'} onClick={() => onOpen?.(node.name)}>
                    <td className="px-4 py-3 font-mono text-xs font-medium">{node.name}</td>
                    <td className="px-4 py-3">
                      <K8sNodeStatusPill status={node.status} />
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex flex-wrap gap-1">
                        {node.roles.length === 0 ? (
                          <span className="text-muted-foreground">—</span>
                        ) : (
                          node.roles.map((r) => <Badge key={r} variant="muted">{r}</Badge>)
                        )}
                      </div>
                    </td>
                    <td className="px-4 py-3 font-mono text-xs">{node.version}</td>
                    <td className="px-4 py-3 text-xs text-muted-foreground">{node.os}/{node.arch}</td>
                    <td className="px-4 py-3 font-mono text-xs">{internalIP}</td>
                    <td className="px-4 py-3 text-xs text-muted-foreground">{formatRelativeTime(node.age)}</td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      </CardContent>
    </Card>
  )
}
