import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useNavigate } from 'react-router-dom'
import { CheckCircle2, KeyRound, ServerCog, ShieldCheck, UserPlus } from 'lucide-react'

import { api, ApiError, type TOTPEnrollment } from '@/lib/api'
import { useSetupComplete } from '@/lib/auth'
import { evaluatePassword } from '@/lib/strength'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { PasswordStrength } from '@/components/PasswordStrength'

type Step = 'token' | 'account' | 'totp' | 'done'

interface AccountValues {
  username: string
  password: string
  confirm: string
}

export function Setup() {
  const [step, setStep] = useState<Step>('token')
  const [account, setAccount] = useState<AccountValues | null>(null)
  const [enrollment, setEnrollment] = useState<TOTPEnrollment | null>(null)
  const [error, setError] = useState<string | null>(null)

  return (
    <div className="min-h-screen flex items-center justify-center bg-background p-6">
      <Card className="w-full max-w-lg">
        <CardHeader className="items-center text-center">
          <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-primary/10 ring-1 ring-primary/20">
            <ServerCog className="h-6 w-6 text-primary" aria-hidden />
          </div>
          <CardTitle>Welcome to ControlRoom</CardTitle>
          <CardDescription>One-time setup. This wizard runs only once.</CardDescription>
          <Stepper step={step} />
        </CardHeader>
        <CardContent>
          {error && (
            <Alert variant="destructive" className="mb-4">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}
          {step === 'token' && (
            <TokenStep
              onSuccess={() => {
                setError(null)
                setStep('account')
              }}
              onError={setError}
            />
          )}
          {step === 'account' && (
            <AccountStep
              onContinue={(values) => {
                setAccount(values)
                setError(null)
                setStep('totp')
              }}
            />
          )}
          {step === 'totp' && account && (
            <TotpStep
              username={account.username}
              account={account}
              enrollment={enrollment}
              setEnrollment={setEnrollment}
              onError={setError}
              onDone={() => setStep('done')}
            />
          )}
          {step === 'done' && <DoneStep />}
        </CardContent>
      </Card>
    </div>
  )
}

function Stepper({ step }: { step: Step }) {
  const steps: { key: Step; label: string; icon: typeof KeyRound }[] = [
    { key: 'token', label: 'Token', icon: KeyRound },
    { key: 'account', label: 'Account', icon: UserPlus },
    { key: 'totp', label: '2FA', icon: ShieldCheck },
  ]
  const order: Step[] = ['token', 'account', 'totp', 'done']
  const idx = order.indexOf(step)

  return (
    <div className="mt-4 flex items-center justify-center gap-3">
      {steps.map((s, i) => {
        const Icon = s.icon
        const active = i === idx
        const done = i < idx
        return (
          <div
            key={s.key}
            className={
              'flex items-center gap-2 text-xs ' +
              (active ? 'text-foreground' : done ? 'text-emerald-500' : 'text-muted-foreground')
            }
          >
            <span
              className={
                'flex h-7 w-7 items-center justify-center rounded-full border ' +
                (active
                  ? 'border-primary bg-primary/10'
                  : done
                    ? 'border-emerald-500/40 bg-emerald-500/10'
                    : 'border-border bg-background')
              }
            >
              <Icon className="h-3.5 w-3.5" aria-hidden />
            </span>
            {s.label}
          </div>
        )
      })}
    </div>
  )
}

function TokenStep({
  onSuccess,
  onError,
}: {
  onSuccess: () => void
  onError: (msg: string) => void
}) {
  const { register, handleSubmit, formState } = useForm<{ token: string }>({
    defaultValues: { token: '' },
  })

  async function onSubmit({ token }: { token: string }) {
    try {
      await api.setupVerifyToken(token.trim())
      onSuccess()
    } catch (err) {
      onError(err instanceof ApiError ? err.message : 'Token check failed')
    }
  }

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="flex flex-col gap-4">
      <p className="text-sm text-muted-foreground">
        Paste the setup token printed in the server logs. It looks like a long
        hexadecimal string.
      </p>
      <div className="flex flex-col gap-2">
        <Label htmlFor="token">Setup token</Label>
        <Input
          id="token"
          autoFocus
          autoComplete="off"
          spellCheck={false}
          className="font-mono text-xs"
          {...register('token', { required: true, minLength: 32 })}
        />
      </div>
      <Button type="submit" disabled={formState.isSubmitting}>
        Continue
      </Button>
    </form>
  )
}

function AccountStep({ onContinue }: { onContinue: (values: AccountValues) => void }) {
  const { register, handleSubmit, watch, formState } = useForm<AccountValues>({
    defaultValues: { username: '', password: '', confirm: '' },
  })
  const password = watch('password') ?? ''

  function onSubmit(values: AccountValues) {
    if (values.password !== values.confirm) return
    if (evaluatePassword(values.password).level === 'too-short') return
    onContinue(values)
  }

  const passwordsMatch =
    !formState.touchedFields.confirm || watch('confirm') === watch('password')

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="flex flex-col gap-4">
      <div className="flex flex-col gap-2">
        <Label htmlFor="username">Username</Label>
        <Input
          id="username"
          autoComplete="username"
          autoFocus
          {...register('username', { required: true, pattern: /^[a-zA-Z0-9_-]{3,32}$/ })}
        />
        <p className="text-xs text-muted-foreground">
          3–32 chars: letters, digits, underscore, dash.
        </p>
      </div>
      <div className="flex flex-col gap-2">
        <Label htmlFor="password">Password</Label>
        <Input
          id="password"
          type="password"
          autoComplete="new-password"
          {...register('password', { required: true, minLength: 12, maxLength: 72 })}
        />
        <PasswordStrength password={password} />
      </div>
      <div className="flex flex-col gap-2">
        <Label htmlFor="confirm">Confirm password</Label>
        <Input
          id="confirm"
          type="password"
          autoComplete="new-password"
          {...register('confirm', { required: true })}
        />
        {!passwordsMatch && (
          <p className="text-xs text-destructive">Passwords do not match.</p>
        )}
      </div>
      <Button type="submit" disabled={formState.isSubmitting}>
        Continue
      </Button>
    </form>
  )
}

function TotpStep({
  username,
  account,
  enrollment,
  setEnrollment,
  onError,
  onDone,
}: {
  username: string
  account: AccountValues
  enrollment: TOTPEnrollment | null
  setEnrollment: (e: TOTPEnrollment | null) => void
  onError: (msg: string) => void
  onDone: () => void
}) {
  const complete = useSetupComplete()
  const [code, setCode] = useState('')
  const navigate = useNavigate()

  async function loadEnrollment() {
    try {
      const e = await api.setupTotpPreview(username)
      setEnrollment(e)
    } catch (err) {
      onError(err instanceof ApiError ? err.message : 'Could not generate 2FA enrollment')
    }
  }

  async function submit(withTotp: boolean) {
    try {
      await complete.mutateAsync({
        username: account.username,
        password: account.password,
        totp:
          withTotp && enrollment ? { secret: enrollment.secret, code: code.trim() } : undefined,
      })
      onDone()
      // Brief pause so the user sees the success state before redirect.
      setTimeout(() => navigate('/', { replace: true }), 800)
    } catch (err) {
      onError(err instanceof ApiError ? err.message : 'Setup failed')
    }
  }

  if (!enrollment) {
    return (
      <div className="flex flex-col gap-4">
        <p className="text-sm text-muted-foreground">
          Two-factor authentication adds an authenticator-app code on top of your password.
          We strongly recommend enabling it.
        </p>
        <div className="flex gap-2">
          <Button onClick={loadEnrollment} className="flex-1">
            Enable 2FA
          </Button>
          <Button variant="outline" onClick={() => submit(false)} className="flex-1">
            Skip for now
          </Button>
        </div>
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-4">
      <p className="text-sm text-muted-foreground">
        Scan the QR with your authenticator app, then enter the 6-digit code to confirm.
      </p>
      <div className="flex justify-center rounded-md border bg-card p-4">
        <img src={enrollment.qr_data_uri} alt="TOTP QR code" className="h-48 w-48" />
      </div>
      <div className="text-center font-mono text-xs text-muted-foreground break-all">
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
          autoFocus
        />
      </div>
      <div className="flex gap-2">
        <Button onClick={() => submit(true)} disabled={code.length !== 6} className="flex-1">
          Verify and finish
        </Button>
        <Button variant="ghost" onClick={() => setEnrollment(null)}>
          Back
        </Button>
      </div>
    </div>
  )
}

function DoneStep() {
  return (
    <div className="flex flex-col items-center gap-4 py-6 text-center">
      <CheckCircle2 className="h-12 w-12 text-emerald-500" aria-hidden />
      <p className="text-lg font-medium">All set</p>
      <p className="text-sm text-muted-foreground">Loading your dashboard…</p>
    </div>
  )
}
