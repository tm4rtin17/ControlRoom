import { useCallback, useEffect, useRef, useState } from 'react'
import { Terminal as XTerm } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'
import { RotateCcw, Terminal as TerminalIcon } from 'lucide-react'
import '@xterm/xterm/css/xterm.css'

import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { SoftKeys } from '@/components/terminal/SoftKeys'

type Status = 'connecting' | 'open' | 'closed' | 'error'

// Theme tuned for our dark CSS-vars palette. We avoid pulling computed styles
// at runtime — these match the slate dark scheme.
const xtermTheme = {
  background: 'rgba(0,0,0,0)', // let the card show through
  foreground: '#e5edf6',
  cursor: '#7dd3fc',
  cursorAccent: '#020617',
  black: '#1e293b',
  red: '#f87171',
  green: '#4ade80',
  yellow: '#fbbf24',
  blue: '#60a5fa',
  magenta: '#c084fc',
  cyan: '#22d3ee',
  white: '#e2e8f0',
  brightBlack: '#475569',
  brightRed: '#fca5a5',
  brightGreen: '#86efac',
  brightYellow: '#fde68a',
  brightBlue: '#93c5fd',
  brightMagenta: '#d8b4fe',
  brightCyan: '#67e8f9',
  brightWhite: '#f1f5f9',
}

export function Terminal() {
  const containerRef = useRef<HTMLDivElement | null>(null)
  const termRef = useRef<XTerm | null>(null)
  const fitRef = useRef<FitAddon | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const [status, setStatus] = useState<Status>('connecting')
  const [error, setError] = useState<string | null>(null)
  const [generation, setGeneration] = useState(0)

  const sendBytes = useCallback((data: Uint8Array) => {
    const ws = wsRef.current
    if (ws && ws.readyState === WebSocket.OPEN) {
      // Copy underlying buffer so we don't ship a shared SharedArrayBuffer.
      ws.send(data.buffer.slice(data.byteOffset, data.byteOffset + data.byteLength))
    }
  }, [])

  // Setup + teardown is keyed on `generation` so the Reconnect button can
  // bump it to force a clean rebuild.
  useEffect(() => {
    if (!containerRef.current) return

    const term = new XTerm({
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
      fontSize: 13,
      lineHeight: 1.2,
      cursorBlink: true,
      allowProposedApi: true,
      theme: xtermTheme,
      scrollback: 5000,
      convertEol: false,
    })
    const fit = new FitAddon()
    term.loadAddon(fit)
    term.loadAddon(new WebLinksAddon())
    term.open(containerRef.current)
    termRef.current = term
    fitRef.current = fit

    setStatus('connecting')
    setError(null)

    // Fit before opening WS so the init frame has accurate dims.
    try {
      fit.fit()
    } catch {
      // ignore; container may still be measuring
    }
    const dims = fit.proposeDimensions() ?? { rows: 24, cols: 80 }

    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const ws = new WebSocket(`${proto}//${window.location.host}/ws/terminal`)
    ws.binaryType = 'arraybuffer'
    wsRef.current = ws

    ws.onopen = () => {
      setStatus('open')
      ws.send(JSON.stringify({ rows: dims.rows, cols: dims.cols }))
      term.focus()
    }
    ws.onmessage = (evt) => {
      if (typeof evt.data === 'string') {
        try {
          const msg = JSON.parse(evt.data) as { type?: string; err?: string }
          if (msg.type === 'error') {
            setError(msg.err ?? 'terminal error')
          }
        } catch {
          // ignore
        }
        return
      }
      // ArrayBuffer → write directly to xterm
      term.write(new Uint8Array(evt.data as ArrayBuffer))
    }
    ws.onclose = () => setStatus('closed')
    ws.onerror = () => {
      setStatus('error')
      setError('connection error')
    }

    const dataDisposable = term.onData((data) => {
      const bytes = new TextEncoder().encode(data)
      sendBytes(bytes)
    })

    const resizeDisposable = term.onResize(({ rows, cols }) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'resize', rows, cols }))
      }
    })

    const ro = new ResizeObserver(() => {
      try {
        fit.fit()
      } catch {
        // ignore
      }
    })
    ro.observe(containerRef.current)

    return () => {
      ro.disconnect()
      dataDisposable.dispose()
      resizeDisposable.dispose()
      ws.close()
      term.dispose()
      termRef.current = null
      fitRef.current = null
      wsRef.current = null
    }
  }, [generation, sendBytes])

  return (
    <div className="flex h-[calc(100vh-9rem)] flex-col gap-3">
      <div className="flex items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          <TerminalIcon className="h-4 w-4 text-muted-foreground" aria-hidden />
          <h1 className="text-base font-semibold">Terminal</h1>
          <StatusPill status={status} />
        </div>
        <Button
          variant="outline"
          size="sm"
          onClick={() => setGeneration((g) => g + 1)}
          disabled={status === 'connecting'}
        >
          <RotateCcw className="h-3.5 w-3.5" aria-hidden />
          {status === 'open' ? 'Reset' : 'Reconnect'}
        </Button>
      </div>

      {error && (
        <div className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-xs text-destructive">
          {error}
        </div>
      )}

      <SoftKeys onKey={sendBytes} />

      <Card className="flex-1 overflow-hidden p-0">
        <CardContent className="h-full p-3">
          <div ref={containerRef} className="h-full w-full" />
        </CardContent>
      </Card>
    </div>
  )
}

function StatusPill({ status }: { status: Status }) {
  const styles = {
    connecting: 'bg-amber-500/10 text-amber-500 ring-amber-500/30',
    open: 'bg-emerald-500/10 text-emerald-500 ring-emerald-500/30',
    closed: 'bg-muted text-muted-foreground ring-muted',
    error: 'bg-destructive/10 text-destructive ring-destructive/30',
  }[status]
  const label = {
    connecting: 'Connecting',
    open: 'Live',
    closed: 'Closed',
    error: 'Error',
  }[status]
  return (
    <span className={`inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs ring-1 ${styles}`}>
      <span
        className={`inline-block h-1.5 w-1.5 rounded-full ${
          status === 'open' ? 'bg-emerald-500 animate-pulse' : status === 'connecting' ? 'bg-amber-500 animate-pulse' : status === 'error' ? 'bg-destructive' : 'bg-muted-foreground'
        }`}
        aria-hidden
      />
      {label}
    </span>
  )
}
