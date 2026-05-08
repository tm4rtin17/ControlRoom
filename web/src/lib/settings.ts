import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { apiFetch } from './api'
import { ME_QUERY_KEY } from './auth'

export interface SettingsResponse {
  server: {
    addr: string
    host_name?: string
    log_level: string
    dev_mode: boolean
    trust_proxy: boolean
    version_check: boolean
    session_hours: number
  }
  tls: {
    mode: string
    acme_host?: string
  }
  build: {
    version: string
    commit: string
    date: string
  }
}

export const SETTINGS_KEY = ['settings'] as const

export function useSettings() {
  return useQuery<SettingsResponse>({
    queryKey: SETTINGS_KEY,
    queryFn: () => apiFetch<SettingsResponse>('/api/settings/'),
    staleTime: 60_000,
  })
}

export function useChangePassword() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (req: { current: string; new: string }) =>
      apiFetch<{ ok: boolean }>('/api/settings/password', {
        method: 'POST',
        body: JSON.stringify(req),
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ME_QUERY_KEY }),
  })
}
