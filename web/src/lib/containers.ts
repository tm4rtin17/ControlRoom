import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { apiFetch } from './api'

export interface ContainerPort {
  ip?: string
  private_port: number
  public_port?: number
  protocol: string
}

export interface ContainerSummary {
  id: string
  name: string
  image: string
  state: string
  status: string
  created_at: string
  ports: ContainerPort[]
  labels: Record<string, string>
  compose_project: string
  compose_service: string
}

export interface ContainerMount {
  type: string
  source: string
  destination: string
  mode: string
  rw: boolean
}

export interface ContainerNetworkAttach {
  name: string
  ip_address: string
  gateway: string
  mac_address: string
}

export interface ContainerDetail extends ContainerSummary {
  command: string[]
  env: string[]
  mounts: ContainerMount[]
  networks: ContainerNetworkAttach[]
  started_at: string
  finished_at: string
  restart_policy: string
  exit_code: number
  health?: string
}

export interface ContainerStats {
  ts: string
  cpu_pct: number
  mem_usage: number
  mem_limit: number
  mem_pct: number
  net_rx: number
  net_tx: number
  blk_read: number
  blk_write: number
  pids: number
}

export interface LogFrame {
  type: 'line' | 'error'
  stream?: 'stdout' | 'stderr' | 'stdin'
  line?: string
  err?: string
}

export const CONTAINERS_KEY = ['containers'] as const

export function useContainers() {
  return useQuery({
    queryKey: CONTAINERS_KEY,
    queryFn: () =>
      apiFetch<{ containers: ContainerSummary[] }>('/api/containers/?all=true'),
    refetchInterval: 5_000,
    staleTime: 1_000,
  })
}

export function useContainerDetail(id: string | null) {
  return useQuery({
    queryKey: ['containers', 'detail', id],
    queryFn: () => apiFetch<ContainerDetail>(`/api/containers/${id}`),
    enabled: !!id,
    refetchInterval: 5_000,
  })
}

type ContainerAction = 'start' | 'stop' | 'restart'

export function useContainerAction() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, action }: { id: string; action: ContainerAction }) =>
      apiFetch<{ ok: boolean }>(`/api/containers/${id}/${action}`, { method: 'POST' }),
    onSuccess: () => qc.invalidateQueries({ queryKey: CONTAINERS_KEY }),
  })
}

export function useContainerRemove() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, force }: { id: string; force: boolean }) =>
      apiFetch<{ ok: boolean }>(
        `/api/containers/${id}${force ? '?force=true' : ''}`,
        { method: 'DELETE' }
      ),
    onSuccess: () => qc.invalidateQueries({ queryKey: CONTAINERS_KEY }),
  })
}
