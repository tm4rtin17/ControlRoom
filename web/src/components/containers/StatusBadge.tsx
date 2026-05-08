import { Badge } from '@/components/ui/badge'

const VARIANT: Record<string, 'success' | 'warn' | 'danger' | 'muted' | 'default'> = {
  running: 'success',
  restarting: 'warn',
  paused: 'warn',
  removing: 'warn',
  exited: 'muted',
  dead: 'danger',
  created: 'muted',
}

export function ContainerStatusBadge({ state }: { state: string }) {
  return <Badge variant={VARIANT[state] ?? 'default'}>{state}</Badge>
}
