import { useEffect, useRef } from 'react'
import { CheckCircle2, Loader2, XCircle } from 'lucide-react'

import { Badge } from '@/components/ui/badge'
import type { JobState } from '@/lib/updates'

export function JobConsole({
  state,
  output,
  error,
}: {
  state: JobState | null
  output: string
  error: string | null
}) {
  const ref = useRef<HTMLPreElement | null>(null)
  // Auto-scroll to the bottom unless the user has scrolled up.
  useEffect(() => {
    const el = ref.current
    if (!el) return
    const nearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 50
    if (nearBottom) el.scrollTop = el.scrollHeight
  }, [output])

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center gap-2 text-xs">
        <StateBadge state={state} />
        {error && <span className="text-destructive">{error}</span>}
      </div>
      <pre
        ref={ref}
        className="h-72 overflow-auto rounded-md border bg-background p-3 font-mono text-[11px] leading-relaxed whitespace-pre-wrap break-all"
      >
        {output || (state === 'running' ? 'Waiting for output…' : '(no output)')}
      </pre>
    </div>
  )
}

function StateBadge({ state }: { state: JobState | null }) {
  if (state === 'running') {
    return (
      <Badge variant="warn" className="inline-flex items-center gap-1">
        <Loader2 className="h-3 w-3 animate-spin" aria-hidden />
        Running
      </Badge>
    )
  }
  if (state === 'succeeded') {
    return (
      <Badge variant="success" className="inline-flex items-center gap-1">
        <CheckCircle2 className="h-3 w-3" aria-hidden />
        Done
      </Badge>
    )
  }
  if (state === 'failed' || state === 'cancelled') {
    return (
      <Badge variant="danger" className="inline-flex items-center gap-1">
        <XCircle className="h-3 w-3" aria-hidden />
        {state}
      </Badge>
    )
  }
  return <Badge variant="muted">—</Badge>
}
