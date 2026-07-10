import express from "express";
import { AuthController } from "../controllers/AuthController.js";
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

router.post("/login", authRateLimit, validate(loginSchema), AuthController.login);

router.post(
  "/token/validate",
  tokenValidationRateLimit,
  validate(validateTokenSchema),
  AuthController.validateToken
);

router.post(
  "/token/refresh",
  validate(refreshTokenSchema),
  AuthController.refreshToken
);

router.post("/logout", AuthController.logout);

export default router;

