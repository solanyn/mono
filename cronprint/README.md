# cronprint

Scheduled document printing service. Runs cron jobs that send PDFs to a CUPS printer via IPP.

```mermaid
graph LR
    cron["Cron Scheduler"] --> render["Render PDF"]
    render --> cups["CUPS/IPP Printer"]
    api["HTTP API :8080"] --> cron
```

## Run

```bash
bazel run //cronprint:app
```

## Config

Env vars: `CRONPRINT_HOST`, `CRONPRINT_PORT`, `CRONPRINT_TIMEZONE`, `CRONPRINT_JOB_DAILY_SCHEDULE`, `CRONPRINT_JOB_DAILY_FILE`
