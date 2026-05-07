const API_BASE = '/api/v1'

export interface Session {
  id: string
  started_at: number
  ended_at: number
  car_code: number
  track_id: string
  lap_count: number
  best_lap_ms: number
}

export interface Lap {
  session_id: string
  lap_number: number
  time_ms: number
  frames: number
  top_speed: number
  s3_key: string
}

export interface TelemetryFrame {
  packet_id: number
  x: number
  y: number
  z: number
  speed: number
  throttle: number
  brake: number
  steering: number
  rpm: number
  gear: number
  tire_temp_fl?: number
  tire_temp_fr?: number
  tire_temp_rl?: number
  tire_temp_rr?: number
  fuel_level?: number
  current_lap: number
  current_time_ms: number
  timestamp_ns: number
}

export interface LiveStatus {
  active: boolean
  session_id?: string
  car_code?: number
  track_id?: string
  current_lap?: number
}

export async function fetchStatus(): Promise<LiveStatus> {
  const res = await fetch(`${API_BASE}/status`)
  return res.json()
}

export async function fetchSessions(): Promise<{ sessions: Session[]; next_cursor: string }> {
  const res = await fetch(`${API_BASE}/sessions`)
  return res.json()
}

export async function fetchSession(id: string): Promise<Session> {
  const res = await fetch(`${API_BASE}/sessions/${id}`)
  return res.json()
}

export async function fetchLaps(sessionId: string): Promise<{ laps: Lap[] }> {
  const res = await fetch(`${API_BASE}/sessions/${sessionId}/laps`)
  return res.json()
}

export async function fetchTelemetry(
  sessionId: string,
  lap: number,
  downsample = 1,
): Promise<{ frames: TelemetryFrame[]; total: number; returned: number }> {
  const res = await fetch(
    `${API_BASE}/sessions/${sessionId}/laps/${lap}/telemetry?downsample=${downsample}`,
  )
  return res.json()
}

export function connectLive(onFrame: (frame: TelemetryFrame) => void, onStatus: (s: LiveStatus) => void): WebSocket {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const ws = new WebSocket(`${proto}//${window.location.host}${API_BASE}/live`)
  let first = true
  ws.onmessage = (ev) => {
    const data = JSON.parse(ev.data)
    if (first) {
      onStatus(data as LiveStatus)
      first = false
    } else {
      onFrame(data as TelemetryFrame)
    }
  }
  return ws
}

export interface CoachMessage {
  type: string
  text: string
  latency_ms: number
}

export function connectCoach(
  onMessage: (msg: CoachMessage) => void,
  onAudio: (audio: ArrayBuffer) => void,
): WebSocket {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const ws = new WebSocket(`${proto}//${window.location.host}${API_BASE}/coach`)
  ws.binaryType = 'arraybuffer'
  ws.onmessage = (ev) => {
    if (ev.data instanceof ArrayBuffer) {
      onAudio(ev.data)
    } else {
      onMessage(JSON.parse(ev.data) as CoachMessage)
    }
  }
  return ws
}

export interface LapMetrics {
  session_id: string
  lap_number: number
  total_distance_m: number
  top_speed: number
  avg_speed: number
  min_speed: number
  max_rpm: number
  brake_count: number
  throttle_pct: number
  coast_pct: number
  brake_pct: number
  avg_tire_temps: Record<string, number>
  fuel_used: number
  frame_count: number
  track_id?: string
  track_name?: string
  corner_count?: number
  corners?: CornerData[]
}

export interface CornerData {
  entry_idx: number
  apex_idx: number
  exit_idx: number
  entry_speed: number
  apex_speed: number
  exit_speed: number
  direction: string
}

export interface SessionSummary {
  session_id: string
  car_code: number
  track_name: string
  lap_count: number
  consistency: {
    consistency_score: number
    lap_time_cv: number
    speed_cv: number
    best_lap_idx: number
    worst_lap_idx: number
    best_worst_delta_ms: number
  }
  tyre_degradation: {
    avg_temp_per_lap: number[]
    degradation_rate: number
    estimated_laps_remaining: number
    compound_guess: string
    front_rear_balance: number
  }
  fuel_strategy: {
    consumption_per_lap: number
    fuel_remaining: number
    laps_remaining: number
    optimal_pit_lap: number
  }
  journal: {
    total_laps: number
    best_lap_ms: number
    worst_lap_ms: number
    consistency_score: number
    highlights: string[]
    areas_to_improve: string[]
    corner_notes: string[]
    summary: string
  }
}

export interface TrackInfo {
  track_id: string
  name: string
  country: string
  length_m: number
  corners: { number: number; name: string; direction: string; notes: string }[]
  source: string
}

export async function fetchLapMetrics(sessionId: string, lap: number): Promise<LapMetrics> {
  const res = await fetch(`${API_BASE}/sessions/${sessionId}/laps/${lap}/metrics`)
  return res.json()
}

export async function fetchSessionSummary(sessionId: string): Promise<SessionSummary> {
  const res = await fetch(`${API_BASE}/sessions/${sessionId}/summary`)
  return res.json()
}

export async function fetchTracks(): Promise<{ tracks: TrackInfo[] }> {
  const res = await fetch(`${API_BASE}/tracks`)
  return res.json()
}
