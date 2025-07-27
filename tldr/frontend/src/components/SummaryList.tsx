import React from 'react'
import MarkdownRenderer from './MarkdownRenderer'

interface Summary {
  id: string
  title: string
  content: string
  publishedAt: string
  source: string
}

const mockSummaries: Summary[] = [
  {
    id: "1",
    title: "AI Breakthrough in Natural Language Processing",
    content: `# Major AI Breakthrough

This week saw significant **advances** in natural language processing with the release of new transformer architectures.

## Key Points:
- 40% improvement in efficiency
- Better multilingual support
- Reduced computational requirements

The implications for content summarization are substantial.`,
    publishedAt: "2024-01-15",
    source: "TechCrunch"
  },
  {
    id: "2", 
    title: "Climate Tech Funding Reaches Record High",
    content: `# Climate Investment Update

Investment in climate technology reached **$8.2 billion** this quarter, marking a new record.

## Highlights:
- Carbon capture technologies lead funding
- Renewable energy storage innovations
- Green transportation solutions

This represents a 65% increase from last year's figures.`,
    publishedAt: "2024-01-14",
    source: "Bloomberg"
  }
]

export default function SummaryList() {
  return (
    <div className="summary-list">
      <h2>Recent Summaries</h2>
      {mockSummaries.map(summary => (
        <article key={summary.id} className="summary-card">
          <header>
            <h3>{summary.title}</h3>
            <div className="summary-meta">
              <span>Published: {new Date(summary.publishedAt).toLocaleDateString()}</span>
              <span>Source: {summary.source}</span>
            </div>
          </header>
          <MarkdownRenderer content={summary.content} />
        </article>
      ))}
    </div>
  )
}