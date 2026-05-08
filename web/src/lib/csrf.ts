// Read the cr_csrf cookie value (set non-HTTPOnly by the server) so the SPA
// can mirror it in the X-CSRF-Token header on state-changing requests.

const CSRF_COOKIE = 'cr_csrf'

export function readCsrfCookie(): string {
  const target = `${CSRF_COOKIE}=`
  const parts = document.cookie.split('; ')
  for (const part of parts) {
    if (part.startsWith(target)) {
      return decodeURIComponent(part.slice(target.length))
    }
  }
  return ''
}
