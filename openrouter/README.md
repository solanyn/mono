# openrouter-credits-exporter

Small Prometheus exporter for the OpenRouter account credit balance.

It polls `GET https://openrouter.ai/api/v1/credits` on a fixed interval, caches the
last successful sample, and serves the cached sample plus its freshness on
`/metrics`. Failed polls never clear the last good sample, so a scrape cannot
silently read a zero balance.

## Requirements

The credits endpoint requires an OpenRouter **management** API key, not a regular
inference key.

- endpoint docs: https://openrouter.ai/docs/api/api-reference/credits/get-remaining-credits
- management key docs: https://openrouter.ai/docs/guides/overview/auth/management-api-keys

## Configuration

All configuration is environment-driven. The key is read from the environment or,
preferably, from a mounted secret file.

| Variable | Default | Notes |
| --- | --- | --- |
| `OPENROUTER_MANAGEMENT_KEY_FILE` | – | Path to a file containing the management key. Takes precedence over the env var. |
| `OPENROUTER_MANAGEMENT_KEY` | – | Management key value. Used only when no secret file is set. |
| `OPENROUTER_EXPORTER_ADDR` | `:9101` | Listen address. |
| `OPENROUTER_API_URL` | `https://openrouter.ai/api/v1/credits` | Override for tests/proxies. |
| `OPENROUTER_EXPORTER_INTERVAL` | `5m` | Poll interval. Minimum `1m`. |
| `OPENROUTER_EXPORTER_TIMEOUT` | `10s` | Per-request timeout. Must be `> 0` and `<= 60s`. |
| `OPENROUTER_EXPORTER_STALE_AFTER` | `3x interval` | Age after which the cached sample is considered stale. Must be `>= timeout`. |

## Metrics

| Metric | Type | Meaning |
| --- | --- | --- |
| `openrouter_credits_total_usd` | gauge | Total credits purchased on the account, USD. |
| `openrouter_credits_used_usd` | gauge | Total credits consumed on the account, USD. |
| `openrouter_credits_remaining_usd` | gauge | `total_credits - total_usage`, USD. |
| `openrouter_exporter_up` | gauge | `1` when the most recent poll succeeded, `0` otherwise. |
| `openrouter_exporter_last_success_timestamp_seconds` | gauge | Unix time of the last successful poll, `0` if never. |
| `openrouter_exporter_last_attempt_timestamp_seconds` | gauge | Unix time of the last poll attempt, `0` if none yet. |
| `openrouter_exporter_poll_duration_seconds` | gauge | Duration of the most recent poll attempt. |
| `openrouter_exporter_sample_age_seconds` | gauge | Age of the cached sample. Absent until the first success. |
| `openrouter_exporter_polls_total{outcome}` | counter | Polls by `outcome` (`success`, `failure`). |
| `openrouter_exporter_errors_total{category}` | counter | Failed polls by bounded category (`auth`, `rate_limited`, `http_status`, `decode`, `network`, `config`). |

The three balance gauges are absent until a poll has succeeded, so a missing
series is distinct from a zero balance. The only label values are a fixed,
bounded set: no account identifiers, no key material, no raw response text.

## Health

`GET /health` returns JSON and `200` when the cached sample is fresh, otherwise
`503`.

```json
{"status":"ok","up":true,"sample_age_seconds":42.1,"last_success":"2026-11-01T00:00:00Z","poll_interval_seconds":300,"stale_after_seconds":900}
```

`status` is one of `ok`, `stale`, `failing`, `never_succeeded`. The response
contains no key material and no API response body.

## Security notes

- The management key is read from `OPENROUTER_MANAGEMENT_KEY_FILE` or
  `OPENROUTER_MANAGEMENT_KEY` only. There is no CLI flag for secrets, so a key
  cannot leak through the process list. Prefer the file form with a mounted
  secret volume.
- The key is never logged, never exported as a metric or label, and never
  returned by `/health`.
- Log lines and HTTP responses are passed through a redactor that replaces the
  configured key with `[REDACTED]`, as defence in depth.
- Redirects are not followed, so the authorization header cannot be replayed
  against an unexpected host.
- Response bodies are read through a 1 MiB limit.
- Polls are synchronous and single-attempt: runs never overlap and there is no
  retry storm against the management API.
- The exporter is read-only. It makes exactly one GET per interval and never
  mutates the OpenRouter account.

## Local development

```sh
cd openrouter
go test ./...        # unit + httptest integration tests, no network access
go vet ./...
go run .             # requires OPENROUTER_MANAGEMENT_KEY(_FILE)
curl -s localhost:9101/health
curl -s localhost:9101/metrics | grep openrouter_
```

## Bazel

```sh
bazel build //openrouter:openrouter
bazel test //openrouter:openrouter_test
```
