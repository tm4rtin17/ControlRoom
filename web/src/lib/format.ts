// Display formatters used across the dashboard tiles.

export function formatBytes(n: number, fixed = 1): string {
  if (n < 1024) return `${n} B`
  const units = ['KiB', 'MiB', 'GiB', 'TiB', 'PiB']
  let v = n / 1024
  let i = 0
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(fixed)} ${units[i]}`
}

export function formatRate(bytesPerSec: number): string {
  return `${formatBytes(bytesPerSec, 1)}/s`
}

export function formatPercent(p: number, fixed = 0): string {
  return `${p.toFixed(fixed)}%`
}

export function formatUptime(seconds: number): string {
  const days = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (days > 0) return `${days}d ${h}h ${m}m`
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
}

export function formatTemp(c: number): string {
  return `${c.toFixed(1)}°C`
}
