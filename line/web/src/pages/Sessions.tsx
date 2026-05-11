import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import { fetchSessions, fetchCars, getCarName, type Session, type Car } from '../api'

export function SessionsPage() {
  const [sessions, setSessions] = useState<Session[]>([])
  const [cars, setCars] = useState<Car[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    Promise.all([fetchSessions(), fetchCars()]).then(([{ sessions }, cars]) => {
      setSessions(sessions ?? [])
      setCars(cars)
      setLoading(false)
    }).catch(() => setLoading(false))
  }, [])

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="w-5 h-5 border-2 border-accent border-t-transparent rounded-full animate-spin" />
      </div>
    )
  }

  if (sessions.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center h-full gap-3 text-text-muted">
        <div className="text-4xl opacity-30">&#127937;</div>
        <p className="text-sm">No sessions recorded yet.</p>
        <p className="text-xs text-text-dim">Start a race in GT7 to begin capturing telemetry.</p>
      </div>
    )
  }

  return (
    <div className="p-5 overflow-auto h-full">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-base font-medium">Sessions</h2>
        <span className="text-xs text-text-muted">{sessions.length} total</span>
      </div>
      <div className="flex flex-col gap-2">
        {sessions.map((s) => (
          <Link
            key={s.id}
            to={`/sessions/${s.id}`}
            className="flex justify-between items-center p-4 bg-surface-2 rounded-lg border border-border hover:border-accent/40 transition-colors group"
          >
            <div>
              <div className="font-medium text-sm group-hover:text-accent transition-colors">
                {s.track_id || 'Unknown Track'}
              </div>
              <div className="text-xs text-text-muted mt-1">
                {getCarName(cars, s.car_code)} &middot; {s.lap_count} laps
              </div>
            </div>
            <div className="text-right">
              <div className="text-accent font-mono text-sm">
                {formatLapTime(s.best_lap_ms)}
              </div>
              <div className="text-xs text-text-dim mt-1">
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
