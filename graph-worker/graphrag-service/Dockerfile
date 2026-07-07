FROM python:3.12-slim AS builder

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# Core (light) dependencies are always installed. Pass
# `--build-arg INSTALL_GRAPHRAG=true` to additionally install the heavy
# GraphRAG/LLM stack (requirements-graphrag.txt) into the same image; left
# off by default so the image builds fast and stays small, with the worker
# running in stub mode (see src/worker/processor.py) until those deps and
# OPENAI_API_KEY are supplied.
ARG INSTALL_GRAPHRAG=false
COPY requirements.txt requirements-graphrag.txt ./
RUN pip install --user --no-cache-dir -r requirements.txt \
    && if [ "$INSTALL_GRAPHRAG" = "true" ]; then \
         pip install --user --no-cache-dir -r requirements-graphrag.txt; \
       fi

FROM python:3.12-slim

RUN groupadd --system app \
    && useradd --system --gid app --create-home --home-dir /home/app app

WORKDIR /app

COPY --from=builder /root/.local /home/app/.local
COPY cmd/ ./cmd/
COPY src/ ./src/
RUN chown -R app:app /app /home/app

ENV HOME=/home/app \
    PATH=/home/app/.local/bin:$PATH \
    PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1

USER app

EXPOSE 8080 8081

CMD ["python", "cmd/main.py"]
