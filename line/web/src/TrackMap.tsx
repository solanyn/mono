import { useMemo, useRef } from 'react'
import { Canvas, useFrame } from '@react-three/fiber'
import { OrbitControls, Line } from '@react-three/drei'
import * as THREE from 'three'

interface TelemetryPoint {
  x: number
  y: number
  z: number
  speed: number
}

function generateSampleTrack(): TelemetryPoint[] {
  const points: TelemetryPoint[] = []
  const numPoints = 600
  for (let i = 0; i < numPoints; i++) {
    const t = (i / numPoints) * Math.PI * 2
    const r = 200 + 60 * Math.sin(t * 2) + 30 * Math.cos(t * 5)
    const x = Math.cos(t) * r
    const z = Math.sin(t) * r
    const y = 5 * Math.sin(t * 3) + 2 * Math.cos(t * 7)
    const curvature = Math.abs(60 * 2 * Math.cos(t * 2) + 30 * -5 * Math.sin(t * 5))
    const speed = Math.max(40, 280 - curvature * 0.8)
    points.push({ x, y, z, speed })
  }
  return points
}

function speedToColor(speed: number, minSpeed: number, maxSpeed: number): THREE.Color {
  const t = (speed - minSpeed) / (maxSpeed - minSpeed)
  if (t < 0.5) {
    return new THREE.Color().setHSL(0, 1, 0.5).lerp(new THREE.Color().setHSL(0.15, 1, 0.5), t * 2)
  }
  return new THREE.Color().setHSL(0.15, 1, 0.5).lerp(new THREE.Color().setHSL(0.35, 1, 0.45), (t - 0.5) * 2)
}

function TrackLine({ data }: { data: TelemetryPoint[] }) {
  const { points, colors } = useMemo(() => {
    const minSpeed = Math.min(...data.map(p => p.speed))
    const maxSpeed = Math.max(...data.map(p => p.speed))
    const pts = data.map(p => new THREE.Vector3(p.x, p.y, p.z))
    pts.push(pts[0])
    const cols = data.map(p => speedToColor(p.speed, minSpeed, maxSpeed))
    cols.push(cols[0])
    return { points: pts, colors: cols }
  }, [data])

  return (
    <Line
      points={points}
      vertexColors={colors.map(c => [c.r, c.g, c.b] as [number, number, number])}
      lineWidth={3}
    />
  )
}

function CarMarker({ data }: { data: TelemetryPoint[] }) {
  const meshRef = useRef<THREE.Mesh>(null)
  const indexRef = useRef(0)

  useFrame((_, delta) => {
    indexRef.current = (indexRef.current + delta * 60) % data.length
    const i = Math.floor(indexRef.current)
    const p = data[i]
    if (meshRef.current && p) {
      meshRef.current.position.set(p.x, p.y + 3, p.z)
      const next = data[(i + 1) % data.length]
      meshRef.current.lookAt(next.x, next.y + 3, next.z)
    }
  })

  return (
    <mesh ref={meshRef}>
      <coneGeometry args={[4, 10, 8]} />
      <meshStandardMaterial color="#ffffff" emissive="#4fc3f7" emissiveIntensity={0.8} />
    </mesh>
  )
}

function TrackScene({ data }: { data: TelemetryPoint[] }) {
  return (
    <>
      <ambientLight intensity={0.4} />
      <directionalLight position={[100, 200, 100]} intensity={0.8} />
      <TrackLine data={data} />
      <CarMarker data={data} />
      <gridHelper args={[800, 40, '#333333', '#222222']} />
      <OrbitControls
        enableDamping
        dampingFactor={0.1}
        maxPolarAngle={Math.PI / 2.1}
      />
    </>
  )
}

export function TrackMap() {
  const data = useMemo(() => generateSampleTrack(), [])

  return (
    <div style={{ width: '100%', height: '100%', background: '#111' }}>
      <Canvas camera={{ position: [0, 300, 400], fov: 50 }}>
        <TrackScene data={data} />
      </Canvas>
    </div>
  )
}
