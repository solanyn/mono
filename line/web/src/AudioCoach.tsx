import { useCallback, useEffect, useRef, useState } from 'react'

interface AudioCoachProps {
  wsUrl?: string
}

interface CoachMessage {
  text: string
  latencyMs: number
  timestamp: number
}

export function useAudioCoach(wsUrl = 'ws://localhost:8090/ws') {
  const wsRef = useRef<WebSocket | null>(null)
  const audioCtxRef = useRef<AudioContext | null>(null)
  const [connected, setConnected] = useState(false)
  const [messages, setMessages] = useState<CoachMessage[]>([])
  const pendingMetaRef = useRef<{ text: string; latency_ms: number } | null>(null)

  useEffect(() => {
    const ws = new WebSocket(wsUrl)
    wsRef.current = ws

    ws.onopen = () => setConnected(true)
    ws.onclose = () => setConnected(false)
    ws.onerror = () => setConnected(false)

    ws.onmessage = async (event) => {
      if (typeof event.data === 'string') {
        const data = JSON.parse(event.data)
        if (data.type === 'audio') {
          pendingMetaRef.current = data
        }
      } else if (event.data instanceof Blob) {
        if (!audioCtxRef.current) {
          audioCtxRef.current = new AudioContext({ sampleRate: 24000 })
        }
        const ctx = audioCtxRef.current
        const arrayBuffer = await event.data.arrayBuffer()
        const audioBuffer = await ctx.decodeAudioData(arrayBuffer)
        const source = ctx.createBufferSource()
        source.buffer = audioBuffer
        source.connect(ctx.destination)
        source.start()

        if (pendingMetaRef.current) {
          setMessages(prev => [...prev.slice(-9), {
            text: pendingMetaRef.current!.text,
            latencyMs: pendingMetaRef.current!.latency_ms,
            timestamp: Date.now(),
          }])
          pendingMetaRef.current = null
        }
      }
    }

    return () => {
      ws.close()
      audioCtxRef.current?.close()
    }
  }, [wsUrl])

  const speak = useCallback((text: string, voice?: string, speed?: number) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ text, voice, speed }))
    }
  }, [])

  return { connected, speak, messages }
}

export function AudioCoach({ wsUrl }: AudioCoachProps) {
  const { connected, speak, messages } = useAudioCoach(wsUrl)
  const [input, setInput] = useState('')

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (input.trim()) {
      speak(input.trim())
      setInput('')
    }
  }

  return (
    <div style={{ padding: 16, fontFamily: 'monospace' }}>
      <div style={{ marginBottom: 8 }}>
        Status: {connected ? '🟢 Connected' : '🔴 Disconnected'}
      </div>
      <form onSubmit={handleSubmit} style={{ marginBottom: 16 }}>
        <input
          value={input}
          onChange={e => setInput(e.target.value)}
          placeholder="Type coaching message..."
          style={{ width: 300, marginRight: 8, padding: 4 }}
        />
        <button type="submit" disabled={!connected}>Speak</button>
      </form>
      <div>
        {messages.map((m, i) => (
          <div key={i} style={{ fontSize: 12, opacity: 0.8, marginBottom: 4 }}>
            [{m.latencyMs}ms] {m.text}
          </div>
        ))}
      </div>
    </div>
  )
}
