import { useEffect, useRef, useState } from 'react'
import { Pause, Play } from 'lucide-react'

import { Button } from '@/components/ui/button'
import type { LogFrame } from '@/lib/services'

const MAX_LINES = 1000

export function LogStream({ unit }: { unit: string }) {
  const [lines, setLines] = useState<string[]>([])
  const [paused, setPaused] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const containerRef = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const url = `${proto}//${window.location.host}/ws/services/${encodeURIComponent(unit)}/logs`
    const socket = new WebSocket(url)

    socket.onmessage = (evt) => {
      try {
        const frame = JSON.parse(evt.data) as LogFrame
        if (frame.type === 'error') {
          setError(frame.err ?? 'log stream error')
          return
        }
        if (frame.type === 'line' && frame.line !== undefined) {
          setLines((prev) => {
            const next = [...prev, frame.line!]
            return next.length > MAX_LINES ? next.slice(-MAX_LINES) : next
          })
        }
      } catch {
        // ignore malformed frames
      }
    }
    socket.onerror = () => setError('connection error')

    return () => {
      socket.close()
    }
  }, [unit])

  // Auto-scroll to bottom unless the user paused.
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
          lines.map((line, i) => (
            <div key={i} className="whitespace-pre-wrap break-all">
              {line}
            </div>
          ))
        )}
      </div>
    </div>
  )
}
