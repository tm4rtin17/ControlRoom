import { useEffect, useState } from 'react'
import { AlertTriangle, Download, Power, RefreshCw, ShieldAlert } from 'lucide-react'

import { ApiError } from '@/lib/api'
import {
  type UpdatePackage,
  useJobStream,
  useReboot,
  useStartApply,
  useStartCheck,
  useUpdatesList,
} from '@/lib/updates'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { JobConsole } from '@/components/updates/JobConsole'

type DrawerKind = 'check' | 'apply' | 'reboot' | null

export function Updates() {
  const list = useUpdatesList()
  const startCheck = useStartCheck()
  const startApply = useStartApply()
  const reboot = useReboot()
  const [drawer, setDrawer] = useState<DrawerKind>(null)
  const [jobId, setJobId] = useState<string | null>(null)
  const [confirmInstall, setConfirmInstall] = useState(false)
  const [rebootText, setRebootText] = useState('')

  const job = useJobStream(jobId)
  // Auto-refresh package list once a job lands successfully.
  useEffect(() => {
    if (job.state === 'succeeded' || job.state === 'failed' || job.state === 'cancelled') {
      list.refetch()
    }
  }, [job.state]) // eslint-disable-line react-hooks/exhaustive-deps

  const packages = list.data?.packages ?? []
  const securityCount = packages.filter((p) => p.security).length

  async function onCheck() {
    setDrawer('check')
    const r = await startCheck.mutateAsync()
    setJobId(r.job_id)
  }
  async function onApply() {
    if (startApply.isPending) return
    setConfirmInstall(false)
    setDrawer('apply')
    try {
      const r = await startApply.mutateAsync()
      setJobId(r.job_id)
    } catch (err) {
      // 409 means an upgrade is already in progress; surface it.
      if (err instanceof ApiError && err.status === 409) {
        const otherID = (err.detail as { job_id?: string } | undefined)?.job_id
        if (otherID) setJobId(otherID)
      }
    }
  }
  async function onReboot() {
    if (rebootText !== 'REBOOT') return
    setDrawer('reboot')
    try {
      const r = await reboot.mutateAsync()
      setJobId(r.job_id)
    } finally {
      setRebootText('')
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h1 className="text-xl font-semibold tracking-tight">Updates</h1>
        <div className="flex flex-wrap gap-2">
          <Button variant="outline" size="sm" onClick={onCheck} disabled={startCheck.isPending}>
            <RefreshCw className="h-3.5 w-3.5" aria-hidden />
            Check
          </Button>
          <Button size="sm" onClick={() => setConfirmInstall(true)} disabled={packages.length === 0}>
            <Download className="h-3.5 w-3.5" aria-hidden />
            Install {packages.length} update{packages.length === 1 ? '' : 's'}
          </Button>
        </div>
      </div>

      {list.data?.reboot_required && (
        <Alert variant="destructive">
          <AlertTitle className="flex items-center gap-2">
            <AlertTriangle className="h-4 w-4" aria-hidden />
            Reboot required
          </AlertTitle>
          <AlertDescription className="flex flex-wrap items-center justify-between gap-3">
            <span>One or more recent upgrades require a reboot to take effect.</span>
            <RebootControl
              text={rebootText}
              setText={setRebootText}
              onReboot={onReboot}
              pending={reboot.isPending}
            />
          </AlertDescription>
        </Alert>
      )}

      {securityCount > 0 && (
        <Alert>
          <AlertTitle className="flex items-center gap-2">
            <ShieldAlert className="h-4 w-4 text-amber-500" aria-hidden />
            {securityCount} security update{securityCount === 1 ? '' : 's'}
          </AlertTitle>
          <AlertDescription>
            Listed below in the table. Install when convenient.
          </AlertDescription>
        </Alert>
      )}

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-sm">Available packages</CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          <div className="border-t">
            {list.isLoading ? (
              <Empty label="Loading…" />
            ) : packages.length === 0 ? (
              <Empty label="System is up to date" />
            ) : (
              <ul className="divide-y">
                {packages.map((p) => (
                  <PackageRow key={`${p.name}-${p.arch}`} pkg={p} />
                ))}
              </ul>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Confirm install modal — built on Sheet for now; M9 polish swaps in shadcn AlertDialog. */}
      <Sheet open={confirmInstall} onOpenChange={setConfirmInstall}>
        <SheetContent side="right">
          <SheetHeader>
            <SheetTitle>Install {packages.length} update{packages.length === 1 ? '' : 's'}?</SheetTitle>
            <SheetDescription>
              Runs <code>apt-get -y upgrade</code> with <code>--force-confold</code>. Configuration files
              left in place; package authors' new defaults are saved as <code>.dpkg-dist</code>.
            </SheetDescription>
          </SheetHeader>
          <ul className="mt-4 max-h-[60vh] overflow-auto text-xs">
            {packages.map((p) => (
              <li key={p.name} className="flex items-baseline justify-between gap-2 border-b py-1.5">
                <span className="font-mono">{p.name}</span>
                <span className="text-muted-foreground">
                  {p.old_version} → {p.new_version}
                </span>
              </li>
            ))}
          </ul>
          <div className="mt-4 flex justify-end gap-2">
            <Button variant="ghost" onClick={() => setConfirmInstall(false)}>Cancel</Button>
            <Button onClick={onApply} disabled={startApply.isPending}>
              {startApply.isPending ? 'Starting…' : 'Install'}
            </Button>
          </div>
        </SheetContent>
      </Sheet>

      {/* Job progress drawer */}
      <Sheet
        open={drawer !== null}
        onOpenChange={(open) => {
          if (!open) {
            setDrawer(null)
            setJobId(null)
          }
        }}
      >
        <SheetContent side="right">
          <SheetHeader>
            <SheetTitle>
              {drawer === 'check'
                ? 'Checking for updates'
                : drawer === 'apply'
                  ? 'Installing updates'
                  : 'Rebooting'}
            </SheetTitle>
            <SheetDescription>
              {drawer === 'reboot'
                ? 'The system will go down momentarily. ControlRoom itself will restart.'
                : 'You can close this drawer and the job will keep running in the background.'}
            </SheetDescription>
          </SheetHeader>
          <div className="mt-4">
            <JobConsole state={job.state} output={job.output} error={job.error} />
          </div>
        </SheetContent>
      </Sheet>
    </div>
  )
}

function PackageRow({ pkg }: { pkg: UpdatePackage }) {
  return (
    <li className="flex items-center gap-3 px-4 py-3">
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="truncate font-mono text-sm">{pkg.name}</span>
          {pkg.security && (
            <Badge variant="warn" className="inline-flex items-center gap-1 text-[10px]">
              <ShieldAlert className="h-3 w-3" aria-hidden />
              security
            </Badge>
          )}
        </div>
        <p className="truncate text-xs text-muted-foreground">
          {pkg.old_version} → {pkg.new_version} · {pkg.source} · {pkg.arch}
        </p>
      </div>
    </li>
  )
}

function RebootControl({
  text,
  setText,
  onReboot,
  pending,
}: {
  text: string
  setText: (s: string) => void
  onReboot: () => void
  pending: boolean
}) {
  return (
    <div className="flex items-center gap-2">
      <Input
        value={text}
        onChange={(e) => setText(e.target.value)}
        placeholder="Type REBOOT"
        className="w-32 text-xs"
      />
      <Button
        size="sm"
        variant="destructive"
        disabled={text !== 'REBOOT' || pending}
        onClick={onReboot}
      >
        <Power className="h-3.5 w-3.5" aria-hidden />
        Reboot now
      </Button>
    </div>
  )
}

function Empty({ label }: { label: string }) {
  return <div className="px-4 py-8 text-center text-sm text-muted-foreground">{label}</div>
}
