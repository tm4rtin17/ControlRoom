import { Monitor, Moon, Sun } from 'lucide-react'

import { type Theme, useTheme } from '@/lib/theme'
import { cn } from '@/lib/utils'

const options: { value: Theme; label: string; Icon: typeof Sun }[] = [
  { value: 'light', label: 'Light', Icon: Sun },
  { value: 'dark', label: 'Dark', Icon: Moon },
  { value: 'system', label: 'System', Icon: Monitor },
]

export function ThemeToggle() {
  const [theme, setTheme] = useTheme()
  return (
    <div role="radiogroup" aria-label="Theme" className="inline-flex items-center gap-1 rounded-md border bg-background p-0.5">
      {options.map(({ value, label, Icon }) => {
        const active = theme === value
        return (
          <button
            key={value}
            type="button"
            role="radio"
            aria-checked={active}
            aria-label={label}
            onClick={() => setTheme(value)}
            className={cn(
              'inline-flex h-7 w-7 items-center justify-center rounded transition-colors',
              active
                ? 'bg-primary/10 text-foreground ring-1 ring-primary/30'
                : 'text-muted-foreground hover:text-foreground'
            )}
          >
            <Icon className="h-3.5 w-3.5" aria-hidden />
          </button>
        )
      })}
    </div>
  )
}
