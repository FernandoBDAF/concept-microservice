import pino from "pino";
import { config } from "../../config/index.js";

const transport = config.logging.pretty
  ? {
      target: "pino-pretty",
      options: {
        colorize: true,
        translateTime: "SYS:standard",
        ignore: "pid,hostname",
      },
    }
  : undefined;

export const logger = pino({
  level: config.logging.level,
  transport,
  base: {
    service: "auth-service",
    version: process.env.npm_package_version ?? "2.0.0",
    env: config.server.nodeEnv,
  },
  timestamp: pino.stdTimeFunctions.isoTime,
  formatters: {
    level: (label) => ({ level: label }),
  },
});

export const createChildLogger = (bindings: Record<string, unknown>) => {
  return logger.child(bindings);
};

export type Logger = typeof logger;

