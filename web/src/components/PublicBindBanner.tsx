import { ShieldAlert } from 'lucide-react'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'

// PublicBindBanner warns when the user is loading the SPA from an address
// that looks public — i.e. the host running ControlRoom is reachable from a
// non-RFC1918 / non-loopback IP. Best-effort heuristic; the operator's
// browser only sees the URL it dialed.
export function PublicBindBanner() {
  if (!isPubliclyAccessed(window.location.hostname)) return null
  return (
    <Alert variant="destructive" className="mx-4 mt-4 sm:mx-6">
      <AlertTitle className="flex items-center gap-2">
        <ShieldAlert className="h-4 w-4" aria-hidden />
        Public-looking address
      </AlertTitle>
      <AlertDescription>
        You're connecting to ControlRoom via <code>{window.location.host}</code>, which doesn't
        look like a private (RFC 1918) or loopback address. Consider putting it behind a VPN or
        reverse proxy with strict firewall rules — this admin surface is not designed to be
        exposed directly to the public internet.
      </AlertDescription>
    </Alert>
  )
}

function isPubliclyAccessed(host: string): boolean {
  if (!host) return false

  // Hostnames (anything non-IP) are ambiguous; we just leave them alone.
  if (!isIP(host)) return false

  // Loopback.
  if (host === '127.0.0.1' || host === '::1' || host.startsWith('127.')) return false

  // RFC 1918 + link-local.
  if (host.startsWith('10.')) return false
  if (host.startsWith('192.168.')) return false
  if (host.startsWith('169.254.')) return false
  if (host.startsWith('172.')) {
    const second = parseInt(host.split('.')[1] ?? '', 10)
    if (second >= 16 && second <= 31) return false
  }

  // IPv6 unique-local (fc00::/7) and link-local (fe80::/10).
  const lower = host.toLowerCase()
  if (lower.startsWith('fc') || lower.startsWith('fd')) return false
  if (lower.startsWith('fe8') || lower.startsWith('fe9') || lower.startsWith('fea') || lower.startsWith('feb')) return false

  return true
}

function isIP(host: string): boolean {
  // Quick-and-dirty: dotted-quad or contains a colon (IPv6).
  if (/^[0-9.]+$/.test(host)) return true
  if (host.includes(':')) return true
  return false
}
