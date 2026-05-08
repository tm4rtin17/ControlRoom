import { useState } from 'react'
import { Search } from 'lucide-react'

import { ApiError } from '@/lib/api'
import { useK8sNodes, useK8sNamespaces, useK8sWorkloads, useK8sPods, useK8sServices } from '@/lib/k8s'
import { Input } from '@/components/ui/input'
import { Card, CardContent } from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import { cn } from '@/lib/utils'
import { NodeList } from '@/components/k8s/NodeList'
import { WorkloadList } from '@/components/k8s/WorkloadList'
import { PodList } from '@/components/k8s/PodList'
import { ServiceList } from '@/components/k8s/ServiceList'
import { NodeDetail } from '@/components/k8s/NodeDetail'
import { WorkloadDetail } from '@/components/k8s/WorkloadDetail'
import { PodDetail } from '@/components/k8s/PodDetail'
import { ServiceDetail } from '@/components/k8s/ServiceDetail'

type Tab = 'nodes' | 'workloads' | 'pods' | 'services'

const TABS: { id: Tab; label: string }[] = [
  { id: 'nodes', label: 'Nodes' },
  { id: 'workloads', label: 'Workloads' },
  { id: 'pods', label: 'Pods' },
  { id: 'services', label: 'Services' },
]

interface NodeOpen { name: string }
interface WorkloadOpen { namespace: string; kind: string; name: string }
interface PodOpen { namespace: string; name: string }
interface ServiceOpen { namespace: string; name: string }

export function Kubernetes() {
  const [tab, setTab] = useState<Tab>('nodes')
  const [namespace, setNamespace] = useState('')
  const [search, setSearch] = useState('')

  const [openNode, setOpenNode] = useState<NodeOpen | null>(null)
  const [openWorkload, setOpenWorkload] = useState<WorkloadOpen | null>(null)
  const [openPod, setOpenPod] = useState<PodOpen | null>(null)
  const [openService, setOpenService] = useState<ServiceOpen | null>(null)

  const namespacesQ = useK8sNamespaces()
  const nodesQ = useK8sNodes()
  const workloadsQ = useK8sWorkloads(namespace)
  const podsQ = useK8sPods(namespace)
  const servicesQ = useK8sServices(namespace)

  // Surface 503 from the namespaces call as a top-level unavailable state.
  const unavailable = namespacesQ.error instanceof ApiError && namespacesQ.error.status === 503

  if (unavailable) {
    return (
      <div className="flex flex-col gap-4">
        <h1 className="text-xl font-semibold tracking-tight">Kubernetes</h1>
        <Card>
          <CardContent className="p-6 text-sm text-muted-foreground">
            {namespacesQ.error?.message ?? 'Kubernetes is not available on this host.'}
          </CardContent>
        </Card>
      </div>
    )
  }

  const namespaces = namespacesQ.data?.namespaces ?? []

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h1 className="text-xl font-semibold tracking-tight">Kubernetes</h1>
      </div>

      {/* Controls row: namespace picker + search */}
      <div className="flex flex-wrap items-end gap-3">
        <div className="flex flex-col gap-1">
          <Label htmlFor="namespace" className="text-xs">Namespace</Label>
          <select
            id="namespace"
            className="h-10 rounded-md border bg-background px-2 text-sm"
            value={namespace}
            onChange={(e) => setNamespace(e.target.value)}
          >
            <option value="">All namespaces</option>
            {namespaces.map((ns) => (
              <option key={ns.name} value={ns.name}>{ns.name}</option>
            ))}
          </select>
        </div>

        <div className="flex flex-1 min-w-[200px] items-center gap-2 rounded-md border bg-card px-3 h-10">
          <Search className="h-4 w-4 text-muted-foreground shrink-0" aria-hidden />
          <Input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Filter by name or namespace"
            className="border-0 bg-transparent shadow-none focus-visible:ring-0"
          />
        </div>
      </div>

      {/* Tab bar */}
      <div className="inline-flex items-center rounded-md border bg-background p-0.5 text-xs">
        {TABS.map(({ id, label }) => (
          <button
            key={id}
            onClick={() => setTab(id)}
            className={cn(
              'rounded-sm px-3 py-1',
              tab === id
                ? 'bg-primary/10 text-foreground ring-1 ring-primary/30'
                : 'text-muted-foreground hover:text-foreground'
            )}
          >
            {label}
          </button>
        ))}
      </div>

      {/* Tab content — namespace picker is hidden on Nodes tab since nodes are cluster-scoped */}
      {tab === 'nodes' && (
        <NodeList
          nodes={nodesQ.data?.nodes ?? []}
          loading={nodesQ.isLoading}
          error={nodesQ.error}
          search={search}
          onOpen={(name) => setOpenNode({ name })}
        />
      )}
      {tab === 'workloads' && (
        <WorkloadList
          workloads={workloadsQ.data?.workloads ?? []}
          loading={workloadsQ.isLoading}
          error={workloadsQ.error}
          search={search}
          onOpen={(ns, kind, name) => setOpenWorkload({ namespace: ns, kind, name })}
        />
      )}
      {tab === 'pods' && (
        <PodList
          pods={podsQ.data?.pods ?? []}
          loading={podsQ.isLoading}
          error={podsQ.error}
          search={search}
          onOpen={(ns, name) => setOpenPod({ namespace: ns, name })}
        />
      )}
      {tab === 'services' && (
        <ServiceList
          services={servicesQ.data?.services ?? []}
          loading={servicesQ.isLoading}
          error={servicesQ.error}
          search={search}
          onOpen={(ns, name) => setOpenService({ namespace: ns, name })}
        />
      )}

      {/* Detail drawers — rendered outside tab content so state persists across tab switches */}
      <NodeDetail
        name={openNode?.name ?? null}
        open={!!openNode}
        onOpenChange={(o) => { if (!o) setOpenNode(null) }}
      />
      <WorkloadDetail
        namespace={openWorkload?.namespace ?? null}
        kind={openWorkload?.kind ?? null}
        name={openWorkload?.name ?? null}
        open={!!openWorkload}
        onOpenChange={(o) => { if (!o) setOpenWorkload(null) }}
      />
      <PodDetail
        namespace={openPod?.namespace ?? null}
        name={openPod?.name ?? null}
        open={!!openPod}
        onOpenChange={(o) => { if (!o) setOpenPod(null) }}
      />
      <ServiceDetail
        namespace={openService?.namespace ?? null}
        name={openService?.name ?? null}
        open={!!openService}
        onOpenChange={(o) => { if (!o) setOpenService(null) }}
      />
    </div>
  )
}
