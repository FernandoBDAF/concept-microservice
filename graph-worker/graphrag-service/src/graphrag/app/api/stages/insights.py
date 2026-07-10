"""
Insights generation engine for pipeline optimization suggestions.

This module analyzes pipeline execution data against historical context
to generate actionable insights and optimization recommendations.

Reference: OBSERVABILITY_IMPLEMENTATION_ROADMAP_PART2.md Section 3.1
"""
from typing import Dict, List, Any, Optional
from dataclasses import dataclass, asdict
from enum import Enum
import logging

logger = logging.getLogger(__name__)


class InsightSeverity(Enum):
    """Severity levels for insights, sorted by urgency."""
    INFO = "info"
    WARNING = "warning"
    CRITICAL = "critical"


class InsightType(Enum):
    """Categories of insights for UI grouping."""
    PERFORMANCE = "performance"
    COST = "cost"
    QUALITY = "quality"
    ERROR = "error"


@dataclass
class Insight:
    """
    An actionable insight generated from pipeline analysis.
    
    Attributes:
        type: Category of the insight (performance, cost, quality, error)
        severity: How urgent the insight is (info, warning, critical)
        title: Short summary of the issue
        message: Detailed description with specific values
        suggestion: Recommended action to address the issue
        config_change: Optional configuration change to apply as fix
        impact: Expected improvement from applying the suggestion
    """
    type: InsightType
    severity: InsightSeverity
    title: str
    message: str
    suggestion: Optional[str] = None
    config_change: Optional[Dict[str, Any]] = None
    impact: Optional[str] = None
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary for JSON serialization."""
        return {
            "type": self.type.value,
            "severity": self.severity.value,
            "title": self.title,
            "message": self.message,
            "suggestion": self.suggestion,
            "config_change": self.config_change,
            "impact": self.impact,
        }


class InsightsEngine:
    """
    Generates actionable insights from pipeline execution data.
    
    Analyzes current metrics against historical context using rule-based checks.
    Each rule can return an Insight if a condition is met, providing specific
    suggestions for optimization.
    
    Rules implemented:
    - _check_slow_stage: Detects stages 50%+ slower than average
    - _check_high_cost: Flags costs 30%+ above average
    - _check_high_retry_rate: Warns if retry rate > 10%
    - _check_low_throughput: Suggests parallelization if < 0.5 docs/sec
    - _check_token_efficiency: Flags high token usage (>5K per doc)
    - _check_error_patterns: Detects recurring errors (3+ occurrences)
    """
    
    def __init__(self):
        """Initialize the insights engine with rule functions."""
        self._rules = [
            self._check_slow_stage,
            self._check_high_cost,
            self._check_high_retry_rate,
            self._check_low_throughput,
            self._check_token_efficiency,
            self._check_error_patterns,
        ]
    
    def generate_insights(
        self,
        current_metrics: Dict[str, Any],
        historical_context: Dict[str, Any],
        stage_durations: Dict[str, float],
        config: Dict[str, Any],
    ) -> List[Insight]:
        """
        Generate insights based on current execution vs historical data.
        
        Args:
            current_metrics: Current pipeline metrics (cost, tokens, docs, etc.)
            historical_context: Historical statistics from HistoricalMetrics
            stage_durations: Duration in seconds for each completed stage
            config: Current pipeline configuration
            
        Returns:
            List of Insight objects, sorted by severity (critical first)
        """
        insights = []
        
        context = {
            "metrics": current_metrics,
            "historical": historical_context,
            "stage_durations": stage_durations,
            "config": config,
        }
        
        for rule in self._rules:
            try:
                insight = rule(context)
                if insight:
                    insights.append(insight)
            except Exception as e:
                # Don't fail insight generation - log and continue
                logger.debug(f"Insight rule {rule.__name__} failed: {e}")
        
        # Sort by severity (critical first)
        severity_order = {
            InsightSeverity.CRITICAL: 0,
            InsightSeverity.WARNING: 1,
            InsightSeverity.INFO: 2,
        }
        return sorted(insights, key=lambda i: severity_order.get(i.severity, 3))
    
    def _check_slow_stage(self, ctx: Dict) -> Optional[Insight]:
        """
        Check if any stage is significantly slower than average.
        
        Triggers when a stage takes 50%+ longer than the historical average.
        Suggests increasing batch_size or enabling parallel processing.
        """
        stage_durations = ctx.get("stage_durations", {})
        historical = ctx.get("historical", {})
        
        if not stage_durations or not historical:
            return None
        
        avg_duration = historical.get("duration", {}).get("avg", 0)
        if avg_duration <= 0:
            return None
        
        for stage, duration in stage_durations.items():
            if duration > avg_duration * 1.5:
                slowdown = ((duration - avg_duration) / avg_duration) * 100
                
                return Insight(
                    type=InsightType.PERFORMANCE,
                    severity=InsightSeverity.WARNING,
                    title=f"Stage '{stage}' is {slowdown:.0f}% slower than average",
                    message=f"Current: {duration:.1f}s, Average: {avg_duration:.1f}s",
                    suggestion="Consider increasing batch_size or enabling parallel processing",
                    config_change={"batch_size": 10},
                    impact="~30% faster execution",
                )
        
        return None
    
    def _check_high_cost(self, ctx: Dict) -> Optional[Insight]:
        """
        Check if cost is significantly higher than average.
        
        Triggers when cost exceeds historical average by 30%+.
        Suggests using a cheaper model for less critical stages.
        """
        metrics = ctx.get("metrics", {})
        historical = ctx.get("historical", {})
        
        current_cost = metrics.get("cost_usd", 0)
        avg_cost = historical.get("cost", {}).get("avg", 0)
        
        if avg_cost > 0 and current_cost > avg_cost * 1.3:
            overage = ((current_cost - avg_cost) / avg_cost) * 100
            
            return Insight(
                type=InsightType.COST,
                severity=InsightSeverity.WARNING,
                title=f"Cost is {overage:.0f}% above average",
                message=f"Current: ${current_cost:.4f}, Average: ${avg_cost:.4f}",
                suggestion="Consider using a smaller/cheaper model for less critical stages",
                config_change={"model": "gpt-4o-mini"},
                impact="~70% cost reduction",
            )
        
        return None
    
    def _check_high_retry_rate(self, ctx: Dict) -> Optional[Insight]:
        """
        Check if retry rate is high.
        
        Triggers when more than 10% of operations require retries.
        Suggests checking API rate limits or increasing retry delays.
        """
        metrics = ctx.get("metrics", {})
        
        retries = metrics.get("retries", 0)
        operations = metrics.get("operations", 0)
        
        if operations <= 0:
            return None
        
        retry_rate = (retries / operations) * 100
        
        if retry_rate > 10:
            return Insight(
                type=InsightType.QUALITY,
                severity=InsightSeverity.WARNING,
                title=f"High retry rate: {retry_rate:.0f}%",
                message=f"{retries} retries out of {operations} operations",
                suggestion="Check API rate limits or increase retry delays",
                config_change={"retry_delay_seconds": 2},
                impact="More stable execution",
            )
        
        return None
    
    def _check_low_throughput(self, ctx: Dict) -> Optional[Insight]:
        """
        Check if document throughput is low.
        
        Triggers when processing less than 0.5 documents per second.
        Suggests enabling parallel processing for faster throughput.
        """
        metrics = ctx.get("metrics", {})
        
        docs = metrics.get("documents_processed", 0)
        duration = metrics.get("duration_seconds", 0)
        
        if docs <= 0 or duration <= 0:
            return None
        
        throughput = docs / duration
        
        if throughput < 0.5:  # Less than 0.5 docs/sec
            return Insight(
                type=InsightType.PERFORMANCE,
                severity=InsightSeverity.INFO,
                title="Low document throughput",
                message=f"Processing {throughput:.2f} documents/second",
                suggestion="Enable parallel processing for faster throughput",
                config_change={"parallel_processing": True, "max_workers": 4},
                impact="~3x faster processing",
            )
        
        return None
    
    def _check_token_efficiency(self, ctx: Dict) -> Optional[Insight]:
        """
        Check token efficiency.
        
        Triggers when using more than 5K tokens per document on average.
        Suggests enabling summarization or using smaller context windows.
        """
        metrics = ctx.get("metrics", {})
        
        tokens = metrics.get("tokens_used", 0)
        docs = metrics.get("documents_processed", 0)
        
        if docs <= 0 or tokens <= 0:
            return None
        
        tokens_per_doc = tokens / docs
        
        if tokens_per_doc > 5000:  # More than 5K tokens per document
            return Insight(
                type=InsightType.COST,
                severity=InsightSeverity.INFO,
                title="High token usage per document",
                message=f"{tokens_per_doc:.0f} tokens per document",
                suggestion="Consider enabling summarization or using smaller context windows",
                config_change={"max_context_tokens": 4000},
                impact="~30% token reduction",
            )
        
        return None
    
    def _check_error_patterns(self, ctx: Dict) -> Optional[Insight]:
        """
        Check for error patterns.
        
        Triggers when the same error type occurs 3+ times.
        Suggests checking input data quality and API configurations.
        """
        metrics = ctx.get("metrics", {})
        
        errors_by_type = metrics.get("errors_by_type", {})
        
        for error_type, count in errors_by_type.items():
            if count >= 3:
                return Insight(
                    type=InsightType.ERROR,
                    severity=InsightSeverity.CRITICAL,
                    title=f"Recurring error: {error_type}",
                    message=f"This error has occurred {count} times",
                    suggestion="Check input data quality and API configurations",
                    impact="Prevent execution failures",
                )
        
        return None


# Singleton instance
_engine: Optional[InsightsEngine] = None


def get_insights_engine() -> InsightsEngine:
    """Get or create the global insights engine instance."""
    global _engine
    if _engine is None:
        _engine = InsightsEngine()
    return _engine

