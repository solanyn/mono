# cronprint

Scheduled printing service using CUPS with health monitoring.

## Features

- Cron-style scheduling with timezone support
- CUPS integration for print job management
- Health monitoring endpoint
- Environment variable configuration

## Development

```bash
# Setup virtual environment
bazelisk run //cronprint:venv

# Run service
cd cronprint
source .venv/bin/activate
python app.py
```

## Usage

Service runs on `http://0.0.0.0:8080` with health check at `/healthz`

## Configuration

Configure via environment variables:

```bash
export CRONPRINT_HOST=0.0.0.0
export CRONPRINT_PORT=8080
export CRONPRINT_TIMEZONE=America/New_York
export CRONPRINT_JOB_DAILY_SCHEDULE="0 9 * * MON-FRI"
export CRONPRINT_JOB_DAILY_FILE="/tmp/report.pdf"
```

## Container

```bash
bazelisk build //cronprint:image
```

