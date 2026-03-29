from pydantic import BaseModel, Field


class RentFairness(BaseModel):
    address: str
    median_rent_pw: float
    percentile: int
    fairness: str
    comparable_count: int
    vacancy_proxy_pct: float = 0.0
    summary: str = Field(description="Plain language summary of the rent analysis")


class PropertyAnalysis(BaseModel):
    address: str
    gross_yield_pct: float
    rent_fairness: str
    suburb_median_price: float = 0.0
    price_vs_median_pct: float = 0.0
    summary: str = Field(description="Investment analysis summary")


class SuburbReport(BaseModel):
    suburb: str
    state: str
    median_price: float = 0.0
    mean_price: float = 0.0
    sale_count: int = 0
    days_on_market: int = 0
    auction_clearance_pct: float = 0.0
    median_yield_pct: float = 0.0
    summary: str = Field(description="Suburb market overview summary")


class ValueEstimate(BaseModel):
    address: str
    estimate_low: float
    estimate_mid: float
    estimate_high: float
    confidence: float
    comparable_count: int
    is_strata: bool = False
    capital_growth_pct: float | None = None
    summary: str = Field(
        description="Value estimate summary with confidence explanation"
    )


class PortfolioSummary(BaseModel):
    property_count: int
    total_value: float
    total_weekly_rent: float
    avg_yield_pct: float
    properties: list[dict]
    summary: str = Field(description="Portfolio performance summary")
