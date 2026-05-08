// Typed fetch + error model for the ControlRoom REST API.
//
// Behavior:
//   - Cookie auth: credentials: 'include' on every request.
//   - CSRF: state-changing methods (POST/PATCH/PUT/DELETE) automatically pull
//     the cr_csrf cookie value into the X-CSRF-Token header.
//   - Auto-refresh: a single 401 on a non-/auth request triggers one
//     /api/auth/refresh attempt; the original request is retried once on
//     success. Concurrent 401s share the same in-flight refresh promise.

import { readCsrfCookie } from './csrf'

export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly detail?: unknown

  constructor(status: number, code: string, message: string, detail?: unknown) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.detail = detail
  }
}

interface ErrorBody {
  error?: { code?: string; message?: string; detail?: unknown }
}

const NEEDS_CSRF = new Set(['POST', 'PUT', 'PATCH', 'DELETE'])

let refreshInFlight: Promise<boolean> | null = null

async function tryRefresh(): Promise<boolean> {
  if (refreshInFlight) return refreshInFlight
  refreshInFlight = (async () => {
    try {
      const res = await fetch('/api/auth/refresh', {
        method: 'POST',
        credentials: 'include',
        headers: { 'X-CSRF-Token': readCsrfCookie(), Accept: 'application/json' },
      })
      return res.ok
    } catch {
      return false
    } finally {
      // Clear after the current microtask so concurrent waiters all see the result.
      queueMicrotask(() => (refreshInFlight = null))
    }
  })()
  return refreshInFlight
}

interface FetchOpts extends RequestInit {
  // skipRefresh prevents an auto-refresh loop on /api/auth/refresh itself.
  skipRefresh?: boolean
}

async function rawFetch<T>(path: string, opts: FetchOpts = {}): Promise<T> {
  const method = (opts.method ?? 'GET').toUpperCase()
  const headers = new Headers(opts.headers ?? {})
  headers.set('Accept', 'application/json')
  if (NEEDS_CSRF.has(method)) {
    headers.set('X-CSRF-Token', readCsrfCookie())
  }
  if (opts.body && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }

  const res = await fetch(path, {
    ...opts,
    method,
    headers,
    credentials: 'include',
  })

  const text = await res.text()
  let body: unknown
  if (text.length > 0) {
    try {
      body = JSON.parse(text)
    } catch {
      body = { error: { code: 'parse_error', message: text } }
    }
  }

  if (!res.ok) {
    const e = (body as ErrorBody | undefined)?.error
    throw new ApiError(
      res.status,
      e?.code ?? 'unknown',
      e?.message ?? res.statusText,
      e?.detail
    )
  }
  return body as T
}

export async function apiFetch<T>(path: string, opts: FetchOpts = {}): Promise<T> {
  try {
    return await rawFetch<T>(path, opts)
  } catch (err) {
    if (
      err instanceof ApiError &&
      err.status === 401 &&
      !opts.skipRefresh &&
      !path.startsWith('/api/auth/refresh') &&
      !path.startsWith('/api/auth/login') &&
      !path.startsWith('/api/setup')
    ) {
      const refreshed = await tryRefresh()
      if (refreshed) {
        return rawFetch<T>(path, { ...opts, skipRefresh: true })
      }
    }
    throw err
  }
}

// ---- typed endpoints ----

export interface VersionInfo {
  version: string
  commit: string
  date: string
}

export interface SetupStatus {
  required: boolean
}

export interface MeUser {
  id: number
  username: string
  role: string
  totp_enabled: boolean
  created_at: string
}

export interface MeResponse {
  user: MeUser
}

export interface TOTPEnrollment {
  secret: string
  uri: string
  qr_data_uri: string
}

export const api = {
  // Public
  version: () => apiFetch<VersionInfo>('/api/version'),
  healthz: () => apiFetch<{ status: string }>('/api/healthz'),

  // Setup wizard
  setupStatus: () => apiFetch<SetupStatus>('/api/setup/status'),
  setupVerifyToken: (token: string) =>
    apiFetch<{ ok: boolean }>('/api/setup/verify-token', {
      method: 'POST',
      body: JSON.stringify({ token }),
    }),
  setupTotpPreview: (username: string) =>
    apiFetch<TOTPEnrollment>('/api/setup/2fa/preview', {
      method: 'POST',
      body: JSON.stringify({ username }),
    }),
  setupComplete: (req: {
    username: string
    password: string
    totp?: { secret: string; code: string }
  }) =>
    apiFetch<MeResponse>('/api/setup/complete', {
      method: 'POST',
      body: JSON.stringify(req),
    }),

  // Auth
  login: (req: { username: string; password: string; totp?: string }) =>
    apiFetch<MeResponse>('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify(req),
    }),
  logout: () => apiFetch<{ ok: boolean }>('/api/auth/logout', { method: 'POST' }),
  me: () => apiFetch<MeResponse>('/api/auth/me'),

  // 2FA management (post-login)
  totpEnroll: () =>
    apiFetch<TOTPEnrollment>('/api/auth/2fa/enroll', { method: 'POST' }),
  totpVerify: (req: { secret: string; code: string }) =>
    apiFetch<{ ok: boolean }>('/api/auth/2fa/verify', {
      method: 'POST',
      body: JSON.stringify(req),
    }),
  totpDisable: (req: { password: string }) =>
    apiFetch<{ ok: boolean }>('/api/auth/2fa/disable', {
      method: 'POST',
      body: JSON.stringify(req),
    }),
}
