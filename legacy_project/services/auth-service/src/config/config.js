class Config {
  constructor() {
    this.server = {
      port: process.env.PORT || 8080,
      nodeEnv: process.env.NODE_ENV || "development",
    };

    // ✅ Service integration configuration
    this.services = {
      storageServiceUrl:
        process.env.STORAGE_SERVICE_URL || "http://storage-service:8080",
      cacheServiceUrl:
        process.env.CACHE_SERVICE_URL || "http://cache-service:8080",
      timeout: parseInt(process.env.SERVICE_TIMEOUT) || 5000,
      retries: parseInt(process.env.SERVICE_RETRIES) || 3,
    };

    this.jwt = {
      accessTokenExpiry: process.env.ACCESS_TOKEN_EXPIRY || "15m",
      refreshTokenExpiry: process.env.REFRESH_TOKEN_EXPIRY || "7d",
      privateKeySecret:
        process.env.JWT_PRIVATE_KEY_SECRET || "jwt-signing-key-dev",
      publicKeySecret:
        process.env.JWT_PUBLIC_KEY_SECRET || "jwt-verification-key-dev",
    };

    this.security = {
      rateLimitWindowMs:
        parseInt(process.env.RATE_LIMIT_WINDOW_MS) || 15 * 60 * 1000, // 15 minutes
      rateLimitMaxRequests: parseInt(process.env.RATE_LIMIT_MAX_REQUESTS) || 5,
      accountLockoutAttempts:
        parseInt(process.env.ACCOUNT_LOCKOUT_ATTEMPTS) || 5,
      accountLockoutDurationMs:
        parseInt(process.env.ACCOUNT_LOCKOUT_DURATION_MS) || 30 * 60 * 1000, // 30 minutes
      passwordMinLength: parseInt(process.env.PASSWORD_MIN_LENGTH) || 8,
    };

    this.metrics = {
      enabled: process.env.METRICS_ENABLED !== "false",
      prefix: process.env.METRICS_PREFIX || "auth_service_",
    };

    // Circuit breaker configuration
    this.circuitBreaker = {
      timeout: parseInt(process.env.CIRCUIT_BREAKER_TIMEOUT) || 3000,
      errorThresholdPercentage:
        parseInt(process.env.CIRCUIT_BREAKER_ERROR_THRESHOLD) || 50,
      resetTimeout:
        parseInt(process.env.CIRCUIT_BREAKER_RESET_TIMEOUT) || 30000,
    };
  }

  get(path) {
    return path.split(".").reduce((obj, key) => obj?.[key], this);
  }

  isDevelopment() {
    return this.server.nodeEnv === "development";
  }

  isProduction() {
    return this.server.nodeEnv === "production";
  }
}

export default new Config();
