# scrib — Meeting Audio Capture

## Architecture

**Client** (macOS only):
- `audio/` — ScreenCaptureKit + CoreAudio recording (Obj-C bridge via cgo)
- `tui/` — Bubble Tea TUI for recording UI
- `main.go` — CLI entry point (cobra). Bare `scrib` = start recording
- `config/` — TOML config (`~/.config/scrib/config.toml`)

**Server** (`cmd/scrib-server/`):
- `server/server.go` — HTTP server, Postgres, S3, migrations
- `server/audio.go` — Transcription + VAD HTTP clients (mlx-audio backend)
- `server/process.go` — Processing pipeline: transcribe → VAD → align → store segments

## Key Design Decisions

- Client is a thin recorder. Server does all processing (transcription, diarisation, summarization)
- On stop: convert stereo→mono, POST to server, trigger processing
- WAV uploaded directly via HTTP. Server stores in S3 (Garage)
- No local database or sync protocol — just record + upload
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
server_url = "https://scrib.goyangi.io"
sample_rate = 16000
output_dir = "~/meetings"
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

- `scrib [name]` / `scrib record [name]` — Record + upload (default)
- `scrib standup [name]` — Record with standup template
- `scrib standup3 [name]` — Record with standup3 template
- `scrib status <uuid>` — Show meeting status + segments
- `scrib list` — List recent meetings from server
- `scrib show <uuid>` — Show full transcript

## Upload Flow

1. `POST /v1/meetings` — create meeting, get UUID
2. `POST /v1/audio/{uuid}` — upload mono WAV
3. `POST /v1/process/{uuid}` — trigger server-side processing
