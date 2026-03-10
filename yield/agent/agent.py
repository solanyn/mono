from pydantic_ai import Agent
from .tools import (
    search_sales,
    rent_analysis,
    suburb_stats,
    value_estimate,
    school_catchment,
    portfolio_summary,
    compare_listings,
)

property_agent = Agent(
    model="openai:gpt-4o-mini",
    system_prompt="""You are a property analysis assistant for the Australian market.
    You have access to NSW and VIC property sales data, Domain listings,
    school catchment boundaries, and suburb statistics.

    Be direct and data-driven. Always cite the data source and date range.
    When giving value estimates, always include a confidence range.
    Flag limitations (e.g. "VIC data is aggregate only, individual sales from Domain cache").""",
    tools=[
        search_sales,
        rent_analysis,
        suburb_stats,
        value_estimate,
        school_catchment,
        portfolio_summary,
        compare_listings,
    ],
)
