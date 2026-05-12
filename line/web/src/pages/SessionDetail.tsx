import { useEffect, useState, useMemo } from 'react'
import { useParams, Link } from 'react-router'
import { Canvas } from '@react-three/fiber'
import { OrbitControls, Line } from '@react-three/drei'
import * as THREE from 'three'
import clsx from 'clsx'
import { fetchLaps, fetchTelemetry, fetchLapMetrics, fetchSessionSummary, fetchSession, fetchReferenceLapTelemetry, setReferenceLap, fetchJournal, generateJournal, fetchLapBraking, fetchLapStability, fetchRacingLine, fetchFatigue, fetchTimeDeltas, type Lap, type TelemetryFrame, type LapMetrics, type SessionSummary, type Journal, type BrakingAnalysis, type StabilityAnalysis, type RacingLineAnalysis, type FatigueAnalysis, type TimeDeltaEntry } from '../api'
import { TelemetryChart } from '../TelemetryChart'

export function SessionDetail() {
  const { id } = useParams<{ id: string }>()
  const [laps, setLaps] = useState<Lap[]>([])
  const [selectedLap, setSelectedLap] = useState<number | null>(null)
  const [compareLap, setCompareLap] = useState<number | null>(null)
  const [frames, setFrames] = useState<TelemetryFrame[]>([])
  const [compareFrames, setCompareFrames] = useState<TelemetryFrame[]>([])
  const [refFrames, setRefFrames] = useState<TelemetryFrame[]>([])
  const [metrics, setMetrics] = useState<LapMetrics | null>(null)
  const [summary, setSummary] = useState<SessionSummary | null>(null)
  const [loading, setLoading] = useState(true)
  const [tab, setTab] = useState<'track' | 'metrics' | 'analytics' | 'summary' | 'journal'>('track')
  const [session, setSession] = useState<{ car_code: number; track_id?: string } | null>(null)
  const [refSaving, setRefSaving] = useState(false)
  const [journal, setJournal] = useState<Journal | null>(null)
  const [journalLoading, setJournalLoading] = useState(false)
  const [braking, setBraking] = useState<BrakingAnalysis | null>(null)
  const [stability, setStability] = useState<StabilityAnalysis | null>(null)
  const [racingLine, setRacingLine] = useState<RacingLineAnalysis | null>(null)
  const [fatigue, setFatigue] = useState<FatigueAnalysis | null>(null)
  const [timeDeltas, setTimeDeltas] = useState<TimeDeltaEntry[]>([])

  useEffect(() => {
    if (!id) return
    fetchSession(id).then(s => setSession(s)).catch(() => {})
    fetchLaps(id).then(({ laps }) => {
      setLaps(laps ?? [])
      setLoading(false)
      if (laps && laps.length > 0) setSelectedLap(laps[0].lap_number)
    }).catch(() => setLoading(false))
    fetchSessionSummary(id).then(setSummary).catch(() => {})
    fetchJournal(id).then(setJournal).catch(() => {})
    fetchRacingLine(id).then(setRacingLine).catch(() => setRacingLine(null))
    fetchFatigue(id).then(setFatigue).catch(() => setFatigue(null))
    fetchTimeDeltas(id).then(d => setTimeDeltas(d.deltas ?? [])).catch(() => setTimeDeltas([]))
  }, [id])

  useEffect(() => {
    if (!session?.track_id || !session?.car_code) return
    fetchReferenceLapTelemetry(session.track_id, session.car_code).then(({ frames }) => {
      setRefFrames(frames ?? [])
    }).catch(() => setRefFrames([]))
  }, [session])

  useEffect(() => {
    if (!id || selectedLap === null) return
    fetchTelemetry(id, selectedLap, 2).then(({ frames }) => setFrames(frames ?? []))
    fetchLapMetrics(id, selectedLap).then(setMetrics).catch(() => setMetrics(null))
    fetchLapBraking(id, selectedLap).then(setBraking).catch(() => setBraking(null))
    fetchLapStability(id, selectedLap).then(setStability).catch(() => setStability(null))
  }, [id, selectedLap])

  useEffect(() => {
    if (!id || compareLap === null) return
    fetchTelemetry(id, compareLap, 2).then(({ frames }) => setCompareFrames(frames ?? []))
  }, [id, compareLap])

  useEffect(() => {
    if (compareLap === null) setCompareFrames([])
  }, [compareLap])

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="w-5 h-5 border-2 border-accent border-t-transparent rounded-full animate-spin" />
      </div>
    )
  }

  const bestLap = laps.reduce((best, l) => (l.time_ms > 0 && l.time_ms < (best?.time_ms ?? Infinity)) ? l : best, laps[0])

  return (
    <div className="flex flex-col md:flex-row h-full">
      <aside className="w-full md:w-52 border-b md:border-b-0 md:border-r border-border overflow-auto p-3 flex md:flex-col gap-1 max-h-24 md:max-h-none">
        <h3 className="hidden md:block text-xs font-medium text-text-muted uppercase tracking-wider mb-2">Laps</h3>
        {laps.length === 0 && <p className="text-xs text-text-dim">No laps yet</p>}
        {laps.map((lap) => (
          <button
            key={lap.lap_number}
            onClick={() => setSelectedLap(lap.lap_number)}
            onContextMenu={(e) => {
              e.preventDefault()
              setCompareLap(compareLap === lap.lap_number ? null : lap.lap_number)
            }}
            className={clsx(
              'flex justify-between items-center w-full px-2.5 py-1.5 rounded text-xs transition-colors text-left',
              selectedLap === lap.lap_number
                ? 'bg-accent-dim border border-accent/50 text-text'
                : compareLap === lap.lap_number
                  ? 'bg-orange/10 border border-orange/30 text-text'
                  : 'bg-surface-2 border border-border hover:border-border-2 text-text',
            )}
          >
            <span className="flex items-center gap-1.5">
              {lap.lap_number === bestLap?.lap_number && <span className="text-yellow text-[10px]">&#9733;</span>}
              Lap {lap.lap_number}
            </span>
            <span className={clsx(
              'font-mono',
              selectedLap === lap.lap_number ? 'text-accent' : 'text-text-muted',
            )}>
              {formatLapTime(lap.time_ms)}
            </span>
          </button>
        ))}
        {laps.length > 1 && (
          <p className="text-[10px] text-text-dim mt-2 px-1">Right-click a lap to compare</p>
        )}
      </aside>

      <div className="flex-1 flex flex-col min-w-0">
        <div className="flex items-center gap-1 px-4 py-2 border-b border-border">
          {(['track', 'metrics', 'analytics', 'summary', 'journal'] as const).map((t) => (
            <button
              key={t}
              onClick={() => setTab(t)}
              className={clsx(
                'px-3 py-1 rounded text-xs font-medium transition-colors capitalize',
                tab === t ? 'bg-accent/15 text-accent' : 'text-text-muted hover:text-text',
              )}
            >
              {t}
            </button>
          ))}
          {selectedLap !== null && (
            <Link
              to={`/sessions/${id}/replay?lap=${selectedLap}`}
              className="px-3 py-1 rounded text-xs font-medium text-text-muted hover:text-accent transition-colors"
            >
              Replay
            </Link>
          )}
          <Link
            to={`/sessions/${id}/briefing`}
            className="px-3 py-1 rounded text-xs font-medium text-text-muted hover:text-accent transition-colors"
          >
            Briefing
          </Link>
          {selectedLap !== null && session?.track_id && (
            <button
              onClick={async () => {
                if (!id || selectedLap === null || !session?.track_id) return
                const lap = laps.find(l => l.lap_number === selectedLap)
                if (!lap) return
                setRefSaving(true)
                try {
                  await setReferenceLap({
                    track_id: session.track_id,
                    car_code: session.car_code,
                    session_id: id,
                    lap_number: selectedLap,
                    time_ms: lap.time_ms,
                    s3_key: lap.s3_key,
                  })
                  fetchReferenceLapTelemetry(session.track_id, session.car_code).then(({ frames }) => {
                    setRefFrames(frames ?? [])
                  }).catch(() => {})
                } catch {}
                setRefSaving(false)
              }}
              disabled={refSaving}
              className="px-3 py-1 rounded text-xs font-medium text-text-muted hover:text-green transition-colors disabled:opacity-50"
            >
              {refSaving ? 'Saving...' : 'Set as Reference'}
            </button>
          )}
          {refFrames.length > 0 && (
            <span className="text-[10px] text-green ml-1">ref loaded</span>
          )}
          {compareLap !== null && (
            <span className="ml-auto text-xs text-orange">
              Comparing with Lap {compareLap}
              <button onClick={() => setCompareLap(null)} className="ml-2 text-text-dim hover:text-text">&times;</button>
            </span>
          )}
        </div>

        {tab === 'track' && (
          <div className="flex-1 flex flex-col min-h-0">
            <div className="flex-1 min-h-0 bg-surface">
              <Canvas camera={{ position: [0, 200, 200], fov: 60 }}>
                <ambientLight intensity={0.4} />
                <directionalLight position={[100, 200, 100]} intensity={0.8} />
                {frames.length > 0 && <TrackLine frames={frames} color="primary" />}
                {compareFrames.length > 0 && <TrackLine frames={compareFrames} color="compare" />}
                {refFrames.length > 0 && <TrackLine frames={refFrames} color="reference" />}
                <gridHelper args={[800, 40, '#333333', '#222222']} />
                <OrbitControls enableDamping dampingFactor={0.1} maxPolarAngle={Math.PI / 2.1} />
              </Canvas>
            </div>
            <div className="h-[35%] border-t border-border overflow-auto p-4">
              <TelemetryChart frames={frames} compareFrames={compareFrames.length > 0 ? compareFrames : undefined} />
            </div>
          </div>
        )}

        {tab === 'metrics' && metrics && (
          <div className="flex-1 overflow-auto p-5">
            <MetricsPanel metrics={metrics} />
          </div>
        )}

        {tab === 'metrics' && !metrics && (
          <div className="flex-1 flex items-center justify-center text-text-dim text-sm">
            No metrics available for this lap
          </div>
        )}

        {tab === 'analytics' && (
          <div className="flex-1 overflow-auto p-5">
            <AnalyticsPanel braking={braking} stability={stability} racingLine={racingLine} fatigue={fatigue} timeDeltas={timeDeltas} />
          </div>
        )}

        {tab === 'summary' && summary && (
          <div className="flex-1 overflow-auto p-5">
            <SummaryPanel summary={summary} />
          </div>
        )}

        {tab === 'summary' && !summary && (
          <div className="flex-1 flex items-center justify-center text-text-dim text-sm">
            No session summary available yet
          </div>
        )}

        {tab === 'journal' && (
          <div className="flex-1 overflow-auto p-5">
            {journal ? (
              <div className="bg-surface-2 rounded-lg border border-border p-5">
                <div className="flex items-center justify-between mb-4">
                  <h4 className="text-xs font-medium text-text-muted uppercase tracking-wider">Session Journal</h4>
                  <span className="text-[10px] text-text-dim">{new Date(journal.created_at).toLocaleString()}</span>
                </div>
                <div className="space-y-3 text-sm text-text leading-relaxed">
                  {journal.content.split('\n\n').map((p, i) => <p key={i}>{p}</p>)}
                </div>
              </div>
            ) : (
              <div className="flex flex-col items-center justify-center h-full gap-4">
                <p className="text-text-dim text-sm">No journal entry yet</p>
                <button
                  onClick={async () => {
                    if (!id) return
                    setJournalLoading(true)
                    try {
                      const j = await generateJournal(id)
                      setJournal(j)
                    } catch {}
                    setJournalLoading(false)
                  }}
                  disabled={journalLoading}
                  className="px-4 py-2 rounded-lg bg-accent/15 text-accent text-sm font-medium hover:bg-accent/25 transition-colors disabled:opacity-50"
                >
                  {journalLoading ? 'Generating...' : 'Generate Journal'}
                </button>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  )
}

function MetricsPanel({ metrics }: { metrics: LapMetrics }) {
  return (
    <div className="space-y-6">
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
        <StatCard label="Distance" value={`${(metrics.total_distance_m / 1000).toFixed(2)} km`} />
        <StatCard label="Top Speed" value={`${metrics.top_speed.toFixed(0)} km/h`} />
        <StatCard label="Avg Speed" value={`${metrics.avg_speed.toFixed(0)} km/h`} />
        <StatCard label="Brake Count" value={`${metrics.brake_count}`} />
        <StatCard label="Throttle" value={`${metrics.throttle_pct.toFixed(0)}%`} accent="green" />
        <StatCard label="Coast" value={`${metrics.coast_pct.toFixed(0)}%`} accent="yellow" />
        <StatCard label="Brake" value={`${metrics.brake_pct.toFixed(0)}%`} accent="red" />
        <StatCard label="Fuel Used" value={`${metrics.fuel_used.toFixed(1)} L`} />
      </div>

      {metrics.corners && metrics.corners.length > 0 && (
        <div>
          <h3 className="text-xs font-medium text-text-muted uppercase tracking-wider mb-3">Corners</h3>
          <div className="grid gap-2">
            {metrics.corners.map((c, i) => (
              <div key={i} className="flex items-center gap-3 bg-surface-2 rounded-lg px-3 py-2 border border-border">
                <span className="text-xs font-mono text-text-dim w-6">T{i + 1}</span>
                <span className={clsx('text-xs px-1.5 py-0.5 rounded', c.direction === 'left' ? 'bg-accent/10 text-accent' : 'bg-orange/10 text-orange')}>
                  {c.direction === 'left' ? 'L' : 'R'}
                </span>
                <div className="flex-1 grid grid-cols-3 gap-2 text-xs">
                  <div><span className="text-text-dim">Entry</span> <span className="font-mono text-text ml-1">{c.entry_speed.toFixed(0)}</span></div>
                  <div><span className="text-text-dim">Apex</span> <span className="font-mono text-text ml-1">{c.apex_speed.toFixed(0)}</span></div>
                  <div><span className="text-text-dim">Exit</span> <span className="font-mono text-text ml-1">{c.exit_speed.toFixed(0)}</span></div>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

function SummaryPanel({ summary }: { summary: SessionSummary }) {
  const { consistency, journal, tyre_degradation, fuel_strategy } = summary
  return (
    <div className="space-y-6">
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
        <StatCard label="Consistency" value={`${(consistency.consistency_score * 100).toFixed(0)}%`} accent={consistency.consistency_score > 0.9 ? 'green' : consistency.consistency_score > 0.7 ? 'yellow' : 'red'} />
        <StatCard label="Best Lap" value={formatLapTime(journal.best_lap_ms)} accent="accent" />
        <StatCard label="Lap Time CV" value={`${(consistency.lap_time_cv * 100).toFixed(1)}%`} />
        <StatCard label="Laps" value={`${journal.total_laps}`} />
      </div>

      <div className="grid md:grid-cols-2 gap-4">
        <div className="bg-surface-2 rounded-lg border border-border p-4">
          <h4 className="text-xs font-medium text-text-muted uppercase tracking-wider mb-2">Tyre Degradation</h4>
          <div className="space-y-1 text-xs">
            <div className="flex justify-between"><span className="text-text-muted">Deg Rate</span><span className="font-mono">{tyre_degradation.degradation_rate.toFixed(2)} C/lap</span></div>
            <div className="flex justify-between"><span className="text-text-muted">Est. Laps Left</span><span className="font-mono">{tyre_degradation.estimated_laps_remaining}</span></div>
            <div className="flex justify-between"><span className="text-text-muted">Compound</span><span className="font-mono">{tyre_degradation.compound_guess}</span></div>
            <div className="flex justify-between"><span className="text-text-muted">F/R Balance</span><span className="font-mono">{tyre_degradation.front_rear_balance.toFixed(2)}</span></div>
          </div>
        </div>

        <div className="bg-surface-2 rounded-lg border border-border p-4">
          <h4 className="text-xs font-medium text-text-muted uppercase tracking-wider mb-2">Fuel Strategy</h4>
          <div className="space-y-1 text-xs">
            <div className="flex justify-between"><span className="text-text-muted">Per Lap</span><span className="font-mono">{fuel_strategy.consumption_per_lap.toFixed(2)} L</span></div>
            <div className="flex justify-between"><span className="text-text-muted">Remaining</span><span className="font-mono">{fuel_strategy.fuel_remaining.toFixed(1)} L</span></div>
            <div className="flex justify-between"><span className="text-text-muted">Laps Left</span><span className="font-mono">{fuel_strategy.laps_remaining}</span></div>
            <div className="flex justify-between"><span className="text-text-muted">Pit Lap</span><span className="font-mono">{fuel_strategy.optimal_pit_lap || 'N/A'}</span></div>
          </div>
        </div>
      </div>

      {journal.highlights.length > 0 && (
        <div className="bg-surface-2 rounded-lg border border-border p-4">
          <h4 className="text-xs font-medium text-green uppercase tracking-wider mb-2">Highlights</h4>
          <ul className="space-y-1 text-xs text-text-muted">
            {journal.highlights.map((h, i) => <li key={i}>{h}</li>)}
          </ul>
        </div>
      )}

      {journal.areas_to_improve.length > 0 && (
        <div className="bg-surface-2 rounded-lg border border-border p-4">
          <h4 className="text-xs font-medium text-orange uppercase tracking-wider mb-2">Areas to Improve</h4>
          <ul className="space-y-1 text-xs text-text-muted">
            {journal.areas_to_improve.map((a, i) => <li key={i}>{a}</li>)}
          </ul>
        </div>
      )}

      {journal.summary && (
        <div className="bg-surface-2 rounded-lg border border-border p-4">
          <h4 className="text-xs font-medium text-text-muted uppercase tracking-wider mb-2">Journal</h4>
          <p className="text-xs text-text leading-relaxed">{journal.summary}</p>
        </div>
      )}
    </div>
  )
}

function StatCard({ label, value, accent }: { label: string; value: string; accent?: string }) {
  const colorClass = accent === 'green' ? 'text-green' : accent === 'red' ? 'text-red' : accent === 'yellow' ? 'text-yellow' : accent === 'accent' ? 'text-accent' : 'text-text'
  return (
    <div className="bg-surface-2 rounded-lg border border-border p-3">
      <div className="text-[10px] text-text-muted uppercase tracking-wider">{label}</div>
      <div className={clsx('text-sm font-mono font-medium mt-1', colorClass)}>{value}</div>
    </div>
  )
}

function AnalyticsPanel({ braking, stability, racingLine, fatigue, timeDeltas }: {
  braking: BrakingAnalysis | null
  stability: StabilityAnalysis | null
  racingLine: RacingLineAnalysis | null
  fatigue: FatigueAnalysis | null
  timeDeltas: TimeDeltaEntry[]
}) {
  return (
    <div className="space-y-6">
      {braking && braking.events && braking.events.length > 0 && (
        <div>
          <h3 className="text-xs font-medium text-text-muted uppercase tracking-wider mb-3">Braking Analysis</h3>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-4">
            <StatCard label="Avg Decel" value={`${braking.avg_deceleration_g.toFixed(2)} G`} />
            <StatCard label="Trail Brake" value={`${braking.avg_trail_brake_pct.toFixed(0)}%`} accent="accent" />
            <StatCard label="Release Smooth" value={`${(braking.avg_release_smoothness * 100).toFixed(0)}%`} accent={braking.avg_release_smoothness > 0.7 ? 'green' : 'yellow'} />
            <StatCard label="Efficiency" value={`${(braking.avg_efficiency * 100).toFixed(0)}%`} accent={braking.avg_efficiency > 0.7 ? 'green' : 'yellow'} />
            <StatCard label="Consistency" value={`${(braking.consistency_score * 100).toFixed(0)}%`} accent={braking.consistency_score > 0.8 ? 'green' : 'red'} />
            <StatCard label="Total Brake Dist" value={`${braking.total_brake_distance_m.toFixed(0)} m`} />
          </div>
          <div className="grid gap-2 max-h-60 overflow-auto">
            {braking.events.map((e, i) => (
              <div key={i} className="flex items-center gap-3 bg-surface-2 rounded-lg px-3 py-2 border border-border">
                <span className="text-xs font-mono text-text-dim w-6">B{i + 1}</span>
                <div className="flex-1 grid grid-cols-4 gap-2 text-xs">
                  <div><span className="text-text-dim">Speed</span> <span className="font-mono text-text ml-1">{e.start_speed.toFixed(0)}&rarr;{e.end_speed.toFixed(0)}</span></div>
                  <div><span className="text-text-dim">Decel</span> <span className="font-mono text-text ml-1">{e.deceleration_g.toFixed(2)}G</span></div>
                  <div><span className="text-text-dim">Trail</span> <span className="font-mono text-text ml-1">{e.trail_brake_pct.toFixed(0)}%</span></div>
                  <div><span className="text-text-dim">Dist</span> <span className="font-mono text-text ml-1">{e.distance_m.toFixed(0)}m</span></div>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {stability && (stability.oversteer_count > 0 || stability.understeer_count > 0) && (
        <div>
          <h3 className="text-xs font-medium text-text-muted uppercase tracking-wider mb-3">Stability</h3>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-4">
            <StatCard label="Stability Score" value={`${(stability.stability_score * 100).toFixed(0)}%`} accent={stability.stability_score > 0.8 ? 'green' : 'red'} />
            <StatCard label="Oversteer" value={`${stability.oversteer_count}`} accent={stability.oversteer_count > 3 ? 'red' : 'yellow'} />
            <StatCard label="Understeer" value={`${stability.understeer_count}`} accent={stability.understeer_count > 3 ? 'red' : 'yellow'} />
            <StatCard label="Avg Yaw Dev" value={`${stability.avg_yaw_deviation.toFixed(3)}`} />
          </div>
          {stability.events.length > 0 && (
            <div className="grid gap-2 max-h-48 overflow-auto">
              {stability.events.map((e, i) => (
                <div key={i} className="flex items-center gap-3 bg-surface-2 rounded-lg px-3 py-2 border border-border">
                  <span className={clsx('text-xs px-1.5 py-0.5 rounded', e.event_type === 'oversteer' ? 'bg-red/10 text-red' : 'bg-orange/10 text-orange')}>
                    {e.event_type === 'oversteer' ? 'OS' : 'US'}
                  </span>
                  <div className="flex-1 grid grid-cols-3 gap-2 text-xs">
                    <div><span className="text-text-dim">Severity</span> <span className="font-mono text-text ml-1">{e.severity.toFixed(2)}</span></div>
                    <div><span className="text-text-dim">Speed</span> <span className="font-mono text-text ml-1">{e.speed.toFixed(0)} km/h</span></div>
                    <div><span className="text-text-dim">Yaw</span> <span className="font-mono text-text ml-1">{e.yaw_rate.toFixed(2)}</span></div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {racingLine && racingLine.consistency > 0 && (
        <div>
          <h3 className="text-xs font-medium text-text-muted uppercase tracking-wider mb-3">Racing Line</h3>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-4">
            <StatCard label="Consistency" value={`${(racingLine.consistency * 100).toFixed(0)}%`} accent={racingLine.consistency > 0.8 ? 'green' : 'yellow'} />
            <StatCard label="Smoothness" value={`${(racingLine.smoothness * 100).toFixed(0)}%`} accent={racingLine.smoothness > 0.7 ? 'green' : 'yellow'} />
            <StatCard label="Avg Deviation" value={`${racingLine.deviation_avg_m.toFixed(2)} m`} />
            <StatCard label="Max Deviation" value={`${racingLine.deviation_max_m.toFixed(2)} m`} />
          </div>
          {racingLine.worst_sections.length > 0 && (
            <div className="bg-surface-2 rounded-lg border border-border p-3">
              <h4 className="text-[10px] text-text-muted uppercase tracking-wider mb-2">Worst Sections</h4>
              <div className="space-y-1">
                {racingLine.worst_sections.map((s, i) => (
                  <div key={i} className="flex justify-between text-xs">
                    <span className="text-text-muted">{s.start_pct.toFixed(0)}% - {s.end_pct.toFixed(0)}%</span>
                    <span className="font-mono text-orange">{s.deviation_m.toFixed(2)} m</span>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}

      {fatigue && fatigue.diagnosis !== 'insufficient_data' && (
        <div>
          <h3 className="text-xs font-medium text-text-muted uppercase tracking-wider mb-3">Fatigue / Degradation</h3>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-4">
            <StatCard label="Diagnosis" value={fatigue.diagnosis.replace('_', ' ')} accent={fatigue.diagnosis === 'stable' ? 'green' : fatigue.diagnosis === 'driver_fatigue' ? 'red' : 'orange'} />
            <StatCard label="Driver Fatigue" value={`${(fatigue.driver_fatigue_score * 100).toFixed(0)}%`} accent={fatigue.driver_fatigue_score > 0.5 ? 'red' : 'green'} />
            <StatCard label="Tyre Deg" value={`${(fatigue.tyre_degradation_score * 100).toFixed(0)}%`} accent={fatigue.tyre_degradation_score > 0.5 ? 'red' : 'green'} />
            <StatCard label="Confidence" value={`${(fatigue.separation_confidence * 100).toFixed(0)}%`} />
          </div>
          <div className="bg-surface-2 rounded-lg border border-border p-3">
            <div className="space-y-1 text-xs">
              <div className="flex justify-between"><span className="text-text-muted">Lap Time Trend</span><span className="font-mono">{fatigue.lap_time_trend > 0 ? '+' : ''}{fatigue.lap_time_trend.toFixed(0)} ms/lap</span></div>
              <div className="flex justify-between"><span className="text-text-muted">Speed Loss</span><span className="font-mono">{fatigue.speed_loss_trend.toFixed(2)} km/h/lap</span></div>
              <div className="flex justify-between"><span className="text-text-muted">Consistency Trend</span><span className="font-mono">{fatigue.consistency_trend > 0 ? '+' : ''}{(fatigue.consistency_trend * 100).toFixed(1)}%</span></div>
              <div className="flex justify-between"><span className="text-text-muted">Brake Drift</span><span className="font-mono">{fatigue.brake_point_drift_trend.toFixed(2)} frames/lap</span></div>
            </div>
          </div>
        </div>
      )}

      {timeDeltas.length > 0 && (
        <div>
          <h3 className="text-xs font-medium text-text-muted uppercase tracking-wider mb-3">Time Deltas (vs Best Lap)</h3>
          <div className="grid gap-2">
            {timeDeltas.map((d, i) => (
              <div key={i} className="flex items-center gap-3 bg-surface-2 rounded-lg px-3 py-2 border border-border">
                <span className="text-xs font-mono text-text-dim w-12">Lap {d.lap_number}</span>
                <span className={clsx('text-xs font-mono font-medium', d.total_delta_s > 0 ? 'text-red' : 'text-green')}>
                  {d.total_delta_s > 0 ? '+' : ''}{d.total_delta_s.toFixed(3)}s
                </span>
                <div className="flex-1 grid grid-cols-2 gap-2 text-xs">
                  <div><span className="text-text-dim">Ahead</span> <span className="font-mono text-text ml-1">{d.ahead_pct.toFixed(0)}%</span></div>
                  <div><span className="text-text-dim">Max gain @</span> <span className="font-mono text-text ml-1">{d.max_gain_m.toFixed(0)}m</span></div>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {!braking && !stability && !racingLine && !fatigue && timeDeltas.length === 0 && (
        <div className="flex items-center justify-center h-40 text-text-dim text-sm">
          No analytics data available yet. Complete a session to generate analysis.
        </div>
      )}
    </div>
  )
}

function TrackLine({ frames, color }: { frames: TelemetryFrame[]; color: 'primary' | 'compare' | 'reference' }) {
  const { points, colors } = useMemo(() => {
    const speeds = frames.map((f) => f.speed)
    const minSpeed = Math.min(...speeds)
    const maxSpeed = Math.max(...speeds)
    const pts = frames.map((f) => new THREE.Vector3(f.x, f.y, f.z))
    const cols = frames.map((f) => {
      const t = maxSpeed > minSpeed ? (f.speed - minSpeed) / (maxSpeed - minSpeed) : 0.5
      if (color === 'compare') {
        return new THREE.Color().setHSL(0.08 + t * 0.05, 0.8, 0.5)
      }
      if (color === 'reference') {
        return new THREE.Color().setHSL(0.55, 0.4, 0.4 + t * 0.2)
      }
      return new THREE.Color().setHSL(t * 0.35, 1, 0.45)
    })
    return { points: pts, colors: cols }
  }, [frames, color])

  return (
    <Line
      points={points}
      vertexColors={colors.map((c) => [c.r, c.g, c.b] as [number, number, number])}
      lineWidth={color === 'primary' ? 3 : 2}
    />
  )
}

function formatLapTime(ms: number): string {
  if (ms <= 0) return '--:--.---'
  const minutes = Math.floor(ms / 60000)
  const seconds = Math.floor((ms % 60000) / 1000)
  const millis = ms % 1000
  return `${minutes}:${seconds.toString().padStart(2, '0')}.${millis.toString().padStart(3, '0')}`
}
