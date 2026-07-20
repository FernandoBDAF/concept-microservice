import { env } from "./env.js";

/**
 * Decode a JWT key env value into PEM text (ADR-009.1).
 * Values arrive base64-encoded (single line) from Secrets; a value that already
 * starts with "-----" is treated as raw PEM (k8s may inject raw). Returns null
 * when unset so the loader can fall back to HS256.
 */
function decodePem(value: string | undefined): string | null {
  if (!value) return null;
  const trimmed = value.trim();
  if (trimmed.startsWith("-----")) return trimmed;
  return Buffer.from(trimmed, "base64").toString("utf8");
}

const jwtPrivateKey = decodePem(env.JWT_PRIVATE_KEY);
const jwtPublicKey = decodePem(env.JWT_PUBLIC_KEY);
const jwtKeysPresent = jwtPrivateKey !== null && jwtPublicKey !== null;

// Default RS256 when both keys are present, else HS256. An explicit
// JWT_ALGORITHM=HS256 always wins; JWT_ALGORITHM=RS256 without keys cannot be
// honoured (keyless), so we warn and keep the HS256 fallback working.
function resolveJwtAlgorithm(): "RS256" | "HS256" {
  if (env.JWT_ALGORITHM === "HS256") return "HS256";
  if (jwtKeysPresent) return "RS256";
  if (env.JWT_ALGORITHM === "RS256") {
    console.warn(
      "JWT_ALGORITHM=RS256 but JWT_PRIVATE_KEY/JWT_PUBLIC_KEY are missing; falling back to HS256"
    );
  }
  return "HS256";
}

const jwtAlgorithm = resolveJwtAlgorithm();

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
    // RS256 signs when a keypair is configured; HS256 stays the keyless
    // fallback. Verification accepts both algs during migration.
    algorithm: jwtAlgorithm,
    privateKey: jwtPrivateKey,
    // PEM(s) published via JWKS and accepted for RS256 verification (newest
    // first; single key today, array leaves room for rotation).
    publicKeys: jwtPublicKey ? [jwtPublicKey] : [],
  },
  security: {
    rateLimitWindowMs: env.RATE_LIMIT_WINDOW_MS,
    rateLimitMaxRequests: env.RATE_LIMIT_MAX_REQUESTS,
    tokenValidationRateLimitMax: env.TOKEN_VALIDATION_RATE_LIMIT_MAX,
    accountLockoutAttempts: env.ACCOUNT_LOCKOUT_ATTEMPTS,
    accountLockoutDurationMs: env.ACCOUNT_LOCKOUT_DURATION_MS,
    passwordMinLength: env.PASSWORD_MIN_LENGTH,
  },
  services: {
    apiServiceUrl: env.API_SERVICE_URL,
  },
  seedAdmin: {
    email: env.SEED_ADMIN_EMAIL,
    password: env.SEED_ADMIN_PASSWORD,
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

