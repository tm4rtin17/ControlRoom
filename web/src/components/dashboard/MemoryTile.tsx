import { MemoryStick } from 'lucide-react'

import { Bar, Tile } from './Tile'
import { formatBytes } from '@/lib/format'
import type { SystemMemory } from '@/lib/system'

export function MemoryTile({ memory }: { memory?: SystemMemory }) {
  const total = memory?.total ?? 0
  const used = memory?.used ?? 0
  const pct = total > 0 ? (used / total) * 100 : 0

  return (
    <Tile
      title="Memory"
      Icon={MemoryStick}
      value={formatBytes(used)}
      hint={memory ? `of ${formatBytes(total)} · cache ${formatBytes(memory.cached)}` : '—'}
    >
      <Bar pct={pct} color={pct >= 90 ? 'bg-destructive' : pct >= 75 ? 'bg-amber-500' : 'bg-primary'} />
      {memory && memory.swap_total > 0 && (
        <div className="pt-2">
          <div className="mb-1 flex justify-between text-xs text-muted-foreground">
            <span>Swap</span>
            <span>
              {formatBytes(memory.swap_used)} / {formatBytes(memory.swap_total)}
            </span>
          </div>
          <Bar
            pct={(memory.swap_used / memory.swap_total) * 100}
            color="bg-amber-500/70"
          />
        </div>
      )}
    </Tile>
  )
}
