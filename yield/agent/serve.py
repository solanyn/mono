import os
import uvicorn

if __name__ == "__main__":
    port = int(os.environ.get("AGENT_PORT", "8000"))
    uvicorn.run("agent.main:app", host="0.0.0.0", port=port)
