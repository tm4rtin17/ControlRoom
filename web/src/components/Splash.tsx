import { ServerCog } from 'lucide-react'

// Splash is shown during gate transitions (setup-status / me checks). It's
// intentionally minimal: no network calls of its own, no flicker on fast loads.
export function Splash() {
  return (
    <div className="min-h-screen flex flex-col items-center justify-center gap-4 bg-background p-6">
      <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-primary/10 ring-1 ring-primary/20">
        <ServerCog className="h-7 w-7 text-primary animate-pulse" aria-hidden />
      </div>
      <p className="text-sm text-muted-foreground">Connecting…</p>
    </div>
  )
}
