import { useEffect, useState } from 'react'
import { MoreVertical } from 'lucide-react'

import type { ContainerStats, ContainerSummary } from '@/lib/containers'
import { useContainerAction, useContainerRemove } from '@/lib/containers'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { ContainerStatusBadge } from './StatusBadge'
import { formatBytes, formatPercent } from '@/lib/format'

export function ContainerCard({
  container,
  onOpen,
}: {
  container: ContainerSummary
  onOpen: () => void
}) {
  const isRunning = container.state === 'running'
  const stats = useContainerLiveStats(container.id, isRunning)

  return (
    <Card className="flex flex-col">
      <CardHeader className="flex-row items-start justify-between gap-2 space-y-0 pb-2">
        <button onClick={onOpen} className="min-w-0 text-left">
          <div className="flex items-center gap-2">
            <span className="truncate font-mono text-sm">{container.name}</span>
            <ContainerStatusBadge state={container.state} />
          </div>
          <p className="truncate text-xs text-muted-foreground">{container.image}</p>
        </button>
        <RowMenu container={container} />
      </CardHeader>
      <CardContent className="flex flex-1 flex-col gap-2 pt-0">
        {isRunning && (
          <div className="grid grid-cols-2 gap-2 text-xs text-muted-foreground">
            <Stat label="CPU" value={stats ? formatPercent(stats.cpu_pct, 1) : '—'} />
            <Stat
              label="MEM"
              value={
                stats
                  ? `${formatBytes(stats.mem_usage)}${stats.mem_limit ? ` / ${formatBytes(stats.mem_limit)}` : ''}`
                  : '—'
              }
            />
          </div>
        )}
        {container.ports.length > 0 && (
          <div className="flex flex-wrap gap-1.5">
            {container.ports.slice(0, 4).map((p, i) => (
              <span
                key={i}
                className="inline-flex items-center rounded border bg-muted/40 px-1.5 py-0.5 font-mono text-[10px]"
              >
                {p.public_port ? `${p.public_port}→${p.private_port}` : `${p.private_port}`}/{p.protocol}
              </span>
            ))}
            {container.ports.length > 4 && (
              <span className="text-[10px] text-muted-foreground">+{container.ports.length - 4} more</span>
            )}
          </div>
        )}
        <p className="text-[11px] text-muted-foreground">{container.status}</p>
      </CardContent>
    </Card>
  )
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <div className="text-[10px] uppercase tracking-wider">{label}</div>
      <div className="font-mono text-foreground tabular-nums">{value}</div>
    </div>
  )
}

function RowMenu({ container }: { container: ContainerSummary }) {
  const action = useContainerAction()
  const remove = useContainerRemove()
  const isRunning = container.state === 'running'
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" aria-label="Container actions">
          <MoreVertical className="h-4 w-4" aria-hidden />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        {!isRunning && (
          <DropdownMenuItem onSelect={() => action.mutate({ id: container.id, action: 'start' })}>
            Start
          </DropdownMenuItem>
        )}
        {isRunning && (
          <DropdownMenuItem
            destructive
            onSelect={() => {
              if (confirm(`Stop ${container.name}?`)) action.mutate({ id: container.id, action: 'stop' })
            }}
          >
            Stop
          </DropdownMenuItem>
        )}
        <DropdownMenuItem
          onSelect={() => {
            if (confirm(`Restart ${container.name}?`)) action.mutate({ id: container.id, action: 'restart' })
          }}
        >
          Restart
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem
          destructive
          onSelect={() => {
            if (confirm(`Delete ${container.name}? This cannot be undone.`)) {
              remove.mutate({ id: container.id, force: isRunning })
            }
          }}
        >
          Delete
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

// Local-state subscription so the card shows live numbers without dragging
// the whole table through React Query.
function useContainerLiveStats(id: string, enabled: boolean): ContainerStats | null {
  const [stats, setStats] = useState<ContainerStats | null>(null)
  useEffect(() => {
    if (!enabled) {
      setStats(null)
      return
    }
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const url = `${proto}//${window.location.host}/ws/containers/${id}/stats`
    const ws = new WebSocket(url)
    ws.onmessage = (evt) => {
      try {
        setStats(JSON.parse(evt.data) as ContainerStats)
      } catch {
        // ignore
      }
    }
    return () => ws.close()
  }, [id, enabled])
  return stats
}
