import { useState } from 'react'
import { Loader2, Terminal as TerminalIcon } from 'lucide-react'

import { useK8sPodDetail, useDeletePod } from '@/lib/k8s'
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription } from '@/components/ui/sheet'
import { PodExecModal } from './PodExecModal'
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogCancel,
  AlertDialogAction,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import { K8sPodStatusPill } from './K8sStatusPill'
import { ConditionsTable } from './ConditionsTable'
import { EventsTable } from './EventsTable'
import { PodLogStream } from './PodLogStream'

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <p className="mb-2 text-xs uppercase tracking-wider text-muted-foreground">{title}</p>
      {children}
    </div>
  )
}

export function PodDetail({
  namespace,
  name,
  open,
  onOpenChange,
  onClose,
}: {
  namespace: string | null
  name: string | null
  open: boolean
  onOpenChange: (open: boolean) => void
  onClose?: () => void
}) {
  const { data } = useK8sPodDetail(open ? namespace : null, open ? name : null)
  const [activeContainer, setActiveContainer] = useState<string | null>(null)
  const [execContainer, setExecContainer] = useState<string | null>(null)
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [force, setForce] = useState(false)

  const deletePod = useDeletePod()

  const containers = data?.containers ?? []
  const resolvedContainer =
    (activeContainer && containers.find((c) => c.name === activeContainer)?.name) ||
    containers[0]?.name ||
    null

  function handleDeleteConfirm() {
    if (!namespace || !name) return
    deletePod.mutate(
      { namespace, name, force },
      {
        onSuccess: () => {
          setDeleteOpen(false)
          onClose?.()
          onOpenChange(false)
        },
      }
    )
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="overflow-y-auto">
        <SheetHeader>
          <div className="flex items-start justify-between gap-2">
            <div className="min-w-0">
              <SheetTitle className="font-mono break-all">{data?.pod.name ?? name}</SheetTitle>
              <SheetDescription>
                {data
                  ? `${data.pod.namespace} · ${data.node_name || '—'} · ${data.pod.pod_ip || '—'} · ${data.qos_class}`
                  : (namespace ?? '')}
              </SheetDescription>
            </div>
            <div className="shrink-0 pt-0.5">
              <Button size="sm" variant="destructive" onClick={() => setDeleteOpen(true)}>
                Delete pod
              </Button>
            </div>
          </div>
        </SheetHeader>

        {deletePod.isError && (
          <Alert variant="destructive" className="mt-3">
            <AlertDescription>{(deletePod.error as Error).message}</AlertDescription>
          </Alert>
        )}

        {data && (
          <div className="mt-4 flex flex-col gap-6">
            <div className="flex items-center gap-2">
              <K8sPodStatusPill status={data.pod.status} />
            </div>

            <Section title="Containers">
              <div className="overflow-x-auto">
                <table className="w-full text-xs">
                  <thead>
                    <tr className="border-b text-left text-muted-foreground">
                      <th className="py-1.5 pr-3 font-medium">Name</th>
                      <th className="py-1.5 pr-3 font-medium">Image</th>
                      <th className="py-1.5 pr-3 font-medium">Ready</th>
                      <th className="py-1.5 pr-3 font-medium">Restarts</th>
                      <th className="py-1.5 pr-3 font-medium">State</th>
                      <th className="py-1.5 font-medium" />
                    </tr>
                  </thead>
                  <tbody className="divide-y">
                    {containers.map((c) => (
                      <tr
                        key={c.name}
                        onClick={() => setActiveContainer(c.name)}
                        className={cn(
                          'cursor-pointer hover:bg-accent/30',
                          resolvedContainer === c.name && 'bg-accent/50'
                        )}
                      >
                        <td className="py-1.5 pr-3 font-mono font-medium">{c.name}</td>
                        <td className="py-1.5 pr-3 font-mono text-muted-foreground max-w-[160px] truncate" title={c.image}>
                          {c.image}
                        </td>
                        <td className="py-1.5 pr-3">
                          <Badge variant={c.ready ? 'success' : 'danger'}>{c.ready ? 'Yes' : 'No'}</Badge>
                        </td>
                        <td className="py-1.5 pr-3">
                          <span className={cn('tabular-nums', c.restart_count > 0 && 'text-amber-500')}>
                            {c.restart_count}
                          </span>
                        </td>
                        <td className="py-1.5 pr-3">
                          <Badge variant={c.state === 'running' ? 'success' : c.state === 'terminated' ? 'muted' : 'warn'}>
                            {c.state}
                          </Badge>
                        </td>
                        <td className="py-1.5">
                          <button
                            title="Open shell"
                            onClick={(e) => { e.stopPropagation(); setExecContainer(c.name) }}
                            className="rounded p-1 text-muted-foreground hover:bg-accent/50 hover:text-foreground transition-colors"
                          >
                            <TerminalIcon className="h-3.5 w-3.5" aria-hidden />
                            <span className="sr-only">Exec into {c.name}</span>
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </Section>

            <Section title="Conditions">
              <ConditionsTable conditions={data.conditions} />
            </Section>

            <Section title="Events">
              <EventsTable events={data.events} />
            </Section>

            {namespace && name && resolvedContainer && (
              <Section title="Logs">
                {containers.length > 1 && (
                  <div className="mb-2 flex flex-wrap gap-1.5">
                    {containers.map((c) => (
                      <button
                        key={c.name}
                        onClick={() => setActiveContainer(c.name)}
                        className={cn(
                          'rounded-full border px-2 py-0.5 text-xs transition-colors',
                          resolvedContainer === c.name
                            ? 'border-primary/30 bg-primary/10 text-foreground'
                            : 'border-transparent bg-muted text-muted-foreground hover:text-foreground'
                        )}
                      >
                        {c.name}
                      </button>
                    ))}
                  </div>
                )}
                <PodLogStream
                  namespace={namespace}
                  podName={name}
                  container={resolvedContainer}
                />
              </Section>
            )}
          </div>
        )}
      </SheetContent>

      {/* Pod exec shell */}
      {namespace && name && execContainer && (
        <PodExecModal
          namespace={namespace}
          podName={name}
          containers={containers}
          initialContainer={execContainer}
          onClose={() => setExecContainer(null)}
        />
      )}

      {/* Delete confirmation */}
      <AlertDialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete pod?</AlertDialogTitle>
            <AlertDialogDescription>
              Delete pod {namespace}/{name}? The controller will recreate it.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <div className="px-1 py-2">
            <label className="flex cursor-pointer items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={force}
                onChange={(e) => setForce(e.target.checked)}
                className="h-4 w-4 rounded border"
              />
              Force (skip graceful shutdown)
            </label>
          </div>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deletePod.isPending}>Cancel</AlertDialogCancel>
            <AlertDialogAction variant="destructive" onClick={handleDeleteConfirm} disabled={deletePod.isPending}>
              {deletePod.isPending && <Loader2 className="h-4 w-4 animate-spin" />}
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Sheet>
  )
}
