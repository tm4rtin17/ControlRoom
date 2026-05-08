import { useQuery } from '@tanstack/react-query'

import { apiFetch } from './api'

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
