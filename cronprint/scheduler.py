import logging
from typing import Dict, Any, List
from apscheduler.schedulers.background import BackgroundScheduler
from apscheduler.triggers.cron import CronTrigger

from printer import PrintManager

logger = logging.getLogger(__name__)


class PrintScheduler:
    """Manages scheduled print jobs using APScheduler."""

    def __init__(self, config: Dict[str, Any]):
        self.config = config
        self.scheduler = BackgroundScheduler()
        self.print_manager = PrintManager(
            default_printer=config.get("printing", {}).get("default_printer")
        )
        self.timezone = config.get("timezone", "UTC")
        self._setup_jobs()

    def _setup_jobs(self):
        """Set up scheduled print jobs from configuration."""
        jobs = self.config.get("printing", {}).get("jobs", [])

        for job_config in jobs:
            if not job_config.get("enabled", True):
                logger.info(
                    f"Skipping disabled job: {job_config.get('name', 'unnamed')}"
                )
                continue

            self._add_print_job(job_config)

    def _add_print_job(self, job_config: Dict[str, Any]):
        """Add a single print job to the scheduler."""
        job_name = job_config.get("name", "unnamed_job")
        schedule = job_config.get("schedule")
        file_path = job_config.get("file_path")
        printer = job_config.get("printer")

        if not schedule or not file_path:
            logger.error(
                f"Invalid job configuration for {job_name}: missing schedule or file_path"
            )
            return

        try:
            # Parse cron expression
            cron_parts = schedule.split()
            if len(cron_parts) != 5:
                logger.error(f"Invalid cron expression for {job_name}: {schedule}")
                return

            minute, hour, day, month, day_of_week = cron_parts

            trigger = CronTrigger(
                minute=minute,
                hour=hour,
                day=day,
                month=month,
                day_of_week=day_of_week,
                timezone=self.timezone,
            )

            self.scheduler.add_job(
                func=self._execute_print_job,
                trigger=trigger,
                args=[job_name, file_path, printer],
                id=job_name,
                name=f"Print job: {job_name}",
                misfire_grace_time=300,  # 5 minutes grace time
            )

            logger.info(f"Scheduled print job '{job_name}' with schedule '{schedule}'")

        except Exception as e:
            logger.error(f"Failed to schedule job {job_name}: {e}")

    def _execute_print_job(self, job_name: str, file_path: str, printer: str = None):
        """Execute a scheduled print job."""
        logger.info(f"Executing print job: {job_name}")

        try:
            success = self.print_manager.print_file(
                file_path=file_path, printer_name=printer, job_name=job_name
            )

            if success:
                logger.info(f"Print job '{job_name}' completed successfully")
            else:
                logger.error(f"Print job '{job_name}' failed")

        except Exception as e:
            logger.error(f"Error executing print job '{job_name}': {e}")

    def start(self):
        """Start the scheduler."""
        if not self.scheduler.running:
            self.scheduler.start()
            logger.info("Print scheduler started")

    def shutdown(self):
        """Shutdown the scheduler."""
        if self.scheduler.running:
            self.scheduler.shutdown()
            logger.info("Print scheduler stopped")

    def get_jobs(self) -> List[Dict[str, Any]]:
        """Get information about scheduled jobs."""
        jobs = []
        for job in self.scheduler.get_jobs():
            jobs.append(
                {
                    "id": job.id,
                    "name": job.name,
                    "next_run": (
                        job.next_run_time.isoformat() if job.next_run_time else None
                    ),
                    "trigger": str(job.trigger),
                }
            )
        return jobs

    def is_healthy(self) -> bool:
        """Check if scheduler is healthy."""
        return self.scheduler.running

