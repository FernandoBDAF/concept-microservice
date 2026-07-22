import client from "prom-client";
import { config } from "../../config/index.js";

export const register = new client.Registry();

if (config.metrics.enabled) {
  client.collectDefaultMetrics({
    register,
    prefix: config.metrics.prefix,
  });
}

export const httpRequestDuration = new client.Histogram({
  name: `${config.metrics.prefix}http_request_duration_seconds`,
  help: "Duration of HTTP requests in seconds",
  labelNames: ["method", "route", "status_code"],
  buckets: [0.005, 0.01, 0.05, 0.1, 0.3, 0.5, 1, 2, 5],
  registers: [register],
});

export const httpRequestsTotal = new client.Counter({
  name: `${config.metrics.prefix}http_requests_total`,
  help: "Total number of HTTP requests",
  labelNames: ["method", "route", "status_code"],
  registers: [register],
});
