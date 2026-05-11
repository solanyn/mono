import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter, Routes, Route } from 'react-router'
import './index.css'
import { App } from './App'
import { SessionsPage } from './pages/Sessions'
import { SessionDetail } from './pages/SessionDetail'
import { ReplayPage } from './pages/Replay'
import { BriefingPage } from './pages/Briefing'
import { LivePage } from './pages/Live'
import { ProgressionPage } from './pages/Progression'
import { TracksPage } from './pages/Tracks'
import { ReferenceLapsPage } from './pages/ReferenceLaps'
import { ComparePage } from './pages/Compare'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <BrowserRouter>
      <Routes>
        <Route element={<App />}>
          <Route index element={<SessionsPage />} />
          <Route path="sessions" element={<SessionsPage />} />
          <Route path="sessions/:id" element={<SessionDetail />} />
          <Route path="sessions/:id/replay" element={<ReplayPage />} />
          <Route path="sessions/:id/briefing" element={<BriefingPage />} />
          <Route path="live" element={<LivePage />} />
          <Route path="progression" element={<ProgressionPage />} />
          <Route path="tracks" element={<TracksPage />} />
          <Route path="reference" element={<ReferenceLapsPage />} />
          <Route path="compare" element={<ComparePage />} />
        </Route>
      </Routes>
    </BrowserRouter>
  </StrictMode>,
)

if ('serviceWorker' in navigator) {
  navigator.serviceWorker.register('/sw.js').then(async (reg) => {
    try {
      const res = await fetch('/api/v1/push/vapid')
      const { public_key } = await res.json()
      if (!public_key) return

      let sub = await reg.pushManager.getSubscription()
      if (!sub) {
        const key = Uint8Array.from(atob(public_key.replace(/-/g, '+').replace(/_/g, '/')), c => c.charCodeAt(0))
        sub = await reg.pushManager.subscribe({
          userVisibleOnly: true,
          applicationServerKey: key,
        })
        await fetch('/api/v1/push/subscribe', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(sub.toJSON()),
        })
      }
    } catch {}
  }).catch(() => {})
}
