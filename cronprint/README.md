# cronprint - Scheduled Printing Service

A Python service for scheduled printing using CUPS with health monitoring.

## Testing the Service

### 1. Setup Virtual Environment

```bash
# Set up Python virtual environment with all dependencies
bazelisk run //cronprint:venv

# Activate the environment
cd cronprint
source .venv/bin/activate
```

### 2. Basic Testing

#### Start the service with defaults

```bash
cd cronprint
python main.py
```

The service will start on `http://0.0.0.0:8080` with no scheduled jobs.

#### Test health endpoint

```bash
curl http://localhost:8080/healthz
# Expected: {"status":"healthy","service":"cronprint"}
```

### 3. Environment Variable Configuration

#### Configure server and jobs via environment variables

```bash
export CRONPRINT_HOST=0.0.0.0
export CRONPRINT_PORT=8081
export CRONPRINT_TIMEZONE=America/New_York
export CRONPRINT_DEFAULT_PRINTER=my_printer

# Define a test job that runs every hour
export CRONPRINT_JOB_HOURLY_SCHEDULE="0 * * * *"
export CRONPRINT_JOB_HOURLY_FILE="/tmp/hourly_report.pdf"
export CRONPRINT_JOB_HOURLY_PRINTER="office_printer"

# Define a daily job (9 AM New York time)
export CRONPRINT_JOB_DAILY_SCHEDULE="0 9 * * MON-FRI"
export CRONPRINT_JOB_DAILY_FILE="/tmp/daily_summary.pdf"
export CRONPRINT_JOB_DAILY_ENABLED=true

python main.py
```

#### Test custom configuration

```bash
curl http://localhost:8081/healthz
```

### 4. Quick Test with One Command

```bash
# Test with environment variables in one command
CRONPRINT_PORT=8082 \
CRONPRINT_TIMEZONE=Europe/London \
CRONPRINT_JOB_TEST_SCHEDULE="*/5 * * * *" \
CRONPRINT_JOB_TEST_FILE="/tmp/test.pdf" \
python main.py
```

### 5. Expected Log Output

When starting successfully, you should see:

```
INFO:     Started server process [PID]
INFO:     Waiting for application startup.
INFO:printer:Connected to CUPS server
INFO:scheduler:Scheduled print job 'test' with schedule '*/5 * * * *'
INFO:__main__:Starting cronprint scheduler...
INFO:scheduler:Print scheduler started
INFO:     Application startup complete.
INFO:     Uvicorn running on http://0.0.0.0:8082 (Press CTRL+C to quit)
```

### 6. Testing Print Jobs

To test actual printing:

1. Ensure CUPS is running on your system
2. Create a test PDF file: `echo "Test" | ps2pdf - /tmp/test.pdf`
3. Set up a job that runs soon: `CRONPRINT_JOB_TEST_SCHEDULE="* * * * *"`
4. Watch the logs for execution

### 7. Health Check Details

The `/healthz` endpoint checks:

- ✅ Service is running
- ✅ Scheduler is active
- ✅ CUPS connection (or simulator fallback)

Returns:

- `200 OK` with `{"status":"healthy","service":"cronprint"}` when healthy
- `503 Service Unavailable` when unhealthy

## Environment Variables Reference

| Variable                        | Description                 | Example            |
| ------------------------------- | --------------------------- | ------------------ |
| `CRONPRINT_HOST`                | Server bind address         | `0.0.0.0`          |
| `CRONPRINT_PORT`                | Server port                 | `8080`             |
| `CRONPRINT_TIMEZONE`            | Timezone for scheduled jobs | `America/New_York` |
| `CRONPRINT_DEFAULT_PRINTER`     | Default printer name        | `office_printer`   |
| `CRONPRINT_JOB_<NAME>_SCHEDULE` | Cron schedule               | `0 9 * * MON-FRI`  |
| `CRONPRINT_JOB_<NAME>_FILE`     | File path to print          | `/tmp/report.pdf`  |
| `CRONPRINT_JOB_<NAME>_PRINTER`  | Specific printer            | `color_printer`    |
| `CRONPRINT_JOB_<NAME>_ENABLED`  | Enable/disable job          | `true`             |

## Cron Schedule Format

```
* * * * *
│ │ │ │ │
│ │ │ │ └── Day of week (0-6, Sunday=0)
│ │ │ └──── Month (1-12)
│ │ └────── Day of month (1-31)
│ └──────── Hour (0-23)
└────────── Minute (0-59)
```

Examples:

- `0 9 * * MON-FRI` - 9 AM on weekdays
- `*/15 * * * *` - Every 15 minutes
- `0 0 1 * *` - First day of each month at midnight

## Timezone Support

Cronprint supports timezone-aware scheduling using the `CRONPRINT_TIMEZONE` environment variable:

```bash
# Jobs will run in New York time
export CRONPRINT_TIMEZONE=America/New_York
export CRONPRINT_JOB_MORNING_SCHEDULE="0 9 * * MON-FRI"  # 9 AM ET

# Jobs will run in London time
export CRONPRINT_TIMEZONE=Europe/London
export CRONPRINT_JOB_AFTERNOON_SCHEDULE="0 14 * * *"     # 2 PM GMT

# Jobs will run in Tokyo time
export CRONPRINT_TIMEZONE=Asia/Tokyo
export CRONPRINT_JOB_EVENING_SCHEDULE="0 18 * * *"       # 6 PM JST
```

**Default:** `UTC` if not specified

**Valid timezones:** Any [IANA timezone identifier](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones) (e.g., `America/Chicago`, `Europe/Paris`, `Asia/Shanghai`)

