import { useState } from 'react'
import { Loader2 } from 'lucide-react'

import { useK8sNodeDetail, useCordonNode } from '@/lib/k8s'
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
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { K8sNodeStatusPill } from './K8sStatusPill'
import { ConditionsTable } from './ConditionsTable'
import { EventsTable } from './EventsTable'

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <p className="mb-2 text-xs uppercase tracking-wider text-muted-foreground">{title}</p>
      {children}
    </div>
  )
}

export function NodeDetail({
  name,
  open,
  onOpenChange,
}: {
  name: string | null
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { data } = useK8sNodeDetail(open ? name : null)
  const cordon = useCordonNode()
  const [cordonOpen, setCordonOpen] = useState(false)

  // Detect current cordon state from taints: NoSchedule taint on node.kubernetes.io/unschedulable
  // is what kubectl cordon applies; fall back to checking SchedulingDisabled in conditions.
  const isCordoned =
    data?.taints.some((t) => t.key === 'node.kubernetes.io/unschedulable' && t.effect === 'NoSchedule') ??
    data?.conditions.some((c) => c.type === 'SchedulingDisabled' && c.status === 'True') ??
    false

  const cordonLabel = isCordoned ? 'Uncordon' : 'Cordon'
  const nextCordoned = !isCordoned

  function handleCordonConfirm() {
    if (!name) return
    cordon.mutate({ name, cordoned: nextCordoned }, { onSuccess: () => setCordonOpen(false) })
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="overflow-y-auto">
        <SheetHeader>
          <div className="flex items-start justify-between gap-2">
            <div className="min-w-0">
              <SheetTitle className="font-mono break-all">{data?.node.name ?? name}</SheetTitle>
              <SheetDescription>
                {data ? `${data.node.os} / ${data.node.arch} · ${data.node.version}` : ''}
              </SheetDescription>
            </div>
            <div className="shrink-0 pt-0.5">
              <Button size="sm" variant="outline" onClick={() => setCordonOpen(true)}>
                {cordonLabel}
              </Button>
            </div>
          </div>
        </SheetHeader>

        {cordon.isError && (
          <Alert variant="destructive" className="mt-3">
            <AlertDescription>{(cordon.error as Error).message}</AlertDescription>
          </Alert>
        )}

        {data && (
          <div className="mt-4 flex flex-col gap-6">
            <div className="flex items-center gap-2">
              <K8sNodeStatusPill status={data.node.status} />
            </div>

            {/* Capacity / Allocatable */}
            <Section title="Capacity / Allocatable">
              <div className="grid grid-cols-3 gap-3 text-xs">
                {(['cpu', 'memory', 'pods'] as const).map((k) => (
                  <div key={k}>
                    <div className="uppercase tracking-wider text-muted-foreground">{k}</div>
                    <div className="font-mono">{data.node.capacity[k]}</div>
                    <div className="font-mono text-muted-foreground">{data.allocatable[k]}</div>
                  </div>
                ))}
              </div>
              <p className="mt-1 text-[10px] text-muted-foreground">capacity / allocatable</p>
            </Section>

            <Section title="Conditions">
              <ConditionsTable conditions={data.conditions} />
            </Section>

            {data.taints.length > 0 && (
              <Section title="Taints">
                <div className="flex flex-wrap gap-1.5">
                  {data.taints.map((t, i) => (
                    <Badge key={i} variant="warn">
                      {t.key}{t.value ? `=${t.value}` : ''}:{t.effect}
                    </Badge>
                  ))}
                </div>
              </Section>
            )}

            <Section title="Events">
              <EventsTable events={data.events} />
            </Section>
          </div>
        )}
      </SheetContent>

      {/* Cordon/Uncordon confirmation */}
      <AlertDialog open={cordonOpen} onOpenChange={setCordonOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{cordonLabel} node?</AlertDialogTitle>
            <AlertDialogDescription>
              {nextCordoned
                ? 'Cordoning prevents new pods from being scheduled on this node. Existing pods are unaffected.'
                : 'Uncordoning allows new pods to be scheduled on this node again.'}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={cordon.isPending}>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={handleCordonConfirm} disabled={cordon.isPending}>
              {cordon.isPending && <Loader2 className="h-4 w-4 animate-spin" />}
              {cordonLabel}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Sheet>
  )
}
