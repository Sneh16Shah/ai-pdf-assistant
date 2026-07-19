"""MarkItDown sidecar — converts uploaded files to Markdown.

Endpoints:
    GET  /health  -> {"status": "ok"}           (used by docker-compose healthcheck)
    POST /convert -> {"markdown", "title", "page_count"}  (multipart "file")

Non-fatal on conversion failure: returns 422 with {"error": "..."} so the Go
caller can fall back to the basic parser.
"""

import logging
import os
import tempfile

from fastapi import FastAPI, File, UploadFile, HTTPException
from fastapi.responses import JSONResponse
from markitdown import MarkItDown

logging.basicConfig(level=logging.INFO)
log = logging.getLogger("markitdown-service")

app = FastAPI(title="AskMyPDF MarkItDown Sidecar")
md = MarkItDown()


@app.get("/health")
def health():
    return {"status": "ok"}


@app.post("/convert")
async def convert(file: UploadFile = File(...)):
    if not file.filename:
        raise HTTPException(status_code=400, detail="missing filename")

    suffix = os.path.splitext(file.filename)[1]
    # Persist to a temp file with the original extension — MarkItDown dispatches
    # converters by extension, so a suffixless temp file would fail.
    with tempfile.NamedTemporaryFile(delete=False, suffix=suffix) as tmp:
        try:
            content = await file.read()
            tmp.write(content)
            tmp.flush()
            tmp_path = tmp.name
        except Exception as exc:
            log.exception("failed to buffer upload")
            return JSONResponse(status_code=500, content={"error": str(exc)})

    try:
        result = md.convert(tmp_path)
        markdown = result.text_content or ""
        title = ""
        # result.metadata is a dict-like object; guard against missing keys
        try:
            meta = result.metadata or {}
            title = meta.get("title", "") or ""
        except Exception:
            pass

        if not markdown.strip():
            return JSONResponse(
                status_code=422,
                content={"error": "no text could be extracted"},
            )

        return {
            "markdown": markdown,
            "title": title,
            "filename": file.filename,
        }
    except Exception as exc:
        log.exception("markitdown conversion failed for %s", file.filename)
        return JSONResponse(status_code=422, content={"error": str(exc)})
    finally:
        try:
            os.unlink(tmp_path)
        except OSError:
            pass
