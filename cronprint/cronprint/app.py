#!/usr/bin/env python3

import logging
from contextlib import asynccontextmanager

import uvicorn
from fastapi import FastAPI

from config import load_config
from health import setup_health_endpoints
from scheduler import PrintScheduler

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


@asynccontextmanager
async def lifespan(app: FastAPI):
    config = load_config()
    scheduler = PrintScheduler(config)

    logger.info("Starting cronprint scheduler...")
    scheduler.start()
    app.state.scheduler = scheduler

    yield

    logger.info("Shutting down cronprint scheduler...")
    scheduler.shutdown()


def create_app() -> FastAPI:
    app = FastAPI(
        title="cronprint",
        description="Scheduled printing service with CUPS integration",
        version="1.0.0",
        lifespan=lifespan,
    )

    setup_health_endpoints(app)

    return app


def main():
    config = load_config()
    app = create_app()

    uvicorn.run(
        app,
        host=config["server"]["host"],
        port=config["server"]["port"],
        log_level="info",
    )


if __name__ == "__main__":
    main()
