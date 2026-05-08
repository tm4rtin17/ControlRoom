// Types and queries for /api/system/*. The shape mirrors
// internal/collectors/types.go.

import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect } from 'react'

import { apiFetch } from './api'

export interface SystemHost {
  hostname: string
  distro: string
  kernel: string
  arch: string
}

export interface SystemLoad {
  one: number
  five: number
  fifteen: number
}

export interface SystemCPU {
  model: string
  freq_mhz: number
  cores: number
  overall_pct: number
  per_core_pct: number[]
}

export interface SystemMemory {
  total: number
  used: number
  free: number
  available: number
  cached: number
  buffers: number
  swap_total: number
  swap_used: number
}

export interface SystemDisk {
  mount: string
  filesystem: string
  total: number
  used: number
  free: number
}

export interface SystemTemp {
  name: string
  label: string
  celsius: number
}

export interface SystemNet {
  name: string
  rx_bytes: number
  tx_bytes: number
  rx_rate: number
  tx_rate: number
}

export interface SystemOverview {
  host: SystemHost
  uptime_seconds: number
  load_avg: SystemLoad
  cpu: SystemCPU
  memory: SystemMemory
  disks: SystemDisk[]
  temperatures: SystemTemp[]
  network: SystemNet[]
}

export const SYSTEM_OVERVIEW_KEY = ['system', 'overview'] as const

export function useSystemOverview() {
  // REST polling at 5s acts as a fallback when the WS isn't connected. Once
  // the WS push pipeline starts updating cache, polling stays in step but is
  // effectively idle.
  return useQuery<SystemOverview>({
    queryKey: SYSTEM_OVERVIEW_KEY,
    queryFn: () => apiFetch<SystemOverview>('/api/system/overview'),
    refetchInterval: 5_000,
    staleTime: 1_000,
  })
}

// useSystemStatsWS opens a WebSocket and pushes each frame into the
// SYSTEM_OVERVIEW_KEY cache. Reconnects with exponential backoff up to 30s.
// The REST polling (useSystemOverview) keeps tiles alive while the socket
// is mid-reconnect.
export function useSystemStatsWS(): void {
  const qc = useQueryClient()

  useEffect(() => {
    let stopped = false
    let attempts = 0
    let socket: WebSocket | null = null
    let reconnectTimer: number | null = null

    function connect() {
      if (stopped) return
      const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      const url = `${proto}//${window.location.host}/ws/system/stats`
      socket = new WebSocket(url)

      socket.onopen = () => {
        attempts = 0
      }
      socket.onmessage = (evt) => {
        try {
          const data = JSON.parse(evt.data) as SystemOverview
          qc.setQueryData(SYSTEM_OVERVIEW_KEY, data)
        } catch {
          // ignore malformed frames
        }
      }
      socket.onclose = () => {
        if (stopped) return
        attempts++
        const delay = Math.min(30_000, 500 * 2 ** Math.min(attempts, 6))
        reconnectTimer = window.setTimeout(connect, delay)
      }
      socket.onerror = () => {
        socket?.close()
      }
    }

    connect()
    return () => {
      stopped = true
      if (reconnectTimer !== null) window.clearTimeout(reconnectTimer)
      socket?.close()
    }
  }, [qc])
}
