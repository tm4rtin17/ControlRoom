import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { apiFetch } from './api'

export interface ServiceUnit {
  name: string
  description: string
  load_state: string
  active_state: string
  sub_state: string
  unit_file_state: string
}

export interface ServiceUnitDetail extends ServiceUnit {
  fragment_path: string
  following: string
  memory_current: number
  tasks_current: number
  triggers: string[]
  triggered_by: string[]
  documentation: string[]
}

export interface ServiceListResponse {
  units: ServiceUnit[]
}

export const SERVICES_KEY = ['services'] as const

export interface ServicesQueryArgs {
  search?: string
  type?: string
  state?: string
}

export function useServices(args: ServicesQueryArgs = {}) {
  return useQuery({
    queryKey: [...SERVICES_KEY, args],
    queryFn: () => {
      const params = new URLSearchParams()
      if (args.search) params.set('q', args.search)
      if (args.type) params.set('type', args.type)
      if (args.state) params.set('state', args.state)
      const qs = params.toString()
      return apiFetch<ServiceListResponse>(`/api/services/${qs ? `?${qs}` : ''}`)
    },
    refetchInterval: 5_000,
    staleTime: 1_000,
  })
}

export function useServiceDetail(name: string | null) {
  return useQuery({
    queryKey: ['services', 'detail', name],
    queryFn: () =>
      apiFetch<ServiceUnitDetail>(`/api/services/${encodeURIComponent(name ?? '')}`),
    enabled: !!name,
    refetchInterval: 5_000,
  })
}

type ServiceAction = 'start' | 'stop' | 'restart' | 'enable' | 'disable'

export function useServiceAction() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ name, action }: { name: string; action: ServiceAction }) =>
      apiFetch<{ ok: boolean }>(`/api/services/${encodeURIComponent(name)}/${action}`, {
        method: 'POST',
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: SERVICES_KEY })
    },
  })
}

export interface LogFrame {
  type: 'line' | 'error'
  line?: string
  err?: string
}
