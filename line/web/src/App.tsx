import { NavLink, Outlet } from 'react-router'
import clsx from 'clsx'
import { ThemeToggle } from './components/ThemeProvider'

const navItems = [
  { to: '/', label: 'Sessions', end: true },
  { to: '/progression', label: 'Progression' },
  { to: '/tracks', label: 'Tracks' },
  { to: '/reference', label: 'Reference' },
  { to: '/compare', label: 'Compare' },
  { to: '/live', label: 'Live' },
] as const

export function App() {
  return (
    <div className="flex flex-col h-screen bg-bg text-text">
      <a href="#main-content" className="skip-link">Skip to content</a>
      <header className="flex items-center gap-4 sm:gap-6 px-4 sm:px-5 py-3 border-b border-border" role="banner">
        <h1 className="text-lg font-semibold tracking-tight">line</h1>
        <nav className="flex gap-3 sm:gap-4 text-sm overflow-x-auto" aria-label="Main navigation">
          {navItems.map(({ to, label, ...rest }) => (
            <NavLink
              key={to}
              to={to}
              className={({ isActive }: { isActive: boolean }) => clsx(
                'transition-colors whitespace-nowrap',
                isActive ? 'text-accent' : 'text-text-muted hover:text-text',
              )}
              {...rest}
            >
              {label}
            </NavLink>
          ))}
        </nav>
        <div className="ml-auto flex items-center gap-2">
          <ThemeToggle />
        </div>
      </header>
      <main id="main-content" className="flex-1 overflow-hidden" role="main">
        <Outlet />
      </main>
    </div>
  )
}
