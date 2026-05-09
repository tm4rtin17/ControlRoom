import type { K8sEvent } from '@/lib/k8s'
import { formatRelativeTime } from '@/lib/format'
import { Badge } from '@/components/ui/badge'

export function EventsTable({ events }: { events: K8sEvent[] }) {
  if (events.length === 0) {
    return <p className="text-xs text-muted-foreground">No events.</p>
  }
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-xs">
        <thead>
          <tr className="border-b text-left text-muted-foreground">
            <th className="py-1.5 pr-3 font-medium">Type</th>
            <th className="py-1.5 pr-3 font-medium">Reason</th>
            <th className="py-1.5 pr-3 font-medium">Source</th>
            <th className="py-1.5 pr-3 font-medium">Message</th>
            <th className="py-1.5 font-medium">Last</th>
          </tr>
        </thead>
        <tbody className="divide-y">
          {events.map((e, i) => (
            <tr key={i}>
              <td className="py-1.5 pr-3">
                <Badge variant={e.type === 'Warning' ? 'warn' : 'muted'}>{e.type}</Badge>
              </td>
              <td className="py-1.5 pr-3 font-mono">{e.reason}</td>
              <td className="py-1.5 pr-3 text-muted-foreground">{e.source}</td>
              <td className="py-1.5 pr-3 max-w-[220px] truncate text-muted-foreground" title={e.message}>
                {e.message}
              </td>
              <td className="py-1.5 text-muted-foreground">
                {e.last ? formatRelativeTime(e.last) : '—'}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
