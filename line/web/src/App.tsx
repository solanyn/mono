import { NavLink, Outlet } from 'react-router'
import clsx from 'clsx'

export function App() {
  return (
    <div className="flex flex-col h-screen bg-bg text-text">
      <header className="flex items-center gap-4 sm:gap-6 px-4 sm:px-5 py-3 border-b border-border">
        <h1 className="text-lg font-semibold tracking-tight">line</h1>
        <nav className="flex gap-3 sm:gap-4 text-sm">
          <NavLink to="/" className={({ isActive }: { isActive: boolean }) => clsx('transition-colors', isActive ? 'text-accent' : 'text-text-muted hover:text-text')} end>
            Sessions
          </NavLink>
          <NavLink to="/progression" className={({ isActive }: { isActive: boolean }) => clsx('transition-colors', isActive ? 'text-accent' : 'text-text-muted hover:text-text')}>
            Progression
          </NavLink>
          <NavLink to="/tracks" className={({ isActive }: { isActive: boolean }) => clsx('transition-colors', isActive ? 'text-accent' : 'text-text-muted hover:text-text')}>
            Tracks
          </NavLink>
          <NavLink to="/reference" className={({ isActive }: { isActive: boolean }) => clsx('transition-colors', isActive ? 'text-accent' : 'text-text-muted hover:text-text')}>
            Reference
          </NavLink>
          <NavLink to="/compare" className={({ isActive }: { isActive: boolean }) => clsx('transition-colors', isActive ? 'text-accent' : 'text-text-muted hover:text-text')}>
            Compare
          </NavLink>
          <NavLink to="/live" className={({ isActive }: { isActive: boolean }) => clsx('transition-colors', isActive ? 'text-accent' : 'text-text-muted hover:text-text')}>
            Live
          </NavLink>
        </nav>
      </header>
      <main className="flex-1 overflow-hidden">
        <Outlet />
      </main>
    </div>
  )
}
