// A deliberately simple password-strength heuristic for v0.1. Replace with
// zxcvbn in M9 if we want dictionary-aware feedback.

export type Strength = 'too-short' | 'weak' | 'ok' | 'good' | 'strong'

export interface StrengthResult {
  level: Strength
  score: number // 0..4
  hint: string
}

export function evaluatePassword(pw: string): StrengthResult {
  if (pw.length < 12) {
    return { level: 'too-short', score: 0, hint: 'At least 12 characters.' }
  }

  let score = 0
  if (/[a-z]/.test(pw)) score++
  if (/[A-Z]/.test(pw)) score++
  if (/[0-9]/.test(pw)) score++
  if (/[^a-zA-Z0-9]/.test(pw)) score++
  if (pw.length >= 16) score++
  if (pw.length >= 20) score++

  if (score <= 2) return { level: 'weak', score: 1, hint: 'Mix upper, lower, digits, and symbols.' }
  if (score === 3) return { level: 'ok', score: 2, hint: 'Decent — longer is better.' }
  if (score === 4) return { level: 'good', score: 3, hint: 'Good — consider 16+ characters.' }
  return { level: 'strong', score: 4, hint: 'Strong.' }
}
