import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { apiFetch } from './api'

export interface K8sCondition {
  type: string
  status: 'True' | 'False' | 'Unknown'
  reason: string
  message: string
  last_transition_time: string
}

export interface K8sEvent {
  type: 'Normal' | 'Warning'
  reason: string
  message: string
  source: string
  count: number
  first: string
  last: string
}

export interface K8sTaint {
  key: string
  value: string
  effect: string
}

export interface K8sContainerStatus {
  name: string
  image: string
  ready: boolean
  restart_count: number
  state: 'running' | 'waiting' | 'terminated' | string
  reason: string
  started: string
}

export interface K8sEndpointAddr {
  ip: string
  node_name: string
  ready: boolean
}

export interface K8sNodeDetail {
  node: K8sNode
  conditions: K8sCondition[]
  allocatable: { cpu: string; memory: string; pods: string }
  taints: K8sTaint[]
  events: K8sEvent[]
}

export interface K8sWorkloadDetail {
  workload: K8sWorkload
  conditions: K8sCondition[]
  selector: Record<string, string>
  strategy: string
  events: K8sEvent[]
}

export interface K8sPodDetail {
  pod: K8sPod
  conditions: K8sCondition[]
  containers: K8sContainerStatus[]
  node_name: string
  qos_class: string
  events: K8sEvent[]
}

export interface K8sServiceDetail {
  service: K8sService
  endpoints: K8sEndpointAddr[]
  selector: Record<string, string>
  events: K8sEvent[]
}

export interface K8sNode {
  name: string
  status: 'Ready' | 'NotReady' | 'Unknown'
  roles: string[]
  version: string
  os: string
  arch: string
  addresses: { type: string; address: string }[]
  capacity: { cpu: string; memory: string; pods: string }
  age: string
}

export interface K8sNamespace {
  name: string
  status: string
  age: string
}

export interface K8sWorkload {
  kind: 'Deployment' | 'StatefulSet' | 'DaemonSet'
  name: string
  namespace: string
  ready: { current: number; desired: number }
  images: string[]
  age: string
  labels: Record<string, string>
}

export interface K8sPod {
  name: string
  namespace: string
  status: string
  ready: { current: number; total: number }
  restarts: number
  node: string
  pod_ip: string
  age: string
  images: string[]
}

export interface K8sService {
  name: string
  namespace: string
  type: 'ClusterIP' | 'NodePort' | 'LoadBalancer'
  cluster_ip: string
  external_ip: string
  ports: { name: string; port: number; target_port: string; protocol: string; node_port?: number }[]
  age: string
}

export function useK8sNodes() {
  return useQuery({
    queryKey: ['k8s', 'nodes'],
    queryFn: () => apiFetch<{ nodes: K8sNode[] }>('/api/k8s/nodes'),
    refetchInterval: 10_000,
    staleTime: 2_000,
  })
}

export function useK8sNamespaces() {
  return useQuery({
    queryKey: ['k8s', 'namespaces'],
    queryFn: () => apiFetch<{ namespaces: K8sNamespace[] }>('/api/k8s/namespaces'),
    refetchInterval: 10_000,
    staleTime: 2_000,
  })
}

export function useK8sWorkloads(namespace: string) {
  return useQuery({
    queryKey: ['k8s', 'workloads', namespace],
    queryFn: () =>
      apiFetch<{ workloads: K8sWorkload[] }>(
        `/api/k8s/workloads${namespace ? `?namespace=${encodeURIComponent(namespace)}` : ''}`
      ),
    refetchInterval: 10_000,
    staleTime: 2_000,
  })
}

export function useK8sPods(namespace: string) {
  return useQuery({
    queryKey: ['k8s', 'pods', namespace],
    queryFn: () =>
      apiFetch<{ pods: K8sPod[] }>(
        `/api/k8s/pods${namespace ? `?namespace=${encodeURIComponent(namespace)}` : ''}`
      ),
    refetchInterval: 10_000,
    staleTime: 2_000,
  })
}

export function useK8sServices(namespace: string) {
  return useQuery({
    queryKey: ['k8s', 'services', namespace],
    queryFn: () =>
      apiFetch<{ services: K8sService[] }>(
        `/api/k8s/services${namespace ? `?namespace=${encodeURIComponent(namespace)}` : ''}`
      ),
    refetchInterval: 10_000,
    staleTime: 2_000,
  })
}

export function useK8sNodeDetail(name: string | null) {
  return useQuery({
    queryKey: ['k8s', 'nodes', name, 'detail'],
    queryFn: () => apiFetch<K8sNodeDetail>(`/api/k8s/nodes/${encodeURIComponent(name!)}`),
    enabled: !!name,
    refetchInterval: 10_000,
  })
}

export function useK8sWorkloadDetail(namespace: string | null, kind: string | null, name: string | null) {
  return useQuery({
    queryKey: ['k8s', 'workloads', namespace, kind, name, 'detail'],
    queryFn: () =>
      apiFetch<K8sWorkloadDetail>(
        `/api/k8s/workloads/${encodeURIComponent(namespace!)}/${encodeURIComponent(kind!.toLowerCase())}/${encodeURIComponent(name!)}`
      ),
    enabled: !!(namespace && kind && name),
    refetchInterval: 10_000,
  })
}

export function useK8sPodDetail(namespace: string | null, name: string | null) {
  return useQuery({
    queryKey: ['k8s', 'pods', namespace, name, 'detail'],
    queryFn: () =>
      apiFetch<K8sPodDetail>(`/api/k8s/pods/${encodeURIComponent(namespace!)}/${encodeURIComponent(name!)}`),
    enabled: !!(namespace && name),
    refetchInterval: 10_000,
  })
}

export function useK8sServiceDetail(namespace: string | null, name: string | null) {
  return useQuery({
    queryKey: ['k8s', 'services', namespace, name, 'detail'],
    queryFn: () =>
      apiFetch<K8sServiceDetail>(
        `/api/k8s/services/${encodeURIComponent(namespace!)}/${encodeURIComponent(name!)}`
      ),
    enabled: !!(namespace && name),
    refetchInterval: 10_000,
  })
}

export function useRestartWorkload() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ namespace, kind, name }: { namespace: string; kind: string; name: string }) =>
      apiFetch<{ ok: true; message: string }>(
        `/api/k8s/workloads/${encodeURIComponent(namespace)}/${encodeURIComponent(kind.toLowerCase())}/${encodeURIComponent(name)}/restart`,
        { method: 'POST' }
      ),
    onSuccess: (_data, { namespace, kind, name }) => {
      qc.invalidateQueries({ queryKey: ['k8s', 'workloads', namespace, kind, name, 'detail'] })
      qc.invalidateQueries({ queryKey: ['k8s', 'workloads', namespace] })
    },
  })
}

export function useScaleWorkload() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ namespace, kind, name, replicas }: { namespace: string; kind: string; name: string; replicas: number }) =>
      apiFetch<{ ok: true; replicas: number }>(
        `/api/k8s/workloads/${encodeURIComponent(namespace)}/${encodeURIComponent(kind.toLowerCase())}/${encodeURIComponent(name)}/scale`,
        { method: 'POST', body: JSON.stringify({ replicas }) }
      ),
    onSuccess: (_data, { namespace, kind, name }) => {
      qc.invalidateQueries({ queryKey: ['k8s', 'workloads', namespace, kind, name, 'detail'] })
      qc.invalidateQueries({ queryKey: ['k8s', 'workloads', namespace] })
    },
  })
}

export function useDeletePod() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ namespace, name, force }: { namespace: string; name: string; force?: boolean }) =>
      apiFetch<{ ok: true }>(
        `/api/k8s/pods/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}${force ? '?force=true' : ''}`,
        { method: 'DELETE' }
      ),
    onSuccess: (_data, { namespace }) => {
      qc.invalidateQueries({ queryKey: ['k8s', 'pods', namespace] })
      qc.invalidateQueries({ queryKey: ['k8s', 'pods'] })
    },
  })
}

export function useCordonNode() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ name, cordoned }: { name: string; cordoned: boolean }) =>
      apiFetch<{ ok: true; cordoned: boolean }>(
        `/api/k8s/nodes/${encodeURIComponent(name)}/cordon`,
        { method: 'POST', body: JSON.stringify({ cordoned }) }
      ),
    onSuccess: (_data, { name }) => {
      qc.invalidateQueries({ queryKey: ['k8s', 'nodes', name, 'detail'] })
      qc.invalidateQueries({ queryKey: ['k8s', 'nodes'] })
    },
  })
}

export function podLogsURL(namespace: string, name: string, container: string): string {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${window.location.host}/ws/k8s/pods/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/logs?container=${encodeURIComponent(container)}&tail=200`
}

export function podExecURL(namespace: string, name: string): string {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${window.location.host}/ws/k8s/pods/${namespace}/${name}/exec`
}
