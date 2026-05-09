import type { K8sCondition } from '@/lib/k8s'
import { formatRelativeTime } from '@/lib/format'
import { Badge } from '@/components/ui/badge'

const STATUS_VARIANT: Record<string, 'success' | 'danger' | 'muted'> = {
  True: 'success',
  False: 'danger',
  Unknown: 'muted',
}

export function ConditionsTable({ conditions }: { conditions: K8sCondition[] }) {
  if (conditions.length === 0) {
    return <p className="text-xs text-muted-foreground">No conditions.</p>
  }
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-xs">
        <thead>
          <tr className="border-b text-left text-muted-foreground">
            <th className="py-1.5 pr-3 font-medium">Type</th>
            <th className="py-1.5 pr-3 font-medium">Status</th>
            <th className="py-1.5 pr-3 font-medium">Reason</th>
            <th className="py-1.5 pr-3 font-medium">Message</th>
            <th className="py-1.5 font-medium">Last Transition</th>
          </tr>
        </thead>
        <tbody className="divide-y">
          {conditions.map((c) => (
            <tr key={c.type}>
              <td className="py-1.5 pr-3 font-mono">{c.type}</td>
              <td className="py-1.5 pr-3">
                <Badge variant={STATUS_VARIANT[c.status] ?? 'muted'}>{c.status}</Badge>
              </td>
              <td className="py-1.5 pr-3 text-muted-foreground">{c.reason || '—'}</td>
              <td className="py-1.5 pr-3 max-w-[200px] truncate text-muted-foreground" title={c.message}>
                {c.message || '—'}
              </td>
              <td className="py-1.5 text-muted-foreground">
                {c.last_transition_time ? formatRelativeTime(c.last_transition_time) : '—'}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
