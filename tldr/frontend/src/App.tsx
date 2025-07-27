import React from 'react'
import SummaryList from './components/SummaryList'
import './App.css'

function App() {
  return (
    <div className="App">
      <header className="App-header">
        <h1>TLDR - RSS Summary Reader</h1>
        <nav>
          <a href="#summaries">Summaries</a>
          <a href="#feeds">Feeds</a>
        </nav>
      </header>
      <main>
        <SummaryList />
      </main>
    </div>
  )
}

export default App