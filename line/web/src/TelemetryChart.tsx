import { useMemo, useState, useCallback, useRef } from 'react'
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, ReferenceArea } from 'recharts'
import type { TelemetryFrame } from './api'

interface TelemetryChartProps {
  frames?: TelemetryFrame[]
  compareFrames?: TelemetryFrame[]
}

function framesToData(frames: TelemetryFrame[]) {
  let dist = 0
  return frames.map((f, i) => {
    if (i > 0) {
      const prev = frames[i - 1]
      const dx = f.x - prev.x
      const dz = f.z - prev.z
      dist += Math.sqrt(dx * dx + dz * dz)
    }
    return {
      distance: Math.round(dist),
      speed: f.speed,
      throttle: f.throttle * 100,
      brake: f.brake * 100,
    }
  })
}

export function TelemetryChart({ frames, compareFrames }: TelemetryChartProps) {
  const [zoomLeft, setZoomLeft] = useState<number | null>(null)
  const [zoomRight, setZoomRight] = useState<number | null>(null)
  const [selecting, setSelecting] = useState(false)
  const [domain, setDomain] = useState<[number, number] | null>(null)
  const chartRef = useRef<HTMLDivElement>(null)

  const data = useMemo(() => {
    if (!frames || frames.length === 0) return []
    return framesToData(frames)
  }, [frames])

  const compareData = useMemo(() => {
    if (!compareFrames || compareFrames.length === 0) return null
    return framesToData(compareFrames)
  }, [compareFrames])

  const merged = useMemo(() => {
    if (!compareData) return data
    return data.map((d, i) => ({
      ...d,
      speed2: compareData[i]?.speed ?? null,
      throttle2: compareData[i]?.throttle ?? null,
      brake2: compareData[i]?.brake ?? null,
    }))
  }, [data, compareData])

  const visibleData = useMemo(() => {
    if (!domain) return merged
    return merged.filter(d => d.distance >= domain[0] && d.distance <= domain[1])
  }, [merged, domain])

  const handleMouseDown = useCallback((e: unknown) => {
    const ev = e as { activeLabel?: string } | undefined
    if (ev?.activeLabel) {
      setZoomLeft(Number(ev.activeLabel))
      setSelecting(true)
    }
  }, [])

  const handleMouseMove = useCallback((e: unknown) => {
    const ev = e as { activeLabel?: string } | undefined
    if (selecting && ev?.activeLabel) {
      setZoomRight(Number(ev.activeLabel))
    }
  }, [selecting])

  const handleMouseUp = useCallback(() => {
    if (zoomLeft !== null && zoomRight !== null && zoomLeft !== zoomRight) {
      const left = Math.min(zoomLeft, zoomRight)
      const right = Math.max(zoomLeft, zoomRight)
      setDomain([left, right])
    }
    setZoomLeft(null)
    setZoomRight(null)
    setSelecting(false)
  }, [zoomLeft, zoomRight])

  const resetZoom = useCallback(() => setDomain(null), [])

  if (merged.length === 0) {
    return <div className="text-xs text-text-dim text-center py-8">Select a lap to view telemetry</div>
  }

  return (
    <div className="space-y-4" ref={chartRef} role="img" aria-label="Telemetry charts showing speed, throttle and brake over distance">
      {domain && (
        <button
          onClick={resetZoom}
          className="text-[10px] text-accent hover:text-text transition-colors focus:outline-none focus:ring-2 focus:ring-accent/50 rounded px-2 py-0.5"
          aria-label="Reset chart zoom"
        >
          Reset zoom
        </button>
      )}
      <div>
        <h3 className="text-xs text-text-muted mb-1">Speed (km/h)</h3>
        <ResponsiveContainer width="100%" height={100}>
          <LineChart
            data={visibleData}
            onMouseDown={handleMouseDown}
            onMouseMove={handleMouseMove}
            onMouseUp={handleMouseUp}
          >
            <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" />
            <XAxis dataKey="distance" stroke="var(--color-text-dim)" tick={{ fontSize: 9 }} />
            <YAxis stroke="var(--color-text-dim)" tick={{ fontSize: 9 }} />
            <Tooltip
              contentStyle={{ background: 'var(--color-surface-2)', border: '1px solid var(--color-border-2)', fontSize: 11, borderRadius: 6 }}
              labelFormatter={(v) => `${v}m`}
            />
            <Line type="monotone" dataKey="speed" stroke="var(--color-accent)" dot={false} strokeWidth={1.5} animationDuration={300} />
            {compareData && <Line type="monotone" dataKey="speed2" stroke="var(--color-orange)" dot={false} strokeWidth={1} strokeDasharray="4 2" animationDuration={300} />}
            {selecting && zoomLeft !== null && zoomRight !== null && (
              <ReferenceArea x1={zoomLeft} x2={zoomRight} strokeOpacity={0.3} fill="var(--color-accent)" fillOpacity={0.1} />
            )}
          </LineChart>
        </ResponsiveContainer>
      </div>

      <div>
        <h3 className="text-xs text-text-muted mb-1">Throttle / Brake (%)</h3>
        <ResponsiveContainer width="100%" height={80}>
          <LineChart data={visibleData}>
            <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" />
            <XAxis dataKey="distance" stroke="var(--color-text-dim)" tick={{ fontSize: 9 }} />
            <YAxis stroke="var(--color-text-dim)" tick={{ fontSize: 9 }} domain={[0, 100]} />
            <Tooltip
              contentStyle={{ background: 'var(--color-surface-2)', border: '1px solid var(--color-border-2)', fontSize: 11, borderRadius: 6 }}
              labelFormatter={(v) => `${v}m`}
            />
            <Line type="monotone" dataKey="throttle" stroke="var(--color-green)" dot={false} strokeWidth={1.5} animationDuration={300} />
            <Line type="monotone" dataKey="brake" stroke="var(--color-red)" dot={false} strokeWidth={1.5} animationDuration={300} />
            {compareData && <Line type="monotone" dataKey="throttle2" stroke="var(--color-green)" dot={false} strokeWidth={1} strokeDasharray="4 2" opacity={0.5} animationDuration={300} />}
            {compareData && <Line type="monotone" dataKey="brake2" stroke="var(--color-red)" dot={false} strokeWidth={1} strokeDasharray="4 2" opacity={0.5} animationDuration={300} />}
          </LineChart>
        </ResponsiveContainer>
      </div>

      <p className="text-[10px] text-text-dim" aria-hidden="true">Drag to zoom. {compareData ? 'Dashed lines = comparison lap.' : ''}</p>
    </div>
  )
}
