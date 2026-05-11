import { useEffect, useState } from 'react'
import {
  fetchComparisonTracks,
  fetchCarComparisons,
  fetchCars,
  getCarName,
  type CarComparison,
  type Car,
} from '../api'

export function ComparePage() {
  const [tracks, setTracks] = useState<string[]>([])
  const [selectedTrack, setSelectedTrack] = useState<string>('')
  const [comparisons, setComparisons] = useState<CarComparison[]>([])
  const [cars, setCars] = useState<Car[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    Promise.all([fetchComparisonTracks(), fetchCars()]).then(([t, c]) => {
      setTracks(t)
      setCars(c)
      if (t.length > 0) setSelectedTrack(t[0])
      setLoading(false)
    })
  }, [])

  useEffect(() => {
    if (!selectedTrack) return
    fetchCarComparisons(selectedTrack).then(setComparisons)
  }, [selectedTrack])

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="w-5 h-5 border-2 border-accent border-t-transparent rounded-full animate-spin" />
      </div>
    )
  }

  if (tracks.length === 0) {
    return (
      <div className="p-6 text-text-muted">
        No track data available yet. Complete some sessions to compare cars.
      </div>
    )
  }

  const best = comparisons.length > 0 ? comparisons[0].best_lap_ms : 0

  return (
    <div className="p-6 overflow-auto h-full">
      <div className="flex items-center gap-4 mb-6">
        <h2 className="text-lg font-semibold">Cross-Car Comparison</h2>
        <select
          value={selectedTrack}
          onChange={(e) => setSelectedTrack(e.target.value)}
          className="bg-surface border border-border rounded px-3 py-1.5 text-sm text-text"
        >
          {tracks.map((t) => (
            <option key={t} value={t}>{t}</option>
          ))}
        </select>
      </div>

      {comparisons.length === 0 ? (
        <p className="text-text-muted text-sm">No lap data for this track yet.</p>
      ) : (
        <div className="space-y-3">
          <div className="grid grid-cols-6 gap-2 text-xs text-text-muted font-medium uppercase tracking-wider px-4 pb-2 border-b border-border">
            <span className="col-span-2">Car</span>
            <span>Best Lap</span>
            <span>Avg Lap</span>
            <span>Sessions</span>
            <span>Laps</span>
          </div>
          {comparisons.map((comp, idx) => {
            const delta = comp.best_lap_ms - best
            return (
              <div
                key={comp.car_code}
                className="grid grid-cols-6 gap-2 items-center px-4 py-3 bg-surface rounded-lg border border-border"
              >
                <div className="col-span-2 flex items-center gap-2">
                  <span className="text-xs text-text-dim w-5">{idx + 1}.</span>
                  <span className="font-medium text-sm">{getCarName(cars, comp.car_code)}</span>
                </div>
                <div className="flex items-center gap-2">
                  <span className="font-mono text-accent text-sm">{formatLapTime(comp.best_lap_ms)}</span>
                  {delta > 0 && (
                    <span className="text-[10px] text-red-400">+{(delta / 1000).toFixed(3)}s</span>
                  )}
                </div>
                <span className="font-mono text-text-muted text-sm">{formatLapTime(comp.avg_lap_ms)}</span>
                <span className="text-sm text-text-muted">{comp.sessions}</span>
                <span className="text-sm text-text-muted">{comp.total_laps}</span>
              </div>
            )
          })}
        </div>
      )}

      {comparisons.length >= 2 && (
        <div className="mt-6 p-4 bg-surface rounded-lg border border-border">
          <h3 className="text-sm font-medium mb-3">Gap Analysis</h3>
          <div className="space-y-2">
            {comparisons.slice(0, 5).map((comp, idx) => {
              const delta = comp.best_lap_ms - best
              const maxDelta = comparisons[comparisons.length - 1].best_lap_ms - best
              const pct = maxDelta > 0 ? (delta / maxDelta) * 100 : 0
              return (
                <div key={comp.car_code} className="flex items-center gap-3">
                  <span className="text-xs text-text-muted w-32 truncate">{getCarName(cars, comp.car_code)}</span>
                  <div className="flex-1 h-4 bg-bg rounded overflow-hidden">
                    <div
                      className="h-full rounded"
                      style={{
                        width: `${idx === 0 ? 100 : Math.max(5, 100 - pct)}%`,
                        backgroundColor: idx === 0 ? 'var(--color-accent)' : `hsl(${Math.max(0, 120 - pct * 1.2)}, 60%, 45%)`,
                      }}
                    />
                  </div>
                  <span className="text-xs font-mono text-text-muted w-16 text-right">
                    {idx === 0 ? 'fastest' : `+${(delta / 1000).toFixed(2)}s`}
                  </span>
                </div>
              )
            })}
          </div>
        </div>
      )}
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
