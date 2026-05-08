import { ArrowDown, ArrowUp, Wifi } from 'lucide-react'

import { Tile } from './Tile'
import { formatRate } from '@/lib/format'
import type { SystemNet } from '@/lib/system'

export function NetworkTile({ network }: { network?: SystemNet[] }) {
  const list = network ?? []
  const totalRx = list.reduce((s, n) => s + n.rx_rate, 0)
  const totalTx = list.reduce((s, n) => s + n.tx_rate, 0)
  return (
    <Tile
      title="Network"
      Icon={Wifi}
      value={`${formatRate(totalRx + totalTx)}`}
      hint={list.length > 0 ? `${list.length} interface${list.length === 1 ? '' : 's'}` : '—'}
    >
      <div className="grid grid-cols-2 gap-3 pt-1 text-xs text-muted-foreground">
        <div className="flex items-center gap-1">
          <ArrowDown className="h-3 w-3 text-emerald-500" aria-hidden />
          <span className="font-mono tabular-nums">{formatRate(totalRx)}</span>
        </div>
        <div className="flex items-center gap-1">
          <ArrowUp className="h-3 w-3 text-sky-500" aria-hidden />
          <span className="font-mono tabular-nums">{formatRate(totalTx)}</span>
        </div>
      </div>
      {list.length > 0 && (
        <ul className="space-y-1 pt-2 text-xs">
          {list.slice(0, 3).map((n) => (
            <li key={n.name} className="flex items-center justify-between gap-2">
              <span className="truncate font-mono">{n.name}</span>
              <span className="text-muted-foreground tabular-nums">
                ↓ {formatRate(n.rx_rate)} · ↑ {formatRate(n.tx_rate)}
              </span>
            </li>
          ))}
        </ul>
      )}
    </Tile>
  )
}
