"""OpenTelemetry tracing for the GraphRAG worker.

Mirrors the conventions of the other services:

- Enabled ONLY when OTEL_EXPORTER_OTLP_ENDPOINT is set; otherwise every
  function here is a safe no-op.
- OTLP HTTP span export (BatchSpanProcessor); the exporter reads
  OTEL_EXPORTER_OTLP_ENDPOINT itself and appends /v1/traces.
- Service name defaults to "graphrag-service", overridable with
  OTEL_SERVICE_NAME.
- W3C tracecontext propagation (parent context extracted from AMQP headers).

The opentelemetry packages are OPTIONAL runtime deps (requirements.txt): all
imports are guarded so this module works -- as a no-op -- when they are not
installed, keeping the light install profile viable.
"""

import logging
import os
from contextlib import contextmanager
from typing import Any, Dict, Iterator, Optional

logger = logging.getLogger(__name__)

_initialized = False
_otel_available = True

try:  # Guarded: opentelemetry is an optional dependency.
    from opentelemetry import trace
    from opentelemetry.propagate import set_global_textmap
    from opentelemetry.trace import SpanKind
    from opentelemetry.trace.propagation.tracecontext import (
        TraceContextTextMapPropagator,
    )
except ImportError:  # pragma: no cover - exercised when otel isn't installed
    _otel_available = False
    trace = None  # type: ignore[assignment]


def init_tracing(service_name: str = "graphrag-service") -> bool:
    """Initialize tracing. Returns True if tracing is active.

    No-op (returns False) when OTEL_EXPORTER_OTLP_ENDPOINT is unset or the
    opentelemetry packages are not importable.
    """
    global _initialized

    if _initialized:
        return True

    endpoint = os.environ.get("OTEL_EXPORTER_OTLP_ENDPOINT")
    if not endpoint:
        logger.info("Tracing disabled (OTEL_EXPORTER_OTLP_ENDPOINT not set)")
        return False

    if not _otel_available:
        logger.warning(
            "OTEL_EXPORTER_OTLP_ENDPOINT is set but the opentelemetry "
            "packages are not installed; tracing disabled"
        )
        return False

    try:
        # SDK imports are separate from the API imports above so a partial
        # install (api without sdk/exporter) still degrades gracefully.
        from opentelemetry.exporter.otlp.proto.http.trace_exporter import (
            OTLPSpanExporter,
        )
        from opentelemetry.sdk.resources import Resource
        from opentelemetry.sdk.trace import TracerProvider
        from opentelemetry.sdk.trace.export import BatchSpanProcessor
    except ImportError:
        logger.warning(
            "OTEL_EXPORTER_OTLP_ENDPOINT is set but the opentelemetry SDK/"
            "exporter packages are not installed; tracing disabled"
        )
        return False

    resource = Resource.create(
        {"service.name": os.environ.get("OTEL_SERVICE_NAME", service_name)}
    )
    provider = TracerProvider(resource=resource)
    provider.add_span_processor(BatchSpanProcessor(OTLPSpanExporter()))
    trace.set_tracer_provider(provider)
    set_global_textmap(TraceContextTextMapPropagator())

    _initialized = True
    logger.info(
        "Tracing initialized", extra={"otlp_endpoint": endpoint}
    )
    return True


def shutdown_tracing() -> None:
    """Flush and shut down the tracer provider (no-op when disabled)."""
    global _initialized

    if not _initialized or not _otel_available:
        return
    provider = trace.get_tracer_provider()
    shutdown = getattr(provider, "shutdown", None)
    if callable(shutdown):
        shutdown()
    _initialized = False


def _headers_to_carrier(headers: Optional[Dict[str, Any]]) -> Dict[str, str]:
    """Coerce AMQP headers (values may be bytes) into a str->str carrier."""
    carrier: Dict[str, str] = {}
    for key, value in (headers or {}).items():
        if isinstance(value, bytes):
            value = value.decode("utf-8", errors="replace")
        carrier[str(key)] = str(value)
    return carrier


@contextmanager
def consumer_span(
    name: str,
    headers: Optional[Dict[str, Any]] = None,
    attributes: Optional[Dict[str, Any]] = None,
) -> Iterator[Any]:
    """Start a CONSUMER span with parent context extracted from AMQP headers.

    Yields the active span (or None when tracing is disabled/unavailable).
    Exceptions raised inside the block are recorded on the span, the span
    status is set to error, and the exception is re-raised.
    """
    if not _initialized or not _otel_available:
        yield None
        return

    parent_ctx = TraceContextTextMapPropagator().extract(
        carrier=_headers_to_carrier(headers)
    )
    tracer = trace.get_tracer("graphrag-service")
    clean_attributes = {
        key: value
        for key, value in (attributes or {}).items()
        if value is not None
    }
    with tracer.start_as_current_span(
        name,
        context=parent_ctx,
        kind=SpanKind.CONSUMER,
        attributes=clean_attributes,
        record_exception=True,
        set_status_on_exception=True,
    ) as span:
        yield span


class TraceContextLogFilter(logging.Filter):
    """Inject trace_id/span_id into log records when a span is active.

    Attach to the root JSON handler (cmd/main.py); python-json-logger emits
    any extra record attributes, so trace_id/span_id show up in the JSON
    output only when set. Safe when opentelemetry is not installed.
    """

    def filter(self, record: logging.LogRecord) -> bool:
        if _otel_available:
            span = trace.get_current_span()
            ctx = span.get_span_context()
            if ctx.is_valid:
                record.trace_id = format(ctx.trace_id, "032x")
                record.span_id = format(ctx.span_id, "016x")
        return True
