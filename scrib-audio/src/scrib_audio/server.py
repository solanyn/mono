"""FastAPI server for scrib-audio pipeline.

Endpoints:
    POST /v1/audio/process  — full pipeline: diarize + transcribe + align
    POST /v1/audio/diarize  — diarization only
    POST /v1/audio/transcribe — transcription only
    GET  /health            — health check (verifies models loaded)
"""

import asyncio
import contextlib
import logging
import os
import tempfile
import time
from typing import AsyncIterator, Callable

from fastapi import FastAPI, File, Form, HTTPException, Request, UploadFile
from fastapi.responses import JSONResponse

from .diarize import diarize_array, load_model as load_diar_model
from .pipeline import _load_and_normalise, process, result_to_dict
from .transcribe import load_model as load_stt_model
from .transcribe import transcribe_array

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(name)s: %(message)s",
)
log = logging.getLogger(__name__)

STT_MODEL = os.environ.get(
    "SCRIB_AUDIO_STT_MODEL", "mlx-community/parakeet-tdt-0.6b-v3"
)

# Cap upload body: ~2 GiB is well above any expected meeting WAV.
MAX_UPLOAD_BYTES = int(os.environ.get("SCRIB_AUDIO_MAX_UPLOAD_BYTES", 2 * 1024 * 1024 * 1024))
# Request-level wall clock: 1 hour covers multi-hour meetings on an M4.
REQUEST_TIMEOUT_SECS = float(os.environ.get("SCRIB_AUDIO_REQUEST_TIMEOUT_SECS", 3600))

# Per-pipeline-stage semaphore. mlx-audio / senko are single-worker; serialise
# at the Python layer instead of letting uvicorn queue requests midway through
# a multi-minute inference.
_inference_sem = asyncio.Semaphore(1)

# Snapshot of model-load state for /health.
_state = {"diar_loaded": False, "stt_loaded": False}


@contextlib.asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncIterator[None]:
    log.info("preloading models...")
    load_diar_model()
    _state["diar_loaded"] = True
    load_stt_model(STT_MODEL)
    _state["stt_loaded"] = True
    log.info("models ready")
    yield
    log.info("shutting down")


app = FastAPI(title="scrib-audio", version="0.1.0", lifespan=lifespan)


@app.middleware("http")
async def size_limit(request: Request, call_next):
    cl = request.headers.get("content-length")
    if cl:
        try:
            if int(cl) > MAX_UPLOAD_BYTES:
                return JSONResponse(
                    {"error": f"upload exceeds {MAX_UPLOAD_BYTES} bytes"},
                    status_code=413,
                )
        except ValueError:
            pass
    return await call_next(request)


@app.get("/health")
async def health():
    if not (_state["diar_loaded"] and _state["stt_loaded"]):
        return JSONResponse({"status": "loading", **_state}, status_code=503)
    return {"status": "ok", **_state}


async def _save_upload(file: UploadFile) -> str:
    """Stream upload to a tempfile, enforcing MAX_UPLOAD_BYTES. Returns path."""
    tmp_fd, tmp_path = tempfile.mkstemp(suffix=".wav")
    try:
        total = 0
        with os.fdopen(tmp_fd, "wb") as out:
            while True:
                chunk = await file.read(1 << 20)
                if not chunk:
                    break
                total += len(chunk)
                if total > MAX_UPLOAD_BYTES:
                    raise HTTPException(
                        status_code=413,
                        detail=f"upload exceeds {MAX_UPLOAD_BYTES} bytes",
                    )
                out.write(chunk)
    except BaseException:
        with contextlib.suppress(OSError):
            os.unlink(tmp_path)
        raise
    return tmp_path


async def _run_with_timeout(fn: Callable, *args, **kwargs):
    """Run blocking fn in a thread, bounded by REQUEST_TIMEOUT_SECS and the semaphore."""
    async with _inference_sem:
        return await asyncio.wait_for(
            asyncio.to_thread(fn, *args, **kwargs),
            timeout=REQUEST_TIMEOUT_SECS,
        )


@app.post("/v1/audio/process")
async def process_endpoint(
    file: UploadFile = File(...),
    threshold: float = Form(0.5),
    merge_gap: float = Form(0.5),
):
    """Full pipeline: diarize + transcribe + align."""
    tmp_path = await _save_upload(file)
    t0 = time.monotonic()
    try:
        result = await _run_with_timeout(
            process, tmp_path, threshold=threshold, merge_gap=merge_gap,
        )
        log.info("process: %.1fs wall", time.monotonic() - t0)
        return JSONResponse(result_to_dict(result))
    except asyncio.TimeoutError:
        log.error("process: timeout after %.0fs", REQUEST_TIMEOUT_SECS)
        return JSONResponse({"error": "timeout"}, status_code=504)
    except Exception as e:
        log.exception("process failed")
        return JSONResponse({"error": str(e)}, status_code=500)
    finally:
        with contextlib.suppress(OSError):
            os.unlink(tmp_path)


@app.post("/v1/audio/diarize")
async def diarize_endpoint(
    file: UploadFile = File(...),
    threshold: float = Form(0.5),
    min_duration: float = Form(0.0),
    merge_gap: float = Form(0.0),
):
    """Speaker diarization only."""
    tmp_path = await _save_upload(file)
    try:
        data, sr = _load_and_normalise(tmp_path)
        result = await _run_with_timeout(
            diarize_array, data, sr,
            threshold=threshold, min_duration=min_duration, merge_gap=merge_gap,
        )
        return JSONResponse({
            "segments": [
                {"speaker": s.speaker, "start": s.start, "end": s.end}
                for s in result.segments
            ],
            "num_speakers": result.num_speakers,
            "duration_seconds": result.duration_seconds,
        })
    except asyncio.TimeoutError:
        return JSONResponse({"error": "timeout"}, status_code=504)
    except Exception as e:
        log.exception("diarize failed")
        return JSONResponse({"error": str(e)}, status_code=500)
    finally:
        with contextlib.suppress(OSError):
            os.unlink(tmp_path)


@app.post("/v1/audio/transcribe")
async def transcribe_endpoint(file: UploadFile = File(...)):
    """Transcription only."""
    tmp_path = await _save_upload(file)
    try:
        data, sr = _load_and_normalise(tmp_path)
        result = await _run_with_timeout(transcribe_array, data, sr)
        return JSONResponse({
            "text": result.text,
            "words": [
                {"word": w.word, "start": w.start, "end": w.end}
                for w in result.words
            ],
        })
    except asyncio.TimeoutError:
        return JSONResponse({"error": "timeout"}, status_code=504)
    except Exception as e:
        log.exception("transcribe failed")
        return JSONResponse({"error": str(e)}, status_code=500)
    finally:
        with contextlib.suppress(OSError):
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
