"""
Loki-Compatible Formatter.

JSON formatter optimized for Grafana Loki with stream labels.
Part of the CORE libraries - logging library.
"""

import json
import os
import logging
from datetime import datetime
from typing import Any, Dict, Optional


class LokiFormatter(logging.Formatter):
    """JSON formatter for Grafana Loki.

    Outputs JSON with Loki-compatible structure including stream labels.

    Example output:
        {
            "timestamp": "2025-11-03T15:30:00.123456",
            "level": "ERROR",
            "message": "Processing failed",
            "stream": {
                "service": "youtuberag",
                "environment": "development",
                "component": "stages"
            },
            "logger": "stages.graph_extraction",
            "function": "handle_doc",
            "line": 145
        }
    """

    def __init__(
        self,
        service_name: str = "youtuberag",
        environment: Optional[str] = None,
        include_extra: bool = True,
    ):
        """Initialize Loki formatter.

        Args:
            service_name: Service identifier (default: "youtuberag")
            environment: Environment name (default: from ENV var or "development")
            include_extra: Include extra context fields (default: True)
        """
        super().__init__()
        self.service_name = service_name
        self.environment = environment or os.getenv("ENV", "development")
        self.include_extra = include_extra

    def format(self, record: logging.LogRecord) -> str:
        """Format log record for Loki.

        Args:
            record: Log record to format

        Returns:
            JSON string with Loki-compatible structure
        """
        # Extract component from logger name (e.g., "stages.extraction" -> "stages")
        component = record.name.split(".")[0] if "." in record.name else record.name

        log_data: Dict[str, Any] = {
            "timestamp": datetime.fromtimestamp(record.created).isoformat(),
            "level": record.levelname,
            "message": record.getMessage(),
            "stream": {
                "service": self.service_name,
                "environment": self.environment,
                "component": component,
            },
            "logger": record.name,
            "module": record.module,
            "function": record.funcName,
            "line": record.lineno,
        }

        # Add exception info if present
        if record.exc_info:
            log_data["exception"] = {
                "type": record.exc_info[0].__name__ if record.exc_info[0] else None,
                "message": str(record.exc_info[1]) if record.exc_info[1] else None,
                "traceback": self.formatException(record.exc_info),
            }

        # Add custom context if present
        if self.include_extra:
            if hasattr(record, "request_id"):
                log_data["request_id"] = record.request_id
            if hasattr(record, "session_id"):
                log_data["session_id"] = record.session_id
            if hasattr(record, "trace_id"):
                log_data["trace_id"] = record.trace_id
            if hasattr(record, "stage"):
                log_data["stream"]["stage"] = record.stage
            if hasattr(record, "agent"):
                log_data["stream"]["agent"] = record.agent

        return json.dumps(log_data, ensure_ascii=False)
