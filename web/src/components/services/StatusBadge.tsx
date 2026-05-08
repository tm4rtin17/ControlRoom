import { Badge } from '@/components/ui/badge'

const VARIANT_BY_STATE: Record<string, 'success' | 'warn' | 'danger' | 'muted' | 'default'> = {
  active: 'success',
  reloading: 'warn',
  activating: 'warn',
  deactivating: 'warn',
  failed: 'danger',
  inactive: 'muted',
  maintenance: 'warn',
}

export function StatusBadge({ state }: { state: string }) {
  const variant = VARIANT_BY_STATE[state] ?? 'default'
  return <Badge variant={variant}>{state}</Badge>
}

export function FileStateBadge({ state }: { state: string }) {
  if (!state) return null
  const variant: 'success' | 'muted' | 'warn' =
    state === 'enabled' ? 'success' : state === 'disabled' ? 'muted' : 'warn'
  return (
    <Badge variant={variant} className="text-[10px]">
      {state}
    </Badge>
  )
}
