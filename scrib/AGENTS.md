# scrib — Meeting Audio Capture & Annotation

## Architecture

**Client** (macOS only):
- `audio/` — ScreenCaptureKit + CoreAudio recording (Obj-C bridge via cgo)
- `tui/` — Bubble Tea TUI for recording UI
- `main.go` — CLI entry point (cobra). Bare `scrib` = start recording
- `config/` — TOML config (`~/.config/scrib/config.toml`)
- `store/` — SQLite local database
- `sync/` — Upload WAV to S3, sync metadata with server
- `client/` — HTTP client for annotation API (STT, VAD, summarize)

**Server** (`cmd/scrib-server/`):
- `server/server.go` — HTTP server, Postgres, S3, migrations
- `server/audio.go` — Transcription + VAD HTTP clients (mlx-audio backend)
- `server/process.go` — Processing pipeline: transcribe → VAD → align → store segments

## Key Design Decisions

- Client is dumb recorder. Server does all processing (transcription, diarisation)
- WAV uploaded to S3 (Garage). Server pulls from S3 for processing
- No streaming STT for v1 — batch upload of complete WAV
- Summarization handled by Hawow (AI assistant), not scrib — has full team/project context
- `audio/` requires macOS (ScreenCaptureKit/CoreAudio) — cannot cross-compile or build on Linux

## Build

```bash
# Client (macOS only — needs cgo + macOS frameworks)
go build -o scrib .

# Server (cross-platform)
bazel build //scrib:scrib-server
# or: go build -o scrib-server ./cmd/scrib-server

# OCI image (server only, distroless)
bazel run //scrib:push_server
```

## Config

`~/.config/scrib/config.toml`:
```toml
[database]
path = "~/.local/share/scrib/scrib.db"

[sync]
server_url = "https://scrib.goyangi.io"

[summarise]
api_url = "https://gateway.goyangi.io/v1/opus"
```

## Test

```bash
go test ./...
```

## CI

- `.github/workflows/scrib.yaml` — Bazel build on PR, push OCI on main
- Only builds server packages (audio/ needs macOS)
- Image: `ghcr.io/solanyn/scrib-server`

## CLI Commands

- `scrib` / `scrib record` — Start recording (default)
- `scrib annotate <file>` — Diarise + summarise a recording
- `scrib transcribe <uuid>` — Trigger server-side processing, poll + print transcript
- `scrib history` — List meetings
- `scrib show <id>` — Show meeting details + transcript
- `scrib search <query>` — Full-text search transcripts
- `scrib speakers` — List known speakers
- `scrib sync` — Sync to/from server
