import express from "express";
import authenticationService from "../service/authenticationService.js";
import rateLimit from "express-rate-limit";

const router = express.Router();

// Rate limiting for auth endpoints
const authRateLimit = rateLimit({
  windowMs: 1 * 60 * 1000, // 1 minute
  max: 10, // 10 attempts per window
  message: {
    status: "error",
    message: "Too many authentication attempts, please try again later",
  },
  standardHeaders: true,
  legacyHeaders: false,
});

// POST /v1/auth/login - Compatible with auth-service-old
router.post("/login", authRateLimit, async (req, res) => {
  try {
    const { user_id, password } = req.body; // Note: user_id is email for compatibility

    if (!user_id || !password) {
      return res.status(400).json({
        status: "error",
        message: "Email and password are required",
      });
    }

    const result = await authenticationService.authenticateUser(
      user_id,
      password,
      req
    );
    res.json(result);
  } catch (error) {
    res.status(401).json({
      status: "error",
      message: error.message,
    });
  }
});

// POST /v1/auth/token/validate - Compatible with auth-service-old
router.post("/token/validate", async (req, res) => {
  try {
    const token = req.headers.authorization?.split(" ")[1] || req.body.token;

    if (!token) {
      return res.status(400).json({
        status: "error",
        message: "Token is required",
      });
    }

    const validation = await authenticationService.validateToken(token);

    if (validation.valid) {
      res.json({
        status: "success",
        message: "Token is valid",
        data: {
          valid: true,
          user: validation.user,
        },
      });
    } else {
      res.status(401).json({
        status: "error",
        message: "Invalid token",
        data: {
          valid: false,
        },
      });
      console.error(validation.error);
    }
  } catch (error) {
    res.status(401).json({
      status: "error",
      message: "Invalid token",
      data: {
        valid: false,
      },
    });
  }
});

// POST /v1/auth/token/refresh - Token refresh endpoint
router.post("/token/refresh", async (req, res) => {
  try {
    const { refresh_token } = req.body;

    if (!refresh_token) {
      return res.status(400).json({
        status: "error",
        message: "Refresh token is required",
      });
    }

    const result = await authenticationService.refreshToken(refresh_token);
    res.json(result);
  } catch (error) {
    res.status(401).json({
      status: "error",
      message: error.message,
    });
  }
});

// POST /v1/auth/logout - Logout endpoint
router.post("/logout", async (req, res) => {
  try {
    const token = req.headers.authorization?.split(" ")[1];

    if (!token) {
      return res.status(400).json({
        status: "error",
        message: "Token is required",
      });
    }

    const result = await authenticationService.logout(token);
    res.json(result);
  } catch (error) {
    res.status(400).json({
      status: "error",
      message: error.message,
    });
  }
});

export default router;
