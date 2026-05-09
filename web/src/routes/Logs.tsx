import { useEffect, useMemo, useRef, useState } from 'react'
import { ChevronDown, ChevronRight, Pause, Play, RefreshCw, Search } from 'lucide-react'

import { ApiError } from '@/lib/api'
import { type ContainerSummary, type LogFrame, useContainers } from '@/lib/containers'
import { type LogEntry, type LogQuery, tailURL, useLogs } from '@/lib/logs'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { cn } from '@/lib/utils'

const MAX_LIVE_ENTRIES = 1000

const SINCE_OPTIONS: { label: string; value: string }[] = [
  { label: 'Last 15m', value: '-15min' },
  { label: 'Last hour', value: '-1h' },
  { label: 'Last 24h', value: '-1d' },
  { label: 'Last 7d', value: '-7d' },
]

const PRIORITIES = [
  { label: 'All', value: -1 },
  { label: 'Errors+', value: 3 },
  { label: 'Warnings+', value: 4 },
  { label: 'Notices+', value: 5 },
  { label: 'Info+', value: 6 },
  { label: 'Debug', value: 7 },
]

type Source = 'journal' | 'containers'

export function Logs() {
  const [source, setSource] = useState<Source>('journal')

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-baseline justify-between">
        <h1 className="text-xl font-semibold tracking-tight">Logs</h1>
        <div className="inline-flex items-center rounded-md border bg-background p-0.5 text-xs">
          <SourceTab active={source === 'journal'} onClick={() => setSource('journal')}>
            Journal
          </SourceTab>
          <SourceTab active={source === 'containers'} onClick={() => setSource('containers')}>
            Containers
          </SourceTab>
        </div>
      </div>
      {source === 'journal' ? <JournalView onSwitchToContainers={() => setSource('containers')} /> : <ContainersView />}
    </div>
  )
}

function SourceTab({
  active,
  onClick,
  children,
}: {
  active: boolean
  onClick: () => void
  children: React.ReactNode
}) {
  return (
    <button
      onClick={onClick}
      className={cn(
        'rounded-sm px-3 py-1',
        active ? 'bg-primary/10 text-foreground ring-1 ring-primary/30' : 'text-muted-foreground hover:text-foreground'
      )}
    >
      {children}
    </button>
  )
}

// ---- Journal ----

function JournalView({ onSwitchToContainers }: { onSwitchToContainers: () => void }) {
  const [unit, setUnit] = useState('')
  const [search, setSearch] = useState('')
  const [since, setSince] = useState('-15min')
  const [priority, setPriority] = useState<number>(-1)
  const [live, setLive] = useState(false)

  const query: LogQuery = useMemo(
    () => ({ unit: unit || undefined, q: search || undefined, since, priority, n: 200 }),
    [unit, search, since, priority]
  )
  const stat = useLogs(query, !live)

  const unavailable = stat.error instanceof ApiError && stat.error.status === 503

  return (
    <>
      <Card>
        <CardContent className="grid grid-cols-1 gap-3 p-4 sm:grid-cols-4">
          <div className="flex flex-col gap-1 sm:col-span-2">
            <Label htmlFor="search" className="text-xs">Search</Label>
            <div className="flex items-center gap-2 rounded-md border bg-background px-2">
              <Search className="h-4 w-4 text-muted-foreground" aria-hidden />
              <Input
                id="search"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder="Substring (no regex)"
                className="border-0 bg-transparent shadow-none focus-visible:ring-0"
              />
            </div>
          </div>
          <div className="flex flex-col gap-1">
            <Label htmlFor="unit" className="text-xs">Unit</Label>
            <Input
              id="unit"
              value={unit}
              onChange={(e) => setUnit(e.target.value)}
              placeholder="nginx.service"
              className="font-mono text-xs"
            />
          </div>
          <div className="flex flex-col gap-1">
            <Label htmlFor="since" className="text-xs">Since</Label>
            <select
              id="since"
              className="h-10 rounded-md border bg-background px-2 text-sm"
              value={since}
              onChange={(e) => setSince(e.target.value)}
            >
              {SINCE_OPTIONS.map((o) => (
                <option key={o.value} value={o.value}>{o.label}</option>
              ))}
            </select>
          </div>
          <div className="flex flex-col gap-1 sm:col-span-4 sm:flex-row sm:items-center sm:gap-3">
            <Label className="text-xs">Priority:</Label>
            <div className="flex flex-wrap gap-1">
              {PRIORITIES.map((p) => (
                <button
                  key={p.value}
                  onClick={() => setPriority(p.value)}
                  className={cn(
                    'rounded-md px-2 py-1 text-xs',
                    priority === p.value
                      ? 'bg-primary/10 text-foreground ring-1 ring-primary/30'
                      : 'text-muted-foreground hover:bg-accent hover:text-foreground'
                  )}
                >
                  {p.label}
                </button>
              ))}
            </div>
            <div className="ml-auto flex gap-2">
              {!live && (
                <Button size="sm" variant="outline" onClick={() => stat.refetch()}>
                  <RefreshCw className="h-3.5 w-3.5" aria-hidden />
                  Refresh
                </Button>
              )}
              <Button size="sm" onClick={() => setLive((v) => !v)} disabled={unavailable}>
                {live ? (
                  <>
                    <Pause className="h-3.5 w-3.5" aria-hidden />
                    Stop tail
                  </>
                ) : (
                  <>
                    <Play className="h-3.5 w-3.5" aria-hidden />
                    Live tail
                  </>
                )}
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>

      {live ? (
        <LiveStream query={query} />
      ) : unavailable ? (
        <Card>
          <CardContent className="flex flex-col items-start gap-3 p-6 text-sm">
            <p className="text-muted-foreground">{stat.error?.message}</p>
            <Button size="sm" variant="outline" onClick={onSwitchToContainers}>
              View container logs instead
            </Button>
          </CardContent>
        </Card>
      ) : (
        <>
          {stat.error && (
            <Alert variant="destructive">
              <AlertDescription>{stat.error.message || 'Could not fetch logs.'}</AlertDescription>
            </Alert>
          )}
          <EntriesList entries={stat.data?.entries ?? []} loading={stat.isLoading} />
        </>
      )}
    </>
  )
}

function LiveStream({ query }: { query: LogQuery }) {
  const [entries, setEntries] = useState<LogEntry[]>([])
  const [error, setError] = useState<string | null>(null)
  const wsRef = useRef<WebSocket | null>(null)

  useEffect(() => {
    setEntries([])
    setError(null)

    const ws = new WebSocket(tailURL(query))
    wsRef.current = ws

    ws.onmessage = (evt) => {
      try {
        const e = JSON.parse(evt.data) as LogEntry & { type?: string; err?: string }
        if (e.type === 'error') {
          setError(e.err ?? 'tail error')
          return
        }
        setEntries((prev) => {
          const next = [...prev, e]
          return next.length > MAX_LIVE_ENTRIES ? next.slice(-MAX_LIVE_ENTRIES) : next
        })
      } catch {
        // ignore
      }
    }
    ws.onerror = () => setError('connection error')
    return () => ws.close()
  }, [query])

  return (
    <>
      {error && (
        <Alert variant="destructive">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}
      <EntriesList entries={entries} loading={false} live />
    </>
  )
}

function EntriesList({
  entries,
  loading,
  live,
}: {
  entries: LogEntry[]
  loading: boolean
  live?: boolean
}) {
  if (loading) {
    return (
      <Card>
        <CardContent className="px-4 py-12 text-center text-sm text-muted-foreground">Loading…</CardContent>
      </Card>
    )
  }
  if (entries.length === 0) {
    return (
      <Card>
        <CardContent className="px-4 py-12 text-center text-sm text-muted-foreground">
          {live ? 'Waiting for output…' : 'No entries match'}
        </CardContent>
      </Card>
    )
  }
  return (
    <Card>
      <CardContent className="p-0">
        <ul className="divide-y">
          {entries.map((e, i) => (
            <LogRow key={i} entry={e} />
          ))}
        </ul>
      </CardContent>
    </Card>
  )
}

function LogRow({ entry }: { entry: LogEntry }) {
  const [open, setOpen] = useState(false)
  const color = priorityColor(entry.priority)
  return (
    <li>
      <button
        onClick={() => setOpen((o) => !o)}
        className="flex w-full items-start gap-3 px-4 py-2 text-left hover:bg-accent/40"
      >
        {open ? (
          <ChevronDown className="mt-0.5 h-3.5 w-3.5 text-muted-foreground" aria-hidden />
        ) : (
          <ChevronRight className="mt-0.5 h-3.5 w-3.5 text-muted-foreground" aria-hidden />
        )}
        <span className="font-mono text-[10px] text-muted-foreground tabular-nums shrink-0 hidden sm:inline">
          {formatTimestamp(entry.timestamp)}
        </span>
        <span className={cn('font-mono text-[10px] shrink-0 hidden md:inline', color)}>
          {priorityLabel(entry.priority)}
        </span>
        <span className="font-mono text-[10px] text-muted-foreground truncate shrink-0 hidden md:inline">
          {entry.unit ?? entry.identifier ?? ''}
        </span>
        <span className={cn('flex-1 truncate font-mono text-xs', color)}>
          {entry.message}
        </span>
      </button>
      {open && (
        <div className="border-t bg-muted/30 px-4 py-2 font-mono text-[11px]">
          <Field label="time" value={entry.timestamp} />
          <Field label="priority" value={`${entry.priority} (${priorityLabel(entry.priority)})`} />
          {entry.unit && <Field label="unit" value={entry.unit} />}
          {entry.identifier && <Field label="ident" value={entry.identifier} />}
          {entry.pid !== undefined && <Field label="pid" value={String(entry.pid)} />}
          {entry.hostname && <Field label="host" value={entry.hostname} />}
          <div className="mt-1 whitespace-pre-wrap break-words">{entry.message}</div>
        </div>
      )}
    </li>
  )
}

function Field({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex gap-2">
      <span className="w-16 text-muted-foreground">{label}</span>
      <span className="break-all">{value}</span>
    </div>
  )
}

function priorityColor(p: number): string {
  if (p <= 3) return 'text-destructive'
  if (p === 4) return 'text-amber-500'
  if (p === 5) return 'text-sky-400'
  return 'text-foreground'
}

function priorityLabel(p: number): string {
  return ['emerg', 'alert', 'crit', 'err', 'warn', 'notice', 'info', 'debug'][p] ?? String(p)
}

function formatTimestamp(s: string): string {
  // Display as HH:MM:SS for tight rows.
  const idx = s.indexOf('T')
  if (idx < 0) return s
  return s.slice(idx + 1, idx + 9)
}

// ---- Containers ----

interface DockerLine {
  stream: 'stdout' | 'stderr' | 'stdin'
  line: string
}

function ContainersView() {
  const list = useContainers()
  const [selectedId, setSelectedId] = useState<string>('')
  const [search, setSearch] = useState('')
  const [paused, setPaused] = useState(false)

  const containers = list.data?.containers ?? []
  // Auto-pick the first running container the first time the list loads.
  useEffect(() => {
    if (selectedId || containers.length === 0) return
    const running = containers.find((c) => c.state === 'running') ?? containers[0]
    if (running) setSelectedId(running.id)
  }, [containers, selectedId])

  const unavailable = list.error instanceof ApiError && list.error.status === 503
  const errMsg = list.error?.message

  if (unavailable) {
    return (
      <Card>
        <CardContent className="p-6 text-sm text-muted-foreground">
          {errMsg ?? 'Docker is not available on this host.'}
        </CardContent>
      </Card>
    )
  }

  return (
    <>
      <Card>
        <CardContent className="grid grid-cols-1 gap-3 p-4 sm:grid-cols-4">
          <div className="flex flex-col gap-1 sm:col-span-2">
            <Label htmlFor="container" className="text-xs">Container</Label>
            <select
              id="container"
              className="h-10 rounded-md border bg-background px-2 font-mono text-xs"
              value={selectedId}
              onChange={(e) => setSelectedId(e.target.value)}
            >
              <option value="">— select a container —</option>
              {containers.map((c) => (
                <option key={c.id} value={c.id}>
                  {labelFor(c)}
                </option>
              ))}
            </select>
          </div>
          <div className="flex flex-col gap-1 sm:col-span-2">
            <Label htmlFor="docker-search" className="text-xs">Search</Label>
            <div className="flex items-center gap-2 rounded-md border bg-background px-2">
              <Search className="h-4 w-4 text-muted-foreground" aria-hidden />
              <Input
                id="docker-search"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder="Filter visible lines (substring)"
                className="border-0 bg-transparent shadow-none focus-visible:ring-0"
              />
            </div>
          </div>
        </CardContent>
      </Card>

      {!selectedId ? (
        <Card>
          <CardContent className="px-4 py-12 text-center text-sm text-muted-foreground">
            {list.isLoading ? 'Loading containers…' : 'Pick a container to start tailing.'}
          </CardContent>
        </Card>
      ) : (
        <DockerLogStream id={selectedId} search={search} paused={paused} setPaused={setPaused} />
      )}
    </>
  )
}

function labelFor(c: ContainerSummary): string {
  const tag = c.state === 'running' ? '●' : '○'
  return `${tag} ${c.name} (${c.image})`
}

function DockerLogStream({
  id,
  search,
  paused,
  setPaused,
}: {
  id: string
  search: string
  paused: boolean
  setPaused: (v: boolean) => void
}) {
  const [lines, setLines] = useState<DockerLine[]>([])
  const [error, setError] = useState<string | null>(null)
  const containerRef = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    setLines([])
    setError(null)
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const ws = new WebSocket(`${proto}//${window.location.host}/ws/containers/${id}/logs`)
    ws.onmessage = (evt) => {
      try {
        const f = JSON.parse(evt.data) as LogFrame
        if (f.type === 'error') {
          setError(f.err ?? 'log stream error')
          return
        }
        if (f.type === 'line' && f.line !== undefined) {
          setLines((prev) => {
            const next = [...prev, { stream: (f.stream ?? 'stdout') as DockerLine['stream'], line: f.line! }]
            return next.length > MAX_LIVE_ENTRIES ? next.slice(-MAX_LIVE_ENTRIES) : next
          })
        }
      } catch {
        // ignore
      }
    }
    ws.onerror = () => setError('connection error')
    return () => ws.close()
  }, [id])

  const filtered = useMemo(() => {
    if (!search) return lines
    const needle = search.toLowerCase()
    return lines.filter((l) => l.line.toLowerCase().includes(needle))
  }, [lines, search])

  useEffect(() => {
    if (paused) return
    const el = containerRef.current
    if (el) el.scrollTop = el.scrollHeight
  }, [filtered, paused])

  return (
    <Card>
      <CardContent className="flex flex-col gap-2 p-4">
        <div className="flex items-center justify-between">
          <span className="text-xs text-muted-foreground">
            {filtered.length}
            {search ? ` / ${lines.length}` : ''} line{filtered.length === 1 ? '' : 's'}
            {paused && ' · paused'}
          </span>
          <div className="flex gap-2">
            <Button variant="ghost" size="sm" onClick={() => setPaused(!paused)}>
              {paused ? <Play className="h-4 w-4" aria-hidden /> : <Pause className="h-4 w-4" aria-hidden />}
              {paused ? 'Resume' : 'Pause'}
            </Button>
            <Button variant="ghost" size="sm" onClick={() => setLines([])}>
              Clear
            </Button>
          </div>
        </div>
        {error && (
          <Alert variant="destructive">
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        )}
        <div
          ref={containerRef}
          className="h-[28rem] overflow-auto rounded-md border bg-background p-3 font-mono text-[11px] leading-relaxed"
        >
          {filtered.length === 0 ? (
            <p className="text-muted-foreground">{lines.length === 0 ? 'Waiting for output…' : 'No lines match the filter.'}</p>
          ) : (
            filtered.map((l, i) => (
              <div
                key={i}
                className={cn('whitespace-pre-wrap break-all', l.stream === 'stderr' && 'text-amber-500')}
              >
                {l.line}
              </div>
            ))
          )}
        </div>
      </CardContent>
    </Card>
  )
}
