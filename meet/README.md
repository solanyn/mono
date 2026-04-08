# meet — Meeting Audio Capture & Annotation

CLI tool for recording meeting audio and generating diarised, summarised notes.

## Usage

```bash
# Record system audio + mic (stereo WAV)
meet record standup
# → Recording... press Ctrl+C to stop
# → Saved to ~/meetings/2026-04-09-standup.wav

# Record and auto-annotate on stop
meet record standup --annotate

# Annotate an existing recording
meet annotate ~/meetings/2026-04-09-standup.wav
# → VAD + STT (concurrent) → merge → summarise → markdown

# List recordings
meet list
```

## Pipeline

```
audio.wav
  ├─ POST :8000/v1/audio/vad             → speaker segments (Sortformer)
  ├─ POST :8000/v1/audio/transcriptions   → transcript + timestamps (Parakeet)
  └─ merge in Go (align words to speakers by time overlap)
       └─ POST :8001/v1/chat/completions  → summary (Opus/Haiku/Gemma 4)
            └─ output: ~/meetings/name.md
```

VAD and STT run concurrently.

## Audio Capture

- **System audio**: ScreenCaptureKit (macOS 13+, no virtual devices needed)
- **Microphone**: CoreAudio AudioQueue
- **Output**: 16kHz stereo WAV (L=mic, R=system)

## Config

`~/.config/meet/config.toml`:
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
- mlx-audio server on localhost:8000 (with VAD overlay for Sortformer)
- kgateway on localhost:8001 (or gateway.goyangi.io)

## Build

```bash
# With Bazel
bazel build //meet

# With Go
cd meet && go build -o meet .
```
