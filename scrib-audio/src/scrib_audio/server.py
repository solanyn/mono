"""FastAPI server for scrib-audio pipeline.

Endpoints:
    POST /v1/audio/process  — full pipeline: diarize + transcribe + align
    POST /v1/audio/diarize  — diarization only
    POST /v1/audio/transcribe — transcription only
    GET  /health            — health check
"""

import logging
import os
import tempfile

import soundfile as sf
from fastapi import FastAPI, File, Form, UploadFile
from fastapi.responses import JSONResponse

from .diarize import diarize, load_model as load_diar_model
from .pipeline import process, result_to_dict
from .transcribe import load_model as load_stt_model
from .transcribe import transcribe

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(name)s: %(message)s",
)
log = logging.getLogger(__name__)

app = FastAPI(title="scrib-audio", version="0.1.0")

# Preload models on startup
STT_MODEL = os.environ.get(
    "SCRIB_AUDIO_STT_MODEL", "mlx-community/parakeet-tdt-0.6b-v3"
)


@app.on_event("startup")
async def startup():
    log.info("preloading models...")
    load_diar_model()
    load_stt_model(STT_MODEL)
    log.info("models ready")


@app.get("/health")
async def health():
    return {"status": "ok"}


@app.post("/v1/audio/process")
async def process_endpoint(
    file: UploadFile = File(...),
    threshold: float = Form(0.5),
    merge_gap: float = Form(0.5),
):
    """Full pipeline: diarize + transcribe + align.

    Accepts audio file (WAV, FLAC, etc.), returns diarized transcript
    with speaker-labeled segments and word-level alignment.
    """
    with tempfile.NamedTemporaryFile(suffix=".wav", delete=False) as tmp:
        tmp.write(await file.read())
        tmp_path = tmp.name

    try:
        result = process(
            tmp_path,
            threshold=threshold,
            merge_gap=merge_gap,
        )
        return JSONResponse(result_to_dict(result))
    except Exception as e:
        log.exception("process failed")
        return JSONResponse({"error": str(e)}, status_code=500)
    finally:
        os.unlink(tmp_path)


@app.post("/v1/audio/diarize")
async def diarize_endpoint(
    file: UploadFile = File(...),
    threshold: float = Form(0.5),
    min_duration: float = Form(0.0),
    merge_gap: float = Form(0.0),
):
    """Speaker diarization only. Returns speaker segments with timestamps."""
    with tempfile.NamedTemporaryFile(suffix=".wav", delete=False) as tmp:
        tmp.write(await file.read())
        tmp_path = tmp.name

    try:
        result = diarize(
            tmp_path,
            threshold=threshold,
            min_duration=min_duration,
            merge_gap=merge_gap,
        )
        return JSONResponse({
            "segments": [
                {"speaker": s.speaker, "start": s.start, "end": s.end}
                for s in result.segments
            ],
            "num_speakers": result.num_speakers,
            "duration_seconds": result.duration_seconds,
        })
    except Exception as e:
        log.exception("diarize failed")
        return JSONResponse({"error": str(e)}, status_code=500)
    finally:
        os.unlink(tmp_path)


@app.post("/v1/audio/transcribe")
async def transcribe_endpoint(
    file: UploadFile = File(...),
):
    """Transcription only. Returns text with word-level timestamps."""
    with tempfile.NamedTemporaryFile(suffix=".wav", delete=False) as tmp:
        tmp.write(await file.read())
        tmp_path = tmp.name

    try:
        result = transcribe(tmp_path)
        return JSONResponse({
            "text": result.text,
            "words": [
                {"word": w.word, "start": w.start, "end": w.end}
                for w in result.words
            ],
        })
    except Exception as e:
        log.exception("transcribe failed")
        return JSONResponse({"error": str(e)}, status_code=500)
    finally:
        os.unlink(tmp_path)


def main():
    import argparse
    import uvicorn

    parser = argparse.ArgumentParser(description="scrib-audio server")
    parser.add_argument("--host", default="0.0.0.0")
    parser.add_argument("--port", type=int, default=8000)
    parser.add_argument("--workers", type=int, default=1)
    args = parser.parse_args()

    uvicorn.run(app, host=args.host, port=args.port, workers=args.workers)


if __name__ == "__main__":
    main()
