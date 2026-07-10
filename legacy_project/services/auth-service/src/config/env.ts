import { z } from "zod";

const envSchema = z.object({
  NODE_ENV: z.enum(["development", "production", "test"]).default("development"),
  PORT: z.coerce.number().default(8080),
  DATABASE_HOST: z.string().default("localhost"),
  DATABASE_PORT: z.coerce.number().default(5432),
  DATABASE_NAME: z.string().default("auth_db"),
  DATABASE_USER: z.string().default("auth_user"),
  DATABASE_PASSWORD: z.string().min(1),
  DATABASE_POOL_MAX: z.coerce.number().default(20),
  DATABASE_SSL: z.coerce.boolean().default(false),
  JWT_SECRET: z.string().min(32),
  JWT_ACCESS_TOKEN_EXPIRY: z.string().default("15m"),
  JWT_REFRESH_TOKEN_EXPIRY: z.string().default("7d"),
  RATE_LIMIT_WINDOW_MS: z.coerce.number().default(900000),
  RATE_LIMIT_MAX_REQUESTS: z.coerce.number().default(100),
  ACCOUNT_LOCKOUT_ATTEMPTS: z.coerce.number().default(5),
  ACCOUNT_LOCKOUT_DURATION_MS: z.coerce.number().default(1800000),
  PASSWORD_MIN_LENGTH: z.coerce.number().default(8),
  STORAGE_SERVICE_URL: z.string().url().optional(),
  CACHE_SERVICE_URL: z.string().url().optional(),
  METRICS_ENABLED: z.coerce.boolean().default(true),
  METRICS_PREFIX: z.string().default("auth_service_"),
  LOG_LEVEL: z
    .enum(["fatal", "error", "warn", "info", "debug", "trace"])
    .default("info"),
  LOG_PRETTY: z.coerce.boolean().default(false),
});

export type Env = z.infer<typeof envSchema>;

function validateEnv(): Env {
  const result = envSchema.safeParse(process.env);

  if (!result.success) {
    console.error("Invalid environment variables:");
    console.error(result.error.format());
    process.exit(1);
  }

  return result.data;
}

export const env = validateEnv();

