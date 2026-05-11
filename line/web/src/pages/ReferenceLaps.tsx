import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import {
  fetchReferenceLaps,
  deleteReferenceLap,
  fetchCars,
  getCarName,
  type ReferenceLap,
  type Car,
} from '../api'

export function ReferenceLapsPage() {
  const [refs, setRefs] = useState<ReferenceLap[]>([])
  const [cars, setCars] = useState<Car[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    Promise.all([fetchReferenceLaps(), fetchCars()]).then(([r, c]) => {
      setRefs(r)
      setCars(c)
      setLoading(false)
    })
  }, [])

  const handleDelete = async (id: number) => {
    await deleteReferenceLap(id)
    setRefs(refs.filter(r => r.id !== id))
  }

  if (loading) {
    return <div className="p-6 text-text-muted">Loading...</div>
  }

  const grouped = refs.reduce<Record<string, ReferenceLap[]>>((acc, r) => {
    const key = `${r.track_id}|${r.car_code}`
    if (!acc[key]) acc[key] = []
    acc[key].push(r)
    return acc
  }, {})

  return (
    <div className="p-6 overflow-auto h-full">
      <h2 className="text-lg font-semibold mb-4">Reference Laps</h2>
      {refs.length === 0 ? (
        <p className="text-text-muted">
          No reference laps saved yet. Set a lap as reference from the session detail page.
        </p>
      ) : (
        <div className="space-y-4">
          {Object.entries(grouped).map(([key, laps]) => {
            const [trackId, carCodeStr] = key.split('|')
            const carCode = parseInt(carCodeStr)
            return (
              <div key={key} className="bg-surface rounded-lg border border-border p-4">
                <div className="flex items-center justify-between mb-3">
                  <div>
                    <span className="font-medium">{trackId}</span>
                    <span className="text-text-muted ml-2 text-sm">
                      {getCarName(cars, carCode)}
                    </span>
                  </div>
                </div>
                <div className="space-y-2">
                  {laps.map(lap => (
                    <div
                      key={lap.id}
                      className="flex items-center justify-between bg-bg rounded px-3 py-2"
                    >
                      <div className="flex items-center gap-4">
                        <span className="text-xs bg-accent/20 text-accent px-2 py-0.5 rounded">
                          {lap.label}
                        </span>
                        <span className="font-mono text-accent">
                          {formatLapTime(lap.time_ms)}
                        </span>
                        <Link
                          to={`/sessions/${lap.session_id}`}
                          className="text-xs text-text-muted hover:text-accent"
                        >
                          Session → Lap {lap.lap_number}
                        </Link>
                      </div>
                      <button
                        onClick={() => handleDelete(lap.id)}
                        className="text-xs text-red-400 hover:text-red-300"
                      >
                        Remove
                      </button>
                    </div>
                  ))}
                </div>
              </div>
            )
          })}
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
