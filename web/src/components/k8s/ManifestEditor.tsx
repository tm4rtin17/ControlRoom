import React, { Suspense, useEffect, useState } from 'react'
import { Loader2, RefreshCw } from 'lucide-react'

import { useK8sManifest, useApplyManifest } from '@/lib/k8s'
import { ApiError } from '@/lib/api'
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription } from '@/components/ui/sheet'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'

// Lazy-load the Monaco editor to keep the initial bundle lean (~2-3 MB avoided).
const MonacoEditor = React.lazy(() => import('@monaco-editor/react'))

const EDITOR_OPTIONS = {
  minimap: { enabled: false },
  fontSize: 13,
  fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
  wordWrap: 'on' as const,
  tabSize: 2,
  scrollBeyondLastLine: false,
}

type EditorKind = 'deployment' | 'statefulset' | 'daemonset' | 'service' | 'configmap'

interface ManifestEditorProps {
  kind: EditorKind
  namespace: string
  name: string
  open: boolean
  onClose: () => void
}

type ActionStatus =
  | { type: 'idle' }
  | { type: 'dry-run-passed'; warnings: string[] }
  | { type: 'applied'; warnings: string[] }
  | { type: 'admission-error'; message: string }
  | { type: 'conflict' }
  | { type: 'error'; message: string }

export function ManifestEditor({ kind, namespace, name, open, onClose }: ManifestEditorProps) {
  const manifestQ = useK8sManifest(open ? kind : '', open ? namespace : '', open ? name : '')
  const applyMut = useApplyManifest()

  const [editedYAML, setEditedYAML] = useState('')
  const [originalYAML, setOriginalYAML] = useState('')
  const [resourceVersion, setResourceVersion] = useState('')
  const [status, setStatus] = useState<ActionStatus>({ type: 'idle' })

  const [applyDialogOpen, setApplyDialogOpen] = useState(false)
  const [reloadDialogOpen, setReloadDialogOpen] = useState(false)
  // Track which action is pending so buttons show the right spinner.
  const [pendingAction, setPendingAction] = useState<'dry-run' | 'apply' | null>(null)

  // Sync content when the manifest loads or is refetched.
  useEffect(() => {
    if (!manifestQ.data) return
    const { yaml, resource_version } = manifestQ.data
    setOriginalYAML(yaml)
    setEditedYAML(yaml)
    setResourceVersion(resource_version)
    setStatus({ type: 'idle' })
  }, [manifestQ.data])

  // Reset state when the sheet is opened (so stale status from a prior open is cleared).
  useEffect(() => {
    if (open) {
      setStatus({ type: 'idle' })
    }
  }, [open])

  const dirty = editedYAML !== originalYAML

  function handleEditorChange(value: string | undefined) {
    setEditedYAML(value ?? '')
    // Clear transient success status when user edits again.
    setStatus((prev) =>
      prev.type === 'dry-run-passed' || prev.type === 'applied' ? { type: 'idle' } : prev
    )
  }

  function triggerReload() {
    manifestQ.refetch()
    setStatus({ type: 'idle' })
  }

  function handleReloadClick() {
    if (dirty) {
      setReloadDialogOpen(true)
    } else {
      triggerReload()
    }
  }

  function runApply(dry_run: boolean) {
    setPendingAction(dry_run ? 'dry-run' : 'apply')
    applyMut.mutate(
      { kind, namespace, name, yaml: editedYAML, resource_version: resourceVersion, dry_run },
      {
        onSuccess: (resp) => {
          setPendingAction(null)
          if (!dry_run) {
            setOriginalYAML(editedYAML)
            setResourceVersion(resp.resource_version)
          }
          setStatus(
            dry_run
              ? { type: 'dry-run-passed', warnings: resp.warnings }
              : { type: 'applied', warnings: resp.warnings }
          )
        },
        onError: (err) => {
          setPendingAction(null)
          if (err instanceof ApiError && err.status === 409) {
            setStatus({ type: 'conflict' })
          } else if (err instanceof ApiError && err.status === 422) {
            setStatus({ type: 'admission-error', message: err.message })
          } else {
            setStatus({ type: 'error', message: err instanceof Error ? err.message : 'Unknown error' })
          }
        },
      }
    )
  }

  const isPending = applyMut.isPending

  return (
    <>
      <Sheet open={open} onOpenChange={(o) => { if (!o) onClose() }}>
        <SheetContent className="flex flex-col overflow-hidden sm:max-w-[82vw]">
          <SheetHeader className="shrink-0">
            <div className="flex items-start justify-between gap-2 pr-6">
              <div className="min-w-0">
                <SheetTitle className="font-mono text-sm break-all">
                  Edit YAML — {kind}/{namespace}/{name}
                </SheetTitle>
                <SheetDescription>
                  {resourceVersion ? (
                    <span>
                      resourceVersion:{' '}
                      <span className="font-mono text-xs">{resourceVersion}</span>
                    </span>
                  ) : (
                    <span>Loading manifest…</span>
                  )}
                </SheetDescription>
              </div>
              <div className="flex shrink-0 items-center gap-2 pt-0.5">
                <Button
                  size="sm"
                  variant="outline"
                  onClick={handleReloadClick}
                  disabled={isPending || manifestQ.isFetching}
                  title="Reload manifest"
                >
                  {manifestQ.isFetching ? (
                    <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  ) : (
                    <RefreshCw className="h-3.5 w-3.5" />
                  )}
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  disabled={isPending || !dirty || !resourceVersion}
                  onClick={() => runApply(true)}
                >
                  {pendingAction === 'dry-run' && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
                  Dry-run
                </Button>
                <Button
                  size="sm"
                  variant="destructive"
                  disabled={isPending || !dirty || !resourceVersion}
                  onClick={() => setApplyDialogOpen(true)}
                >
                  {pendingAction === 'apply' && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
                  Apply
                </Button>
              </div>
            </div>
          </SheetHeader>

          {/* Status alerts */}
          <div className="shrink-0">
            {status.type === 'conflict' && (
              <Alert variant="destructive" className="mt-3">
                <AlertDescription className="flex items-center justify-between gap-2">
                  <span>
                    Conflict: this resource was modified by someone else. Reload to see the latest
                    version, then re-apply your changes.
                  </span>
                  <Button size="sm" variant="outline" className="h-6 shrink-0 text-xs" onClick={triggerReload}>
                    Reload
                  </Button>
                </AlertDescription>
              </Alert>
            )}
            {status.type === 'admission-error' && (
              <Alert variant="destructive" className="mt-3">
                <AlertDescription>{status.message}</AlertDescription>
              </Alert>
            )}
            {status.type === 'error' && (
              <Alert variant="destructive" className="mt-3">
                <AlertDescription>{status.message}</AlertDescription>
              </Alert>
            )}
            {manifestQ.isError && !manifestQ.data && (
              <Alert variant="destructive" className="mt-3">
                <AlertDescription>
                  {manifestQ.error instanceof Error ? manifestQ.error.message : 'Failed to load manifest.'}
                </AlertDescription>
              </Alert>
            )}
          </div>

          {/* Editor area */}
          <div className="mt-3 min-h-0 flex-1 overflow-hidden rounded-md border">
            <Suspense
              fallback={
                <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
                  Loading editor…
                </div>
              }
            >
              <MonacoEditor
                height="100%"
                language="yaml"
                theme="vs-dark"
                value={editedYAML}
                onChange={handleEditorChange}
                options={EDITOR_OPTIONS}
              />
            </Suspense>
          </div>

          {/* Bottom status row */}
          <div className="shrink-0 pt-2 text-xs text-muted-foreground flex items-center gap-3">
            {dirty ? (
              <span className="text-amber-500">Unsaved changes</span>
            ) : (
              <span>No changes</span>
            )}
            {status.type === 'dry-run-passed' && (
              <span className="text-emerald-500">
                Dry-run passed — apply to commit
                {status.warnings.length > 0 && ` (${status.warnings.length} warning${status.warnings.length > 1 ? 's' : ''})`}
              </span>
            )}
            {status.type === 'applied' && (
              <span className="text-emerald-500">
                Applied successfully
                {status.warnings.length > 0 && ` (${status.warnings.length} warning${status.warnings.length > 1 ? 's' : ''})`}
              </span>
            )}
            {(status.type === 'dry-run-passed' || status.type === 'applied') &&
              status.warnings.length > 0 && (
                <span className="truncate text-amber-400" title={status.warnings.join('; ')}>
                  {status.warnings[0]}
                </span>
              )}
          </div>
        </SheetContent>
      </Sheet>

      {/* Apply confirmation */}
      <AlertDialog open={applyDialogOpen} onOpenChange={setApplyDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Apply changes?</AlertDialogTitle>
            <AlertDialogDescription>
              Apply changes to{' '}
              <span className="font-mono">
                {kind}/{namespace}/{name}
              </span>
              ? This is the same as <span className="font-mono">kubectl apply</span> — the change is
              live immediately.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isPending}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={isPending}
              onClick={() => {
                setApplyDialogOpen(false)
                runApply(false)
              }}
            >
              Apply
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Reload confirmation (only shown when dirty) */}
      <AlertDialog open={reloadDialogOpen} onOpenChange={setReloadDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Discard changes?</AlertDialogTitle>
            <AlertDialogDescription>
              Reload the manifest from the server? Your unsaved edits will be lost.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                setReloadDialogOpen(false)
                triggerReload()
              }}
            >
              Reload
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
