import { useCallback, useEffect, useRef, useState } from 'react'
import { Terminal as XTerm } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'
import { RotateCcw, Terminal as TerminalIcon } from 'lucide-react'
import '@xterm/xterm/css/xterm.css'

import { Button } from '@/components/ui/button'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription } from '@/components/ui/sheet'
import { K8sContainerStatus, podExecURL } from '@/lib/k8s'

type Status = 'connecting' | 'open' | 'closed' | 'error'

const COMMAND_OPTIONS = ['/bin/sh', '/bin/bash']

const xtermTheme = {
  background: 'rgba(0,0,0,0)',
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

export function PodExecModal({
  namespace,
  podName,
  containers,
  initialContainer,
  onClose,
}: {
  namespace: string
  podName: string
  containers: K8sContainerStatus[]
  initialContainer: string
  onClose: () => void
}) {
  const containerRef = useRef<HTMLDivElement | null>(null)
  const termRef = useRef<XTerm | null>(null)
  const fitRef = useRef<FitAddon | null>(null)
  const wsRef = useRef<WebSocket | null>(null)

  const [status, setStatus] = useState<Status>('connecting')
  const [error, setError] = useState<string | null>(null)
  const [generation, setGeneration] = useState(0)

  const [selectedContainer, setSelectedContainer] = useState(initialContainer)
  const [selectedCommand, setSelectedCommand] = useState('/bin/sh')
  const [customCommand, setCustomCommand] = useState('')
  const [useCustom, setUseCustom] = useState(false)

  // Resolved command array — never empty
  const resolvedCommand = useCustom
    ? customCommand.trim() || '/bin/sh'
    : selectedCommand

  const sendBytes = useCallback((data: Uint8Array) => {
    const ws = wsRef.current
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(data.buffer.slice(data.byteOffset, data.byteOffset + data.byteLength))
    }
  }, [])

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

    try {
      fit.fit()
    } catch {
      // ignore; container may still be measuring
    }
    const dims = fit.proposeDimensions() ?? { rows: 24, cols: 80 }

    const ws = new WebSocket(podExecURL(namespace, podName))
    ws.binaryType = 'arraybuffer'
    wsRef.current = ws

    ws.onopen = () => {
      setStatus('open')
      ws.send(
        JSON.stringify({
          rows: dims.rows,
          cols: dims.cols,
          container: selectedContainer,
          command: [resolvedCommand],
        })
      )
      term.focus()
    }
    ws.onmessage = (evt) => {
      if (typeof evt.data === 'string') {
        try {
          const msg = JSON.parse(evt.data) as { type?: string; err?: string }
          if (msg.type === 'error') {
            setError(msg.err ?? 'exec error')
          }
        } catch {
          // ignore
        }
        return
      }
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
    // generation is the reconnect key; selectedContainer + resolvedCommand change it via handlers below
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [generation, sendBytes])

  function handleContainerChange(name: string) {
    setSelectedContainer(name)
    setGeneration((g) => g + 1)
  }

  function handleCommandChange(val: string) {
    if (val === '__custom__') {
      setUseCustom(true)
    } else {
      setUseCustom(false)
      setSelectedCommand(val)
      setGeneration((g) => g + 1)
    }
  }

  function handleCustomCommandCommit() {
    if (!customCommand.trim()) return
    setGeneration((g) => g + 1)
  }

  return (
    <Sheet open onOpenChange={(open) => { if (!open) onClose() }}>
      <SheetContent className="flex flex-col sm:max-w-3xl">
        <SheetHeader className="shrink-0">
          <div className="flex items-center gap-2">
            <TerminalIcon className="h-4 w-4 text-muted-foreground" aria-hidden />
            <SheetTitle className="font-mono text-sm">
              {podName}
            </SheetTitle>
            <StatusPill status={status} />
          </div>
          <SheetDescription className="sr-only">
            Interactive exec shell for pod {podName}
          </SheetDescription>
        </SheetHeader>

        {/* Top bar: container + command pickers + reconnect */}
        <div className="flex flex-wrap items-center gap-2 shrink-0 pt-1">
          <select
            value={selectedContainer}
            onChange={(e) => handleContainerChange(e.target.value)}
            className="h-7 rounded border bg-background px-2 text-xs text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
          >
            {containers.map((c) => (
              <option key={c.name} value={c.name}>
                {c.name}
              </option>
            ))}
          </select>

          <select
            value={useCustom ? '__custom__' : selectedCommand}
            onChange={(e) => handleCommandChange(e.target.value)}
            className="h-7 rounded border bg-background px-2 text-xs text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
          >
            {COMMAND_OPTIONS.map((cmd) => (
              <option key={cmd} value={cmd}>
                {cmd}
              </option>
            ))}
            <option value="__custom__">Custom…</option>
          </select>

          {useCustom && (
            <input
              type="text"
              value={customCommand}
              onChange={(e) => setCustomCommand(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter') handleCustomCommandCommit() }}
              placeholder="/bin/sh"
              className="h-7 rounded border bg-background px-2 text-xs text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
            />
          )}

          <Button
            variant="outline"
            size="sm"
            className="h-7 text-xs"
            onClick={() => setGeneration((g) => g + 1)}
            disabled={status === 'connecting'}
          >
            <RotateCcw className="h-3 w-3" aria-hidden />
            {status === 'open' ? 'Reset' : 'Reconnect'}
          </Button>
        </div>

        {error && (
          <Alert variant="destructive" className="shrink-0">
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        )}

        {/* Terminal area */}
        <div className="flex-1 overflow-hidden rounded border bg-black/80 p-2 min-h-0">
          <div ref={containerRef} className="h-full w-full" />
        </div>

        {/* Footer */}
        <p className="shrink-0 text-xs text-muted-foreground">
          Connected as the pod's user. exec exits when the shell exits.
        </p>
      </SheetContent>
    </Sheet>
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
          status === 'open'
            ? 'bg-emerald-500 animate-pulse'
            : status === 'connecting'
            ? 'bg-amber-500 animate-pulse'
            : status === 'error'
            ? 'bg-destructive'
            : 'bg-muted-foreground'
        }`}
        aria-hidden
      />
      {label}
    </span>
  )
}
