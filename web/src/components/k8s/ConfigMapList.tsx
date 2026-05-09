import type { K8sConfigMap } from '@/lib/k8s'
import { formatRelativeTime } from '@/lib/format'
import { Card, CardContent } from '@/components/ui/card'
import { Alert, AlertDescription } from '@/components/ui/alert'

interface Props {
  configmaps: K8sConfigMap[]
  loading: boolean
  error: Error | null
  search: string
  onOpen?: (namespace: string, name: string) => void
}

export function ConfigMapList({ configmaps, loading, error, search, onOpen }: Props) {
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
        <AlertDescription>{error.message || 'Could not fetch configmaps.'}</AlertDescription>
      </Alert>
    )
  }

  const needle = search.toLowerCase()
  const filtered = needle
    ? configmaps.filter(
        (cm) =>
          cm.name.toLowerCase().includes(needle) ||
          cm.namespace.toLowerCase().includes(needle)
      )
    : configmaps

  if (filtered.length === 0) {
    return (
      <Card>
        <CardContent className="px-4 py-12 text-center text-sm text-muted-foreground">
          {configmaps.length === 0 ? 'No configmaps found' : 'No configmaps match'}
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
                <th className="px-4 py-3 font-medium">Keys</th>
                <th className="px-4 py-3 font-medium">Age</th>
              </tr>
            </thead>
            <tbody className="divide-y">
              {filtered.map((cm) => (
                <tr
                  key={`${cm.namespace}/${cm.name}`}
                  className={onOpen ? 'cursor-pointer hover:bg-accent/30' : 'hover:bg-accent/30'}
                  onClick={() => onOpen?.(cm.namespace, cm.name)}
                >
                  <td className="px-4 py-3 font-mono text-xs font-medium">{cm.name}</td>
                  <td className="px-4 py-3 text-xs text-muted-foreground">{cm.namespace}</td>
                  <td className="px-4 py-3 text-xs tabular-nums">{cm.keys.length}</td>
                  <td className="px-4 py-3 text-xs text-muted-foreground">{formatRelativeTime(cm.age)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </CardContent>
    </Card>
  )
}
