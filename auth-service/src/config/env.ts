import { z } from "zod";

// `z.coerce.boolean()` runs the JS `Boolean()` constructor under the hood, so any
// non-empty string -- including the literal "false" -- coerces to `true`. Env vars are
// always strings, so that footgun would make `SOME_FLAG=false` silently mean "true".
// Parse the textual value explicitly instead.
const booleanEnv = (defaultValue: boolean) =>
  z
    .string()
    .optional()
    .transform((val) => {
      if (val === undefined) return defaultValue;
      return val.toLowerCase() === "true" || val === "1";
    });

const envSchema = z.object({
  NODE_ENV: z.enum(["development", "production", "test"]).default("development"),
  PORT: z.coerce.number().default(3000),
  DATABASE_HOST: z.string().default("postgres"),
  DATABASE_PORT: z.coerce.number().default(5432),
  DATABASE_NAME: z.string().default("auth_db"),
  DATABASE_USER: z.string().default("auth_user"),
  DATABASE_PASSWORD: z.string().min(1),
  DATABASE_POOL_MAX: z.coerce.number().default(20),
  DATABASE_SSL: booleanEnv(false),
  JWT_SECRET: z.string().min(32),
  JWT_ACCESS_TOKEN_EXPIRY: z.string().default("15m"),
  JWT_REFRESH_TOKEN_EXPIRY: z.string().default("7d"),
  RATE_LIMIT_WINDOW_MS: z.coerce.number().default(900000),
  RATE_LIMIT_MAX_REQUESTS: z.coerce.number().default(100),
  // /v1/auth/token/validate is called by api-service once per authenticated
  // request, so its budget is service-to-service traffic, not user logins.
  TOKEN_VALIDATION_RATE_LIMIT_MAX: z.coerce.number().default(1000),
  ACCOUNT_LOCKOUT_ATTEMPTS: z.coerce.number().default(5),
  ACCOUNT_LOCKOUT_DURATION_MS: z.coerce.number().default(1800000),
  PASSWORD_MIN_LENGTH: z.coerce.number().default(8),
  API_SERVICE_URL: z.string()
    .url()
    .optional()
    .default("http://api-service:8080"),
  METRICS_ENABLED: booleanEnv(true),
  METRICS_PREFIX: z.string().default("auth_service_"),
  LOG_LEVEL: z
    .enum(["fatal", "error", "warn", "info", "debug", "trace", "silent"])
    .default("info"),
  LOG_PRETTY: booleanEnv(false),
});

export type Env = z.infer<typeof envSchema>;

function validateEnv(): Env {
  const isTestEnv =
    process.env.NODE_ENV === "test" || process.env.VITEST !== undefined;
  const rawEnv = { ...process.env };

  if (isTestEnv) {
    rawEnv.JWT_SECRET ??= "test-secret-key-at-least-32-characters-long";
    rawEnv.DATABASE_PASSWORD ??= "test-password";
  }

  const result = envSchema.safeParse(rawEnv);

  if (!result.success) {
    console.error("Invalid environment variables:");
    console.error(result.error.format());
    process.exit(1);
  }

  return result.data;
}

export const env = validateEnv();

