"""
Logging Setup and Configuration.

Part of the CORE libraries - provides centralized logging for all domains.
Tier 1: Full implementation.
"""

import logging
import sys
from pathlib import Path
from typing import Optional, Dict, Any
from datetime import datetime


def setup_logging(
    level: int = logging.INFO,
    log_file: Optional[str] = None,
    verbose: bool = False,
    silence_third_party: bool = True,
    json_format: bool = False,
    rotate_logs: bool = True,
    max_bytes: int = 10_000_000,
    backup_count: int = 5,
) -> logging.Logger:
    """Setup application logging with console and optional file handlers.

    Args:
        level: Base logging level (default: INFO)
        log_file: Optional log file path
        verbose: If True, set level to DEBUG
        silence_third_party: If True, silence noisy third-party loggers
        json_format: If True, use JSON structured logging

    Returns:
        Root logger instance
    """
    # Set level based on verbose flag
    if verbose:
        level = logging.DEBUG

    # Get root logger
    root_logger = logging.getLogger()
    root_logger.setLevel(level)

    # Remove existing handlers to avoid duplicates
    root_logger.handlers = []

    # Choose formatter
    if json_format:
        from src.lib.logging.formatters import JSONFormatter

        console_formatter = JSONFormatter()
        file_formatter = JSONFormatter(include_extra=True)
    else:
        console_formatter = logging.Formatter(
            "%(asctime)s | %(levelname)-8s | %(name)s | %(message)s",
            datefmt="%Y-%m-%d %H:%M:%S",
        )
        file_formatter = logging.Formatter(
            "%(asctime)s | %(levelname)-8s | %(name)s | %(funcName)s:%(lineno)d | %(message)s",
            datefmt="%Y-%m-%d %H:%M:%S",
        )

    # Console handler
    console_handler = logging.StreamHandler(sys.stdout)
    console_handler.setLevel(level)
    console_handler.setFormatter(console_formatter)
    root_logger.addHandler(console_handler)

    # File handler (optional)
    if log_file:
        try:
            log_path = Path(log_file)
            log_path.parent.mkdir(parents=True, exist_ok=True)

            # Use rotating file handler for production
            if rotate_logs:
                from logging.handlers import RotatingFileHandler

                file_handler = RotatingFileHandler(
                    str(log_path),
                    maxBytes=max_bytes,
                    backupCount=backup_count,
                    encoding="utf-8",
                )
            else:
                file_handler = logging.FileHandler(log_path, encoding="utf-8")

            file_handler.setLevel(logging.DEBUG)  # Always DEBUG in file
            file_handler.setFormatter(file_formatter)
            root_logger.addHandler(file_handler)

            rotation_info = (
                f" (rotating: {max_bytes/1_000_000:.0f}MB, {backup_count} backups)"
                if rotate_logs
                else ""
            )
            root_logger.info(f"Logging to file: {log_path.absolute()}{rotation_info}")
        except Exception as e:
            root_logger.warning(f"Failed to create log file {log_file}: {e}")

    # Silence noisy third-party loggers
    if silence_third_party:
        third_party_loggers = [
            "numba",
            "graspologic",
            "pymongo",
            "urllib3",
            "openai",
            "httpcore",
        ]
        for logger_name in third_party_loggers:
            logging.getLogger(logger_name).setLevel(logging.WARNING)

        # Keep httpx at INFO to see API calls (useful for debugging)
        logging.getLogger("httpx").setLevel(logging.INFO)

    return root_logger


def get_logger(name: str, context: Optional[Dict[str, Any]] = None) -> logging.Logger:
    """Get a logger for a specific module with optional context.

    Args:
        name: Logger name (typically __name__ or class name)
        context: Optional context dict to attach to all log messages

    Returns:
        Logger instance

    Note:
        If context is provided, returns a LoggerAdapter with context.
    """
    logger = logging.getLogger(name)

    if context:
        return logging.LoggerAdapter(logger, context)

    return logger


def create_timestamped_log_path(
    base_dir: str = "logs", prefix: str = "app", extension: str = "log"
) -> str:
    """Create a timestamped log file path.

    Args:
        base_dir: Base directory for logs (default: "logs")
        prefix: Log file prefix (default: "app")
        extension: File extension (default: "log")

    Returns:
        Log file path string (e.g., "logs/app_20251031_150000.log")
    """
    timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    log_dir = Path(base_dir)
    log_dir.mkdir(parents=True, exist_ok=True)
    return str(log_dir / f"{prefix}_{timestamp}.{extension}")


def configure_logger_for_component(
    component_name: str, level: Optional[int] = None, propagate: bool = True
) -> logging.Logger:
    """Configure a logger for a specific component (agent, stage, service).

    Args:
        component_name: Component name (e.g., 'graphrag.extraction', 'ingestion.clean')
        level: Optional specific level for this component
        propagate: Whether to propagate to parent loggers (default: True)

    Returns:
        Configured logger instance

    Example:
        logger = configure_logger_for_component('graphrag.extraction', level=logging.DEBUG)
    """
    logger = logging.getLogger(component_name)

    if level is not None:
        logger.setLevel(level)

    logger.propagate = propagate

    return logger


def setup_session_logger(
    session_id: str,
    log_dir: str = "logs/sessions",
    level: int = logging.INFO,
    console_level: Optional[int] = logging.WARNING,
    verbose: bool = False,
) -> logging.Logger:
    """Setup a session-specific logger with file and optional console handlers.

    Creates a dedicated logger for a session (e.g., chat session) that writes
    to a session-specific log file. Useful for tracking individual user sessions
    or conversation threads.

    Args:
        session_id: Unique session identifier (used in logger name and filename)
        log_dir: Directory for session logs (default: "logs/sessions")
        level: Logging level for file handler (default: INFO)
        console_level: Logging level for console handler (default: WARNING, None to disable)
        verbose: If True, set file level to DEBUG

    Returns:
        Configured logger instance with session-specific handlers

    Example:
        logger = setup_session_logger("user-123", log_dir="chat_logs")
        logger.info("User query received")  # Logs to chat_logs/user-123.log
    """
    if verbose:
        level = logging.DEBUG

    # Create logger with session-specific name
    logger_name = f"session_{session_id}"
    logger = logging.getLogger(logger_name)
    logger.setLevel(level)

    # Avoid duplicate handlers if logger already configured
    if logger.handlers:
        return logger

    # Ensure log directory exists
    log_path = Path(log_dir)
    log_path.mkdir(parents=True, exist_ok=True)

    # File handler - session-specific log file
    log_file = log_path / f"{session_id}.log"
    try:
        file_handler = logging.FileHandler(str(log_file), encoding="utf-8")
        file_handler.setLevel(level)
        file_formatter = logging.Formatter(
            "%(asctime)s | %(levelname)-8s | %(name)s | %(message)s",
            datefmt="%Y-%m-%d %H:%M:%S",
        )
        file_handler.setFormatter(file_formatter)
        logger.addHandler(file_handler)
    except Exception as e:
        # Fallback to root logger if file creation fails
        root_logger = logging.getLogger()
        root_logger.warning(f"Failed to create session log file {log_file}: {e}")

    # Optional console handler with minimal format
    if console_level is not None:
        console_handler = logging.StreamHandler(sys.stdout)
        console_handler.setLevel(console_level)
        console_formatter = logging.Formatter("%(levelname)s: %(message)s")
        console_handler.setFormatter(console_formatter)
        logger.addHandler(console_handler)

    return logger
