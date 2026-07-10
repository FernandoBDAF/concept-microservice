import promClient from "prom-client";
import config from "../config/config.js";

// Create a Registry to register the metrics
const register = new promClient.Registry();

// Add default metrics
promClient.collectDefaultMetrics({
  register,
  prefix: config.metrics.prefix,
});

// HTTP request metrics
const httpRequestDuration = new promClient.Histogram({
  name: `${config.metrics.prefix}http_request_duration_seconds`,
  help: "Duration of HTTP requests in seconds",
  labelNames: ["method", "route", "status_code"],
  buckets: [0.1, 0.3, 0.5, 0.7, 1, 3, 5, 7, 10],
});

const httpRequestTotal = new promClient.Counter({
  name: `${config.metrics.prefix}http_requests_total`,
  help: "Total number of HTTP requests",
  labelNames: ["method", "route", "status_code"],
});

// Authentication metrics
const authAttemptTotal = new promClient.Counter({
  name: `${config.metrics.prefix}auth_attempts_total`,
  help: "Total number of authentication attempts",
  labelNames: ["type", "result"], // type: login, token_validation, refresh; result: success, failure
});

const authSessionsActive = new promClient.Gauge({
  name: `${config.metrics.prefix}auth_sessions_active`,
  help: "Number of active authentication sessions",
});

const authAccountsLocked = new promClient.Gauge({
  name: `${config.metrics.prefix}auth_accounts_locked_total`,
  help: "Number of currently locked accounts",
});

const authRateLimitHits = new promClient.Counter({
  name: `${config.metrics.prefix}auth_rate_limit_hits_total`,
  help: "Total number of rate limit hits",
  labelNames: ["endpoint"],
});

// Database metrics
const dbOperationDuration = new promClient.Histogram({
  name: `${config.metrics.prefix}db_operation_duration_seconds`,
  help: "Duration of database operations in seconds",
  labelNames: ["operation", "table"],
  buckets: [0.01, 0.05, 0.1, 0.3, 0.5, 1, 2, 5],
});

const dbConnectionsActive = new promClient.Gauge({
  name: `${config.metrics.prefix}db_connections_active`,
  help: "Number of active database connections",
});

// Register all metrics
register.registerMetric(httpRequestDuration);
register.registerMetric(httpRequestTotal);
register.registerMetric(authAttemptTotal);
register.registerMetric(authSessionsActive);
register.registerMetric(authAccountsLocked);
register.registerMetric(authRateLimitHits);
register.registerMetric(dbOperationDuration);
register.registerMetric(dbConnectionsActive);

// Middleware function to track HTTP metrics
export const metricsMiddleware = (req, res, next) => {
  if (!config.metrics.enabled) {
    return next();
  }

  const startTime = Date.now();

  // Override res.end to capture metrics when response is sent
  const originalEnd = res.end;
  res.end = function (...args) {
    const duration = (Date.now() - startTime) / 1000;
    const route = req.route ? req.route.path : req.path;

    // Record metrics
    httpRequestDuration
      .labels(req.method, route, res.statusCode.toString())
      .observe(duration);

    httpRequestTotal.labels(req.method, route, res.statusCode.toString()).inc();

    // Call original end method
    originalEnd.apply(this, args);
  };

  next();
};

// Helper functions to record specific metrics
export const recordAuthAttempt = (type, result) => {
  if (config.metrics.enabled) {
    authAttemptTotal.labels(type, result).inc();
  }
};

export const recordRateLimitHit = (endpoint) => {
  if (config.metrics.enabled) {
    authRateLimitHits.labels(endpoint).inc();
  }
};

export const setActiveSessionsCount = (count) => {
  if (config.metrics.enabled) {
    authSessionsActive.set(count);
  }
};

export const setLockedAccountsCount = (count) => {
  if (config.metrics.enabled) {
    authAccountsLocked.set(count);
  }
};

export const recordDbOperation = (operation, table, durationMs) => {
  if (config.metrics.enabled) {
    dbOperationDuration.labels(operation, table).observe(durationMs / 1000);
  }
};

export const setActiveConnectionsCount = (count) => {
  if (config.metrics.enabled) {
    dbConnectionsActive.set(count);
  }
};

// Export the registry and individual metrics
export { register };

export default {
  metricsMiddleware,
  recordAuthAttempt,
  recordRateLimitHit,
  setActiveSessionsCount,
  setLockedAccountsCount,
  recordDbOperation,
  setActiveConnectionsCount,
  register,
};
