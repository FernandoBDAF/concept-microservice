import type { Request, Response } from "express";
import { AuthService } from "../../domain/services/AuthService.js";
import { tokenService } from "../../domain/services/TokenService.js";
import { userRepository } from "../../domain/repositories/UserRepository.js";
import { sessionRepository } from "../../domain/repositories/SessionRepository.js";
import { auditLogRepository } from "../../domain/repositories/AuditLogRepository.js";

const authService = new AuthService(
  userRepository,
  tokenService,
  sessionRepository,
  auditLogRepository
);

// eslint-disable-next-line @typescript-eslint/no-extraneous-class
export class AuthController {
  static async login(req: Request, res: Response): Promise<void> {
    const { email, password } = req.body as { email: string; password: string };
    const metadata: { ip?: string; userAgent?: string } = {};
    if (req.ip) {
      metadata.ip = req.ip;
    }
    const userAgent = req.get("User-Agent");
    if (userAgent) {
      metadata.userAgent = userAgent;
    }

    const result = await authService.login({ email, password }, metadata);

    res.json({
      status: "success",
      message: "Authentication successful",
      data: {
        access_token: result.tokens.accessToken,
        refresh_token: result.tokens.refreshToken,
        token_type: result.tokens.tokenType,
        expires_in: result.tokens.expiresIn,
        user: result.user,
      },
    });
  }

  static async validateToken(req: Request, res: Response): Promise<void> {
    const body = req.body as { token?: string };
    const token =
      req.headers.authorization?.split(" ")[1] ?? body.token ?? "";

    const validation = await authService.validateToken(token);

    if (validation.valid) {
      res.json({
        status: "success",
        message: "Token is valid",
        data: {
          valid: true,
          user: validation.user,
        },
      });
      return;
    }

    res.status(401).json({
      status: "error",
      message: "Invalid token",
      data: {
        valid: false,
      },
    });
  }

  static async refreshToken(req: Request, res: Response): Promise<void> {
    const { refresh_token } = req.body as { refresh_token: string };
    const tokens = await authService.refreshTokens(refresh_token);

    res.json({
      status: "success",
      message: "Token refreshed successfully",
      data: {
        access_token: tokens.accessToken,
        refresh_token: tokens.refreshToken,
        token_type: tokens.tokenType,
        expires_in: tokens.expiresIn,
      },
    });
  }

  static async logout(req: Request, res: Response): Promise<void> {
    const token = req.headers.authorization?.split(" ")[1];
    if (token) {
      await authService.logout(token);
    }

    res.json({
      status: "success",
      message: "Logged out successfully",
      data: null,
    });
  }
}

