import { Navigate, useLocation } from 'react-router-dom'

import { useMe, useSetupStatus } from '@/lib/auth'
import { Splash } from '@/components/Splash'

// AuthGate is the guard for the authenticated app: route to /setup if the
// install needs setup, else /login if no session, else render children.
export function AuthGate({ children }: { children: React.ReactNode }) {
  const setup = useSetupStatus()
  const me = useMe()

  if (setup.isLoading) return <Splash />
  if (setup.data?.required) return <Navigate to="/setup" replace />

  if (me.isLoading) return <Splash />
  if (me.isError) return <Navigate to="/login" replace />

  return <>{children}</>
}

// SetupGate is wrapped around /setup: if setup isn't required, redirect home.
export function SetupGate({ children }: { children: React.ReactNode }) {
  const setup = useSetupStatus()
  const location = useLocation()

  if (setup.isLoading) return <Splash />
  if (!setup.data?.required) {
    // Setup already done — get out of here.
    return <Navigate to="/" replace state={{ from: location }} />
  }
  return <>{children}</>
}

// LoginGate: if already authenticated, redirect home.
export function LoginGate({ children }: { children: React.ReactNode }) {
  const setup = useSetupStatus()
  const me = useMe()

  if (setup.isLoading) return <Splash />
  if (setup.data?.required) return <Navigate to="/setup" replace />

  if (me.isLoading) return <Splash />
  if (me.data) return <Navigate to="/" replace />
  return <>{children}</>
}
