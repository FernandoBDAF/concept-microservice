import rateLimit from "express-rate-limit";
import config from "../config/config.js";

// Global rate limiting for all requests
export const globalRateLimit = rateLimit({
  windowMs: config.security.rateLimitWindowMs,
  max: config.security.rateLimitMaxRequests * 4, // More lenient for general requests
  message: {
    status: "error",
    message: "Too many requests from this IP, please try again later.",
    data: null,
  },
  standardHeaders: true,
  legacyHeaders: false,
  handler: (req, res) => {
    res.status(429).json({
      status: "error",
      message: "Too many requests from this IP, please try again later.",
      data: null,
    });
  },
});

// Strict rate limiting for authentication endpoints
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
  skipSuccessfulRequests: true, // Don't count successful requests against the limit
  handler: (req, res) => {
    res.status(429).json({
      status: "error",
      message:
        "Too many authentication attempts from this IP, please try again later.",
      data: {
        retryAfter:
          Math.ceil(config.security.rateLimitWindowMs / 1000 / 60) + " minutes",
      },
    });
  },
});

// Very strict rate limiting for failed login attempts
export const failedLoginRateLimit = rateLimit({
  windowMs: config.security.rateLimitWindowMs,
  max: Math.floor(config.security.rateLimitMaxRequests / 2), // Half the normal limit
  message: {
    status: "error",
    message:
      "Too many failed login attempts from this IP, please try again later.",
    data: null,
  },
  standardHeaders: true,
  legacyHeaders: false,
  skip: (req, res) => {
    // Only apply this limit to failed requests
    return res.statusCode < 400;
  },
  handler: (req, res) => {
    res.status(429).json({
      status: "error",
      message:
        "Too many failed login attempts from this IP, please try again later.",
      data: {
        retryAfter:
          Math.ceil(config.security.rateLimitWindowMs / 1000 / 60) + " minutes",
        hint: "If you continue to have issues, please contact support.",
      },
    });
  },
});

// Rate limit for token validation (more lenient since it's used frequently)
export const tokenValidationRateLimit = rateLimit({
  windowMs: 60 * 1000, // 1 minute
  max: 100, // 100 requests per minute
  message: {
    status: "error",
    message: "Too many token validation requests from this IP.",
    data: null,
  },
  standardHeaders: true,
  legacyHeaders: false,
  handler: (req, res) => {
    res.status(429).json({
      status: "error",
      message: "Too many token validation requests from this IP.",
      data: {
        retryAfter: "1 minute",
      },
    });
  },
});

// Rate limit for user registration
export const registrationRateLimit = rateLimit({
  windowMs: 60 * 60 * 1000, // 1 hour
  max: 3, // 3 registrations per hour per IP
  message: {
    status: "error",
    message:
      "Too many registration attempts from this IP, please try again later.",
    data: null,
  },
  standardHeaders: true,
  legacyHeaders: false,
  handler: (req, res) => {
    res.status(429).json({
      status: "error",
      message:
        "Too many registration attempts from this IP, please try again later.",
      data: {
        retryAfter: "1 hour",
        hint: "If you need to register multiple accounts, please contact support.",
      },
    });
  },
});

export default {
  globalRateLimit,
  authRateLimit,
  failedLoginRateLimit,
  tokenValidationRateLimit,
  registrationRateLimit,
};
