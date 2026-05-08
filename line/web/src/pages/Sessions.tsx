import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import { fetchSessions, type Session } from '../api'

export function SessionsPage() {
  const [sessions, setSessions] = useState<Session[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetchSessions().then(({ sessions }) => {
      setSessions(sessions)
      setLoading(false)
    })
  }, [])

  if (loading) return <div style={{ padding: '2rem' }}>Loading...</div>

  if (sessions.length === 0) {
    return (
      <div style={{ padding: '2rem', color: '#666' }}>
        <p>No sessions recorded yet. Start a race in GT7 to begin capturing telemetry.</p>
      </div>
    )
  }

  return (
    <div style={{ padding: '1.5rem', overflow: 'auto', height: '100%' }}>
      <h2 style={{ margin: '0 0 1rem', fontSize: '1.1rem' }}>Sessions</h2>
      <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
        {sessions.map((s) => (
          <Link
            key={s.id}
            to={`/sessions/${s.id}`}
            style={{
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
              padding: '1rem',
              background: '#1a1a1a',
              borderRadius: '8px',
              textDecoration: 'none',
              color: '#e0e0e0',
              border: '1px solid #2a2a2a',
            }}
          >
            <div>
              <div style={{ fontWeight: 500 }}>{s.track_id || 'Unknown Track'}</div>
              <div style={{ fontSize: '0.85rem', color: '#888', marginTop: '0.25rem' }}>
                Car {s.car_code} · {s.lap_count} laps
              </div>
            </div>
            <div style={{ textAlign: 'right' }}>
              <div style={{ color: '#4fc3f7', fontFamily: 'monospace' }}>
                {formatLapTime(s.best_lap_ms)}
              </div>
              <div style={{ fontSize: '0.8rem', color: '#666', marginTop: '0.25rem' }}>
                {new Date(s.started_at).toLocaleDateString()}
              </div>
            </div>
          </Link>
        ))}
      </div>
    </div>
  )
}

function formatLapTime(ms: number): string {
  if (ms <= 0) return '--:--.---'
  const minutes = Math.floor(ms / 60000)
  const seconds = Math.floor((ms % 60000) / 1000)
  const millis = ms % 1000
  return `${minutes}:${seconds.toString().padStart(2, '0')}.${millis.toString().padStart(3, '0')}`
}
