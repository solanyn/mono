const API_BASE = '/api'

export async function fetchJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...init,
  })
  if (!res.ok) throw new Error(`API error: ${res.status}`)
  return res.json()
}

export const api = {
  health: () => fetchJSON<{ status: string }>('/health'),
  property: (id: string) => fetchJSON(`/property/${id}`),
  rentCheck: (body: unknown) => fetchJSON('/rent-check', { method: 'POST', body: JSON.stringify(body) }),
  suburbStats: (slug: string) => fetchJSON(`/suburb/${slug}`),
  analyze: (body: unknown) => fetchJSON('/analyze', { method: 'POST', body: JSON.stringify(body) }),
  search: (params: string) => fetchJSON(`/search?${params}`),
  portfolio: () => fetchJSON('/portfolio'),
}
