import { createContext, useContext, useEffect, useState, useCallback, type ReactNode } from 'react'

type Theme = 'dark' | 'light' | 'system'

interface ThemeContextValue {
  theme: Theme
  resolved: 'dark' | 'light'
  setTheme: (t: Theme) => void
}

const ThemeContext = createContext<ThemeContextValue>({
  theme: 'system',
  resolved: 'dark',
  setTheme: () => {},
})

export function useTheme() {
  return useContext(ThemeContext)
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<Theme>(() => {
    const stored = localStorage.getItem('line-theme')
    return (stored as Theme) || 'system'
  })
  const [systemDark, setSystemDark] = useState(() =>
    window.matchMedia('(prefers-color-scheme: dark)').matches,
  )

  useEffect(() => {
    const mq = window.matchMedia('(prefers-color-scheme: dark)')
    const handler = (e: MediaQueryListEvent) => setSystemDark(e.matches)
    mq.addEventListener('change', handler)
    return () => mq.removeEventListener('change', handler)
  }, [])

  const resolved = theme === 'system' ? (systemDark ? 'dark' : 'light') : theme

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', resolved)
  }, [resolved])

  const setTheme = useCallback((t: Theme) => {
    setThemeState(t)
    localStorage.setItem('line-theme', t)
  }, [])

  return (
    <ThemeContext.Provider value={{ theme, resolved, setTheme }}>
      {children}
    </ThemeContext.Provider>
  )
}

export function ThemeToggle() {
  const { theme, setTheme } = useTheme()
  const options: { value: Theme; label: string; icon: string }[] = [
    { value: 'dark', label: 'Dark', icon: '◐' },
    { value: 'light', label: 'Light', icon: '○' },
    { value: 'system', label: 'System', icon: '◑' },
  ]

  return (
    <div className="flex items-center gap-0.5 bg-surface rounded-md p-0.5" role="radiogroup" aria-label="Theme">
      {options.map((opt) => (
        <button
          key={opt.value}
          role="radio"
          aria-checked={theme === opt.value}
          aria-label={opt.label}
          onClick={() => setTheme(opt.value)}
          className={`px-2 py-0.5 rounded text-xs transition-colors ${
            theme === opt.value
              ? 'bg-accent/15 text-accent'
              : 'text-text-muted hover:text-text'
          }`}
        >
          {opt.icon}
        </button>
      ))}
    </div>
  )
}
