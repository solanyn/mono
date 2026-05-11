import { useEffect, useState, useMemo, useRef, useCallback } from 'react'
import { useParams, useSearchParams } from 'react-router'
import { Canvas, useFrame } from '@react-three/fiber'
import { OrbitControls, Line } from '@react-three/drei'
import * as THREE from 'three'
import clsx from 'clsx'
import { fetchLaps, fetchTelemetry, fetchAnnotations, createAnnotation, deleteAnnotation, type Lap, type TelemetryFrame, type Annotation } from '../api'

export function ReplayPage() {
  const { id } = useParams<{ id: string }>()
  const [searchParams] = useSearchParams()
  const lapParam = searchParams.get('lap')
  const [laps, setLaps] = useState<Lap[]>([])
  const [selectedLap, setSelectedLap] = useState<number | null>(null)
  const [frames, setFrames] = useState<TelemetryFrame[]>([])
  const [loading, setLoading] = useState(true)
  const [playing, setPlaying] = useState(false)
  const [frameIdx, setFrameIdx] = useState(0)
  const [speed, setSpeed] = useState(1)
  const [annotations, setAnnotations] = useState<Annotation[]>([])
  const [annotating, setAnnotating] = useState(false)
  const [annotationText, setAnnotationText] = useState('')
  const animRef = useRef<number | null>(null)
  const lastTimeRef = useRef<number>(0)

  useEffect(() => {
    if (!id) return
    fetchLaps(id).then(({ laps }) => {
      setLaps(laps ?? [])
      setLoading(false)
      const initial = lapParam ? parseInt(lapParam) : laps?.[0]?.lap_number
      if (initial) setSelectedLap(initial)
    }).catch(() => setLoading(false))
  }, [id, lapParam])

  useEffect(() => {
    if (!id || selectedLap === null) return
    setFrames([])
    setFrameIdx(0)
    setPlaying(false)
    setAnnotations([])
    fetchTelemetry(id, selectedLap, 1).then(({ frames }) => setFrames(frames ?? []))
    fetchAnnotations(id, selectedLap).then(({ annotations }) => setAnnotations(annotations ?? [])).catch(() => {})
  }, [id, selectedLap])

  const animate = useCallback((time: number) => {
    if (!lastTimeRef.current) lastTimeRef.current = time
    const delta = time - lastTimeRef.current
    lastTimeRef.current = time
    const advance = (delta / 1000) * 60 * speed
    setFrameIdx((prev) => {
      const next = prev + advance
      if (next >= frames.length - 1) {
        setPlaying(false)
        return frames.length - 1
      }
      return next
    })
    animRef.current = requestAnimationFrame(animate)
  }, [frames.length, speed])

  useEffect(() => {
    if (playing) {
      lastTimeRef.current = 0
      animRef.current = requestAnimationFrame(animate)
    } else if (animRef.current) {
      cancelAnimationFrame(animRef.current)
    }
    return () => { if (animRef.current) cancelAnimationFrame(animRef.current) }
  }, [playing, animate])

  const handleAddAnnotation = async () => {
    if (!id || selectedLap === null || !annotationText.trim()) return
    const a = await createAnnotation(id, selectedLap, Math.floor(frameIdx), annotationText.trim())
    setAnnotations((prev) => [...prev, a].sort((x, y) => x.frame_idx - y.frame_idx))
    setAnnotationText('')
    setAnnotating(false)
  }

  const handleDeleteAnnotation = async (annotationId: number) => {
    await deleteAnnotation(annotationId)
    setAnnotations((prev) => prev.filter((a) => a.id !== annotationId))
  }

  const currentIdx = Math.floor(frameIdx)
  const currentFrame = frames[currentIdx]
  const nearbyAnnotation = annotations.find((a) => Math.abs(a.frame_idx - currentIdx) < 30)

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="w-5 h-5 border-2 border-accent border-t-transparent rounded-full animate-spin" />
      </div>
    )
  }

  return (
    <div className="flex flex-col h-full">
      <div className="flex-1 flex flex-col md:flex-row min-h-0">
        <div className="flex-1 min-h-0 bg-surface relative">
          <Canvas camera={{ position: [0, 250, 300], fov: 50 }}>
            <ambientLight intensity={0.4} />
            <directionalLight position={[100, 200, 100]} intensity={0.8} />
            {frames.length > 0 && <ReplayTrack frames={frames} currentIdx={currentIdx} annotations={annotations} />}
            <gridHelper args={[800, 40, '#333333', '#222222']} />
            <OrbitControls enableDamping dampingFactor={0.1} maxPolarAngle={Math.PI / 2.1} />
          </Canvas>
          {currentFrame && (
            <div className="absolute top-3 right-3 bg-bg/80 backdrop-blur rounded-lg border border-border p-2 sm:p-3 text-xs space-y-1">
              <div className="font-mono text-accent text-base sm:text-lg">{currentFrame.speed.toFixed(0)} <span className="text-xs text-text-muted">km/h</span></div>
              <div className="flex gap-3">
                <span className="text-text-muted">Gear <span className="text-text font-mono">{currentFrame.gear}</span></span>
                <span className="text-text-muted">RPM <span className="text-text font-mono">{currentFrame.rpm.toFixed(0)}</span></span>
              </div>
            </div>
          )}
          {nearbyAnnotation && (
            <div className="absolute bottom-3 left-3 right-3 md:left-auto md:right-3 md:max-w-xs bg-yellow/10 backdrop-blur border border-yellow/30 rounded-lg p-2.5 text-xs">
              <div className="text-yellow font-medium mb-0.5">Note</div>
              <div className="text-text">{nearbyAnnotation.text}</div>
            </div>
          )}
        </div>

        <aside className="hidden md:block w-52 border-l border-border overflow-auto p-3">
          <h3 className="text-xs font-medium text-text-muted uppercase tracking-wider mb-2">Laps</h3>
          <div className="space-y-1 mb-4">
            {laps.map((lap) => (
              <button
                key={lap.lap_number}
                onClick={() => setSelectedLap(lap.lap_number)}
                className={clsx(
                  'flex justify-between items-center w-full px-2 py-1.5 rounded text-xs transition-colors',
                  selectedLap === lap.lap_number
                    ? 'bg-accent-dim border border-accent/50 text-text'
                    : 'bg-surface-2 border border-border hover:border-border-2 text-text',
                )}
              >
                <span>Lap {lap.lap_number}</span>
                <span className="font-mono text-text-muted">{formatLapTime(lap.time_ms)}</span>
              </button>
            ))}
          </div>

          <h3 className="text-xs font-medium text-text-muted uppercase tracking-wider mb-2">Notes ({annotations.length})</h3>
          <div className="space-y-1.5">
            {annotations.map((a) => (
              <div
                key={a.id}
                className="bg-surface-2 border border-border rounded p-2 text-xs group cursor-pointer hover:border-yellow/30"
                onClick={() => { setFrameIdx(a.frame_idx); setPlaying(false) }}
              >
                <div className="flex items-center justify-between mb-0.5">
                  <span className="text-text-dim font-mono">{formatTime(a.frame_idx / 60)}</span>
                  <button
                    onClick={(e) => { e.stopPropagation(); handleDeleteAnnotation(a.id) }}
                    className="text-text-dim hover:text-red opacity-0 group-hover:opacity-100 transition-opacity"
                  >
                    &times;
                  </button>
                </div>
                <div className="text-text leading-snug">{a.text}</div>
              </div>
            ))}
            {annotations.length === 0 && (
              <p className="text-[10px] text-text-dim">Pause playback and click + to add notes</p>
            )}
          </div>
        </aside>
      </div>

      <div className="border-t border-border">
        <div className="h-14 sm:h-20 px-3 sm:px-4 pt-1 relative">
          <MiniTelemetry frames={frames} currentIdx={currentIdx} annotations={annotations} />
        </div>
        <div className="flex items-center gap-2 sm:gap-3 px-3 sm:px-4 py-2 border-t border-border">
          <button
            onClick={() => { setFrameIdx(0); setPlaying(false) }}
            className="text-text-muted hover:text-text text-sm"
            title="Reset"
          >
            &#9632;
          </button>
          <button
            onClick={() => setPlaying(!playing)}
            className="w-8 h-8 flex items-center justify-center rounded-full bg-accent text-bg font-bold text-sm shrink-0"
          >
            {playing ? '&#10074;&#10074;' : '&#9654;'}
          </button>
          <input
            type="range"
            min={0}
            max={Math.max(0, frames.length - 1)}
            step={1}
            value={frameIdx}
            onChange={(e) => { setFrameIdx(Number(e.target.value)); setPlaying(false) }}
            className="flex-1 h-1.5 accent-accent cursor-pointer min-w-0"
          />
          <button
            onClick={() => { setPlaying(false); setAnnotating(!annotating) }}
            className={clsx(
              'w-7 h-7 flex items-center justify-center rounded text-sm font-bold transition-colors',
              annotating ? 'bg-yellow/20 text-yellow border border-yellow/30' : 'text-text-muted hover:text-yellow border border-border hover:border-yellow/30',
            )}
            title="Add note at current position"
          >
            +
          </button>
          <div className="hidden sm:flex items-center gap-2">
            {[0.5, 1, 2, 4].map((s) => (
              <button
                key={s}
                onClick={() => setSpeed(s)}
                className={clsx(
                  'text-xs px-1.5 py-0.5 rounded',
                  speed === s ? 'bg-accent/20 text-accent' : 'text-text-muted hover:text-text',
                )}
              >
                {s}x
              </button>
            ))}
          </div>
          <span className="text-xs text-text-muted font-mono w-20 text-right">
            {formatTime(currentIdx / 60)} / {formatTime((frames.length - 1) / 60)}
          </span>
        </div>
        {annotating && (
          <div className="flex items-center gap-2 px-3 sm:px-4 py-2 border-t border-border bg-surface-2">
            <span className="text-xs text-yellow font-mono shrink-0">{formatTime(currentIdx / 60)}</span>
            <input
              type="text"
              value={annotationText}
              onChange={(e) => setAnnotationText(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter') handleAddAnnotation(); if (e.key === 'Escape') setAnnotating(false) }}
              placeholder="Add a note at this moment..."
              className="flex-1 bg-bg border border-border rounded px-2.5 py-1.5 text-xs text-text placeholder:text-text-dim focus:outline-none focus:border-accent"
              autoFocus
            />
            <button
              onClick={handleAddAnnotation}
              disabled={!annotationText.trim()}
              className="px-3 py-1.5 rounded text-xs font-medium bg-yellow/15 text-yellow border border-yellow/30 hover:bg-yellow/25 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            >
              Save
            </button>
            <button
              onClick={() => setAnnotating(false)}
              className="text-text-dim hover:text-text text-xs"
            >
              Cancel
            </button>
          </div>
        )}
      </div>
    </div>
  )
}

function ReplayTrack({ frames, currentIdx, annotations }: { frames: TelemetryFrame[]; currentIdx: number; annotations: Annotation[] }) {
  const meshRef = useRef<THREE.Mesh>(null)

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

  const markerPositions = useMemo(() =>
    annotations
      .filter((a) => a.frame_idx < frames.length)
      .map((a) => new THREE.Vector3(frames[a.frame_idx].x, frames[a.frame_idx].y + 12, frames[a.frame_idx].z)),
  [annotations, frames])

  useFrame(() => {
    if (!meshRef.current || !frames[currentIdx]) return
    const f = frames[currentIdx]
    meshRef.current.position.set(f.x, f.y + 4, f.z)
    const next = frames[Math.min(currentIdx + 1, frames.length - 1)]
    if (next && next !== f) {
      meshRef.current.lookAt(next.x, next.y + 4, next.z)
    }
  })

  return (
    <>
      <Line
        points={points}
        vertexColors={colors.map((c) => [c.r, c.g, c.b] as [number, number, number])}
        lineWidth={3}
      />
      <mesh ref={meshRef}>
        <coneGeometry args={[4, 10, 8]} />
        <meshStandardMaterial color="#ffffff" emissive="#4fc3f7" emissiveIntensity={0.8} />
      </mesh>
      {markerPositions.map((pos, i) => (
        <mesh key={i} position={pos}>
          <sphereGeometry args={[3, 12, 12]} />
          <meshStandardMaterial color="#eab308" emissive="#eab308" emissiveIntensity={0.5} />
        </mesh>
      ))}
    </>
  )
}

function MiniTelemetry({ frames, currentIdx, annotations }: { frames: TelemetryFrame[]; currentIdx: number; annotations: Annotation[] }) {
  const canvasRef = useRef<HTMLCanvasElement>(null)

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas || frames.length === 0) return
    const ctx = canvas.getContext('2d')
    if (!ctx) return

    const w = canvas.width
    const h = canvas.height
    ctx.clearRect(0, 0, w, h)

    const maxSpeed = Math.max(...frames.map((f) => f.speed))

    ctx.beginPath()
    ctx.strokeStyle = '#4fc3f7'
    ctx.lineWidth = 1
    for (let i = 0; i < w; i++) {
      const fi = Math.min(Math.floor((i / w) * frames.length), frames.length - 1)
      const y = h - (frames[fi].speed / maxSpeed) * h * 0.9
      if (i === 0) ctx.moveTo(i, y)
      else ctx.lineTo(i, y)
    }
    ctx.stroke()

    ctx.beginPath()
    ctx.strokeStyle = '#66bb6a44'
    ctx.lineWidth = 1
    for (let i = 0; i < w; i++) {
      const fi = Math.min(Math.floor((i / w) * frames.length), frames.length - 1)
      const y = h - frames[fi].throttle * h * 0.9
      if (i === 0) ctx.moveTo(i, y)
      else ctx.lineTo(i, y)
    }
    ctx.stroke()

    ctx.beginPath()
    ctx.strokeStyle = '#ef535044'
    ctx.lineWidth = 1
    for (let i = 0; i < w; i++) {
      const fi = Math.min(Math.floor((i / w) * frames.length), frames.length - 1)
      const y = h - frames[fi].brake * h * 0.9
      if (i === 0) ctx.moveTo(i, y)
      else ctx.lineTo(i, y)
    }
    ctx.stroke()

    for (const a of annotations) {
      const ax = (a.frame_idx / (frames.length - 1)) * w
      ctx.beginPath()
      ctx.strokeStyle = '#eab308'
      ctx.lineWidth = 1.5
      ctx.moveTo(ax, 0)
      ctx.lineTo(ax, h)
      ctx.stroke()
      ctx.beginPath()
      ctx.fillStyle = '#eab308'
      ctx.arc(ax, 4, 3, 0, Math.PI * 2)
      ctx.fill()
    }

    const px = (currentIdx / (frames.length - 1)) * w
    ctx.beginPath()
    ctx.strokeStyle = '#ffffff'
    ctx.lineWidth = 1
    ctx.moveTo(px, 0)
    ctx.lineTo(px, h)
    ctx.stroke()
  }, [frames, currentIdx, annotations])

  return (
    <canvas
      ref={canvasRef}
      width={800}
      height={60}
      className="w-full h-full"
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

function formatTime(seconds: number): string {
  const m = Math.floor(seconds / 60)
  const s = Math.floor(seconds % 60)
  return `${m}:${s.toString().padStart(2, '0')}`
}
