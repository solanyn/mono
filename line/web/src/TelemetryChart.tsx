import { useMemo } from 'react'
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts'
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

  if (merged.length === 0) {
    return <div className="text-xs text-text-dim text-center py-8">Select a lap to view telemetry</div>
  }

  return (
    <div className="space-y-4">
      <div>
        <h3 className="text-xs text-text-muted mb-1">Speed (km/h)</h3>
        <ResponsiveContainer width="100%" height={100}>
          <LineChart data={merged}>
            <CartesianGrid strokeDasharray="3 3" stroke="#2a2a2a" />
            <XAxis dataKey="distance" stroke="#555" tick={{ fontSize: 9 }} />
            <YAxis stroke="#555" tick={{ fontSize: 9 }} />
            <Tooltip contentStyle={{ background: '#1a1a1a', border: '1px solid #333', fontSize: 11 }} />
            <Line type="monotone" dataKey="speed" stroke="#4fc3f7" dot={false} strokeWidth={1.5} />
            {compareData && <Line type="monotone" dataKey="speed2" stroke="#ff7043" dot={false} strokeWidth={1} strokeDasharray="4 2" />}
          </LineChart>
        </ResponsiveContainer>
      </div>

      <div>
        <h3 className="text-xs text-text-muted mb-1">Throttle / Brake (%)</h3>
        <ResponsiveContainer width="100%" height={80}>
          <LineChart data={merged}>
            <CartesianGrid strokeDasharray="3 3" stroke="#2a2a2a" />
            <XAxis dataKey="distance" stroke="#555" tick={{ fontSize: 9 }} />
            <YAxis stroke="#555" tick={{ fontSize: 9 }} domain={[0, 100]} />
            <Tooltip contentStyle={{ background: '#1a1a1a', border: '1px solid #333', fontSize: 11 }} />
            <Line type="monotone" dataKey="throttle" stroke="#66bb6a" dot={false} strokeWidth={1.5} />
            <Line type="monotone" dataKey="brake" stroke="#ef5350" dot={false} strokeWidth={1.5} />
            {compareData && <Line type="monotone" dataKey="throttle2" stroke="#66bb6a" dot={false} strokeWidth={1} strokeDasharray="4 2" opacity={0.5} />}
            {compareData && <Line type="monotone" dataKey="brake2" stroke="#ef5350" dot={false} strokeWidth={1} strokeDasharray="4 2" opacity={0.5} />}
          </LineChart>
        </ResponsiveContainer>
      </div>
    </div>
  )
}
