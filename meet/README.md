# meet

macOS CLI for capturing meeting audio (system output + microphone input).

## Usage

```bash
# Start recording — captures system audio + mic as stereo WAV
meet record [name]
# → Recording to ~/meetings/2026-04-09-standup.wav
# → Press Ctrl+C to stop...

# List recordings
meet list
```

## Config

Optional `~/.config/meet/config.toml`:

```toml
gateway_url = "https://gateway.goyangi.io"
output_dir = "~/meetings"
sample_rate = 16000
format = "wav"
```

## Architecture

- System audio captured via **ScreenCaptureKit** (macOS 13+, no virtual audio device needed)
- Microphone captured via **CoreAudio AudioQueue**
- Output: 16kHz stereo WAV — left channel = mic, right channel = system audio
- Stereo separation enables better speaker diarisation downstream

## Requirements

- macOS 13+ (Ventura)
- Screen Recording permission (for ScreenCaptureKit)
- Microphone permission

## Build

```bash
# With Bazel
bazel build //meet:meet

# With Go directly (macOS only)
cd meet && go build -o meet .
```

## Roadmap

- `meet annotate <file>` — STT + diarisation + summarisation via kgateway
- `meet record --annotate` — one-shot record then annotate
- Speaker voiceprint learning
- Obsidian vault integration
