import { env } from "./env.js";

export const config = {
  server: {
    port: env.PORT,
    nodeEnv: env.NODE_ENV,
    isDevelopment: env.NODE_ENV === "development",
    isProduction: env.NODE_ENV === "production",
    isTest: env.NODE_ENV === "test",
  },
  database: {
    host: env.DATABASE_HOST,
    port: env.DATABASE_PORT,
    database: env.DATABASE_NAME,
    user: env.DATABASE_USER,
    password: env.DATABASE_PASSWORD,
    max: env.DATABASE_POOL_MAX,
    ssl: env.DATABASE_SSL,
  },
  jwt: {
    secret: env.JWT_SECRET,
    accessTokenExpiry: env.JWT_ACCESS_TOKEN_EXPIRY,
    refreshTokenExpiry: env.JWT_REFRESH_TOKEN_EXPIRY,
  },
  security: {
    rateLimitWindowMs: env.RATE_LIMIT_WINDOW_MS,
    rateLimitMaxRequests: env.RATE_LIMIT_MAX_REQUESTS,
    accountLockoutAttempts: env.ACCOUNT_LOCKOUT_ATTEMPTS,
    accountLockoutDurationMs: env.ACCOUNT_LOCKOUT_DURATION_MS,
    passwordMinLength: env.PASSWORD_MIN_LENGTH,
  },
  services: {
    storageUrl: env.STORAGE_SERVICE_URL,
    cacheUrl: env.CACHE_SERVICE_URL,
  },
  metrics: {
    enabled: env.METRICS_ENABLED,
    prefix: env.METRICS_PREFIX,
  },
  logging: {
    level: env.LOG_LEVEL,
    pretty: env.LOG_PRETTY,
  },
} as const;

export type Config = typeof config;
export { env };

