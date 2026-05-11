import { useEffect, useState, useMemo } from 'react'
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, ScatterChart, Scatter, Cell } from 'recharts'
import clsx from 'clsx'
import { fetchProgression, fetchTracks, fetchCars, getCarName, type ProgressionPoint, type TrackInfo, type Car } from '../api'

export function ProgressionPage() {
  const [points, setPoints] = useState<ProgressionPoint[]>([])
  const [tracks, setTracks] = useState<TrackInfo[]>([])
  const [cars, setCars] = useState<Car[]>([])
  const [selectedTrack, setSelectedTrack] = useState<string>('')
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetchTracks().then(({ tracks }) => setTracks(tracks ?? [])).catch(() => {})
    fetchCars().then(setCars).catch(() => {})
  }, [])

  useEffect(() => {
    setLoading(true)
    fetchProgression(selectedTrack || undefined)
      .then(({ points }) => {
        setPoints(points ?? [])
        setLoading(false)
      })
      .catch(() => setLoading(false))
  }, [selectedTrack])

  const chartData = useMemo(() =>
    points.map((p) => ({
      date: new Date(p.date).toLocaleDateString('en-AU', { day: 'numeric', month: 'short' }),
      dateRaw: new Date(p.date).getTime(),
      bestLap: p.best_lap_ms / 1000,
      lapCount: p.lap_count,
      consistency: p.consistency_score != null ? Math.round(p.consistency_score * 100) : null,
      trackName: p.track_name || p.track_id || 'Unknown',
      carCode: p.car_code,
      carName: getCarName(cars, p.car_code),
      sessionId: p.session_id,
    })),
  [points])

  const bestEver = useMemo(() => {
    if (chartData.length === 0) return null
    return chartData.reduce((best, p) => p.bestLap < best.bestLap ? p : best)
  }, [chartData])

  const recentTrend = useMemo(() => {
    if (chartData.length < 3) return null
    const recent = chartData.slice(-5)
    const first = recent[0].bestLap
    const last = recent[recent.length - 1].bestLap
    const delta = last - first
    return { delta, improving: delta < 0 }
  }, [chartData])

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="w-5 h-5 border-2 border-accent border-t-transparent rounded-full animate-spin" />
      </div>
    )
  }

  return (
    <div className="h-full overflow-auto p-4 sm:p-6 space-y-6">
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
        <h2 className="text-base font-semibold">Progression</h2>
        <select
          value={selectedTrack}
          onChange={(e) => setSelectedTrack(e.target.value)}
          className="bg-surface-2 border border-border rounded px-3 py-1.5 text-xs text-text focus:outline-none focus:border-accent"
        >
          <option value="">All Tracks</option>
          {tracks.map((t) => (
            <option key={t.track_id} value={t.track_id}>{t.name}</option>
          ))}
          {points.length > 0 && !tracks.length && (
            [...new Set(points.map(p => p.track_id).filter(Boolean))].map((tid) => (
              <option key={tid} value={tid!}>{points.find(p => p.track_id === tid)?.track_name || tid}</option>
            ))
          )}
        </select>
      </div>

      {points.length === 0 && (
        <div className="flex items-center justify-center h-48 text-text-dim text-sm">
          No session data yet. Complete some races to see your progression.
        </div>
      )}

      {points.length > 0 && (
        <>
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
            <StatCard label="Sessions" value={`${points.length}`} />
            <StatCard
              label="Best Ever"
              value={bestEver ? `${bestEver.bestLap.toFixed(3)}s` : '--'}
              accent="accent"
            />
            <StatCard
              label="Total Laps"
              value={`${points.reduce((sum, p) => sum + p.lap_count, 0)}`}
            />
            <StatCard
              label="Trend (last 5)"
              value={recentTrend ? `${recentTrend.delta > 0 ? '+' : ''}${recentTrend.delta.toFixed(3)}s` : '--'}
              accent={recentTrend?.improving ? 'green' : recentTrend ? 'red' : undefined}
            />
          </div>

          <div className="bg-surface-2 rounded-lg border border-border p-4">
            <h3 className="text-xs font-medium text-text-muted uppercase tracking-wider mb-3">Best Lap Time</h3>
            <ResponsiveContainer width="100%" height={200}>
              <LineChart data={chartData}>
                <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" />
                <XAxis dataKey="date" stroke="var(--color-text-dim)" tick={{ fontSize: 10 }} />
                <YAxis
                  stroke="var(--color-text-dim)"
                  tick={{ fontSize: 10 }}
                  domain={['auto', 'auto']}
                  tickFormatter={(v: number) => `${v.toFixed(1)}s`}
                  reversed
                />
                <Tooltip
                  contentStyle={{ background: 'var(--color-surface)', border: '1px solid var(--color-border)', borderRadius: '6px', fontSize: '11px' }}
                  formatter={(value: unknown) => [`${Number(value).toFixed(3)}s`, 'Best Lap']}
                  labelFormatter={(label: string) => label}
                />
                <Line type="monotone" dataKey="bestLap" stroke="var(--color-accent)" dot={{ r: 3, fill: 'var(--color-accent)' }} strokeWidth={2} />
              </LineChart>
            </ResponsiveContainer>
          </div>

          <div className="bg-surface-2 rounded-lg border border-border p-4">
            <h3 className="text-xs font-medium text-text-muted uppercase tracking-wider mb-3">Consistency Score</h3>
            <ResponsiveContainer width="100%" height={160}>
              <LineChart data={chartData.filter(d => d.consistency !== null)}>
                <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" />
                <XAxis dataKey="date" stroke="var(--color-text-dim)" tick={{ fontSize: 10 }} />
                <YAxis stroke="var(--color-text-dim)" tick={{ fontSize: 10 }} domain={[0, 100]} tickFormatter={(v: number) => `${v}%`} />
                <Tooltip
                  contentStyle={{ background: 'var(--color-surface)', border: '1px solid var(--color-border)', borderRadius: '6px', fontSize: '11px' }}
                  formatter={(value: unknown) => [`${value}%`, 'Consistency']}
                />
                <Line type="monotone" dataKey="consistency" stroke="var(--color-green)" dot={{ r: 3, fill: 'var(--color-green)' }} strokeWidth={2} />
              </LineChart>
            </ResponsiveContainer>
          </div>

          <div className="bg-surface-2 rounded-lg border border-border p-4">
            <h3 className="text-xs font-medium text-text-muted uppercase tracking-wider mb-3">Laps Per Session</h3>
            <ResponsiveContainer width="100%" height={120}>
              <ScatterChart>
                <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" />
                <XAxis dataKey="date" stroke="var(--color-text-dim)" tick={{ fontSize: 10 }} type="category" allowDuplicatedCategory={false} data={chartData} />
                <YAxis dataKey="lapCount" stroke="var(--color-text-dim)" tick={{ fontSize: 10 }} />
                <Tooltip
                  contentStyle={{ background: 'var(--color-surface)', border: '1px solid var(--color-border)', borderRadius: '6px', fontSize: '11px' }}
                  formatter={(value: unknown) => [`${value}`, 'Laps']}
                />
                <Scatter data={chartData}>
                  {chartData.map((_, i) => (
                    <Cell key={i} fill="var(--color-accent)" opacity={0.7} />
                  ))}
                </Scatter>
              </ScatterChart>
            </ResponsiveContainer>
          </div>

          <div className="bg-surface-2 rounded-lg border border-border p-4">
            <h3 className="text-xs font-medium text-text-muted uppercase tracking-wider mb-3">Session History</h3>
            <div className="space-y-1.5 max-h-64 overflow-auto">
              {[...chartData].reverse().map((d, i) => (
                <div key={i} className="flex items-center gap-3 text-xs px-2 py-1.5 rounded hover:bg-surface transition-colors">
                  <span className="text-text-dim w-16">{d.date}</span>
                  <span className="flex-1 truncate">{d.trackName}</span>
                  <span className="text-text-muted truncate max-w-32">{d.carName}</span>
                  <span className="font-mono text-accent">{d.bestLap.toFixed(3)}s</span>
                  <span className="text-text-dim w-12 text-right">{d.lapCount} laps</span>
                  {d.consistency !== null && (
                    <span className={clsx('w-10 text-right font-mono', d.consistency >= 90 ? 'text-green' : d.consistency >= 70 ? 'text-yellow' : 'text-red')}>
                      {d.consistency}%
                    </span>
                  )}
                </div>
              ))}
            </div>
          </div>
        </>
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
