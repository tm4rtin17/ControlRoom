import { Badge } from '@/components/ui/badge'

type PillVariant = 'success' | 'warn' | 'danger' | 'muted' | 'default'

const NODE_STATUS: Record<string, PillVariant> = {
  Ready: 'success',
  NotReady: 'danger',
  Unknown: 'warn',
}

const POD_STATUS: Record<string, PillVariant> = {
  Running: 'success',
  Pending: 'warn',
  Succeeded: 'muted',
  Failed: 'danger',
}

const SERVICE_TYPE: Record<string, PillVariant> = {
  ClusterIP: 'default',
  NodePort: 'warn',
  LoadBalancer: 'success',
}

export function K8sNodeStatusPill({ status }: { status: string }) {
  return <Badge variant={NODE_STATUS[status] ?? 'muted'}>{status}</Badge>
}

export function K8sPodStatusPill({ status }: { status: string }) {
  return <Badge variant={POD_STATUS[status] ?? 'muted'}>{status}</Badge>
}

export function K8sServiceTypePill({ type }: { type: string }) {
  return <Badge variant={SERVICE_TYPE[type] ?? 'default'}>{type}</Badge>
}
