import { Cpu } from 'lucide-react'

import { Bar, Tile } from './Tile'
import { formatPercent } from '@/lib/format'
import type { SystemCPU } from '@/lib/system'
import { cn } from '@/lib/utils'

export function CpuTile({ cpu }: { cpu?: SystemCPU }) {
  const overall = cpu?.overall_pct ?? 0
  return (
    <Tile
      title="CPU"
      Icon={Cpu}
      value={formatPercent(overall)}
      hint={cpu ? `${cpu.cores} cores · ${shortenModel(cpu.model)}` : '—'}
    >
      <Bar pct={overall} color={cpuColor(overall)} />
      {cpu?.per_core_pct?.length ? (
        <div className="grid grid-cols-4 gap-1.5 pt-2 sm:grid-cols-6">
          {cpu.per_core_pct.map((p, i) => (
            <CoreBar key={i} pct={p} index={i} />
          ))}
        </div>
      ) : null}
    </Tile>
  )
}

function CoreBar({ pct, index }: { pct: number; index: number }) {
  return (
    <div title={`Core ${index}: ${formatPercent(pct)}`} className="flex h-8 items-end overflow-hidden rounded-sm bg-muted">
      <div
        className={cn('w-full transition-all', cpuColor(pct))}
        style={{ height: `${Math.min(100, Math.max(2, pct))}%` }}
      />
    </div>
  )
}

function cpuColor(pct: number): string {
  if (pct >= 90) return 'bg-destructive'
  if (pct >= 70) return 'bg-amber-500'
  return 'bg-primary'
}

function shortenModel(model?: string): string {
  if (!model) return ''
  return model
    .replace(/\(R\)/g, '')
    .replace(/\(TM\)/g, '')
    .replace(/CPU @.*$/, '')
    .replace(/\s+/g, ' ')
    .trim()
}
