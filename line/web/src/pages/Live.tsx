import { useEffect, useRef, useState, useMemo, useCallback } from 'react'
import { Canvas } from '@react-three/fiber'
import { OrbitControls, Line } from '@react-three/drei'
import * as THREE from 'three'
import { connectLive, connectCoach, type TelemetryFrame, type LiveStatus, type CoachMessage } from '../api'

const MAX_TRAIL = 600

export function LivePage() {
  const [status, setStatus] = useState<LiveStatus>({ active: false })
  const [frames, setFrames] = useState<TelemetryFrame[]>([])
  const [latest, setLatest] = useState<TelemetryFrame | null>(null)
  const [coachMessages, setCoachMessages] = useState<CoachMessage[]>([])
  const [coachEnabled, setCoachEnabled] = useState(true)
  const wsRef = useRef<WebSocket | null>(null)
  const coachRef = useRef<WebSocket | null>(null)
  const audioCtxRef = useRef<AudioContext | null>(null)

  const playAudio = useCallback((buffer: ArrayBuffer) => {
    if (!audioCtxRef.current) {
      audioCtxRef.current = new AudioContext()
    }
    const ctx = audioCtxRef.current
    ctx.decodeAudioData(buffer.slice(0)).then((decoded) => {
      const source = ctx.createBufferSource()
      source.buffer = decoded
      source.connect(ctx.destination)
      source.start()
    })
  }, [])

  useEffect(() => {
    const ws = connectLive(
      (frame) => {
        setLatest(frame)
        setFrames((prev) => {
          const next = [...prev, frame]
          return next.length > MAX_TRAIL ? next.slice(-MAX_TRAIL) : next
        })
      },
      (s) => setStatus(s),
    )
    wsRef.current = ws
    return () => ws.close()
  }, [])

  useEffect(() => {
    if (!coachEnabled) {
      coachRef.current?.close()
      coachRef.current = null
      return
    }
    const ws = connectCoach(
      (msg) => setCoachMessages((prev) => [...prev.slice(-9), msg]),
      (audio) => playAudio(audio),
    )
    coachRef.current = ws
    return () => ws.close()
  }, [coachEnabled, playAudio])

  return (
    <div style={{ display: 'flex', height: '100%' }}>
      <div style={{ flex: 1, minHeight: 0 }}>
        <Canvas camera={{ position: [0, 200, 200], fov: 60 }}>
          <ambientLight intensity={0.4} />
          <directionalLight position={[100, 200, 100]} intensity={0.8} />
          {frames.length > 1 && <LiveTrail frames={frames} />}
          <gridHelper args={[800, 40, '#333333', '#222222']} />
          <OrbitControls enableDamping dampingFactor={0.1} maxPolarAngle={Math.PI / 2.1} />
        </Canvas>
      </div>
      <aside style={{ width: '280px', borderLeft: '1px solid #222', padding: '1rem', overflow: 'auto' }}>
        <StatusBadge active={status.active} />
        {status.active && (
          <div style={{ marginTop: '1rem', fontSize: '0.85rem', color: '#aaa' }}>
            <div>Car: {status.car_code}</div>
            <div>Lap: {status.current_lap}</div>
            {status.track_id && <div>Track: {status.track_id}</div>}
          </div>
        )}
        {latest && (
          <div style={{ marginTop: '1.5rem' }}>
            <h3 style={{ margin: '0 0 0.75rem', fontSize: '0.9rem', color: '#888' }}>Telemetry</h3>
            <Gauge label="Speed" value={latest.speed} unit="km/h" max={350} color="#4fc3f7" />
            <Gauge label="RPM" value={latest.rpm} unit="" max={9000} color="#ff7043" />
            <Gauge label="Throttle" value={latest.throttle * 100} unit="%" max={100} color="#66bb6a" />
            <Gauge label="Brake" value={latest.brake * 100} unit="%" max={100} color="#ef5350" />
            <div style={{ marginTop: '0.75rem', fontSize: '0.85rem', color: '#ccc' }}>
              Gear: {latest.gear}
            </div>
          </div>
        )}
        <div style={{ marginTop: '1.5rem', borderTop: '1px solid #333', paddingTop: '1rem' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <h3 style={{ margin: 0, fontSize: '0.9rem', color: '#888' }}>Coach</h3>
            <button
              onClick={() => setCoachEnabled((v) => !v)}
              style={{
                background: coachEnabled ? '#66bb6a' : '#444',
                border: 'none',
                borderRadius: 4,
                padding: '2px 8px',
                fontSize: '0.75rem',
                color: '#fff',
                cursor: 'pointer',
              }}
            >
              {coachEnabled ? 'ON' : 'OFF'}
            </button>
          </div>
          <div style={{ marginTop: '0.75rem', maxHeight: '200px', overflow: 'auto' }}>
            {coachMessages.map((msg, i) => (
              <div key={i} style={{ fontSize: '0.8rem', color: '#bbb', marginBottom: '0.5rem', borderLeft: '2px solid #4fc3f7', paddingLeft: '0.5rem' }}>
                <div>{msg.text}</div>
                <div style={{ fontSize: '0.7rem', color: '#666' }}>{msg.latency_ms}ms</div>
              </div>
            ))}
          </div>
        </div>
      </aside>
    </div>
  )
}

function LiveTrail({ frames }: { frames: TelemetryFrame[] }) {
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

function StatusBadge({ active }: { active: boolean }) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
      <div
        style={{
          width: 10,
          height: 10,
          borderRadius: '50%',
          background: active ? '#66bb6a' : '#666',
          boxShadow: active ? '0 0 8px #66bb6a' : 'none',
        }}
      />
      <span style={{ fontSize: '0.9rem', color: active ? '#66bb6a' : '#666' }}>
        {active ? 'Live' : 'Waiting for session'}
      </span>
    </div>
  )
}

function Gauge({ label, value, unit, max, color }: { label: string; value: number; unit: string; max: number; color: string }) {
  const pct = Math.min(100, (value / max) * 100)
  return (
    <div style={{ marginBottom: '0.5rem' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.8rem', color: '#888' }}>
        <span>{label}</span>
        <span style={{ color: '#ccc', fontFamily: 'monospace' }}>{Math.round(value)} {unit}</span>
      </div>
      <div style={{ height: 4, background: '#2a2a2a', borderRadius: 2, marginTop: 2 }}>
        <div style={{ height: '100%', width: `${pct}%`, background: color, borderRadius: 2 }} />
      </div>
    </div>
  )
}
