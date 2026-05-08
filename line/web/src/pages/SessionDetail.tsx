import { useEffect, useState, useMemo } from 'react'
import { useParams } from 'react-router'
import { Canvas } from '@react-three/fiber'
import { OrbitControls, Line } from '@react-three/drei'
import * as THREE from 'three'
import { fetchLaps, fetchTelemetry, type Lap, type TelemetryFrame } from '../api'
import { TelemetryChart } from '../TelemetryChart'

export function SessionDetail() {
  const { id } = useParams<{ id: string }>()
  const [laps, setLaps] = useState<Lap[]>([])
  const [selectedLap, setSelectedLap] = useState<number | null>(null)
  const [frames, setFrames] = useState<TelemetryFrame[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!id) return
    fetchLaps(id).then(({ laps }) => {
      setLaps(laps)
      setLoading(false)
      if (laps.length > 0) setSelectedLap(laps[0].lap_number)
    })
  }, [id])

  useEffect(() => {
    if (!id || selectedLap === null) return
    fetchTelemetry(id, selectedLap, 2).then(({ frames }) => {
      setFrames(frames)
    })
  }, [id, selectedLap])

  if (loading) return <div style={{ padding: '2rem' }}>Loading...</div>

  return (
    <div style={{ display: 'flex', height: '100%' }}>
      <aside style={{ width: '200px', borderRight: '1px solid #222', overflow: 'auto', padding: '1rem' }}>
        <h3 style={{ margin: '0 0 0.75rem', fontSize: '0.9rem', color: '#888' }}>Laps</h3>
        {laps.length === 0 && <p style={{ color: '#666', fontSize: '0.85rem' }}>No laps yet</p>}
        {laps.map((lap) => (
          <button
            key={lap.lap_number}
            onClick={() => setSelectedLap(lap.lap_number)}
            style={{
              display: 'block',
              width: '100%',
              padding: '0.5rem',
              marginBottom: '0.25rem',
              background: selectedLap === lap.lap_number ? '#1a3a4a' : '#1a1a1a',
              border: selectedLap === lap.lap_number ? '1px solid #4fc3f7' : '1px solid #2a2a2a',
              borderRadius: '4px',
              color: '#e0e0e0',
              cursor: 'pointer',
              textAlign: 'left',
              fontSize: '0.85rem',
            }}
          >
            <span>Lap {lap.lap_number}</span>
            <span style={{ float: 'right', fontFamily: 'monospace', color: '#4fc3f7' }}>
              {formatLapTime(lap.time_ms)}
            </span>
          </button>
        ))}
      </aside>
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column' }}>
        <div style={{ flex: 1, minHeight: 0 }}>
          <Canvas camera={{ position: [0, 200, 200], fov: 60 }}>
            <ambientLight intensity={0.4} />
            <directionalLight position={[100, 200, 100]} intensity={0.8} />
            {frames.length > 0 && <TrackLine frames={frames} />}
            <gridHelper args={[800, 40, '#333333', '#222222']} />
            <OrbitControls enableDamping dampingFactor={0.1} maxPolarAngle={Math.PI / 2.1} />
          </Canvas>
        </div>
        <div style={{ height: '35%', borderTop: '1px solid #222', overflow: 'auto', padding: '1rem' }}>
          <TelemetryChart />
        </div>
      </div>
    </div>
  )
}

function TrackLine({ frames }: { frames: TelemetryFrame[] }) {
  const { points, colors } = useMemo(() => {
    const speeds = frames.map((f) => f.speed)
    const minSpeed = Math.min(...speeds)
    const maxSpeed = Math.max(...speeds)
    const pts = frames.map((f) => new THREE.Vector3(f.x, f.y, f.z))
    const cols = frames.map((f) => {
      const t = maxSpeed > minSpeed ? (f.speed - minSpeed) / (maxSpeed - minSpeed) : 0.5
      return new THREE.Color().setHSL(t * 0.35, 1, 0.45)
    })
    return { points: pts, colors: cols }
  }, [frames])

  return (
    <Line
      points={points}
      vertexColors={colors.map((c) => [c.r, c.g, c.b] as [number, number, number])}
      lineWidth={3}
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
