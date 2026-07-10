"""
Historical metrics aggregation for pipeline performance analysis.

NOTE: Uses file-based storage to keep historical metrics separate from
the MongoDB observability database (pipeline_executions). This is intentional:
- repository.py → MongoDB → Real-time execution state
- history.py → JSON file → Historical analysis & trends
"""
import json
import os
import logging
from datetime import datetime, timedelta
from typing import Dict, List, Optional, Any
from pathlib import Path
import statistics

logger = logging.getLogger(__name__)

# Storage path for historical data
HISTORY_DIR = Path(os.getenv("GRAPHRAG_WORKSPACE", ".")) / "work-space" / "metrics_history"


class HistoricalMetrics:
    """Manages historical pipeline metrics for context comparison."""
    
    def __init__(self):
        HISTORY_DIR.mkdir(parents=True, exist_ok=True)
        self._history_file = HISTORY_DIR / "pipeline_history.json"
        self._history: List[Dict] = self._load_history()
        logger.info(f"HistoricalMetrics initialized with {len(self._history)} records from {self._history_file}")
    
    def _load_history(self) -> List[Dict]:
        """Load historical data from disk."""
        if self._history_file.exists():
            try:
                with open(self._history_file, 'r') as f:
                    data = json.load(f)
                    logger.debug(f"Loaded {len(data)} historical records")
                    return data
            except (json.JSONDecodeError, IOError) as e:
                logger.warning(f"Failed to load history file: {e}")
        return []
    
    def _save_history(self):
        """Persist history to disk."""
        try:
            # Keep last 1000 executions to avoid unbounded growth
            data_to_save = self._history[-1000:]
            with open(self._history_file, 'w') as f:
                json.dump(data_to_save, f, indent=2)
            logger.debug(f"Saved {len(data_to_save)} historical records")
        except IOError as e:
            logger.warning(f"Could not save history: {e}")
    
    def record_execution(
        self,
        pipeline_id: str,
        pipeline: str,
        status: str,
        duration_seconds: float,
        cost_usd: float,
        tokens_used: int,
        documents_processed: int,
        stages: List[str],
        stage_durations: Dict[str, float],
        error: Optional[str] = None
    ):
        """
        Record a pipeline execution for historical analysis.
        
        Args:
            pipeline_id: Unique identifier for this execution
            pipeline: Pipeline type (e.g., 'ingestion', 'graphrag')
            status: Final status ('completed', 'failed', 'cancelled')
            duration_seconds: Total execution time
            cost_usd: LLM API cost (if available)
            tokens_used: Total tokens consumed
            documents_processed: Number of documents processed
            stages: List of stages executed
            stage_durations: Duration per stage
            error: Error message if failed
        """
        entry = {
            "pipeline_id": pipeline_id,
            "pipeline": pipeline,
            "status": status,
            "timestamp": datetime.utcnow().isoformat() + "Z",
            "duration_seconds": duration_seconds,
            "cost_usd": cost_usd,
            "tokens_used": tokens_used,
            "documents_processed": documents_processed,
            "stages": stages,
            "stage_durations": stage_durations,
            "error": error,
        }
        self._history.append(entry)
        self._save_history()
        logger.info(f"Recorded execution {pipeline_id}: {pipeline}/{status} in {duration_seconds:.1f}s")
    
    def get_historical_context(
        self, 
        pipeline: str, 
        limit: int = 10
    ) -> Dict[str, Any]:
        """
        Get historical context for comparison.
        
        Args:
            pipeline: Pipeline type to filter by
            limit: Maximum number of recent executions to consider
            
        Returns:
            Dictionary with sample_size and stats for duration, cost, tokens
        """
        # Filter by pipeline type and completed status
        relevant = [
            h for h in self._history 
            if h["pipeline"] == pipeline and h["status"] == "completed"
        ][-limit:]
        
        if not relevant:
            return {
                "sample_size": 0,
                "duration": {"avg": 0, "min": 0, "max": 0, "p90": 0},
                "cost": {"avg": 0, "min": 0, "max": 0, "p90": 0},
                "tokens": {"avg": 0, "min": 0, "max": 0, "p90": 0},
            }
        
        durations = [h["duration_seconds"] for h in relevant]
        costs = [h["cost_usd"] for h in relevant]
        tokens = [h["tokens_used"] for h in relevant]
        
        return {
            "sample_size": len(relevant),
            "duration": self._calculate_stats(durations),
            "cost": self._calculate_stats(costs),
            "tokens": self._calculate_stats(tokens),
        }
    
    def _calculate_stats(self, values: List[float]) -> Dict[str, float]:
        """Calculate statistical summary for a list of values."""
        if not values:
            return {"avg": 0, "min": 0, "max": 0, "p90": 0}
        
        sorted_values = sorted(values)
        p90_index = int(len(sorted_values) * 0.9)
        
        return {
            "avg": round(statistics.mean(values), 2),
            "min": round(min(values), 2),
            "max": round(max(values), 2),
            "p90": round(sorted_values[p90_index] if p90_index < len(sorted_values) else sorted_values[-1], 2),
        }
    
    def get_trend(
        self, 
        pipeline: str, 
        metric: str = "duration",
        days: int = 7
    ) -> Dict[str, Any]:
        """
        Calculate trend for a metric over specified days.
        
        Args:
            pipeline: Pipeline type to analyze
            metric: Metric to trend ('duration', 'cost', 'tokens')
            days: Number of days to look back
            
        Returns:
            Dictionary with direction (improving/degrading/stable) and percent_change
        """
        cutoff = datetime.utcnow() - timedelta(days=days)
        
        recent = []
        older = []
        
        for h in self._history:
            if h["pipeline"] != pipeline or h["status"] != "completed":
                continue
            
            try:
                ts = datetime.fromisoformat(h["timestamp"].replace("Z", "+00:00"))
            except (ValueError, KeyError):
                continue
            
            # Get the right field based on metric
            if metric == "duration":
                value = h.get("duration_seconds", 0)
            elif metric == "cost":
                value = h.get("cost_usd", 0)
            elif metric == "tokens":
                value = h.get("tokens_used", 0)
            else:
                value = h.get(metric, 0)
            
            # Make timezone-naive for comparison
            ts_naive = ts.replace(tzinfo=None)
            if ts_naive >= cutoff:
                recent.append(value)
            else:
                older.append(value)
        
        if not recent or not older:
            return {"direction": "stable", "percent_change": 0, "recent_avg": 0, "older_avg": 0}
        
        recent_avg = statistics.mean(recent)
        older_avg = statistics.mean(older)
        
        if older_avg == 0:
            return {"direction": "stable", "percent_change": 0, "recent_avg": round(recent_avg, 2), "older_avg": 0}
        
        percent_change = ((recent_avg - older_avg) / older_avg) * 100
        
        # For duration and cost, lower is better
        if metric in ("duration", "cost"):
            if percent_change < -5:
                direction = "improving"
            elif percent_change > 5:
                direction = "degrading"
            else:
                direction = "stable"
        else:
            direction = "stable"
        
        return {
            "direction": direction,
            "percent_change": round(percent_change, 1),
            "recent_avg": round(recent_avg, 2),
            "older_avg": round(older_avg, 2),
        }
    
    def get_recent_executions(self, pipeline: Optional[str] = None, limit: int = 10) -> List[Dict]:
        """
        Get recent executions, optionally filtered by pipeline type.
        
        Args:
            pipeline: Optional pipeline type filter
            limit: Maximum number to return
            
        Returns:
            List of recent execution records
        """
        if pipeline:
            filtered = [h for h in self._history if h["pipeline"] == pipeline]
        else:
            filtered = self._history
        
        return filtered[-limit:][::-1]  # Most recent first


# Global instance
_historical_metrics: Optional[HistoricalMetrics] = None


def get_historical_metrics() -> HistoricalMetrics:
    """Get or create the global historical metrics instance."""
    global _historical_metrics
    if _historical_metrics is None:
        _historical_metrics = HistoricalMetrics()
    return _historical_metrics

