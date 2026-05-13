import { StrictMode, lazy, Suspense } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter, Routes, Route } from 'react-router'
import './index.css'
import { App } from './App'

const SessionsPage = lazy(() => import('./pages/Sessions').then(m => ({ default: m.SessionsPage })))
const SessionDetail = lazy(() => import('./pages/SessionDetail').then(m => ({ default: m.SessionDetail })))
const ReplayPage = lazy(() => import('./pages/Replay').then(m => ({ default: m.ReplayPage })))
const BriefingPage = lazy(() => import('./pages/Briefing').then(m => ({ default: m.BriefingPage })))
const LivePage = lazy(() => import('./pages/Live').then(m => ({ default: m.LivePage })))
const ProgressionPage = lazy(() => import('./pages/Progression').then(m => ({ default: m.ProgressionPage })))
const TracksPage = lazy(() => import('./pages/Tracks').then(m => ({ default: m.TracksPage })))
const ReferenceLapsPage = lazy(() => import('./pages/ReferenceLaps').then(m => ({ default: m.ReferenceLapsPage })))
const ComparePage = lazy(() => import('./pages/Compare').then(m => ({ default: m.ComparePage })))

function Loading() {
  return <div className="flex items-center justify-center h-full text-text-muted text-sm">Loading...</div>
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <BrowserRouter>
      <Routes>
        <Route element={<App />}>
          <Route index element={<Suspense fallback={<Loading />}><SessionsPage /></Suspense>} />
          <Route path="sessions" element={<Suspense fallback={<Loading />}><SessionsPage /></Suspense>} />
          <Route path="sessions/:id" element={<Suspense fallback={<Loading />}><SessionDetail /></Suspense>} />
          <Route path="sessions/:id/replay" element={<Suspense fallback={<Loading />}><ReplayPage /></Suspense>} />
          <Route path="sessions/:id/briefing" element={<Suspense fallback={<Loading />}><BriefingPage /></Suspense>} />
          <Route path="live" element={<Suspense fallback={<Loading />}><LivePage /></Suspense>} />
          <Route path="progression" element={<Suspense fallback={<Loading />}><ProgressionPage /></Suspense>} />
          <Route path="tracks" element={<Suspense fallback={<Loading />}><TracksPage /></Suspense>} />
          <Route path="reference" element={<Suspense fallback={<Loading />}><ReferenceLapsPage /></Suspense>} />
          <Route path="compare" element={<Suspense fallback={<Loading />}><ComparePage /></Suspense>} />
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
