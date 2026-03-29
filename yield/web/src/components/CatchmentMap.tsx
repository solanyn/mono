import { MapContainer, TileLayer, GeoJSON } from 'react-leaflet'
import type { LatLngExpression } from 'leaflet'
import 'leaflet/dist/leaflet.css'

interface CatchmentMapProps {
  lat: number
  lon: number
  catchments?: {
    primary: Array<{ school: string; geojson?: object }>
    secondary: Array<{ school: string; geojson?: object }>
  }
}

export function CatchmentMap({ lat, lon, catchments }: CatchmentMapProps) {
  const center: LatLngExpression = [lat, lon]

  return (
    <div className="h-[400px] w-full rounded-lg overflow-hidden border">
      <MapContainer center={center} zoom={14} className="h-full w-full">
        <TileLayer
          attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
          url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
        />
        {catchments?.primary.map((c, i) =>
          c.geojson ? (
            <GeoJSON
              key={`primary-${i}`}
              data={c.geojson as GeoJSON.GeoJsonObject}
              style={{ color: '#2563eb', weight: 2, fillOpacity: 0.1 }}
            />
          ) : null
        )}
        {catchments?.secondary.map((c, i) =>
          c.geojson ? (
            <GeoJSON
              key={`secondary-${i}`}
              data={c.geojson as GeoJSON.GeoJsonObject}
              style={{ color: '#dc2626', weight: 2, fillOpacity: 0.1 }}
            />
          ) : null
        )}
      </MapContainer>
    </div>
  )
}
