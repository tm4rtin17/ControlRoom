import { useEffect, useRef, useState } from 'react'
import { Pause, Play } from 'lucide-react'

import { podLogsURL } from '@/lib/k8s'
import { Button } from '@/components/ui/button'

const MAX_LINES = 1000

interface LogLine {
  line: string
}

interface LogFrame {
  type: 'line' | 'error'
  stream?: string
  line?: string
  err?: string
}

export function PodLogStream({
  namespace,
  podName,
  container,
}: {
  namespace: string
  podName: string
  container: string
}) {
  const [lines, setLines] = useState<LogLine[]>([])
  const [paused, setPaused] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const containerRef = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    setLines([])
    setError(null)
    const url = podLogsURL(namespace, podName, container)
    const ws = new WebSocket(url)
    ws.onmessage = (evt) => {
      try {
        const f = JSON.parse(evt.data) as LogFrame
        if (f.type === 'error') {
          setError(f.err ?? 'log stream error')
          return
        }
        if (f.type === 'line' && f.line !== undefined) {
          setLines((prev) => {
            const next = [...prev, { line: f.line! }]
            return next.length > MAX_LINES ? next.slice(-MAX_LINES) : next
          })
        }
      } catch {
        // ignore
      }
    }
    ws.onerror = () => setError('connection error')
    return () => ws.close()
  }, [namespace, podName, container])

  useEffect(() => {
    if (paused) return
    const el = containerRef.current
    if (el) el.scrollTop = el.scrollHeight
  }, [lines, paused])

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center justify-between">
        <span className="text-xs text-muted-foreground">
          {lines.length} line{lines.length === 1 ? '' : 's'}
          {paused && ' · paused'}
        </span>
        <div className="flex gap-2">
          <Button variant="ghost" size="sm" onClick={() => setPaused((p) => !p)}>
            {paused ? <Play className="h-4 w-4" aria-hidden /> : <Pause className="h-4 w-4" aria-hidden />}
            {paused ? 'Resume' : 'Pause'}
          </Button>
          <Button variant="ghost" size="sm" onClick={() => setLines([])}>
            Clear
          </Button>
        </div>
      </div>
      {error && (
        <div className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-xs text-destructive">
          {error}
        </div>
      )}
      <div
        ref={containerRef}
        className="h-72 overflow-auto rounded-md border bg-background p-3 font-mono text-[11px] leading-relaxed"
      >
        {lines.length === 0 ? (
          <p className="text-muted-foreground">Waiting for output…</p>
        ) : (
          lines.map((l, i) => (
            <div key={i} className="whitespace-pre-wrap break-all">
              {l.line}
            </div>
          ))
        )}
      </div>
    </div>
  )
}
