import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { BrowserRouter, Route, Routes } from 'react-router-dom'

import { AuthenticatedApp } from '@/routes/AuthenticatedApp'
import { AuthGate, LoginGate, SetupGate } from '@/routes/Gates'
import { Login } from '@/routes/Login'
import { Setup } from '@/routes/Setup'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { refetchOnWindowFocus: false, retry: false },
  },
})

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route
            path="/setup"
            element={
              <SetupGate>
                <Setup />
              </SetupGate>
            }
          />
          <Route
            path="/login"
            element={
              <LoginGate>
                <Login />
              </LoginGate>
            }
          />
          <Route
            path="/*"
            element={
              <AuthGate>
                <AuthenticatedApp />
              </AuthGate>
            }
          />
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  )
}
