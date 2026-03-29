import { MapContainer, TileLayer, CircleMarker, Popup } from 'react-leaflet'
import type { LatLngExpression } from 'leaflet'
import 'leaflet/dist/leaflet.css'

interface SalePoint {
  lat: number
  lon: number
  price: number
  address: string
  date?: string
}

interface SuburbHeatmapProps {
  center: LatLngExpression
  sales: SalePoint[]
  medianPrice?: number
}

export function SuburbHeatmap({ center, sales, medianPrice }: SuburbHeatmapProps) {
  const median = medianPrice || calcMedian(sales.map(s => s.price))

  return (
    <div className="h-[500px] w-full rounded-lg overflow-hidden border">
      <MapContainer center={center} zoom={15} className="h-full w-full">
        <TileLayer
          attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
          url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
        />
        {sales.map((s, i) => (
          <CircleMarker
            key={i}
            center={[s.lat, s.lon]}
            radius={6}
            pathOptions={{
              color: priceColor(s.price, median),
              fillColor: priceColor(s.price, median),
              fillOpacity: 0.7,
              weight: 1,
            }}
          >
            <Popup>
              <div className="text-sm">
                <p className="font-semibold">{s.address}</p>
                <p>{formatPrice(s.price)}</p>
                {s.date && <p className="text-muted-foreground">{s.date}</p>}
              </div>
            </Popup>
          </CircleMarker>
        ))}
      </MapContainer>
      <div className="flex items-center justify-center gap-4 py-2 text-xs text-muted-foreground">
        <span className="flex items-center gap-1">
          <span className="inline-block w-3 h-3 rounded-full bg-green-500" /> Below median
        </span>
        <span className="flex items-center gap-1">
          <span className="inline-block w-3 h-3 rounded-full bg-yellow-500" /> Near median
        </span>
        <span className="flex items-center gap-1">
          <span className="inline-block w-3 h-3 rounded-full bg-red-500" /> Above median
        </span>
      </div>
    </div>
  )
}

function priceColor(price: number, median: number): string {
  const ratio = price / median
  if (ratio < 0.85) return '#22c55e'
  if (ratio < 1.15) return '#eab308'
  return '#ef4444'
}

function calcMedian(values: number[]): number {
  if (values.length === 0) return 0
  const sorted = [...values].sort((a, b) => a - b)
  return sorted[Math.floor(sorted.length / 2)]
}

function formatPrice(n: number): string {
  if (n >= 1_000_000) return `$${(n / 1_000_000).toFixed(2)}M`
  if (n >= 1_000) return `$${(n / 1_000).toFixed(0)}k`
  return `$${n}`
}
