import express from "express";
import { AuthController } from "../controllers/AuthController.js";
import { asyncHandler } from "../middleware/asyncHandler.js";
import { validate } from "../middleware/validation.middleware.js";
import {
  authRateLimit,
  tokenValidationRateLimit,
} from "../middleware/rateLimit.middleware.js";
import {
  loginSchema,
  refreshTokenSchema,
  validateTokenSchema,
} from "../../schemas/auth.schema.js";

const router = express.Router();

router.post(
  "/login",
  authRateLimit,
  validate(loginSchema),
  asyncHandler(async (req, res) => {
    await AuthController.login(req, res);
  })
);

router.post(
  "/token/validate",
  tokenValidationRateLimit,
  validate(validateTokenSchema),
  asyncHandler(async (req, res) => {
    await AuthController.validateToken(req, res);
  })
);

router.post(
  "/token/refresh",
  validate(refreshTokenSchema),
  asyncHandler(async (req, res) => {
    await AuthController.refreshToken(req, res);
  })
);

router.post(
  "/logout",
  asyncHandler(async (req, res) => {
    await AuthController.logout(req, res);
  })
);

export default router;

