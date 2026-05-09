import type { K8sSecret } from '@/lib/k8s'
import { formatRelativeTime } from '@/lib/format'
import { Card, CardContent } from '@/components/ui/card'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'

function typeLabel(t: string): string {
  const prefix = 'kubernetes.io/'
  if (t.startsWith(prefix)) return t.slice(prefix.length)
  return t
}

interface Props {
  secrets: K8sSecret[]
  loading: boolean
  error: Error | null
  search: string
  onOpen?: (namespace: string, name: string) => void
}

export function SecretList({ secrets, loading, error, search, onOpen }: Props) {
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
        <AlertDescription>{error.message || 'Could not fetch secrets.'}</AlertDescription>
      </Alert>
    )
  }

  const needle = search.toLowerCase()
  const filtered = needle
    ? secrets.filter(
        (s) =>
          s.name.toLowerCase().includes(needle) ||
          s.namespace.toLowerCase().includes(needle)
      )
    : secrets

  if (filtered.length === 0) {
    return (
      <Card>
        <CardContent className="px-4 py-12 text-center text-sm text-muted-foreground">
          {secrets.length === 0 ? 'No secrets found' : 'No secrets match'}
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
                <th className="px-4 py-3 font-medium">Keys</th>
                <th className="px-4 py-3 font-medium">Age</th>
              </tr>
            </thead>
            <tbody className="divide-y">
              {filtered.map((s) => (
                <tr
                  key={`${s.namespace}/${s.name}`}
                  className={onOpen ? 'cursor-pointer hover:bg-accent/30' : 'hover:bg-accent/30'}
                  onClick={() => onOpen?.(s.namespace, s.name)}
                >
                  <td className="px-4 py-3 font-mono text-xs font-medium">{s.name}</td>
                  <td className="px-4 py-3 text-xs text-muted-foreground">{s.namespace}</td>
                  <td className="px-4 py-3 text-xs">
                    <Badge variant="muted">{typeLabel(s.type)}</Badge>
                  </td>
                  <td className="px-4 py-3 text-xs tabular-nums">{s.keys.length}</td>
                  <td className="px-4 py-3 text-xs text-muted-foreground">{formatRelativeTime(s.age)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </CardContent>
    </Card>
  )
}
