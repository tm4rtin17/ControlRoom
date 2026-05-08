import { HardDrive } from 'lucide-react'

import { Bar, Tile } from './Tile'
import { formatBytes } from '@/lib/format'
import type { SystemDisk } from '@/lib/system'
import { cn } from '@/lib/utils'

export function DiskTile({ disks }: { disks?: SystemDisk[] }) {
  const list = disks ?? []
  const root = list.find((d) => d.mount === '/') ?? list[0]
  const headlinePct = root && root.total > 0 ? (root.used / root.total) * 100 : 0
  return (
    <Tile
      title="Disk"
      Icon={HardDrive}
      value={root ? `${headlinePct.toFixed(0)}%` : '—'}
      hint={root ? `${formatBytes(root.used)} / ${formatBytes(root.total)} on ${root.mount}` : '—'}
    >
      {list.length > 1 && (
        <div className="space-y-2 pt-2">
          {list.slice(0, 4).map((d) => {
            const pct = d.total > 0 ? (d.used / d.total) * 100 : 0
            return (
              <div key={d.mount}>
                <div className="mb-1 flex justify-between text-xs">
                  <span className="truncate font-mono text-muted-foreground">{d.mount}</span>
                  <span className="text-muted-foreground">{formatBytes(d.used)} / {formatBytes(d.total)}</span>
                </div>
                <Bar
                  pct={pct}
                  color={cn(pct >= 90 ? 'bg-destructive' : pct >= 75 ? 'bg-amber-500' : 'bg-primary')}
                />
              </div>
            )
          })}
        </div>
      )}
    </Tile>
  )
}
