import { NavLink, Outlet } from 'react-router'

const navStyle = (active: boolean) => ({ color: active ? '#4fc3f7' : '#888', textDecoration: 'none' as const })

export function App() {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100vh', background: '#0a0a0a', color: '#e0e0e0' }}>
      <header style={{ display: 'flex', alignItems: 'center', gap: '2rem', padding: '0.75rem 1.5rem', borderBottom: '1px solid #222' }}>
        <h1 style={{ margin: 0, fontSize: '1.25rem', fontWeight: 600 }}>line</h1>
        <nav style={{ display: 'flex', gap: '1rem' }}>
          <NavLink to="/" style={({ isActive }: { isActive: boolean }) => navStyle(isActive)}>Sessions</NavLink>
          <NavLink to="/live" style={({ isActive }: { isActive: boolean }) => navStyle(isActive)}>Live</NavLink>
        </nav>
      </header>
      <main style={{ flex: 1, overflow: 'hidden' }}>
        <Outlet />
      </main>
    </div>
  )
}
