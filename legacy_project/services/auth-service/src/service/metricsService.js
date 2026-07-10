import promClient from "prom-client";

class MetricsService {
  constructor() {
    // Create metrics registry
    this.register = new promClient.Registry();

    // Add default metrics
    promClient.collectDefaultMetrics({ register: this.register });

    // Custom auth metrics
    this.authAttempts = new promClient.Counter({
      name: "auth_attempts_total",
      help: "Total number of authentication attempts",
      labelNames: ["status", "method"],
      registers: [this.register],
    });

    this.authDuration = new promClient.Histogram({
      name: "auth_duration_seconds",
      help: "Authentication request duration",
      labelNames: ["method", "status"],
      buckets: [0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5],
      registers: [this.register],
    });

    this.serviceIntegrationDuration = new promClient.Histogram({
      name: "auth_service_integration_duration_seconds",
      help: "Duration of service integration calls",
      labelNames: ["service", "operation", "status"],
      buckets: [0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5],
      registers: [this.register],
    });

    this.activeTokens = new promClient.Gauge({
      name: "auth_active_tokens_total",
      help: "Number of active JWT tokens",
      registers: [this.register],
    });

    this.circuitBreakerState = new promClient.Gauge({
      name: "auth_circuit_breaker_state",
      help: "Circuit breaker state (0=closed, 1=open, 2=half-open)",
      labelNames: ["service", "operation"],
      registers: [this.register],
    });
  }

  recordAuthAttempt(status, method = "password") {
    this.authAttempts.inc({ status, method });
  }

  recordAuthDuration(duration, method = "password", status = "success") {
    this.authDuration.observe({ method, status }, duration / 1000);
  }

  recordServiceIntegration(service, operation, duration, status = "success") {
    this.serviceIntegrationDuration.observe(
      { service, operation, status },
      duration / 1000
    );
  }

  updateCircuitBreakerState(service, operation, state) {
    // 0=closed, 1=open, 2=half-open
    const stateValue = state === "closed" ? 0 : state === "open" ? 1 : 2;
    this.circuitBreakerState.set({ service, operation }, stateValue);
  }

  getMetrics() {
    return this.register.metrics();
  }
}

export default new MetricsService();
