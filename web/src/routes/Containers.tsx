import { useMemo, useState } from 'react'
import { Search } from 'lucide-react'

import { useContainers, type ContainerSummary } from '@/lib/containers'
import { ApiError } from '@/lib/api'
import { Input } from '@/components/ui/input'
import { Card, CardContent } from '@/components/ui/card'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { ContainerCard } from '@/components/containers/ContainerCard'
import { ContainerDetail } from '@/components/containers/ContainerDetail'

export function Containers() {
  const [search, setSearch] = useState('')
  const [openId, setOpenId] = useState<string | null>(null)
  const containers = useContainers()

  const unavailable = containers.error instanceof ApiError && containers.error.status === 503

  const groups = useMemo(() => groupByProject(containers.data?.containers ?? [], search), [
    containers.data,
    search,
  ])

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h1 className="text-xl font-semibold tracking-tight">Containers</h1>
        <div className="text-xs text-muted-foreground">
          {containers.data?.containers.length ?? 0} containers
        </div>
      </div>

      {unavailable ? (
        <Alert variant="destructive">
          <AlertDescription>
            Docker daemon is not reachable. Containers cannot be managed remotely.
          </AlertDescription>
        </Alert>
      ) : (
        <>
          <div className="flex flex-1 items-center gap-2 rounded-md border bg-card px-3">
            <Search className="h-4 w-4 text-muted-foreground" aria-hidden />
            <Input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search by name, image, or compose project"
              className="border-0 bg-transparent shadow-none focus-visible:ring-0"
            />
          </div>

          {groups.length === 0 ? (
            <Card>
              <CardContent className="px-4 py-12 text-center text-sm text-muted-foreground">
                {containers.isLoading ? 'Loading…' : 'No containers found'}
              </CardContent>
            </Card>
          ) : (
            groups.map((g) => (
              <ProjectGroup
                key={g.project || '__none__'}
                project={g.project}
                containers={g.containers}
                onOpen={(id) => setOpenId(id)}
              />
            ))
          )}
        </>
      )}

      <ContainerDetail
        id={openId}
        open={!!openId}
        onOpenChange={(open) => !open && setOpenId(null)}
      />
    </div>
  )
}

function ProjectGroup({
  project,
  containers,
  onOpen,
}: {
  project: string
  containers: ContainerSummary[]
  onOpen: (id: string) => void
}) {
  return (
    <section className="flex flex-col gap-2">
      <div className="flex items-baseline justify-between">
        <h2 className="text-sm font-semibold">
          {project ? project : <span className="text-muted-foreground">Standalone</span>}
        </h2>
        <span className="text-xs text-muted-foreground">{containers.length}</span>
      </div>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
        {containers.map((c) => (
          <ContainerCard key={c.id} container={c} onOpen={() => onOpen(c.id)} />
        ))}
      </div>
    </section>
  )
}

interface ProjectGroup {
  project: string
  containers: ContainerSummary[]
}

function groupByProject(list: ContainerSummary[], search: string): ProjectGroup[] {
  const needle = search.trim().toLowerCase()
  const filtered = needle
    ? list.filter((c) =>
        [c.name, c.image, c.compose_project, c.compose_service]
          .filter(Boolean)
          .some((s) => s.toLowerCase().includes(needle))
      )
    : list

  const buckets = new Map<string, ContainerSummary[]>()
  for (const c of filtered) {
    const key = c.compose_project ?? ''
    const arr = buckets.get(key) ?? []
    arr.push(c)
    buckets.set(key, arr)
  }
  return Array.from(buckets.entries())
    .sort(([a], [b]) => {
      // Standalone (empty key) comes last.
      if (a === '' && b !== '') return 1
      if (b === '' && a !== '') return -1
      return a.localeCompare(b)
    })
    .map(([project, containers]) => ({
      project,
      containers: containers.sort((a, b) => a.name.localeCompare(b.name)),
    }))
}
