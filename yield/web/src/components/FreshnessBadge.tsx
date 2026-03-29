import { Badge } from '@/components/ui/badge'

type Freshness = 'fresh' | 'stale' | 'pending'

interface FreshnessBadgeProps {
  status: Freshness
}

const config: Record<Freshness, { label: string; variant: 'success' | 'warning' | 'secondary' }> = {
  fresh: { label: 'Fresh', variant: 'success' },
  stale: { label: 'Stale', variant: 'warning' },
  pending: { label: 'Pending', variant: 'secondary' },
}

export function FreshnessBadge({ status }: FreshnessBadgeProps) {
  const { label, variant } = config[status]
  return <Badge variant={variant}>{label}</Badge>
}
