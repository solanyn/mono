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
        </Route>
      </Routes>
    </BrowserRouter>
  </StrictMode>,
)
