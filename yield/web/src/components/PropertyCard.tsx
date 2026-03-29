import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'

interface PropertyCardProps {
  address: string
  suburb: string
  postcode?: string
  price?: number
  date?: string
  bedrooms?: number
  bathrooms?: number
  propertyType?: string
  strataLot?: string
  area?: number
  onClick?: () => void
}

export function PropertyCard({ address, suburb, postcode, price, date, bedrooms, bathrooms, propertyType, strataLot, area, onClick }: PropertyCardProps) {
  return (
    <Card className={onClick ? 'cursor-pointer hover:border-primary/50 transition-colors' : ''} onClick={onClick}>
      <CardContent className="pt-6">
        <div className="flex items-start justify-between">
          <div>
            <p className="font-semibold">{address}</p>
            <p className="text-sm text-muted-foreground">{suburb} {postcode}</p>
          </div>
          {price != null && <span className="font-semibold text-lg">{formatPrice(price)}</span>}
        </div>
        <div className="flex flex-wrap gap-2 mt-3">
          {bedrooms != null && <Badge variant="outline">{bedrooms} bed</Badge>}
          {bathrooms != null && <Badge variant="outline">{bathrooms} bath</Badge>}
          {propertyType && <Badge variant="outline" className="capitalize">{propertyType}</Badge>}
          {strataLot && <Badge variant="secondary">Strata</Badge>}
          {area != null && area > 0 && <Badge variant="outline">{area}m²</Badge>}
          {date && <Badge variant="secondary">{new Date(date).toLocaleDateString()}</Badge>}
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
