import { Thermometer } from 'lucide-react'

import { Tile } from './Tile'
import { formatTemp } from '@/lib/format'
import type { SystemTemp } from '@/lib/system'
import { cn } from '@/lib/utils'

export function TemperatureTile({ temps }: { temps?: SystemTemp[] }) {
  const list = temps ?? []
  const max = list.length > 0 ? list[0] : null
  return (
    <Tile
      title="Temperature"
      Icon={Thermometer}
      value={max ? formatTemp(max.celsius) : '—'}
      hint={max ? max.label : 'No sensors'}
    >
      {list.length > 1 && (
        <ul className="space-y-1 pt-2 text-xs text-muted-foreground">
          {list.slice(0, 4).map((t) => (
            <li key={t.name} className="flex justify-between gap-2">
              <span className="truncate">{t.label}</span>
              <span className={cn('tabular-nums', tempColor(t.celsius))}>
                {formatTemp(t.celsius)}
              </span>
            </li>
          ))}
        </ul>
      )}
    </Tile>
  )
}

function tempColor(c: number): string {
  if (c >= 80) return 'text-destructive'
  if (c >= 65) return 'text-amber-500'
  return ''
}
