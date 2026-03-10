import { BrowserRouter, Routes, Route } from 'react-router'
import { CopilotKit } from '@copilotkit/react-core'
import { CopilotSidebar } from '@copilotkit/react-ui'
import { RentCheck } from './routes/RentCheck'
import { Property } from './routes/Property'
import { Suburb } from './routes/Suburb'
import { Analyze } from './routes/Analyze'
import { Portfolio } from './routes/Portfolio'
import { Search } from './routes/Search'

export function App() {
  return (
    <CopilotKit runtimeUrl="/copilotkit" agent="property_agent">
      <BrowserRouter>
        <div className="min-h-screen bg-background text-foreground">
          <nav className="border-b px-6 py-3 flex items-center gap-6">
            <a href="/" className="font-bold text-lg">Yield</a>
            <a href="/rent-check" className="text-sm text-muted-foreground hover:text-foreground">Rent Check</a>
            <a href="/search" className="text-sm text-muted-foreground hover:text-foreground">Search</a>
            <a href="/portfolio" className="text-sm text-muted-foreground hover:text-foreground">Portfolio</a>
          </nav>
          <main className="px-6 py-4">
            <Routes>
              <Route path="/" element={<Search />} />
              <Route path="/rent-check" element={<RentCheck />} />
              <Route path="/property/:id" element={<Property />} />
              <Route path="/suburb/:slug" element={<Suburb />} />
              <Route path="/analyze" element={<Analyze />} />
              <Route path="/portfolio" element={<Portfolio />} />
              <Route path="/search" element={<Search />} />
            </Routes>
          </main>
        </div>
      </BrowserRouter>
      <CopilotSidebar
        defaultOpen={false}
        labels={{
          title: 'Property Assistant',
          initial: 'Ask me about any property, suburb, or your portfolio.',
        }}
      />
    </CopilotKit>
  )
}
