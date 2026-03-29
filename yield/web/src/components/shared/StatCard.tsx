import { Card, CardContent } from '@/components/ui/card'

interface StatCardProps {
  label: string
  value: string
  sub?: string
  className?: string
}

export function StatCard({ label, value, sub, className }: StatCardProps) {
  return (
    <Card className={className}>
      <CardContent className="pt-5 pb-4">
        <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">{label}</p>
        <p className="text-2xl font-bold mt-1">{value}</p>
        {sub && <p className="text-xs text-muted-foreground mt-0.5">{sub}</p>}
      </CardContent>
    </Card>
  )
}
