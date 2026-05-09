import { Route, Routes } from 'react-router-dom'

import { AppLayout } from '@/components/AppLayout'
import { Dashboard } from '@/routes/Dashboard'
import { Services } from '@/routes/Services'
import { Containers } from '@/routes/Containers'
import { Kubernetes } from '@/routes/Kubernetes'
import { Terminal } from '@/routes/Terminal'
import { Updates } from '@/routes/Updates'
import { Network } from '@/routes/Network'
import { Logs } from '@/routes/Logs'
import { Settings } from '@/routes/Settings'

export function AuthenticatedApp() {
  return (
    <AppLayout>
      <Routes>
        <Route index element={<Dashboard />} />
        <Route path="/updates" element={<Updates />} />
        <Route path="/services" element={<Services />} />
        <Route path="/containers" element={<Containers />} />
        <Route path="/kubernetes" element={<Kubernetes />} />
        <Route path="/terminal" element={<Terminal />} />
        <Route path="/network" element={<Network />} />
        <Route path="/logs" element={<Logs />} />
        <Route path="/settings" element={<Settings />} />
      </Routes>
    </AppLayout>
  )
}
