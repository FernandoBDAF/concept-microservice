"""
Logging Setup and Configuration.

This module provides centralized logging configuration for the application.
Part of the DEPENDENCIES layer - abstracts logging infrastructure.
"""

import logging
import sys
from pathlib import Path
from typing import Optional
from datetime import datetime


def setup_logging(
    level: int = logging.INFO,
    log_file: Optional[str] = None,
    verbose: bool = False,
    silence_third_party: bool = True,
) -> logging.Logger:
    """Setup application logging with console and optional file handlers.

    Args:
        level: Base logging level (default: INFO)
        log_file: Optional log file path
        verbose: If True, set level to DEBUG
        silence_third_party: If True, silence noisy third-party loggers

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

    # Console handler
    console_handler = logging.StreamHandler(sys.stdout)
    console_handler.setLevel(level)
    console_format = logging.Formatter(
        "%(asctime)s | %(levelname)-8s | %(name)s | %(message)s",
        datefmt="%Y-%m-%d %H:%M:%S",
    )
    console_handler.setFormatter(console_format)
    root_logger.addHandler(console_handler)

    # File handler (optional)
    if log_file:
        try:
            log_path = Path(log_file)
            log_path.parent.mkdir(parents=True, exist_ok=True)

            file_handler = logging.FileHandler(log_path, encoding="utf-8")
            file_handler.setLevel(logging.DEBUG)  # Always DEBUG in file
            file_format = logging.Formatter(
                "%(asctime)s | %(levelname)-8s | %(name)s | %(funcName)s:%(lineno)d | %(message)s",
                datefmt="%Y-%m-%d %H:%M:%S",
            )
            file_handler.setFormatter(file_format)
            root_logger.addHandler(file_handler)

            root_logger.info(f"Logging to file: {log_path.absolute()}")
        except Exception as e:
            root_logger.warning(f"Failed to create log file {log_file}: {e}")

    # Silence noisy third-party loggers
    if silence_third_party:
        logging.getLogger("numba").setLevel(logging.WARNING)
        logging.getLogger("graspologic").setLevel(logging.WARNING)
        logging.getLogger("pymongo").setLevel(logging.WARNING)
        logging.getLogger("httpx").setLevel(logging.INFO)  # Keep API calls visible
        logging.getLogger("urllib3").setLevel(logging.WARNING)
        logging.getLogger("openai").setLevel(logging.WARNING)

    return root_logger


def get_logger(name: str) -> logging.Logger:
    """Get a logger for a specific module.

    Args:
        name: Logger name (typically __name__)

    Returns:
        Logger instance
    """
    return logging.getLogger(name)


def create_timestamped_log_path(base_dir: str = "logs", prefix: str = "app") -> str:
    """Create a timestamped log file path.

    Args:
        base_dir: Base directory for logs
        prefix: Log file prefix

    Returns:
        Log file path string
    """
    timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    log_dir = Path(base_dir)
    log_dir.mkdir(parents=True, exist_ok=True)
    return str(log_dir / f"{prefix}_{timestamp}.log")
