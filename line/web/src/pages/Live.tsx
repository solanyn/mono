import { useEffect, useRef, useState, useMemo, useCallback } from 'react'
import { Canvas } from '@react-three/fiber'
import { OrbitControls, Line } from '@react-three/drei'
import * as THREE from 'three'
import { connectLive, connectCoach, fetchCars, getCarName, type TelemetryFrame, type LiveStatus, type CoachMessage, type Car } from '../api'
import clsx from 'clsx'

const MAX_TRAIL = 600

export function LivePage() {
  const [status, setStatus] = useState<LiveStatus>({ active: false })
  const [frames, setFrames] = useState<TelemetryFrame[]>([])
  const [latest, setLatest] = useState<TelemetryFrame | null>(null)
  const [coachMessages, setCoachMessages] = useState<CoachMessage[]>([])
  const [cars, setCars] = useState<Car[]>([])
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
    fetchCars().then(setCars).catch(() => {})
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
    <div className="flex flex-col-reverse md:flex-row h-full">
      <div className="flex-1 min-h-0 bg-surface">
        <Canvas camera={{ position: [0, 200, 200], fov: 60 }}>
          <ambientLight intensity={0.4} />
          <directionalLight position={[100, 200, 100]} intensity={0.8} />
          {frames.length > 1 && <LiveTrail frames={frames} />}
          <gridHelper args={[800, 40, '#333333', '#222222']} />
          <OrbitControls enableDamping dampingFactor={0.1} maxPolarAngle={Math.PI / 2.1} />
        </Canvas>
      </div>
      <aside className="w-full md:w-72 border-b md:border-b-0 md:border-l border-border p-4 overflow-auto flex flex-col gap-5 max-h-64 md:max-h-none">
        <StatusBadge active={status.active} />

        {status.active && (
          <div className="text-xs text-text-muted space-y-1">
            <div>Car: <span className="text-text font-mono">{getCarName(cars, status.car_code ?? 0)}</span></div>
            <div>Lap: <span className="text-text font-mono">{status.current_lap}</span></div>
            {status.track_id && <div>Track: <span className="text-text">{status.track_id}</span></div>}
          </div>
        )}

        {latest && (
          <div>
            <h3 className="text-xs font-medium text-text-muted mb-3 uppercase tracking-wider">Telemetry</h3>
            <div className="space-y-2">
              <Gauge label="Speed" value={latest.speed} unit="km/h" max={350} color="bg-accent" />
              <Gauge label="RPM" value={latest.rpm} unit="" max={9000} color="bg-orange" />
              <Gauge label="Throttle" value={latest.throttle * 100} unit="%" max={100} color="bg-green" />
              <Gauge label="Brake" value={latest.brake * 100} unit="%" max={100} color="bg-red" />
            </div>
            <div className="mt-3 text-center">
              <span className="text-3xl font-mono font-bold text-text">{latest.gear}</span>
              <span className="text-xs text-text-muted ml-1">gear</span>
            </div>
          </div>
        )}

        <div className="border-t border-border pt-4">
          <div className="flex justify-between items-center mb-3">
            <h3 className="text-xs font-medium text-text-muted uppercase tracking-wider">Coach</h3>
            <button
              onClick={() => setCoachEnabled((v) => !v)}
              className={clsx(
                'px-2 py-0.5 rounded text-xs font-medium transition-colors',
                coachEnabled ? 'bg-green text-white' : 'bg-border-2 text-text-muted',
              )}
            >
              {coachEnabled ? 'ON' : 'OFF'}
            </button>
          </div>
          <div className="space-y-2 max-h-48 overflow-auto">
            {coachMessages.length === 0 && (
              <p className="text-xs text-text-dim">Waiting for coaching events...</p>
            )}
            {coachMessages.map((msg, i) => (
              <div key={i} className="text-xs text-text-muted border-l-2 border-accent pl-2">
                <div className="text-text">{msg.text}</div>
                <div className="text-text-dim mt-0.5">{msg.latency_ms}ms</div>
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
    <div className="flex items-center gap-2">
      <div className={clsx(
        'w-2.5 h-2.5 rounded-full',
        active ? 'bg-green shadow-[0_0_8px_var(--color-green)]' : 'bg-text-dim',
      )} />
      <span className={clsx('text-sm', active ? 'text-green' : 'text-text-dim')}>
        {active ? 'Live' : 'Waiting for session'}
      </span>
    </div>
  )
}

function Gauge({ label, value, unit, max, color }: { label: string; value: number; unit: string; max: number; color: string }) {
  const pct = Math.min(100, (value / max) * 100)
  return (
    <div>
      <div className="flex justify-between text-xs mb-0.5">
        <span className="text-text-muted">{label}</span>
        <span className="text-text font-mono">{Math.round(value)}{unit && ` ${unit}`}</span>
      </div>
      <div className="h-1.5 bg-border rounded-full overflow-hidden">
        <div className={clsx('h-full rounded-full transition-all duration-75', color)} style={{ width: `${pct}%` }} />
      </div>
    </div>
  )
}
