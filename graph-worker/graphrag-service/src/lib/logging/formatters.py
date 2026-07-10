"""
Custom Log Formatters.

Part of the CORE libraries - logging library.
Provides structured (JSON) logging and custom formatters.
"""

import json
import logging
from datetime import datetime
from typing import Any, Dict


class JSONFormatter(logging.Formatter):
    """JSON formatter for structured logging.

    Converts log records to JSON for easier parsing and analysis.
    """

    def __init__(self, include_extra: bool = False):
        """Initialize JSON formatter.

        Args:
            include_extra: Include extra fields from LogRecord (default: False)
        """
        super().__init__()
        self.include_extra = include_extra

    def format(self, record: logging.LogRecord) -> str:
        """Format log record as JSON.

        Args:
            record: Log record to format

        Returns:
            JSON string
        """
        log_data: Dict[str, Any] = {
            "timestamp": datetime.fromtimestamp(record.created).isoformat(),
            "level": record.levelname,
            "logger": record.name,
            "message": record.getMessage(),
            "module": record.module,
            "function": record.funcName,
            "line": record.lineno,
        }

        # Add exception info if present
        if record.exc_info:
            log_data["exception"] = self.formatException(record.exc_info)

        # Add extra context if requested
        if self.include_extra and hasattr(record, "extra_context"):
            log_data["context"] = record.extra_context

        # Add any custom fields from context
        if hasattr(record, "request_id"):
            log_data["request_id"] = record.request_id
        if hasattr(record, "session_id"):
            log_data["session_id"] = record.session_id
        if hasattr(record, "trace_id"):
            log_data["trace_id"] = record.trace_id

        return json.dumps(log_data, ensure_ascii=False)


class ColoredFormatter(logging.Formatter):
    """Colored console formatter for better readability.

    TODO: Enhance with more sophisticated coloring and styling.
    """

    # ANSI color codes
    COLORS = {
        "DEBUG": "\033[36m",  # Cyan
        "INFO": "\033[32m",  # Green
        "WARNING": "\033[33m",  # Yellow
        "ERROR": "\033[31m",  # Red
        "CRITICAL": "\033[35m",  # Magenta
    }
    RESET = "\033[0m"

    def format(self, record: logging.LogRecord) -> str:
        """Format log record with colors.

        Args:
            record: Log record to format

        Returns:
            Colored log string
        """
        color = self.COLORS.get(record.levelname, self.RESET)
        record.levelname = f"{color}{record.levelname}{self.RESET}"
        return super().format(record)


class CompactFormatter(logging.Formatter):
    """Compact formatter for CLI applications.

    Minimal format for user-facing CLI logs.
    """

    def __init__(self):
        """Initialize compact formatter."""
        super().__init__(fmt="%(levelname)s: %(message)s", datefmt="%H:%M:%S")
