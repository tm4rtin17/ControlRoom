import { useSystemOverview, useSystemStatsWS } from '@/lib/system'
import { CpuTile } from '@/components/dashboard/CpuTile'
import { MemoryTile } from '@/components/dashboard/MemoryTile'
import { DiskTile } from '@/components/dashboard/DiskTile'
import { TemperatureTile } from '@/components/dashboard/TemperatureTile'
import { UptimeTile } from '@/components/dashboard/UptimeTile'
import { NetworkTile } from '@/components/dashboard/NetworkTile'
import { Alert, AlertDescription } from '@/components/ui/alert'

export function Dashboard() {
  // useSystemStatsWS pushes WS frames into the same query cache as the REST
  // fallback below, so tiles see whichever transport is current.
  useSystemStatsWS()
  const overview = useSystemOverview()

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-baseline justify-between">
        <h1 className="text-xl font-semibold tracking-tight">Dashboard</h1>
        <p className="text-xs text-muted-foreground">Live system overview</p>
      </div>

      {overview.isError && (
        <Alert variant="destructive">
          <AlertDescription>
            Could not reach the server. Retrying every 5 seconds.
          </AlertDescription>
        </Alert>
      )}

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
        <CpuTile cpu={overview.data?.cpu} />
        <MemoryTile memory={overview.data?.memory} />
        <DiskTile disks={overview.data?.disks} />
        <TemperatureTile temps={overview.data?.temperatures} />
        <UptimeTile
          uptimeSeconds={overview.data?.uptime_seconds}
          loadAvg={overview.data?.load_avg}
        />
        <NetworkTile network={overview.data?.network} />
      </div>
    </div>
  )
}
