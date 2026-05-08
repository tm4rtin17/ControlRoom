import { useQuery } from '@tanstack/react-query'

import { apiFetch } from './api'

export interface LogEntry {
  timestamp: string
  priority: number
  unit?: string
  identifier?: string
  pid?: number
  hostname?: string
  message: string
}

export interface LogQuery {
  unit?: string
  priority?: number
  since?: string
  until?: string
  q?: string
  n?: number
}

function toParams(q: LogQuery): string {
  const params = new URLSearchParams()
  if (q.unit) params.set('unit', q.unit)
  if (q.priority !== undefined && q.priority >= 0) params.set('priority', String(q.priority))
  if (q.since) params.set('since', q.since)
  if (q.until) params.set('until', q.until)
  if (q.q) params.set('q', q.q)
  if (q.n) params.set('n', String(q.n))
  const s = params.toString()
  return s ? `?${s}` : ''
}

export function useLogs(q: LogQuery, enabled = true) {
  return useQuery({
    queryKey: ['logs', q],
    queryFn: () => apiFetch<{ entries: LogEntry[] }>(`/api/logs/journal${toParams(q)}`),
    enabled,
    staleTime: 5_000,
  })
}

// tailURL returns the WS URL for live tail with the same filter as a static
// query.
export function tailURL(q: LogQuery): string {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${window.location.host}/ws/logs/journal${toParams(q)}`
}
