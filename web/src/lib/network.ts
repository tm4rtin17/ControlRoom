import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { apiFetch } from './api'

export interface NetworkInterface {
  name: string
  mac: string
  state: string
  mtu: number
  type?: string
  ips: string[]
  flags: string[]
  stats: { rx_bytes: number; tx_bytes: number }
}

export interface UFWRule {
  index: number
  to: string
  action: string
  from: string
}

export interface UFWStatus {
  active: boolean
  logging?: string
  default?: string
  rules: UFWRule[]
}

export interface AddRuleSpec {
  action: 'allow' | 'deny' | 'reject' | 'limit'
  port?: string
  protocol?: 'tcp' | 'udp'
  from?: string
  comment?: string
}

export const INTERFACES_KEY = ['network', 'interfaces'] as const
export const FIREWALL_KEY = ['network', 'firewall'] as const

export function useInterfaces() {
  return useQuery({
    queryKey: INTERFACES_KEY,
    queryFn: () => apiFetch<{ interfaces: NetworkInterface[] }>('/api/network/interfaces'),
    refetchInterval: 5_000,
  })
}

export function useFirewall() {
  return useQuery({
    queryKey: FIREWALL_KEY,
    queryFn: () => apiFetch<UFWStatus>('/api/network/firewall'),
    refetchInterval: 10_000,
  })
}

export function useAddRule() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (spec: AddRuleSpec) =>
      apiFetch<{ ok: boolean }>('/api/network/firewall/rules', {
        method: 'POST',
        body: JSON.stringify(spec),
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: FIREWALL_KEY }),
  })
}

export function useDeleteRule() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (index: number) =>
      apiFetch<{ ok: boolean }>(`/api/network/firewall/rules/${index}`, { method: 'DELETE' }),
    onSuccess: () => qc.invalidateQueries({ queryKey: FIREWALL_KEY }),
  })
}

export function useFirewallToggle() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (enable: boolean) =>
      apiFetch<{ ok: boolean }>(
        `/api/network/firewall/${enable ? 'enable' : 'disable'}`,
        { method: 'POST' }
      ),
    onSuccess: () => qc.invalidateQueries({ queryKey: FIREWALL_KEY }),
  })
}
