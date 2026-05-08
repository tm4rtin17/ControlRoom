import { useState } from 'react'
import { Plus, Power, Trash2, Wifi } from 'lucide-react'

import {
  type AddRuleSpec,
  type NetworkInterface,
  useAddRule,
  useDeleteRule,
  useFirewall,
  useFirewallToggle,
  useInterfaces,
} from '@/lib/network'
import { formatBytes } from '@/lib/format'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

export function Network() {
  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-baseline justify-between">
        <h1 className="text-xl font-semibold tracking-tight">Network</h1>
      </div>

      <InterfacesCard />
      <FirewallCard />
    </div>
  )
}

function InterfacesCard() {
  const ifaces = useInterfaces()
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm">Interfaces</CardTitle>
      </CardHeader>
      <CardContent className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        {ifaces.data?.interfaces.map((i) => (
          <InterfaceRow key={i.name} iface={i} />
        ))}
        {ifaces.isLoading && <p className="text-sm text-muted-foreground">Loading…</p>}
      </CardContent>
    </Card>
  )
}

function InterfaceRow({ iface }: { iface: NetworkInterface }) {
  const stateVariant: 'success' | 'muted' | 'danger' =
    iface.state === 'UP' ? 'success' : iface.state === 'DOWN' ? 'danger' : 'muted'
  return (
    <div className="rounded-lg border bg-card p-3">
      <div className="flex items-center gap-2">
        <Wifi className="h-4 w-4 text-muted-foreground" aria-hidden />
        <span className="font-mono text-sm">{iface.name}</span>
        <Badge variant={stateVariant}>{iface.state}</Badge>
        {iface.mtu > 0 && (
          <span className="text-[10px] text-muted-foreground">MTU {iface.mtu}</span>
        )}
      </div>
      <div className="mt-2 flex flex-col gap-0.5 text-xs">
        {iface.ips.length === 0 ? (
          <span className="text-muted-foreground">No addresses</span>
        ) : (
          iface.ips.map((ip) => (
            <span key={ip} className="font-mono">
              {ip}
            </span>
          ))
        )}
      </div>
      <div className="mt-2 flex justify-between text-[11px] text-muted-foreground">
        <span className="font-mono">{iface.mac}</span>
        <span className="font-mono tabular-nums">
          ↓ {formatBytes(iface.stats.rx_bytes)} · ↑ {formatBytes(iface.stats.tx_bytes)}
        </span>
      </div>
    </div>
  )
}

function FirewallCard() {
  const fw = useFirewall()
  const toggle = useFirewallToggle()
  const del = useDeleteRule()

  return (
    <Card>
      <CardHeader className="flex-row items-start justify-between gap-2 space-y-0">
        <div>
          <CardTitle className="text-sm">Firewall (UFW)</CardTitle>
          {fw.data?.default && (
            <p className="mt-1 text-xs text-muted-foreground">{fw.data.default}</p>
          )}
        </div>
        <Button
          size="sm"
          variant={fw.data?.active ? 'destructive' : 'default'}
          disabled={toggle.isPending}
          onClick={() => {
            if (!fw.data) return
            const enable = !fw.data.active
            if (!enable && !confirm('Disable the firewall?')) return
            toggle.mutate(enable)
          }}
        >
          <Power className="h-3.5 w-3.5" aria-hidden />
          {fw.data?.active ? 'Disable' : 'Enable'}
        </Button>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        {fw.error && (
          <Alert variant="destructive">
            <AlertDescription>UFW unreachable. Is it installed?</AlertDescription>
          </Alert>
        )}
        {fw.data && (
          <>
            {fw.data.rules.length === 0 ? (
              <p className="text-sm text-muted-foreground">No rules.</p>
            ) : (
              <div className="overflow-hidden rounded-md border">
                <table className="w-full text-sm">
                  <thead className="bg-muted/30 text-xs uppercase tracking-wider text-muted-foreground">
                    <tr>
                      <th className="px-3 py-2 text-left">#</th>
                      <th className="px-3 py-2 text-left">To</th>
                      <th className="px-3 py-2 text-left">Action</th>
                      <th className="px-3 py-2 text-left">From</th>
                      <th className="px-3 py-2"></th>
                    </tr>
                  </thead>
                  <tbody>
                    {fw.data.rules.map((r) => (
                      <tr key={r.index} className="border-t">
                        <td className="px-3 py-2 text-xs text-muted-foreground">{r.index}</td>
                        <td className="px-3 py-2 font-mono text-xs">{r.to}</td>
                        <td className="px-3 py-2 text-xs">
                          <Badge variant={r.action.startsWith('ALLOW') ? 'success' : 'danger'}>
                            {r.action}
                          </Badge>
                        </td>
                        <td className="px-3 py-2 font-mono text-xs">{r.from}</td>
                        <td className="px-3 py-2 text-right">
                          <Button
                            size="icon"
                            variant="ghost"
                            aria-label={`Delete rule ${r.index}`}
                            onClick={() => {
                              if (confirm(`Delete rule ${r.index}?`)) del.mutate(r.index)
                            }}
                          >
                            <Trash2 className="h-3.5 w-3.5" aria-hidden />
                          </Button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}

            <AddRuleForm />
          </>
        )}
      </CardContent>
    </Card>
  )
}

function AddRuleForm() {
  const add = useAddRule()
  const [spec, setSpec] = useState<AddRuleSpec>({ action: 'allow' })
  const [error, setError] = useState<string | null>(null)

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    try {
      await add.mutateAsync(spec)
      setSpec({ action: 'allow' })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not add rule')
    }
  }

  return (
    <form onSubmit={onSubmit} className="flex flex-col gap-3">
      <p className="text-xs uppercase tracking-wider text-muted-foreground">Add rule</p>
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-5">
        <div className="flex flex-col gap-1">
          <Label htmlFor="action" className="text-xs">Action</Label>
          <select
            id="action"
            className="h-10 rounded-md border bg-background px-2 text-sm"
            value={spec.action}
            onChange={(e) => setSpec({ ...spec, action: e.target.value as AddRuleSpec['action'] })}
          >
            <option value="allow">allow</option>
            <option value="deny">deny</option>
            <option value="reject">reject</option>
            <option value="limit">limit</option>
          </select>
        </div>
        <div className="flex flex-col gap-1">
          <Label htmlFor="port" className="text-xs">Port</Label>
          <Input
            id="port"
            placeholder="22 or 80,443"
            value={spec.port ?? ''}
            onChange={(e) => setSpec({ ...spec, port: e.target.value })}
          />
        </div>
        <div className="flex flex-col gap-1">
          <Label htmlFor="proto" className="text-xs">Proto</Label>
          <select
            id="proto"
            className="h-10 rounded-md border bg-background px-2 text-sm"
            value={spec.protocol ?? ''}
            onChange={(e) =>
              setSpec({
                ...spec,
                protocol: (e.target.value || undefined) as 'tcp' | 'udp' | undefined,
              })
            }
          >
            <option value="">—</option>
            <option value="tcp">tcp</option>
            <option value="udp">udp</option>
          </select>
        </div>
        <div className="flex flex-col gap-1 sm:col-span-2">
          <Label htmlFor="from" className="text-xs">From (optional)</Label>
          <Input
            id="from"
            placeholder="192.168.1.0/24"
            value={spec.from ?? ''}
            onChange={(e) => setSpec({ ...spec, from: e.target.value })}
          />
        </div>
      </div>
      {error && <p className="text-xs text-destructive">{error}</p>}
      <div className="flex justify-end">
        <Button type="submit" size="sm" disabled={add.isPending}>
          <Plus className="h-3.5 w-3.5" aria-hidden />
          Add rule
        </Button>
      </div>
    </form>
  )
}
