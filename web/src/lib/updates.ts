import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useState } from 'react'

import { apiFetch } from './api'

export interface UpdatePackage {
  name: string
  source: string
  new_version: string
  old_version: string
  arch: string
  security: boolean
}

export interface UpdatesListResponse {
  packages: UpdatePackage[]
  reboot_required: boolean
}

export type JobState = 'running' | 'succeeded' | 'failed' | 'cancelled' | 'not_found'

export interface JobStatus {
  id: string
  action: string
  state: JobState
  started_at: string
  finished_at?: string
  error?: string
  output: string
}

export const UPDATES_LIST_KEY = ['updates', 'list'] as const

export function useUpdatesList() {
  return useQuery<UpdatesListResponse>({
    queryKey: UPDATES_LIST_KEY,
    queryFn: () => apiFetch<UpdatesListResponse>('/api/updates/list'),
    staleTime: 30_000,
  })
}

export function useStartCheck() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => apiFetch<{ job_id: string }>('/api/updates/check', { method: 'POST' }),
    onSuccess: () => {
      // Refresh package list once the job finishes (the page also refetches
      // when the WS reports state=succeeded).
      qc.invalidateQueries({ queryKey: UPDATES_LIST_KEY })
    },
  })
}

export function useStartApply() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => apiFetch<{ job_id: string }>('/api/updates/apply', { method: 'POST' }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: UPDATES_LIST_KEY })
    },
  })
}

export function useReboot() {
  return useMutation({
    mutationFn: () =>
      apiFetch<{ job_id: string }>('/api/system/reboot', {
        method: 'POST',
        body: JSON.stringify({ confirm: 'REBOOT' }),
      }),
  })
}

interface JobFrame {
  type: 'snapshot' | 'chunk' | 'state'
  output?: string
  state?: JobState
}

// useJobStream returns the live state + accumulated output for a job. It first
// hits GET /jobs/:id for the snapshot (so the page renders even before the WS
// connects), then opens the WS for live updates.
export function useJobStream(jobId: string | null): {
  state: JobState | null
  output: string
  error: string | null
} {
  const [output, setOutput] = useState('')
  const [state, setState] = useState<JobState | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!jobId) {
      setOutput('')
      setState(null)
      setError(null)
      return
    }

    let cancelled = false
    setOutput('')
    setState('running')
    setError(null)

    // 1) Initial snapshot via REST.
    apiFetch<JobStatus>(`/api/updates/jobs/${jobId}`)
      .then((j) => {
        if (cancelled) return
        setOutput(j.output ?? '')
        setState(j.state)
        if (j.error) setError(j.error)
      })
      .catch(() => {
        if (!cancelled) setError('failed to load job')
      })

    // 2) Live tail via WS.
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const ws = new WebSocket(`${proto}//${window.location.host}/ws/updates/jobs/${jobId}`)
    ws.onmessage = (evt) => {
      try {
        const f = JSON.parse(evt.data) as JobFrame
        if (f.type === 'snapshot' && f.output !== undefined) {
          setOutput(f.output)
        } else if (f.type === 'chunk' && f.output) {
          setOutput((prev) => prev + f.output)
        } else if (f.type === 'state' && f.state) {
          setState(f.state)
        }
      } catch {
        // ignore
      }
    }
    ws.onerror = () => {
      if (!cancelled) setError('connection error')
    }

    return () => {
      cancelled = true
      ws.close()
    }
  }, [jobId])

  return { state, output, error }
}
