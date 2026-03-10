from fastapi import FastAPI
from copilotkit.integrations.fastapi import add_copilotkit_endpoint
from copilotkit import CopilotKitRemoteEndpoint
from .agent import property_agent

app = FastAPI()

copilotkit = CopilotKitRemoteEndpoint(
    agents=[property_agent],
)

add_copilotkit_endpoint(app, copilotkit, "/copilotkit")


@app.get("/health")
async def health():
    return {"status": "ok"}
