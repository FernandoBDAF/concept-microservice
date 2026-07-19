import { trace } from "@opentelemetry/api";
import pino from "pino";
import { config } from "../../config/index.js";

const loggerOptions: pino.LoggerOptions = {
  level: config.logging.level,
  // Correlate logs with traces: when a span is active (tracing enabled and
  // inside an instrumented request), stamp its ids onto every log line.
  mixin() {
    const span = trace.getActiveSpan();
    if (!span) {
      return {};
    }
    const { traceId, spanId } = span.spanContext();
    return { trace_id: traceId, span_id: spanId };
  },
  base: {
    service: "auth-service",
    version: process.env.npm_package_version ?? "2.0.0",
    env: config.server.nodeEnv,
  },
  timestamp: pino.stdTimeFunctions.isoTime,
  formatters: {
    level: (label) => ({ level: label }),
  },
};

if (config.logging.pretty) {
  loggerOptions.transport = {
    target: "pino-pretty",
    options: {
      colorize: true,
      translateTime: "SYS:standard",
      ignore: "pid,hostname",
    },
  };
}

export const logger = pino(loggerOptions);

export const createChildLogger = (bindings: Record<string, unknown>) => {
  return logger.child(bindings);
};

export type Logger = typeof logger;

