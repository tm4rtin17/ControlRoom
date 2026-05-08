import { useEffect, useState } from 'react'

export type Theme = 'light' | 'dark' | 'system'

const STORAGE_KEY = 'cr-theme'

function applyTheme(theme: Theme) {
  const root = document.documentElement
  const media = window.matchMedia('(prefers-color-scheme: dark)')
  const dark = theme === 'dark' || (theme === 'system' && media.matches)
  root.classList.toggle('dark', dark)
}

export function readStoredTheme(): Theme {
  const value = window.localStorage.getItem(STORAGE_KEY)
  if (value === 'light' || value === 'dark' || value === 'system') return value
  return 'dark'
}

export function useTheme(): [Theme, (t: Theme) => void] {
  const [theme, setTheme] = useState<Theme>(readStoredTheme)

  useEffect(() => {
    applyTheme(theme)
    window.localStorage.setItem(STORAGE_KEY, theme)
  }, [theme])

  // Track OS preference changes when in "system" mode.
  useEffect(() => {
    if (theme !== 'system') return
    const media = window.matchMedia('(prefers-color-scheme: dark)')
    const onChange = () => applyTheme('system')
    media.addEventListener('change', onChange)
    return () => media.removeEventListener('change', onChange)
  }, [theme])

  return [theme, setTheme]
}
