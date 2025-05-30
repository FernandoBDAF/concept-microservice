# Queue API Implementation Prompt

> NOTE: The current metrics implementation in the queue service does **not** support dynamic label values (e.g., per-queue, per-type, per-error). All metrics are recorded globally. This is a technical debt and should be addressed in the future to enable full Prometheus-style label support. See the service implementation for details.
