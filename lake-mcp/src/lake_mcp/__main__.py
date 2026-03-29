import asyncio
import sys

from lake_mcp.agent import lake_agent


async def main():
    print("Macro-financial reasoning agent. Type 'quit' to exit.\n")
    async with lake_agent:
        while True:
            try:
                query = input("> ")
            except (EOFError, KeyboardInterrupt):
                break
            if query.strip().lower() in ("quit", "exit", "q"):
                break
            if not query.strip():
                continue
            result = await lake_agent.run(query)
            print(f"\n{result.output}\n")


if __name__ == "__main__":
    asyncio.run(main())
