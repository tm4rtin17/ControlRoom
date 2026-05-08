import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Info, KeyRound, ShieldCheck, ShieldOff, User } from 'lucide-react'

import { api, ApiError, type TOTPEnrollment } from '@/lib/api'
import { ME_QUERY_KEY, useMe } from '@/lib/auth'
import { useChangePassword, useSettings } from '@/lib/settings'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { PasswordStrength } from '@/components/PasswordStrength'

export function Settings() {
  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-xl font-semibold tracking-tight">Settings</h1>
      <AccountCard />
      <TwoFactorCard />
      <ServerCard />
      <AboutCard />
    </div>
  )
}

// ---- Account ----

function AccountCard() {
  const me = useMe()
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-sm">
          <User className="h-4 w-4 text-muted-foreground" aria-hidden />
          Account
        </CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-6">
        <div className="grid grid-cols-2 gap-3 text-sm">
          <Field label="Username" value={me.data?.username ?? '—'} mono />
          <Field label="Role" value={me.data?.role ?? '—'} />
        </div>
        <ChangePasswordForm />
      </CardContent>
    </Card>
  )
}

interface PwForm {
  current: string
  new: string
  confirm: string
}

function ChangePasswordForm() {
  const change = useChangePassword()
  const { register, handleSubmit, watch, reset, formState } = useForm<PwForm>({
    defaultValues: { current: '', new: '', confirm: '' },
  })
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState(false)
  const newPw = watch('new') ?? ''

  async function onSubmit(values: PwForm) {
    setError(null)
    setSuccess(false)
    if (values.new !== values.confirm) {
      setError('New password and confirmation do not match')
      return
    }
    try {
      await change.mutateAsync({ current: values.current, new: values.new })
      setSuccess(true)
      reset({ current: '', new: '', confirm: '' })
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Could not change password')
    }
  }

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="flex flex-col gap-4">
      <p className="text-sm font-medium">Change password</p>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <div className="flex flex-col gap-2">
          <Label htmlFor="current">Current</Label>
          <Input id="current" type="password" autoComplete="current-password" {...register('current', { required: true })} />
        </div>
        <div className="flex flex-col gap-2">
          <Label htmlFor="new">New</Label>
          <Input id="new" type="password" autoComplete="new-password" {...register('new', { required: true, minLength: 12, maxLength: 72 })} />
        </div>
        <div className="flex flex-col gap-2">
          <Label htmlFor="confirm">Confirm</Label>
          <Input id="confirm" type="password" autoComplete="new-password" {...register('confirm', { required: true })} />
        </div>
      </div>
      <PasswordStrength password={newPw} />
      {error && (
        <Alert variant="destructive">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}
      {success && (
        <Alert>
          <AlertDescription>
            Password changed. All other sessions have been signed out.
          </AlertDescription>
        </Alert>
      )}
      <div className="flex justify-end">
        <Button type="submit" size="sm" disabled={formState.isSubmitting || change.isPending}>
          {change.isPending ? 'Updating…' : 'Update password'}
        </Button>
      </div>
    </form>
  )
}

// ---- 2FA ----

function TwoFactorCard() {
  const me = useMe()
  const qc = useQueryClient()
  const [enrollment, setEnrollment] = useState<TOTPEnrollment | null>(null)
  const [code, setCode] = useState('')
  const [disablePw, setDisablePw] = useState('')
  const [error, setError] = useState<string | null>(null)

  const startEnroll = useMutation({
    mutationFn: api.totpEnroll,
    onSuccess: (e) => {
      setEnrollment(e)
      setError(null)
    },
    onError: (err) =>
      setError(err instanceof ApiError ? err.message : 'Could not start enrollment'),
  })
  const verify = useMutation({
    mutationFn: () =>
      api.totpVerify({ secret: enrollment!.secret, code: code.trim() }),
    onSuccess: () => {
      setEnrollment(null)
      setCode('')
      qc.invalidateQueries({ queryKey: ME_QUERY_KEY })
    },
    onError: (err) =>
      setError(err instanceof ApiError ? err.message : 'Could not verify code'),
  })
  const disable = useMutation({
    mutationFn: () => api.totpDisable({ password: disablePw }),
    onSuccess: () => {
      setDisablePw('')
      qc.invalidateQueries({ queryKey: ME_QUERY_KEY })
    },
    onError: (err) =>
      setError(err instanceof ApiError ? err.message : 'Could not disable 2FA'),
  })

  const enabled = me.data?.totp_enabled

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-sm">
          {enabled ? (
            <ShieldCheck className="h-4 w-4 text-emerald-500" aria-hidden />
          ) : (
            <ShieldOff className="h-4 w-4 text-amber-500" aria-hidden />
          )}
          Two-factor authentication
          <Badge variant={enabled ? 'success' : 'warn'} className="ml-1">
            {enabled ? 'enabled' : 'disabled'}
          </Badge>
        </CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        {error && (
          <Alert variant="destructive">
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        )}

        {!enabled && !enrollment && (
          <div className="flex items-center justify-between gap-3">
            <p className="text-sm text-muted-foreground">
              Add a second factor — strongly recommended for any host reachable beyond LAN.
            </p>
            <Button size="sm" onClick={() => startEnroll.mutate()} disabled={startEnroll.isPending}>
              Enable 2FA
            </Button>
          </div>
        )}

        {enrollment && (
          <div className="flex flex-col gap-3">
            <p className="text-sm">Scan the QR with your authenticator app and enter the 6-digit code.</p>
            <div className="flex justify-center rounded-md border bg-card p-4">
              <img src={enrollment.qr_data_uri} alt="TOTP QR code" className="h-48 w-48" />
            </div>
            <div className="break-all text-center font-mono text-xs text-muted-foreground">
              {enrollment.secret}
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="code">Authenticator code</Label>
              <Input
                id="code"
                inputMode="numeric"
                pattern="[0-9]{6}"
                maxLength={6}
                autoComplete="one-time-code"
                value={code}
                onChange={(e) => setCode(e.target.value.replace(/\D/g, ''))}
              />
            </div>
            <div className="flex justify-end gap-2">
              <Button
                variant="ghost"
                onClick={() => {
                  setEnrollment(null)
                  setCode('')
                }}
              >
                Cancel
              </Button>
              <Button onClick={() => verify.mutate()} disabled={code.length !== 6 || verify.isPending}>
                Verify and enable
              </Button>
            </div>
          </div>
        )}

        {enabled && (
          <div className="flex flex-col gap-2">
            <Label htmlFor="disablePw" className="text-xs">Confirm with password to disable</Label>
            <div className="flex gap-2">
              <Input
                id="disablePw"
                type="password"
                value={disablePw}
                onChange={(e) => setDisablePw(e.target.value)}
                autoComplete="current-password"
              />
              <Button
                variant="destructive"
                size="sm"
                disabled={!disablePw || disable.isPending}
                onClick={() => disable.mutate()}
              >
                Disable 2FA
              </Button>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

// ---- Server (read-only) ----

function ServerCard() {
  const settings = useSettings()
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-sm">
          <KeyRound className="h-4 w-4 text-muted-foreground" aria-hidden />
          Server configuration
        </CardTitle>
      </CardHeader>
      <CardContent className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <Field label="Bind address" value={settings.data?.server.addr ?? '—'} mono />
        <Field label="Host name" value={settings.data?.server.host_name ?? '(kernel)'} />
        <Field label="TLS mode" value={settings.data?.tls.mode ?? '—'} />
        {settings.data?.tls.acme_host && (
          <Field label="ACME host" value={settings.data.tls.acme_host} />
        )}
        <Field label="Log level" value={settings.data?.server.log_level ?? '—'} />
        <Field label="Trust proxy" value={String(settings.data?.server.trust_proxy ?? false)} />
        <Field label="Dev mode" value={String(settings.data?.server.dev_mode ?? false)} />
        <Field
          label="Version check"
          value={settings.data?.server.version_check ? 'enabled' : 'disabled'}
        />
        <Field label="Session lifetime" value={`${settings.data?.server.session_hours ?? '—'} h`} />
      </CardContent>
    </Card>
  )
}

// ---- About ----

function AboutCard() {
  const settings = useSettings()
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-sm">
          <Info className="h-4 w-4 text-muted-foreground" aria-hidden />
          About
        </CardTitle>
      </CardHeader>
      <CardContent className="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <Field label="Version" value={settings.data?.build.version ?? '—'} mono />
        <Field label="Commit" value={settings.data?.build.commit ?? '—'} mono />
        <Field label="Build date" value={settings.data?.build.date ?? '—'} mono />
      </CardContent>
    </Card>
  )
}

// ---- shared ----

function Field({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div>
      <div className="text-xs uppercase tracking-wider text-muted-foreground">{label}</div>
      <div className={mono ? 'font-mono text-xs' : 'text-sm'}>{value}</div>
    </div>
  )
}
