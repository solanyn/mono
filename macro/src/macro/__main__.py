import asyncio
import sys

from macro.agent import macro_agent


async def main():
    print("Macro-financial reasoning agent. Type 'quit' to exit.\n")
    async with macro_agent:
        while True:
            try:
                query = input("> ")
            except (EOFError, KeyboardInterrupt):
                break
            if query.strip().lower() in ("quit", "exit", "q"):
                break
            if not query.strip():
                continue
            result = await macro_agent.run(query)
            print(f"\n{result.output}\n")


if __name__ == "__main__":
    asyncio.run(main())
