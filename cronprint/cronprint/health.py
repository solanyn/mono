import logging
from fastapi import FastAPI, HTTPException
from fastapi.responses import JSONResponse

logger = logging.getLogger(__name__)


def setup_health_endpoints(app: FastAPI):
    """Set up health check endpoints for the FastAPI application."""

    @app.get("/healthz")
    async def health_check():
        """Health check endpoint - indicates if the application is ready to serve requests."""
        try:
            # Check if scheduler is running
            scheduler = getattr(app.state, "scheduler", None)
            if not scheduler:
                raise HTTPException(status_code=503, detail="Scheduler not initialized")

            if not scheduler.is_healthy():
                raise HTTPException(status_code=503, detail="Scheduler not running")

            # Check if print manager can connect to printers
            printers = scheduler.print_manager.get_printers()

            return JSONResponse(
                status_code=200, content={"status": "healthy", "service": "cronprint"}
            )
        except HTTPException:
            raise
        except Exception as e:
            logger.error(f"Health check failed: {e}")
            raise HTTPException(status_code=503, detail=f"Service not ready: {str(e)}")
