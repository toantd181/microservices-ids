FROM python:3.11-slim AS builder
WORKDIR /build

COPY requirements.txt .

RUN pip install --upgrade pip --no-cache-dir && \
    pip install --no-cache-dir -r requirements.txt --target=/build/packages

FROM python:3.11-slim AS runtime
RUN groupadd --gid 1001 mlengine && \
    useradd --uid 1001 --gid 1001 --no-create-home --shell /bin/false mlengine
WORKDIR /app

COPY --from=builder /build/packages /usr/local/lib/python3.11/site-packages/

COPY infer.py .

RUN chown -R mlengine:mlengine /app
USER mlengine

EXPOSE 8000

ENTRYPOINT [ "python", "-m", "uvicorn", "infer:app" ]
CMD ["--host", "0.0.0.0", "--port", "8000", "--workers", "1"]