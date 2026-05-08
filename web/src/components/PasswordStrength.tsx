import { evaluatePassword } from '@/lib/strength'
import { cn } from '@/lib/utils'

const colors: Record<string, string> = {
  'too-short': 'bg-muted',
  weak: 'bg-destructive',
  ok: 'bg-amber-500',
  good: 'bg-sky-500',
  strong: 'bg-emerald-500',
}

export function PasswordStrength({ password }: { password: string }) {
  const result = evaluatePassword(password)
  const score = result.score
  const segments = 4

  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex gap-1">
        {Array.from({ length: segments }).map((_, i) => (
          <div
            key={i}
            className={cn(
              'h-1.5 flex-1 rounded-full transition-colors',
              i < score ? colors[result.level] : 'bg-muted'
            )}
          />
        ))}
      </div>
      <p className="text-xs text-muted-foreground">{result.hint}</p>
    </div>
  )
}
