import { useMemo, useState } from 'react'
import { ChevronRight, MoreVertical, Search } from 'lucide-react'

import {
  type ServiceUnit,
  useServiceAction,
  useServices,
} from '@/lib/services'
import { ApiError } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Alert, AlertDescription } from '@/components/ui/alert'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { ServiceDetail } from '@/components/services/ServiceDetail'
import { FileStateBadge, StatusBadge } from '@/components/services/StatusBadge'
import { cn } from '@/lib/utils'

const TYPE_TABS = [
  { key: 'service', label: 'Services' },
  { key: 'socket', label: 'Sockets' },
  { key: 'timer', label: 'Timers' },
  { key: 'mount', label: 'Mounts' },
  { key: 'target', label: 'Targets' },
] as const

type TypeKey = (typeof TYPE_TABS)[number]['key']

export function Services() {
  const [type, setType] = useState<TypeKey>('service')
  const [search, setSearch] = useState('')
  const [openUnit, setOpenUnit] = useState<string | null>(null)
  const services = useServices({ type, search: search || undefined })
  const action = useServiceAction()

  const unavailable =
    services.error instanceof ApiError && services.error.status === 503

  const counts = useMemo(() => countByState(services.data?.units ?? []), [services.data])

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h1 className="text-xl font-semibold tracking-tight">Services</h1>
        <div className="text-xs text-muted-foreground">
          {services.data?.units.length ?? 0} units · {counts.active} active · {counts.failed} failed
        </div>
      </div>

      {unavailable ? (
        <Alert variant="destructive">
          <AlertDescription>
            systemd is not available on this host. Services cannot be managed remotely.
          </AlertDescription>
        </Alert>
      ) : (
        <>
          <div className="flex flex-wrap items-center gap-2">
            <div className="flex flex-1 items-center gap-2 rounded-md border bg-card px-3">
              <Search className="h-4 w-4 text-muted-foreground" aria-hidden />
              <Input
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder="Search by name or description"
                className="border-0 bg-transparent shadow-none focus-visible:ring-0"
              />
            </div>
            <div className="flex flex-wrap gap-1 rounded-md border bg-card p-1">
              {TYPE_TABS.map((t) => (
                <button
                  key={t.key}
                  onClick={() => setType(t.key)}
                  className={cn(
                    'rounded px-2.5 py-1 text-xs',
                    type === t.key
                      ? 'bg-primary/10 text-foreground ring-1 ring-primary/30'
                      : 'text-muted-foreground hover:text-foreground'
                  )}
                >
                  {t.label}
                </button>
              ))}
            </div>
          </div>

          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-sm">Units</CardTitle>
            </CardHeader>
            <CardContent className="p-0">
              <div className="border-t">
                {services.isLoading ? (
                  <Empty label="Loading…" />
                ) : services.data?.units.length === 0 ? (
                  <Empty label="No units match" />
                ) : (
                  <ul className="divide-y">
                    {services.data?.units.map((u) => (
                      <ServiceRow
                        key={u.name}
                        unit={u}
                        onOpen={() => setOpenUnit(u.name)}
                        onAction={(act) => action.mutate({ name: u.name, action: act })}
                      />
                    ))}
                  </ul>
                )}
              </div>
            </CardContent>
          </Card>
        </>
      )}

      <ServiceDetail
        unit={openUnit}
        open={!!openUnit}
        onOpenChange={(open) => {
          if (!open) setOpenUnit(null)
        }}
      />
    </div>
  )
}

function ServiceRow({
  unit,
  onOpen,
  onAction,
}: {
  unit: ServiceUnit
  onOpen: () => void
  onAction: (action: 'start' | 'stop' | 'restart' | 'enable' | 'disable') => void
}) {
  return (
    <li className="group flex items-center gap-3 px-4 py-3 hover:bg-accent/40">
      <button
        onClick={onOpen}
        className="min-w-0 flex-1 text-left"
        aria-label={`Open details for ${unit.name}`}
      >
        <div className="flex items-center gap-2">
          <span className="truncate font-mono text-sm">{unit.name}</span>
          <StatusBadge state={unit.active_state} />
          <FileStateBadge state={unit.unit_file_state} />
        </div>
        {unit.description && (
          <p className="truncate text-xs text-muted-foreground">{unit.description}</p>
        )}
      </button>
      <RowMenu unit={unit} onAction={onAction} />
      <ChevronRight className="h-4 w-4 text-muted-foreground" aria-hidden />
    </li>
  )
}

function RowMenu({
  unit,
  onAction,
}: {
  unit: ServiceUnit
  onAction: (action: 'start' | 'stop' | 'restart' | 'enable' | 'disable') => void
}) {
  const isActive = unit.active_state === 'active' || unit.active_state === 'activating'
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          size="icon"
          aria-label="Actions"
          onClick={(e) => e.stopPropagation()}
        >
          <MoreVertical className="h-4 w-4" aria-hidden />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        {!isActive && (
          <DropdownMenuItem onSelect={() => onAction('start')}>Start</DropdownMenuItem>
        )}
        {isActive && (
          <DropdownMenuItem
            destructive
            onSelect={() => {
              if (confirm(`Stop ${unit.name}?`)) onAction('stop')
            }}
          >
            Stop
          </DropdownMenuItem>
        )}
        <DropdownMenuItem
          onSelect={() => {
            if (confirm(`Restart ${unit.name}?`)) onAction('restart')
          }}
        >
          Restart
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        {unit.unit_file_state !== 'enabled' && (
          <DropdownMenuItem onSelect={() => onAction('enable')}>Enable on boot</DropdownMenuItem>
        )}
        {unit.unit_file_state === 'enabled' && (
          <DropdownMenuItem onSelect={() => onAction('disable')}>Disable on boot</DropdownMenuItem>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function Empty({ label }: { label: string }) {
  return <div className="px-4 py-8 text-center text-sm text-muted-foreground">{label}</div>
}

function countByState(units: ServiceUnit[]) {
  const counts = { active: 0, failed: 0 }
  for (const u of units) {
    if (u.active_state === 'active') counts.active++
    if (u.active_state === 'failed') counts.failed++
  }
  return counts
}
