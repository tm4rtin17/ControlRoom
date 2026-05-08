import { ExternalLink } from 'lucide-react'

import { useServiceDetail } from '@/lib/services'
import { formatBytes } from '@/lib/format'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { LogStream } from './LogStream'
import { FileStateBadge, StatusBadge } from './StatusBadge'

export function ServiceDetail({
  unit,
  open,
  onOpenChange,
}: {
  unit: string | null
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const detail = useServiceDetail(open ? unit : null)
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="overflow-y-auto">
        <SheetHeader>
          <SheetTitle className="font-mono break-all">{unit ?? '—'}</SheetTitle>
          <SheetDescription>{detail.data?.description ?? ''}</SheetDescription>
        </SheetHeader>

        {detail.data && (
          <div className="mt-4 flex flex-col gap-6">
            <div className="flex flex-wrap items-center gap-2">
              <StatusBadge state={detail.data.active_state} />
              <span className="text-xs text-muted-foreground">/</span>
              <span className="text-xs">{detail.data.sub_state}</span>
              <FileStateBadge state={detail.data.unit_file_state} />
            </div>

            <div className="grid grid-cols-2 gap-3 text-sm">
              <Field label="Memory" value={detail.data.memory_current ? formatBytes(detail.data.memory_current) : '—'} />
              <Field label="Tasks" value={detail.data.tasks_current ? String(detail.data.tasks_current) : '—'} />
              <Field label="Load" value={detail.data.load_state} />
              <Field label="Following" value={detail.data.following || '—'} />
              <Field label="Fragment" value={detail.data.fragment_path || '—'} className="col-span-2 break-all" />
            </div>

            {detail.data.documentation?.length ? (
              <div>
                <p className="mb-1 text-xs uppercase tracking-wider text-muted-foreground">Docs</p>
                <ul className="space-y-1 text-xs">
                  {detail.data.documentation.map((u) => (
                    <li key={u}>
                      <a
                        href={u}
                        target="_blank"
                        rel="noreferrer"
                        className="inline-flex items-center gap-1 text-primary hover:underline"
                      >
                        {u}
                        <ExternalLink className="h-3 w-3" aria-hidden />
                      </a>
                    </li>
                  ))}
                </ul>
              </div>
            ) : null}

            <div>
              <p className="mb-2 text-xs uppercase tracking-wider text-muted-foreground">Logs</p>
              {unit && <LogStream unit={unit} />}
            </div>
          </div>
        )}
      </SheetContent>
    </Sheet>
  )
}

function Field({ label, value, className }: { label: string; value: string; className?: string }) {
  return (
    <div className={className}>
      <div className="text-xs uppercase tracking-wider text-muted-foreground">{label}</div>
      <div className="font-mono text-xs">{value}</div>
    </div>
  )
}
