# scrib — Meeting Audio Capture & Annotation

CLI + TUI for recording meeting audio and generating diarised, summarised notes.
Pure Go, all processing via HTTP calls to mlx-audio server and kgateway.

## Usage

```bash
# TUI mode (default) — record + live feedback + auto-process
scrib standup
scrib standup -t 1on1

# Headless recording
scrib record standup
scrib record standup --annotate    # auto-process on stop

# Post-hoc annotation
scrib annotate ~/meetings/2026-04-09-standup.wav

# Database queries
scrib history                      # list meetings
scrib show 1                       # view meeting + transcript
scrib search "gateway migration"   # FTS across all transcripts

# Speaker management
scrib speakers                     # list known speakers
scrib speakers add Andrew          # add speaker for future matching
```

## Architecture

```
scrib (Go + BubbleTea)
  │
  ├─ TUI: record → process → results (all in one)
  ├─ record: headless capture
  └─ annotate: post-hoc pipeline
       │
       ├─ POST :8000/v1/audio/vad             → Sortformer (speaker segments)
       ├─ POST :8000/v1/audio/transcriptions   → Parakeet (STT + timestamps)
       ├─ merge in Go (align words to speakers by time overlap)
       └─ POST :8001/v1/chat/completions       → LLM summary
            └─ output: markdown + SQLite
```

## Audio Capture

- **System audio**: ScreenCaptureKit (macOS 13+)
- **Microphone**: CoreAudio AudioQueue
- **Output**: 16kHz stereo WAV (L=mic, R=system)

## Storage

SQLite at `~/.local/share/scrib/scrib.db`:
- **meetings** — recording metadata
- **segments** — diarised transcript with timestamps
- **speakers** — known speakers with voiceprint embeddings
- **summaries** — generated notes (re-summarisable with different templates)
- **segments_fts** — FTS5 full-text search across all transcripts

## Config

`~/.config/scrib/config.toml`:
```toml
gateway_url = "https://gateway.goyangi.io"
audio_url = "http://localhost:8000"
output_dir = "~/meetings"
obsidian_vault = "~/vault/Meetings"

[summarise]
model = "auto"
template = "standup"
```

## Requirements

- macOS 13+ (ScreenCaptureKit)
- mlx-audio server with VAD overlay on :8000
- kgateway on :8001 (or gateway.goyangi.io)

## Build

```bash
cd scrib && go build -o ~/bin/scrib .
```
