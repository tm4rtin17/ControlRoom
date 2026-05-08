import { ArrowDown, ArrowLeft, ArrowRight, ArrowUp } from 'lucide-react'

import { cn } from '@/lib/utils'

// SoftKeys is a row of one-tap terminal shortcuts for mobile, where most of
// these characters aren't on the soft keyboard. Each button calls back with
// the raw bytes to feed into the PTY.
export function SoftKeys({ onKey }: { onKey: (bytes: Uint8Array) => void }) {
  const send = (s: string) => onKey(new TextEncoder().encode(s))
  return (
    <div className="flex items-center gap-1 overflow-x-auto pb-1">
      <Key onClick={() => onKey(new Uint8Array([0x03]))} label="Ctrl-C" />
      <Key onClick={() => onKey(new Uint8Array([0x04]))} label="Ctrl-D" />
      <Key onClick={() => send('\t')} label="Tab" />
      <Key onClick={() => send('\x1b')} label="Esc" />
      <Key onClick={() => send('\x1b[A')} ariaLabel="Up arrow">
        <ArrowUp className="h-3.5 w-3.5" aria-hidden />
      </Key>
      <Key onClick={() => send('\x1b[B')} ariaLabel="Down arrow">
        <ArrowDown className="h-3.5 w-3.5" aria-hidden />
      </Key>
      <Key onClick={() => send('\x1b[D')} ariaLabel="Left arrow">
        <ArrowLeft className="h-3.5 w-3.5" aria-hidden />
      </Key>
      <Key onClick={() => send('\x1b[C')} ariaLabel="Right arrow">
        <ArrowRight className="h-3.5 w-3.5" aria-hidden />
      </Key>
    </div>
  )
}

function Key({
  onClick,
  label,
  ariaLabel,
  children,
  className,
}: {
  onClick: () => void
  label?: string
  ariaLabel?: string
  children?: React.ReactNode
  className?: string
}) {
  return (
    <button
      type="button"
      aria-label={ariaLabel ?? label}
      onClick={onClick}
      className={cn(
        'inline-flex h-8 min-w-[2.25rem] items-center justify-center rounded-md border bg-card px-2 text-xs font-medium text-foreground hover:bg-accent active:bg-accent/70',
        className
      )}
    >
      {children ?? label}
    </button>
  )
}
