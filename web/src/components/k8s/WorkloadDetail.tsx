import { useState } from 'react'
import { Loader2 } from 'lucide-react'

import { useK8sWorkloadDetail, useRestartWorkload, useScaleWorkload } from '@/lib/k8s'
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription } from '@/components/ui/sheet'
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
import { Input } from '@/components/ui/input'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import { ConditionsTable } from './ConditionsTable'
import { EventsTable } from './EventsTable'
import { ManifestEditor } from './ManifestEditor'

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <p className="mb-2 text-xs uppercase tracking-wider text-muted-foreground">{title}</p>
      {children}
    </div>
  )
}

export function WorkloadDetail({
  namespace,
  kind,
  name,
  open,
  onOpenChange,
}: {
  namespace: string | null
  kind: string | null
  name: string | null
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { data } = useK8sWorkloadDetail(
    open ? namespace : null,
    open ? kind : null,
    open ? name : null
  )

  const restart = useRestartWorkload()
  const scale = useScaleWorkload()

  const [restartOpen, setRestartOpen] = useState(false)
  const [scaleOpen, setScaleOpen] = useState(false)
  const [scaleReplicas, setScaleReplicas] = useState<number>(0)
  const [manifestEditorOpen, setManifestEditorOpen] = useState(false)

  const w = data?.workload
  const pct = w && w.ready.desired > 0 ? (w.ready.current / w.ready.desired) * 100 : 0
  const healthy = w ? w.ready.current >= w.ready.desired : false
  const canScale = w?.kind === 'Deployment' || w?.kind === 'StatefulSet'

  function handleScaleOpen() {
    setScaleReplicas(w?.ready.desired ?? 0)
    setScaleOpen(true)
  }

  function handleRestartConfirm() {
    if (!namespace || !kind || !name) return
    restart.mutate({ namespace, kind, name }, { onSuccess: () => setRestartOpen(false) })
  }

  function handleScaleConfirm() {
    if (!namespace || !kind || !name) return
    scale.mutate({ namespace, kind, name, replicas: scaleReplicas }, { onSuccess: () => setScaleOpen(false) })
  }

  return (
  <>
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="overflow-y-auto">
        <SheetHeader>
          <div className="flex items-start justify-between gap-2">
            <div className="min-w-0">
              <SheetTitle className="font-mono break-all">{w?.name ?? name}</SheetTitle>
              <SheetDescription>
                {w ? `${w.kind} · ${w.namespace}` : (kind && namespace ? `${kind} · ${namespace}` : '')}
              </SheetDescription>
            </div>
            <div className="flex shrink-0 gap-2 pt-0.5">
              {canScale && (
                <Button size="sm" variant="outline" onClick={handleScaleOpen}>
                  Scale
                </Button>
              )}
              <Button size="sm" variant="outline" onClick={() => setRestartOpen(true)}>
                Restart
              </Button>
              <Button size="sm" variant="outline" onClick={() => setManifestEditorOpen(true)}>
                Edit YAML
              </Button>
            </div>
          </div>
        </SheetHeader>

        {(restart.isError || scale.isError) && (
          <Alert variant="destructive" className="mt-3">
            <AlertDescription>
              {restart.isError ? (restart.error as Error).message : (scale.error as Error).message}
            </AlertDescription>
          </Alert>
        )}

        {data && w && (
          <div className="mt-4 flex flex-col gap-6">
            {/* Ready bar */}
            <div className="flex items-center gap-3">
              <span className={cn('text-sm tabular-nums font-medium', healthy ? 'text-emerald-500' : 'text-amber-500')}>
                {w.ready.current}/{w.ready.desired} ready
              </span>
              <div className="h-2 flex-1 overflow-hidden rounded-full bg-muted">
                <div
                  className={cn('h-full rounded-full', healthy ? 'bg-emerald-500' : 'bg-amber-500')}
                  style={{ width: `${Math.min(pct, 100)}%` }}
                />
              </div>
            </div>

            <Section title="Strategy">
              <span className="text-sm">{data.strategy || '—'}</span>
            </Section>

            {Object.keys(data.selector).length > 0 && (
              <Section title="Selector">
                <div className="flex flex-wrap gap-1.5">
                  {Object.entries(data.selector).map(([k, v]) => (
                    <Badge key={k} variant="muted">{k}={v}</Badge>
                  ))}
                </div>
              </Section>
            )}

            <Section title="Conditions">
              <ConditionsTable conditions={data.conditions} />
            </Section>

            <Section title="Events">
              <EventsTable events={data.events} />
            </Section>
          </div>
        )}
      </SheetContent>

      {/* Restart confirmation */}
      <AlertDialog open={restartOpen} onOpenChange={setRestartOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Restart workload?</AlertDialogTitle>
            <AlertDialogDescription>
              Trigger a rollout restart of {kind}/{name}? Pods will be recreated one by one
              according to the workload&apos;s strategy.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={restart.isPending}>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={handleRestartConfirm} disabled={restart.isPending}>
              {restart.isPending && <Loader2 className="h-4 w-4 animate-spin" />}
              Restart
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Scale modal */}
      <AlertDialog open={scaleOpen} onOpenChange={setScaleOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Scale workload</AlertDialogTitle>
            <AlertDialogDescription>
              Set the desired replica count for {kind}/{name}.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <div className="px-1 py-2">
            <label className="mb-1.5 block text-sm font-medium">Replicas</label>
            <Input
              type="number"
              min={0}
              max={1000}
              value={scaleReplicas}
              onChange={(e) => setScaleReplicas(Number(e.target.value))}
            />
          </div>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={scale.isPending}>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={handleScaleConfirm} disabled={scale.isPending}>
              {scale.isPending && <Loader2 className="h-4 w-4 animate-spin" />}
              Apply
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Sheet>

    {namespace && kind && name && (
      <ManifestEditor
        kind={kind.toLowerCase() as 'deployment' | 'statefulset' | 'daemonset'}
        namespace={namespace}
        name={name}
        open={manifestEditorOpen}
        onClose={() => setManifestEditorOpen(false)}
      />
    )}
  </>
  )
}
