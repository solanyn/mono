import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'

interface ValueEstimateProps {
  low: number
  mid: number
  high: number
  confidence: number
  comparableCount: number
  isStrata: boolean
}

export function ValueEstimate({ low, mid, high, confidence, comparableCount, isStrata }: ValueEstimateProps) {
  const pct = Math.min(confidence * 100, 100)
  const color = pct >= 70 ? 'bg-green-500' : pct >= 40 ? 'bg-yellow-500' : 'bg-red-500'
  const label = pct >= 70 ? 'High' : pct >= 40 ? 'Medium' : 'Low'

  return (
    <Card>
      <CardHeader className="flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle>Value Estimate</CardTitle>
        <div className="flex gap-2">
          <Badge variant={isStrata ? 'secondary' : 'outline'}>{isStrata ? 'Strata' : 'Freehold'}</Badge>
          <Badge variant={pct >= 70 ? 'success' : pct >= 40 ? 'warning' : 'destructive'}>{label} confidence</Badge>
        </div>
      </CardHeader>
      <CardContent>
        <div className="flex items-baseline gap-2 mb-4">
          <span className="text-3xl font-bold">{formatPrice(mid)}</span>
          <span className="text-sm text-muted-foreground">
            ({formatPrice(low)} — {formatPrice(high)})
          </span>
        </div>

        <div className="grid grid-cols-2 gap-4 mb-4">
          <div>
            <p className="text-sm text-muted-foreground">Confidence</p>
            <p className="text-xl font-semibold">{pct.toFixed(0)}%</p>
          </div>
          <div>
            <p className="text-sm text-muted-foreground">Comparables</p>
            <p className="text-xl font-semibold">{comparableCount}</p>
          </div>
        </div>

        <div>
          <p className="text-sm text-muted-foreground mb-1">Estimate range</p>
          <div className="relative h-3 rounded-full bg-muted overflow-hidden">
            <div className={`absolute inset-y-0 left-0 rounded-full ${color}`} style={{ width: `${pct}%` }} />
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

function formatPrice(n: number): string {
  if (n >= 1_000_000) return `$${(n / 1_000_000).toFixed(2)}M`
  if (n >= 1_000) return `$${(n / 1_000).toFixed(0)}k`
  return `$${n}`
}
