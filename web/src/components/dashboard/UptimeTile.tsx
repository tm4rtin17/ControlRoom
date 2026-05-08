import { Activity } from 'lucide-react'

import { Tile } from './Tile'
import { formatUptime } from '@/lib/format'
import type { SystemLoad } from '@/lib/system'

export function UptimeTile({
  uptimeSeconds,
  loadAvg,
}: {
  uptimeSeconds?: number
  loadAvg?: SystemLoad
}) {
  return (
    <Tile
      title="Uptime"
      Icon={Activity}
      value={uptimeSeconds !== undefined ? formatUptime(uptimeSeconds) : '—'}
      hint="Time since boot"
    >
      {loadAvg && (
        <div className="space-y-1 pt-2 text-xs">
          <div className="flex justify-between text-muted-foreground">
            <span>Load (1 / 5 / 15 min)</span>
          </div>
          <div className="flex justify-between font-mono tabular-nums">
            <span>{loadAvg.one.toFixed(2)}</span>
            <span>{loadAvg.five.toFixed(2)}</span>
            <span>{loadAvg.fifteen.toFixed(2)}</span>
          </div>
        </div>
      )}
    </Tile>
  )
}
