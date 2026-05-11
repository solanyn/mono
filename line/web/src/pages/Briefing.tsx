import { useEffect, useState } from 'react'
import { useParams } from 'react-router'
import clsx from 'clsx'
import { fetchSessionSummary, fetchTracks, fetchLaps, generateBriefing, type SessionSummary, type TrackInfo, type Lap } from '../api'

export function BriefingPage() {
  const { id } = useParams<{ id: string }>()
  const [summary, setSummary] = useState<SessionSummary | null>(null)
  const [track, setTrack] = useState<TrackInfo | null>(null)
  const [laps, setLaps] = useState<Lap[]>([])
  const [loading, setLoading] = useState(true)
  const [llmBriefing, setLlmBriefing] = useState<string | null>(null)
  const [generating, setGenerating] = useState(false)

  useEffect(() => {
    if (!id) return
    Promise.all([
      fetchSessionSummary(id).catch(() => null),
      fetchTracks().catch(() => ({ tracks: [] })),
      fetchLaps(id).catch(() => ({ laps: [] })),
    ]).then(([sum, { tracks }, { laps }]) => {
      setSummary(sum)
      setLaps(laps ?? [])
      if (sum && tracks) {
        const t = tracks.find((t) => t.track_id === sum.track_name || t.name === sum.track_name)
        if (t) setTrack(t)
      }
      setLoading(false)
    })
  }, [id])

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="w-5 h-5 border-2 border-accent border-t-transparent rounded-full animate-spin" />
      </div>
    )
  }

  if (!summary) {
    return (
      <div className="flex items-center justify-center h-full text-text-dim text-sm">
        No data available for pre-race briefing. Complete at least one session first.
      </div>
    )
  }

  const { consistency, journal, tyre_degradation, fuel_strategy } = summary
  const sortedLaps = [...laps].filter((l) => l.time_ms > 0).sort((a, b) => a.time_ms - b.time_ms)
  const medianLap = sortedLaps[Math.floor(sortedLaps.length / 2)]

  return (
    <div className="h-full overflow-auto p-4 sm:p-6 max-w-4xl mx-auto space-y-6">
      <div>
        <h2 className="text-lg font-semibold">Pre-Race Briefing</h2>
        <p className="text-xs text-text-muted mt-1">Based on your last session at this track</p>
      </div>

      <div className="grid sm:grid-cols-3 gap-3">
        <BriefCard label="Target Lap" value={formatLapTime(medianLap?.time_ms ?? journal.best_lap_ms)} sub="Realistic target (median)" accent="accent" />
        <BriefCard label="Best Lap" value={formatLapTime(journal.best_lap_ms)} sub="Personal best" accent="green" />
        <BriefCard label="Consistency" value={`${(consistency.consistency_score * 100).toFixed(0)}%`} sub={`CV: ${(consistency.lap_time_cv * 100).toFixed(1)}%`} accent={consistency.consistency_score > 0.9 ? 'green' : 'yellow'} />
      </div>

      {track && track.corners.length > 0 && (
        <section>
          <h3 className="text-xs font-medium text-text-muted uppercase tracking-wider mb-3">Corner Guide</h3>
          <div className="grid gap-2">
            {track.corners.map((c) => (
              <div key={c.number} className="flex items-start gap-3 bg-surface-2 rounded-lg px-3 py-2 border border-border">
                <span className="text-xs font-mono text-text-dim w-6 shrink-0">T{c.number}</span>
                <span className={clsx('text-xs px-1.5 py-0.5 rounded shrink-0', c.direction === 'left' ? 'bg-accent/10 text-accent' : 'bg-orange/10 text-orange')}>
                  {c.direction === 'left' ? 'L' : 'R'}
                </span>
                <div className="flex-1 min-w-0">
                  <div className="text-xs font-medium text-text">{c.name}</div>
                  {c.notes && <div className="text-[11px] text-text-muted mt-0.5">{c.notes}</div>}
                </div>
              </div>
            ))}
          </div>
        </section>
      )}

      <section>
        <h3 className="text-xs font-medium text-text-muted uppercase tracking-wider mb-3">Focus Areas</h3>
        <div className="grid sm:grid-cols-2 gap-3">
          {journal.areas_to_improve.length > 0 && (
            <div className="bg-surface-2 rounded-lg border border-border p-4">
              <h4 className="text-xs font-medium text-orange mb-2">Work On</h4>
              <ul className="space-y-1.5 text-xs text-text-muted">
                {journal.areas_to_improve.map((a, i) => <li key={i} className="flex gap-2"><span className="text-orange shrink-0">&#8226;</span>{a}</li>)}
              </ul>
            </div>
          )}
          {journal.highlights.length > 0 && (
            <div className="bg-surface-2 rounded-lg border border-border p-4">
              <h4 className="text-xs font-medium text-green mb-2">Strengths</h4>
              <ul className="space-y-1.5 text-xs text-text-muted">
                {journal.highlights.map((h, i) => <li key={i} className="flex gap-2"><span className="text-green shrink-0">&#8226;</span>{h}</li>)}
              </ul>
            </div>
          )}
        </div>
      </section>

      <section className="grid sm:grid-cols-2 gap-3">
        <div className="bg-surface-2 rounded-lg border border-border p-4">
          <h4 className="text-xs font-medium text-text-muted uppercase tracking-wider mb-2">Tyre Strategy</h4>
          <div className="space-y-1 text-xs">
            <div className="flex justify-between"><span className="text-text-muted">Compound</span><span className="font-mono text-text">{tyre_degradation.compound_guess}</span></div>
            <div className="flex justify-between"><span className="text-text-muted">Deg Rate</span><span className="font-mono text-text">{tyre_degradation.degradation_rate.toFixed(2)} C/lap</span></div>
            <div className="flex justify-between"><span className="text-text-muted">F/R Balance</span><span className="font-mono text-text">{tyre_degradation.front_rear_balance.toFixed(2)}</span></div>
          </div>
        </div>
        <div className="bg-surface-2 rounded-lg border border-border p-4">
          <h4 className="text-xs font-medium text-text-muted uppercase tracking-wider mb-2">Fuel Plan</h4>
          <div className="space-y-1 text-xs">
            <div className="flex justify-between"><span className="text-text-muted">Per Lap</span><span className="font-mono text-text">{fuel_strategy.consumption_per_lap.toFixed(2)} L</span></div>
            <div className="flex justify-between"><span className="text-text-muted">Pit Window</span><span className="font-mono text-text">Lap {fuel_strategy.optimal_pit_lap || 'N/A'}</span></div>
            <div className="flex justify-between"><span className="text-text-muted">Range</span><span className="font-mono text-text">{fuel_strategy.laps_remaining} laps</span></div>
          </div>
        </div>
      </section>

      {journal.corner_notes.length > 0 && (
        <section>
          <h3 className="text-xs font-medium text-text-muted uppercase tracking-wider mb-3">Corner Notes</h3>
          <div className="bg-surface-2 rounded-lg border border-border p-4 space-y-1.5">
            {journal.corner_notes.map((n, i) => (
              <p key={i} className="text-xs text-text-muted">{n}</p>
            ))}
          </div>
        </section>
      )}

      <section>
        <div className="flex items-center justify-between mb-3">
          <h3 className="text-xs font-medium text-text-muted uppercase tracking-wider">AI Briefing</h3>
          <button
            onClick={() => {
              if (!id || generating) return
              setGenerating(true)
              generateBriefing(id)
                .then(({ briefing }) => setLlmBriefing(briefing))
                .catch(() => setLlmBriefing('Failed to generate briefing. Check LLM connection.'))
                .finally(() => setGenerating(false))
            }}
            disabled={generating}
            className={clsx(
              'px-3 py-1.5 rounded text-xs font-medium transition-colors',
              generating ? 'bg-surface-2 text-text-dim cursor-wait' : 'bg-accent/10 text-accent hover:bg-accent/20'
            )}
          >
            {generating ? 'Generating...' : llmBriefing ? 'Regenerate' : 'Generate Briefing'}
          </button>
        </div>
        {llmBriefing && (
          <div className="bg-surface-2 rounded-lg border border-border p-4 prose prose-invert prose-sm max-w-none">
            {llmBriefing.split('\n').map((line, i) => (
              line.trim() ? <p key={i} className="text-xs text-text-muted leading-relaxed mb-2 last:mb-0">{line}</p> : null
            ))}
          </div>
        )}
        {!llmBriefing && !generating && (
          <p className="text-xs text-text-dim">Click generate to get a personalized pre-race briefing from the AI coach.</p>
        )}
      </section>
    </div>
  )
}

function BriefCard({ label, value, sub, accent }: { label: string; value: string; sub: string; accent: string }) {
  const colorClass = accent === 'green' ? 'text-green' : accent === 'yellow' ? 'text-yellow' : 'text-accent'
  return (
    <div className="bg-surface-2 rounded-lg border border-border p-4 text-center">
      <div className="text-[10px] text-text-muted uppercase tracking-wider">{label}</div>
      <div className={clsx('text-xl font-mono font-bold mt-1', colorClass)}>{value}</div>
      <div className="text-[10px] text-text-dim mt-1">{sub}</div>
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
