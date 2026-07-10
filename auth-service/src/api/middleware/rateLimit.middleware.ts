import rateLimit from "express-rate-limit";
import { config } from "../../config/index.js";

export const globalRateLimit = rateLimit({
  windowMs: config.security.rateLimitWindowMs,
  max: config.security.rateLimitMaxRequests * 4,
  message: {
    status: "error",
    message: "Too many requests from this IP, please try again later.",
    data: null,
  },
  standardHeaders: true,
  legacyHeaders: false,
  handler: (_req, res) => {
    res.status(429).json({
      status: "error",
      message: "Too many requests from this IP, please try again later.",
      data: null,
    });
  },
});

export const authRateLimit = rateLimit({
  windowMs: config.security.rateLimitWindowMs,
  max: config.security.rateLimitMaxRequests,
  message: {
    status: "error",
    message:
      "Too many authentication attempts from this IP, please try again later.",
    data: null,
  },
  standardHeaders: true,
  legacyHeaders: false,
  skipSuccessfulRequests: true,
  handler: (_req, res) => {
    res.status(429).json({
      status: "error",
      message:
        "Too many authentication attempts from this IP, please try again later.",
      data: {
        retryAfter: `${String(
          Math.ceil(config.security.rateLimitWindowMs / 1000 / 60)
        )} minutes`,
      },
    });
  },
});

export const tokenValidationRateLimit = rateLimit({
  windowMs: 60 * 1000,
  max: config.security.tokenValidationRateLimitMax,
  message: {
    status: "error",
    message: "Too many token validation requests from this IP.",
    data: null,
  },
  standardHeaders: true,
  legacyHeaders: false,
  handler: (_req, res) => {
    res.status(429).json({
      status: "error",
      message: "Too many token validation requests from this IP.",
      data: {
        retryAfter: "1 minute",
      },
    });
  },
});

