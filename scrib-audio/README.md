# scrib-audio

ML audio pipeline for [scrib](../scrib/) meeting recordings. Runs on Apple Silicon (macOS only).

## Architecture

Three-stage pipeline: **diarize → transcribe → align**

| Stage | Model | Backend |
|-------|-------|---------|
| Diarization | [Senko](https://github.com/narcotic-sh/senko) (CAM++ embeddings + spectral clustering) | CoreML/ANE |
| Transcription | [Parakeet TDT 0.6B](https://huggingface.co/mlx-community/parakeet-tdt-0.6b-v3) | MLX |
| Alignment | Word-to-speaker temporal overlap | CPU |

Performance: ~21 seconds for 9 minutes of audio on M4 Mac Mini.

## API

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/audio/process` | POST | Full pipeline: diarize + transcribe + align |
| `/v1/audio/diarize` | POST | Speaker diarization only |
| `/v1/audio/transcribe` | POST | Transcription only |
| `/health` | GET | Health check |

All audio endpoints accept multipart form upload with a `file` field.

## Development

### Prerequisites

- macOS 14+ on Apple Silicon
- [uv](https://docs.astral.sh/uv/)
- [Bazel](https://bazel.build/) (via bazelisk)

### Run locally

```bash
# Using uv (recommended)
cd scrib-audio
uv run python -m scrib_audio.server --host 127.0.0.1 --port 8002 --workers 1

# Or create a venv via Bazel
bazel run //scrib-audio:create_venv
source scrib-audio/venv/bin/activate
python -m scrib_audio.server --host 127.0.0.1 --port 8002 --workers 1
```

### Update dependencies

```bash
# Regenerate lock file from pyproject.toml
bazel run //scrib-audio:generate_requirements_txt

# Or manually with uv
uv pip compile pyproject.toml --python-version 3.12 --python-platform aarch64-apple-darwin -o requirements_lock.txt
```

### Tests

```bash
bazel test //scrib-audio:test_align
```

### Build

```bash
bazel build //scrib-audio:scrib-audio
```

## Deployment

Runs as a launchd agent on Mac Mini (`com.scrib.audio`), managed by nix-darwin.

```nix
# In nix-darwin hosts/personal.nix
launchd.user.agents.scrib-audio = {
  command = "/opt/homebrew/bin/uv run --project /Users/andrew/git/mono/scrib-audio python -m scrib_audio.server --host 0.0.0.0 --port 8002 --workers 1";
  ...
};
```

The server preloads Senko (diarization) and Parakeet (STT) models on startup.
First request after cold start may be slower while CoreML compiles the models.

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SCRIB_AUDIO_STT_MODEL` | `mlx-community/parakeet-tdt-0.6b-v3` | HuggingFace STT model |
| `HF_HOME` | `~/.cache/huggingface` | HuggingFace model cache directory |
