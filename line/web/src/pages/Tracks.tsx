import { useEffect, useState } from 'react'
import clsx from 'clsx'
import { fetchTracks, type TrackInfo } from '../api'

export function TracksPage() {
  const [tracks, setTracks] = useState<TrackInfo[]>([])
  const [selectedTrack, setSelectedTrack] = useState<TrackInfo | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetchTracks()
      .then(({ tracks }) => {
        setTracks(tracks ?? [])
        if (tracks && tracks.length > 0) setSelectedTrack(tracks[0])
        setLoading(false)
      })
      .catch(() => setLoading(false))
  }, [])

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="w-5 h-5 border-2 border-accent border-t-transparent rounded-full animate-spin" />
      </div>
    )
  }

  if (tracks.length === 0) {
    return (
      <div className="flex items-center justify-center h-full text-text-dim text-sm">
        No track data available. Tracks are learned automatically from telemetry or loaded from community data.
      </div>
    )
  }

  return (
    <div className="flex flex-col md:flex-row h-full">
      <aside className="w-full md:w-64 border-b md:border-b-0 md:border-r border-border overflow-auto p-3">
        <h3 className="text-xs font-medium text-text-muted uppercase tracking-wider mb-2">
          Tracks ({tracks.length})
        </h3>
        <div className="flex md:flex-col gap-1.5 overflow-auto max-h-20 md:max-h-none">
          {tracks.map((track) => (
            <button
              key={track.track_id}
              onClick={() => setSelectedTrack(track)}
              className={clsx(
                'flex flex-col items-start w-full px-3 py-2 rounded-lg text-left text-xs transition-colors shrink-0',
                selectedTrack?.track_id === track.track_id
                  ? 'bg-accent-dim border border-accent/50 text-text'
                  : 'bg-surface-2 border border-border hover:border-border-2 text-text',
              )}
            >
              <span className="font-medium truncate w-full">{track.name}</span>
              <span className="text-text-dim mt-0.5">
                {track.country} · {track.corners.length} corners · {(track.length_m / 1000).toFixed(1)} km
              </span>
            </button>
          ))}
        </div>
      </aside>

      {selectedTrack && (
        <div className="flex-1 overflow-auto p-4 sm:p-6 space-y-5">
          <div>
            <h2 className="text-base font-semibold">{selectedTrack.name}</h2>
            <div className="flex items-center gap-3 mt-1 text-xs text-text-muted">
              <span>{selectedTrack.country}</span>
              <span className="text-text-dim">·</span>
              <span>{(selectedTrack.length_m / 1000).toFixed(2)} km</span>
              <span className="text-text-dim">·</span>
              <span>{selectedTrack.corners.length} corners</span>
              <span className="text-text-dim">·</span>
              <span className={clsx(
                'px-1.5 py-0.5 rounded',
                selectedTrack.source === 'community' ? 'bg-accent/10 text-accent' : 'bg-green/10 text-green',
              )}>
                {selectedTrack.source}
              </span>
            </div>
          </div>

          <div className="grid sm:grid-cols-3 gap-3">
            <StatCard label="Length" value={`${(selectedTrack.length_m / 1000).toFixed(2)} km`} />
            <StatCard label="Corners" value={`${selectedTrack.corners.length}`} />
            <StatCard
              label="Direction Split"
              value={`${selectedTrack.corners.filter(c => c.direction === 'left').length}L / ${selectedTrack.corners.filter(c => c.direction === 'right').length}R`}
            />
          </div>

          <div>
            <h3 className="text-xs font-medium text-text-muted uppercase tracking-wider mb-3">Corner Guide</h3>
            <div className="space-y-2">
              {selectedTrack.corners.map((corner) => (
                <div
                  key={corner.number}
                  className="flex items-start gap-3 bg-surface-2 rounded-lg border border-border px-4 py-3"
                >
                  <div className="flex items-center gap-2 shrink-0">
                    <span className="text-xs font-mono text-text-dim w-5 text-right">
                      {corner.number}
                    </span>
                    <span className={clsx(
                      'text-xs px-1.5 py-0.5 rounded font-medium',
                      corner.direction === 'left' ? 'bg-accent/10 text-accent' : 'bg-orange/10 text-orange',
                    )}>
                      {corner.direction === 'left' ? 'L' : 'R'}
                    </span>
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="text-sm font-medium text-text">{corner.name}</div>
                    {corner.notes && (
                      <div className="text-xs text-text-muted mt-0.5">{corner.notes}</div>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

function StatCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-surface-2 rounded-lg border border-border p-3">
      <div className="text-[10px] text-text-muted uppercase tracking-wider">{label}</div>
      <div className="text-sm font-mono font-medium mt-1 text-text">{value}</div>
    </div>
  )
}
