from pydantic_ai import tool
import httpx

GO_API = "http://localhost:8080/api"


@tool
async def search_sales(
    suburb: str,
    state: str = "NSW",
    property_type: str = "house",
    bedrooms: int | None = None,
    months: int = 12,
) -> list[dict]:
    """Search historical property sales in a suburb."""
    params = {
        "suburb": suburb,
        "state": state,
        "type": property_type,
        "months": str(months),
    }
    if bedrooms:
        params["bedrooms"] = str(bedrooms)
    async with httpx.AsyncClient() as client:
        r = await client.get(f"{GO_API}/search", params=params)
        return r.json()


@tool
async def rent_analysis(
    address: str,
    bedrooms: int | None = None,
    property_type: str | None = None,
) -> dict:
    """Analyze rent fairness for an address."""
    body: dict[str, str | int] = {"address": address}
    if bedrooms:
        body["bedrooms"] = bedrooms
    if property_type:
        body["property_type"] = property_type
    async with httpx.AsyncClient() as client:
        r = await client.post(f"{GO_API}/rent-check", json=body)
        return r.json()


@tool
async def suburb_stats(suburb: str, state: str = "NSW") -> dict:
    """Get suburb market statistics."""
    async with httpx.AsyncClient() as client:
        r = await client.get(f"{GO_API}/suburb/{suburb}")
        return r.json()


@tool
async def value_estimate(
    address: str,
    purchase_price: float | None = None,
    purchase_date: str | None = None,
) -> dict:
    """Estimate current property value based on comparable sales."""
    body = {"address": address, "price": int(purchase_price or 0), "rent_per_week": 0}
    async with httpx.AsyncClient() as client:
        r = await client.post(f"{GO_API}/analyze", json=body)
        return r.json()


@tool
async def school_catchment(address: str) -> dict:
    """Find school catchments for an address."""
    async with httpx.AsyncClient() as client:
        r = await client.get(
            f"{GO_API}/property/catchment", params={"address": address}
        )
        return r.json()


@tool
async def portfolio_summary() -> dict:
    """Get summary of all tracked investment properties."""
    async with httpx.AsyncClient() as client:
        r = await client.get(f"{GO_API}/portfolio")
        return r.json()


@tool
async def compare_listings(listing_urls: list[str]) -> dict:
    """Compare multiple property listings side by side on key metrics."""
    async with httpx.AsyncClient() as client:
        r = await client.post(f"{GO_API}/analyze/compare", json={"urls": listing_urls})
        return r.json()
