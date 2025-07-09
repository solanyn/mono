import os
from typing import Dict, Any, List, Optional


def load_config() -> Dict[str, Any]:
    """Load configuration from environment variables."""
    config = {
        "server": {
            "host": os.getenv("CRONPRINT_HOST", "0.0.0.0"),
            "port": int(os.getenv("CRONPRINT_PORT", "8080")),
        },
        "printing": {
            "default_printer": os.getenv("CRONPRINT_DEFAULT_PRINTER"),
            "add_printer": _parse_printer_config_from_env(),
            "jobs": _parse_job_schedules_from_env(),
        },
        "timezone": os.getenv("CRONPRINT_TIMEZONE", "UTC"),
    }

    return config


def _parse_job_schedules_from_env() -> List[Dict[str, Any]]:
    """Parse job schedules from environment variables.

    Format: CRONPRINT_JOB_<NAME>_SCHEDULE, CRONPRINT_JOB_<NAME>_FILE, etc.
    """
    jobs = []
    job_names = set()

    # Find all job names from environment variables
    for key in os.environ:
        if key.startswith("CRONPRINT_JOB_") and key.endswith("_SCHEDULE"):
            job_name = key[len("CRONPRINT_JOB_") : -len("_SCHEDULE")].lower()
            job_names.add(job_name)

    # Build job configurations
    for job_name in job_names:
        prefix = f"CRONPRINT_JOB_{job_name.upper()}"

        schedule = os.getenv(f"{prefix}_SCHEDULE")
        file_path = os.getenv(f"{prefix}_FILE")

        if schedule and file_path:
            job = {
                "name": job_name,
                "schedule": schedule,
                "file_path": file_path,
                "printer": os.getenv(f"{prefix}_PRINTER"),
                "enabled": os.getenv(f"{prefix}_ENABLED", "true").lower() == "true",
            }
            jobs.append(job)

    return jobs


def _parse_printer_config_from_env() -> Optional[Dict[str, str]]:
    """Parse printer configuration from environment variables.

    Format: CRONPRINT_PRINTER_NAME, CRONPRINT_PRINTER_URI (driver optional for IPP)
    """
    name = os.getenv("CRONPRINT_PRINTER_NAME")
    uri = os.getenv("CRONPRINT_PRINTER_URI")

    if name and uri:
        return {
            "name": name,
            "uri": uri,
            "driver": os.getenv("CRONPRINT_PRINTER_DRIVER"),
            "description": os.getenv("CRONPRINT_PRINTER_DESCRIPTION", ""),
            "location": os.getenv("CRONPRINT_PRINTER_LOCATION", ""),
        }

    return None
