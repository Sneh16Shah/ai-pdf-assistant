FROM python:3.12-slim

WORKDIR /app

# System deps for some markitdown converters (pdfminer needs no extra system libs,
# but install curl for a richer healthcheck if needed later)
RUN apt-get update && apt-get install -y --no-install-recommends \
    wget \
    && rm -rf /var/lib/apt/lists/*

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY app.py .

EXPOSE 8000

CMD ["uvicorn", "app:app", "--host", "0.0.0.0", "--port", "8000"]
