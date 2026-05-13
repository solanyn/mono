import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import { fetchSessions, fetchCars, getCarName, type Session, type Car } from '../api'
import { SessionCardSkeleton } from '../components/Skeleton'

export function SessionsPage() {
  const [sessions, setSessions] = useState<Session[]>([])
  const [cars, setCars] = useState<Car[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    Promise.all([fetchSessions(), fetchCars()]).then(([{ sessions }, cars]) => {
      setSessions(sessions ?? [])
      setCars(cars)
      setLoading(false)
    }).catch((err) => {
      setError(err.message || 'Failed to load sessions')
      setLoading(false)
    })
  }, [])

  if (loading) {
    return (
      <div className="p-5 space-y-2" aria-label="Loading sessions" role="status">
        {Array.from({ length: 5 }).map((_, i) => (
          <SessionCardSkeleton key={i} />
        ))}
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center h-full gap-3 p-8" role="alert">
        <p className="text-sm text-red">{error}</p>
        <button
          onClick={() => { setLoading(true); setError(null); window.location.reload() }}
          className="px-4 py-1.5 rounded-md text-xs font-medium bg-accent/10 text-accent hover:bg-accent/20 transition-colors focus:outline-none focus:ring-2 focus:ring-accent/50"
        >
          Retry
        </button>
      </div>
    )
  }

  if (sessions.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center h-full gap-3 text-text-muted animate-fade-in">
        <div className="text-4xl opacity-30" aria-hidden="true">&#127937;</div>
        <p className="text-sm">No sessions recorded yet.</p>
        <p className="text-xs text-text-dim">Start a race in GT7 to begin capturing telemetry.</p>
      </div>
    )
  }

  return (
    <div className="p-5 overflow-auto h-full">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-base font-medium">Sessions</h2>
        <span className="text-xs text-text-muted" aria-label={`${sessions.length} total sessions`}>{sessions.length} total</span>
      </div>
      <div className="flex flex-col gap-2" role="list" aria-label="Session list">
        {sessions.map((s, i) => (
          <Link
            key={s.id}
            to={`/sessions/${s.id}`}
            className="flex justify-between items-center p-4 bg-surface-2 rounded-lg border border-border hover:border-accent/40 transition-all duration-200 group animate-fade-in hover:translate-x-0.5"
            style={{ animationDelay: `${i * 30}ms` }}
            role="listitem"
            aria-label={`${s.track_id || 'Unknown Track'}, ${getCarName(cars, s.car_code)}, best lap ${formatLapTime(s.best_lap_ms)}`}
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
