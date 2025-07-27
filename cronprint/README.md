# cronprint

Scheduled printing service using CUPS.

## Development

```bash
bazelisk run //cronprint:app
```

## Usage

Service runs on `http://0.0.0.0:8080`

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
