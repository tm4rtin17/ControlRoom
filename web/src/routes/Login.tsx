import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useNavigate } from 'react-router-dom'
import { ServerCog } from 'lucide-react'

import { ApiError } from '@/lib/api'
import { useLogin } from '@/lib/auth'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Alert, AlertDescription } from '@/components/ui/alert'

interface FormValues {
  username: string
  password: string
  totp: string
}

export function Login() {
  const navigate = useNavigate()
  const login = useLogin()
  const [needsTotp, setNeedsTotp] = useState(false)

  const { register, handleSubmit, formState } = useForm<FormValues>({
    defaultValues: { username: '', password: '', totp: '' },
  })

  async function onSubmit(values: FormValues) {
    try {
      await login.mutateAsync({
        username: values.username,
        password: values.password,
        totp: values.totp || undefined,
      })
      navigate('/', { replace: true })
    } catch (err) {
      if (err instanceof ApiError && err.message.includes('totp')) {
        setNeedsTotp(true)
      }
    }
  }

  const errorMessage =
    login.error instanceof ApiError
      ? login.error.message
      : login.error
        ? 'Login failed'
        : null

  return (
    <div className="min-h-screen flex items-center justify-center bg-background p-6">
      <Card className="w-full max-w-md">
        <CardHeader className="items-center">
          <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-primary/10 ring-1 ring-primary/20">
            <ServerCog className="h-6 w-6 text-primary" aria-hidden />
          </div>
          <CardTitle className="text-center">Sign in to ControlRoom</CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit(onSubmit)} className="flex flex-col gap-4">
            <div className="flex flex-col gap-2">
              <Label htmlFor="username">Username</Label>
              <Input
                id="username"
                autoComplete="username"
                autoFocus
                {...register('username', { required: true })}
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="password">Password</Label>
              <Input
                id="password"
                type="password"
                autoComplete="current-password"
                {...register('password', { required: true })}
              />
            </div>
            {needsTotp && (
              <div className="flex flex-col gap-2">
                <Label htmlFor="totp">Authenticator code</Label>
                <Input
                  id="totp"
                  inputMode="numeric"
                  autoComplete="one-time-code"
                  pattern="[0-9]{6}"
                  maxLength={6}
                  {...register('totp', { required: needsTotp })}
                />
              </div>
            )}
            {errorMessage && (
              <Alert variant="destructive">
                <AlertDescription>{errorMessage}</AlertDescription>
              </Alert>
            )}
            <Button type="submit" disabled={formState.isSubmitting || login.isPending}>
              {login.isPending ? 'Signing in…' : 'Sign in'}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
