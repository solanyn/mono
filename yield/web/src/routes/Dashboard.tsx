import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router'
import { useCopilotReadable } from '@copilotkit/react-core'
import { api, type PortfolioProperty } from '../lib/api'
import { formatPrice, formatPercent } from '../lib/format'
import { PageHeader } from '@/components/layout/PageHeader'
import { StatCard } from '@/components/shared/StatCard'
import { EmptyState } from '@/components/shared/EmptyState'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'

export function Dashboard() {
  const navigate = useNavigate()
  const [properties, setProperties] = useState<PortfolioProperty[]>([])
  const [loading, setLoading] = useState(true)

  useCopilotReadable({
    description: 'Dashboard overview state',
    value: {
      page: 'dashboard',
      propertyCount: properties.length,
      totalValue: properties.reduce((s, p) => s + (p.purchase_price || 0), 0),
      totalWeeklyRent: properties.reduce((s, p) => s + (p.current_rent_pw || 0), 0),
    },
  })

  useEffect(() => {
    api.portfolio()
      .then(setProperties)
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [])

  const totalValue = properties.reduce((s, p) => s + (p.purchase_price || 0), 0)
  const totalRent = properties.reduce((s, p) => s + (p.current_rent_pw || 0), 0)
  const avgYield = totalValue > 0 ? (totalRent * 52) / totalValue * 100 : 0

  if (loading) {
    return <p className="text-muted-foreground">Loading...</p>
  }

  return (
    <div className="max-w-5xl">
      <PageHeader title="Dashboard" description="Your property investment overview." />

      {properties.length > 0 ? (
        <>
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-8">
            <StatCard label="Portfolio Value" value={formatPrice(totalValue)} />
            <StatCard label="Weekly Rent" value={`$${totalRent}/pw`} sub={`$${(totalRent * 52).toLocaleString()}/yr`} />
            <StatCard label="Avg Gross Yield" value={formatPercent(avgYield, 2)} />
          </div>

          <div className="rounded-lg border mb-8">
            <div className="p-4 border-b flex items-center justify-between">
              <p className="font-semibold text-sm">Properties</p>
              <Button variant="ghost" size="sm" onClick={() => navigate('/portfolio')}>
                View all
              </Button>
            </div>
            <div className="divide-y">
              {properties.slice(0, 5).map(p => {
                const y = p.purchase_price && p.current_rent_pw
                  ? ((p.current_rent_pw * 52) / p.purchase_price * 100)
                  : 0
                return (
                  <div key={p.id} className="flex items-center justify-between px-4 py-3">
                    <div>
                      <p className="text-sm font-medium">{p.address}</p>
                      <p className="text-xs text-muted-foreground">{p.suburb} {p.postcode}</p>
                    </div>
                    <div className="text-right">
                      {p.purchase_price && <p className="text-sm font-semibold">{formatPrice(p.purchase_price)}</p>}
                      {y > 0 && <p className="text-xs text-muted-foreground">{formatPercent(y, 2)} yield</p>}
                    </div>
                  </div>
                )
              })}
            </div>
          </div>
        </>
      ) : (
        <EmptyState
          title="No properties yet"
          description="Add your first investment property to see portfolio analytics here."
          action={{ label: 'Add Property', onClick: () => navigate('/portfolio') }}
        />
      )}

      <div className="mt-8">
        <p className="text-xs font-semibold uppercase tracking-widest text-muted-foreground mb-3">Quick Actions</p>
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
          <ActionCard title="Search Sales" description="Browse recent property sales by suburb" onClick={() => navigate('/search')} />
          <ActionCard title="Rent Check" description="Check if a rental price is fair" onClick={() => navigate('/rent-check')} />
          <ActionCard title="Analyze" description="Run a full investment analysis" onClick={() => navigate('/analyze')} />
        </div>
      </div>
    </div>
  )
}

function ActionCard({ title, description, onClick }: { title: string; description: string; onClick: () => void }) {
  return (
    <Card className="cursor-pointer hover:bg-accent/50 transition-colors" onClick={onClick}>
      <CardContent className="pt-5 pb-4">
        <p className="font-semibold text-sm">{title}</p>
        <p className="text-xs text-muted-foreground mt-1">{description}</p>
      </CardContent>
    </Card>
  )
}
