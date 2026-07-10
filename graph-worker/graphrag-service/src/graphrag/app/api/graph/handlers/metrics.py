"""
Prometheus Metrics Handler - Business Logic

Pure functions for Prometheus metrics export.
No HTTP handling - that's in router.py

Achievement 1.1: Prometheus Metrics Export
"""

import logging
import os
import sys

# Add project root to Python path for imports
_project_root = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "..", ".."))
if _project_root not in sys.path:
    sys.path.insert(0, _project_root)

logger = logging.getLogger(__name__)


def get_prometheus_metrics() -> str:
    """
    Get Prometheus-formatted metrics text.

    Returns:
        Prometheus text format metrics
    """
    try:
        from src.domain.services.observability.prometheus_metrics import get_metrics_text
        return get_metrics_text()
    except ImportError as e:
        logger.warning(f"Prometheus metrics not available: {e}")
        return "# Prometheus metrics not available\n"
    except Exception as e:
        logger.error(f"Error getting Prometheus metrics: {e}")
        return f"# Error: {str(e)}\n"

