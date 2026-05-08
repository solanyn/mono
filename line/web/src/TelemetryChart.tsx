import { useMemo } from 'react'
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts'
import type { TelemetryFrame } from './api'

interface TelemetryChartProps {
  frames?: TelemetryFrame[]
}

export function TelemetryChart({ frames }: TelemetryChartProps) {
  const data = useMemo(() => {
    if (!frames || frames.length === 0) {
      return Array.from({ length: 200 }, (_, i) => ({
        distance: i * 10,
        speed: 80 + 40 * Math.sin(i * 0.05) + Math.random() * 5,
        throttle: Math.max(0, Math.min(100, 50 + 50 * Math.cos(i * 0.05))),
        brake: Math.max(0, Math.min(100, -50 * Math.cos(i * 0.05))),
      }))
    }
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
  }, [frames])

  return (
    <div>
      <h3 style={{ margin: '0 0 0.5rem', fontSize: '0.9rem', color: '#888' }}>Speed (km/h)</h3>
      <ResponsiveContainer width="100%" height={120}>
        <LineChart data={data}>
          <CartesianGrid strokeDasharray="3 3" stroke="#2a2a2a" />
          <XAxis dataKey="distance" stroke="#555" tick={{ fontSize: 10 }} />
          <YAxis stroke="#555" tick={{ fontSize: 10 }} />
          <Tooltip contentStyle={{ background: '#1a1a1a', border: '1px solid #333' }} />
          <Line type="monotone" dataKey="speed" stroke="#4fc3f7" dot={false} strokeWidth={1.5} />
        </LineChart>
      </ResponsiveContainer>

      <h3 style={{ margin: '1rem 0 0.5rem', fontSize: '0.9rem', color: '#888' }}>Throttle / Brake (%)</h3>
      <ResponsiveContainer width="100%" height={120}>
        <LineChart data={data}>
          <CartesianGrid strokeDasharray="3 3" stroke="#2a2a2a" />
          <XAxis dataKey="distance" stroke="#555" tick={{ fontSize: 10 }} />
          <YAxis stroke="#555" tick={{ fontSize: 10 }} domain={[0, 100]} />
          <Tooltip contentStyle={{ background: '#1a1a1a', border: '1px solid #333' }} />
          <Line type="monotone" dataKey="throttle" stroke="#66bb6a" dot={false} strokeWidth={1.5} />
          <Line type="monotone" dataKey="brake" stroke="#ef5350" dot={false} strokeWidth={1.5} />
        </LineChart>
      </ResponsiveContainer>
    </div>
  )
}
