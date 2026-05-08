import { useState } from 'react'
import { NavLink } from 'react-router-dom'
import {
  Activity,
  Boxes,
  Cog,
  Container,
  Download,
  LogOut,
  Menu,
  Package,
  ScrollText,
  ServerCog,
  Settings,
  Terminal as TerminalIcon,
  Wifi,
  X,
} from 'lucide-react'

import { Button } from '@/components/ui/button'
import { ThemeToggle } from '@/components/ThemeToggle'
import { PublicBindBanner } from '@/components/PublicBindBanner'
import { useLogout, useMe } from '@/lib/auth'
import { type SystemCapabilities, useCapabilities, useSystemOverview } from '@/lib/system'
import { cn } from '@/lib/utils'
import { useNavigate } from 'react-router-dom'

// `requires` names a backend capability that must be true (per
// /api/system/capabilities) for the entry to render. Unset = always show.
type NavEntry = {
  to: string
  label: string
  Icon: typeof Activity
  ready?: boolean
  requires?: keyof SystemCapabilities
}

const NAV: NavEntry[] = [
  { to: '/', label: 'Dashboard', Icon: Activity, ready: true },
  { to: '/updates', label: 'Updates', Icon: Download, ready: true },
  { to: '/services', label: 'Services', Icon: Cog, ready: true, requires: 'systemd' },
  { to: '/containers', label: 'Containers', Icon: Container, ready: true, requires: 'docker' },
  { to: '/kubernetes', label: 'Kubernetes', Icon: Package, ready: true, requires: 'kubernetes' },
  { to: '/terminal', label: 'Terminal', Icon: TerminalIcon, ready: true },
  { to: '/network', label: 'Network', Icon: Wifi, ready: true },
  { to: '/logs', label: 'Logs', Icon: ScrollText, ready: true },
  { to: '/settings', label: 'Settings', Icon: Settings, ready: true },
]

export function AppLayout({ children }: { children: React.ReactNode }) {
  const [drawerOpen, setDrawerOpen] = useState(false)
  return (
    <div className="min-h-screen bg-background text-foreground">
      <Sidebar open={drawerOpen} onClose={() => setDrawerOpen(false)} />
      <div className="md:pl-60">
        <TopBar onOpenDrawer={() => setDrawerOpen(true)} />
        <PublicBindBanner />
        <main className="mx-auto w-full max-w-7xl p-4 sm:p-6">{children}</main>
      </div>
    </div>
  )
}

function Sidebar({ open, onClose }: { open: boolean; onClose: () => void }) {
  const caps = useCapabilities()
  // While capabilities are loading we show every entry; if the request fails
  // we also fall back to showing them (better to surface a 503 once than to
  // hide tabs that should be available).
  const visible = NAV.filter((entry) => {
    if (!entry.requires) return true
    if (!caps.data) return true
    return caps.data[entry.requires]
  })

  return (
    <>
      {/* Mobile drawer overlay */}
      {open && (
        <div
          aria-hidden
          className="fixed inset-0 z-40 bg-background/80 backdrop-blur-sm md:hidden"
          onClick={onClose}
        />
      )}
      <aside
        className={cn(
          'fixed inset-y-0 left-0 z-50 flex w-60 flex-col border-r border-border/60 bg-card transition-transform md:translate-x-0',
          open ? 'translate-x-0' : '-translate-x-full'
        )}
        aria-label="Primary"
      >
        <div className="flex items-center justify-between gap-2 px-4 py-4">
          <Brand />
          <button
            className="md:hidden text-muted-foreground hover:text-foreground"
            onClick={onClose}
            aria-label="Close menu"
          >
            <X className="h-5 w-5" aria-hidden />
          </button>
        </div>
        <nav className="flex-1 space-y-0.5 px-2">
          {visible.map(({ to, label, Icon, ready }) => (
            <NavLink
              key={to}
              to={to}
              end={to === '/'}
              onClick={onClose}
              className={({ isActive }) =>
                cn(
                  'flex items-center gap-3 rounded-md px-3 py-2 text-sm transition-colors',
                  isActive
                    ? 'bg-primary/10 text-foreground'
                    : 'text-muted-foreground hover:bg-accent hover:text-foreground',
                  !ready && 'opacity-50'
                )
              }
            >
              <Icon className="h-4 w-4" aria-hidden />
              <span>{label}</span>
              {!ready && <span className="ml-auto text-[10px] uppercase tracking-wider">soon</span>}
            </NavLink>
          ))}
        </nav>
        <SidebarFooter />
      </aside>
    </>
  )
}

function Brand() {
  return (
    <div className="flex items-center gap-2">
      <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary/10 ring-1 ring-primary/20">
        <ServerCog className="h-4 w-4 text-primary" aria-hidden />
      </div>
      <span className="text-sm font-semibold tracking-tight">ControlRoom</span>
    </div>
  )
}

function SidebarFooter() {
  const me = useMe()
  const logout = useLogout()
  const navigate = useNavigate()
  return (
    <div className="border-t border-border/60 p-3">
      <div className="flex items-center justify-between">
        <div className="min-w-0 text-xs">
          <div className="truncate font-medium">{me.data?.username ?? '—'}</div>
          <div className="truncate text-muted-foreground">{me.data?.role ?? ''}</div>
        </div>
        <Button
          variant="ghost"
          size="icon"
          aria-label="Log out"
          onClick={async () => {
            await logout.mutateAsync()
            navigate('/login', { replace: true })
          }}
        >
          <LogOut className="h-4 w-4" aria-hidden />
        </Button>
      </div>
      <div className="mt-3 flex items-center justify-between gap-2">
        <ThemeToggle />
        <span className="text-[10px] uppercase tracking-wider text-muted-foreground">v0.1</span>
      </div>
    </div>
  )
}

function TopBar({ onOpenDrawer }: { onOpenDrawer: () => void }) {
  const overview = useSystemOverview()
  const host = overview.data?.host?.hostname
  const distro = overview.data?.host?.distro

  return (
    <header className="sticky top-0 z-30 flex h-14 items-center gap-3 border-b border-border/60 bg-background/80 px-4 backdrop-blur sm:px-6">
      <button
        className="md:hidden text-muted-foreground hover:text-foreground"
        onClick={onOpenDrawer}
        aria-label="Open menu"
      >
        <Menu className="h-5 w-5" aria-hidden />
      </button>
      <div className="flex min-w-0 flex-1 items-center gap-3">
        <div className="min-w-0">
          <div className="flex items-center gap-2 text-sm font-medium">
            <Boxes className="h-4 w-4 text-muted-foreground" aria-hidden />
            <span className="truncate">{host ?? 'localhost'}</span>
          </div>
          {distro && (
            <p className="truncate text-xs text-muted-foreground">{distro}</p>
          )}
        </div>
      </div>
      <StatusPill state={overview.isError ? 'error' : overview.data ? 'ok' : 'pending'} />
    </header>
  )
}

function StatusPill({ state }: { state: 'ok' | 'pending' | 'error' }) {
  const styles: Record<typeof state, string> = {
    ok: 'bg-emerald-500/10 text-emerald-500 ring-emerald-500/30',
    pending: 'bg-amber-500/10 text-amber-500 ring-amber-500/30',
    error: 'bg-destructive/10 text-destructive ring-destructive/30',
  }
  const label = state === 'ok' ? 'Live' : state === 'pending' ? 'Connecting' : 'Offline'
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs ring-1',
        styles[state]
      )}
    >
      <span
        className={cn(
          'inline-block h-1.5 w-1.5 rounded-full',
          state === 'ok' ? 'bg-emerald-500 animate-pulse' : state === 'pending' ? 'bg-amber-500 animate-pulse' : 'bg-destructive'
        )}
        aria-hidden
      />
      {label}
    </span>
  )
}
