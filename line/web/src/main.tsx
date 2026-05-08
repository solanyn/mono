import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter, Routes, Route } from 'react-router'
import { App } from './App'
import { SessionsPage } from './pages/Sessions'
import { SessionDetail } from './pages/SessionDetail'
import { LivePage } from './pages/Live'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <BrowserRouter>
      <Routes>
        <Route element={<App />}>
          <Route index element={<SessionsPage />} />
          <Route path="sessions" element={<SessionsPage />} />
          <Route path="sessions/:id" element={<SessionDetail />} />
          <Route path="live" element={<LivePage />} />
        </Route>
      </Routes>
    </BrowserRouter>
  </StrictMode>,
)
